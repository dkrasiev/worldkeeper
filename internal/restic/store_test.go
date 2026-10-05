package restic

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/snapshot"
	"github.com/dkrasiev/worldkeeper/internal/testworld"
)

// newRepo returns a store on a fresh repository. Tests need a real restic
// binary and are skipped without one.
func newRepo(t *testing.T) *Store {
	t.Helper()
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic not installed")
	}
	s := NewStore(&Runner{
		Repo:     filepath.Join(t.TempDir(), "repo"),
		Password: func() (string, error) { return "test-password", nil },
	})
	ctx := context.Background()
	if st := s.Check(ctx); st.State != "missing" {
		t.Fatalf("fresh repo status = %+v", st)
	}
	if err := s.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if st := s.Check(ctx); st.State != "ok" {
		t.Fatalf("after init status = %+v", st)
	}
	return s
}

func TestCreateGetExtract(t *testing.T) {
	s := newRepo(t)
	world := testworld.Create(t, t.TempDir(), "w", testworld.Options{})
	ref := snapshot.WorldRef{ID: "minecraft--w", Name: "Мой мир, с запятой", Folder: "w", SourceLabel: "Minecraft Launcher", Path: world}
	lp := time.UnixMilli(1759680000000).UTC()

	snap, err := s.Create(ref, world, snapshot.Meta{Kind: snapshot.KindManual, Label: "before dragon", GameVersion: "1.21.4", LastPlayed: lp})
	if err != nil {
		t.Fatal(err)
	}
	if snap.ID == "" || snap.SizeBytes == 0 {
		t.Fatalf("snapshot = %+v", snap)
	}

	ix, err := s.Get(ref.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Snapshots) != 1 {
		t.Fatalf("index = %+v", ix)
	}
	got := ix.Snapshots[0]
	if got.Label != "before dragon" || got.Kind != snapshot.KindManual || !got.LastPlayed.Equal(lp) || got.GameVersion != "1.21.4" {
		t.Errorf("snapshot from tags = %+v", got)
	}
	realWorld, _ := filepath.EvalSymlinks(world) // restic records the resolved path (/var -> /private/var on macOS)
	if ix.World.Name != ref.Name || ix.World.Path != realWorld || ix.World.SourceLabel != ref.SourceLabel {
		t.Errorf("world ref = %+v", ix.World)
	}

	dest := filepath.Join(t.TempDir(), "restored")
	if err := s.Extract(ref.ID, snap.ID, dest); err != nil {
		t.Fatal(err)
	}
	want, _ := os.ReadFile(filepath.Join(world, "region", "r.0.0.mca"))
	if b, err := os.ReadFile(filepath.Join(dest, "region", "r.0.0.mca")); err != nil || string(b) != string(want) {
		t.Errorf("restored region = %q, %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "session.lock")); !os.IsNotExist(err) {
		t.Error("session.lock must be excluded")
	}
	if err := s.Extract(ref.ID, "nope", filepath.Join(t.TempDir(), "x")); !errors.Is(err, snapshot.ErrNotFound) {
		t.Errorf("unknown snapshot err = %v", err)
	}
}

func TestPruneKeepsManual(t *testing.T) {
	s := newRepo(t)
	world := testworld.Create(t, t.TempDir(), "w", testworld.Options{})
	ref := snapshot.WorldRef{ID: "w"}
	other := snapshot.WorldRef{ID: "other"}

	for _, k := range []snapshot.Kind{snapshot.KindManual, snapshot.KindAuto, snapshot.KindPreRestore, snapshot.KindAuto} {
		if _, err := s.Create(ref, world, snapshot.Meta{Kind: k}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Create(other, world, snapshot.Meta{Kind: snapshot.KindAuto}); err != nil {
		t.Fatal(err)
	}

	removed, err := s.Prune("w", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 {
		t.Fatalf("removed = %v", removed)
	}
	ix, _ := s.Get("w")
	kinds := map[snapshot.Kind]int{}
	for _, snap := range ix.Snapshots {
		kinds[snap.Kind]++
	}
	if kinds[snapshot.KindManual] != 1 || kinds[snapshot.KindAuto]+kinds[snapshot.KindPreRestore] != 2 {
		t.Errorf("left = %v", kinds)
	}
	if o, _ := s.Get("other"); len(o.Snapshots) != 1 {
		t.Error("prune touched another world")
	}

	if err := s.Delete("w", ix.Snapshots[0].ID); err != nil {
		t.Fatal(err)
	}
	if ix, _ := s.Get("w"); len(ix.Snapshots) != 2 {
		t.Errorf("after delete: %d snapshots", len(ix.Snapshots))
	}
}

func TestWrongPasswordAndMissingBinary(t *testing.T) {
	s := newRepo(t)
	bad := NewStore(&Runner{Repo: s.r.Repo, Password: func() (string, error) { return "nope", nil }})
	if st := bad.Check(context.Background()); st.State != "wrong_password" {
		t.Errorf("wrong password status = %+v", st)
	}
	none := NewStore(&Runner{Binary: filepath.Join(t.TempDir(), "no-restic"), Repo: s.r.Repo})
	if st := none.Check(context.Background()); st.State != "not_installed" {
		t.Errorf("missing binary status = %+v", st)
	}
}

func TestAtLeast(t *testing.T) {
	for v, want := range map[string]bool{"0.16.4": false, "0.17.0": true, "0.18.1": true, "1.0.0": true} {
		if got := atLeast(v, MinVersion); got != want {
			t.Errorf("atLeast(%s) = %v", v, got)
		}
	}
}

func TestErrorMessage(t *testing.T) {
	stderr := `{"message_type":"exit_error","code":10,"message":"Fatal: repository does not exist: unable to open config file"}`
	if got := errorMessage(stderr); got != "repository does not exist: unable to open config file" {
		t.Errorf("got %q", got)
	}
	if got := errorMessage("plain text\n"); got != "plain text" {
		t.Errorf("got %q", got)
	}
}
