package restic

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/snapshot"
	"github.com/dkrasiev/worldkeeper/internal/testworld"
	"github.com/dkrasiev/worldkeeper/internal/worldinfo"
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
	if err := s.Extract(ref.ID, snap.ID, dest, nil); err != nil {
		t.Fatal(err)
	}
	want, _ := os.ReadFile(filepath.Join(world, "region", "r.0.0.mca"))
	if b, err := os.ReadFile(filepath.Join(dest, "region", "r.0.0.mca")); err != nil || string(b) != string(want) {
		t.Errorf("restored region = %q, %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "session.lock")); !os.IsNotExist(err) {
		t.Error("session.lock must be excluded")
	}
	if err := s.Extract(ref.ID, "nope", filepath.Join(t.TempDir(), "x"), nil); !errors.Is(err, snapshot.ErrNotFound) {
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

func TestOpenListsAllAndRestoresOnlyMetadata(t *testing.T) {
	s := newRepo(t)
	world := testworld.Create(t, t.TempDir(), "w", testworld.Options{Layout26: true})
	snap, err := s.Create(snapshot.WorldRef{ID: "w", Folder: "w"}, world, snapshot.Meta{Kind: snapshot.KindManual})
	if err != nil {
		t.Fatal(err)
	}
	fsys, closer, err := s.Open("w", snap.ID)
	if err != nil {
		t.Fatal(err)
	}
	tmp := string(closer.(removeDir))

	info, err := worldinfo.ReadFS(fsys, "w")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := worldinfo.Read(world)
	if info.Seed != want.Seed || info.Advancements != want.Advancements || info.Player == nil || info.Stats == nil {
		t.Errorf("from restic = %+v", info)
	}
	// Sizes and regions come from the listing, without restoring regions.
	if info.SizeBytes != want.SizeBytes-int64(len("☃")) || len(info.Dimensions) != 1 {
		t.Errorf("size %d (dir %d), dims %+v", info.SizeBytes, want.SizeBytes, info.Dimensions)
	}
	if _, err := os.Stat(filepath.Join(tmp, "dimensions")); !os.IsNotExist(err) {
		t.Error("region files must not be restored for a preview")
	}
	if _, err := fs.ReadFile(fsys, "dimensions/minecraft/overworld/region/r.0.0.mca"); err == nil {
		t.Error("unrestored file should not be readable")
	}

	closer.Close()
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Error("temp dir not removed")
	}
}

func TestProgress(t *testing.T) {
	s := newRepo(t)
	world := testworld.Create(t, t.TempDir(), "w", testworld.Options{})
	type report struct{ done, total int64 }
	var got []report
	record := func(done, total int64) { got = append(got, report{done, total}) }

	snap, err := s.Create(snapshot.WorldRef{ID: "w"}, world, snapshot.Meta{Kind: snapshot.KindManual, Progress: record})
	if err != nil {
		t.Fatal(err)
	}
	if last := got[len(got)-1]; len(got) < 2 || last.total == 0 || last.done != last.total {
		t.Errorf("backup progress = %v", got)
	}

	got = nil
	if err := s.Extract("w", snap.ID, filepath.Join(t.TempDir(), "out"), record); err != nil {
		t.Fatal(err)
	}
	if last := got[len(got)-1]; len(got) < 2 || last.total != snap.SizeBytes || last.done != last.total {
		t.Errorf("restore progress = %v, size %d", got, snap.SizeBytes)
	}
}

func TestStatusLines(t *testing.T) {
	var done, total int64
	on := statusLines(func(d, tot int64) { done, total = d, tot })
	for _, tc := range []struct {
		line        string
		done, total int64
	}{
		{`{"message_type":"status","percent_done":0.5,"total_files":2,"files_done":1,"total_bytes":100,"bytes_done":50}`, 50, 100},
		{`{"message_type":"status","percent_done":0.7,"total_files":2,"total_bytes":100,"bytes_restored":70}`, 70, 100},
		{`{"message_type":"summary","snapshot_id":"x","total_bytes_processed":100}`, 70, 100}, // backup summary: ignored
		{`not json`, 70, 100},
		{`{"message_type":"summary","total_files":2,"files_restored":2,"total_bytes":100,"bytes_restored":100}`, 100, 100},
	} {
		on([]byte(tc.line))
		if done != tc.done || total != tc.total {
			t.Errorf("%s: got %d/%d, want %d/%d", tc.line, done, total, tc.done, tc.total)
		}
	}
	if statusLines(nil) != nil {
		t.Error("nil progress should not read lines")
	}
}

func TestLineWriter(t *testing.T) {
	var buf bytes.Buffer
	var lines []string
	w := &lineWriter{buf: &buf, onLine: func(b []byte) { lines = append(lines, string(b)) }}
	w.Write([]byte("one\ntw"))
	w.Write([]byte("o\nthree"))
	if len(lines) != 2 || lines[0] != "one" || lines[1] != "two" || buf.String() != "one\ntwo\nthree" {
		t.Errorf("lines = %q, buf = %q", lines, buf.String())
	}
}
