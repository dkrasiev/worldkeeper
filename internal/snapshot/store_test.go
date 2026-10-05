package snapshot

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/testworld"
	"github.com/dkrasiev/worldkeeper/internal/worldinfo"
)

func newTestStore(t *testing.T) (*Store, string) {
	root := t.TempDir()
	s := NewStore(func() string { return root })
	clock := time.Date(2026, 10, 5, 18, 30, 0, 0, time.UTC)
	s.now = func() time.Time { clock = clock.Add(time.Minute); return clock }
	return s, root
}

func TestCreateAndExtract(t *testing.T) {
	s, root := newTestStore(t)
	world := testworld.Create(t, t.TempDir(), "w", testworld.Options{})
	ref := WorldRef{ID: "minecraft--w", Name: "w", Folder: "w", Path: world}

	snap, err := s.Create(ref, world, Meta{Kind: KindManual, Label: "before dragon"})
	if err != nil {
		t.Fatal(err)
	}
	if snap.ID != "20261005-183100-manual" || snap.SizeBytes == 0 {
		t.Fatalf("snapshot = %+v", snap)
	}
	if _, err := os.Stat(filepath.Join(root, "minecraft--w", snap.ID+".zip.partial")); !os.IsNotExist(err) {
		t.Error("partial file left behind")
	}

	ix, err := s.Get(ref.ID)
	if err != nil || len(ix.Snapshots) != 1 || ix.World.Path != world {
		t.Fatalf("index = %+v, err %v", ix, err)
	}

	dest := filepath.Join(t.TempDir(), "restored")
	if err := s.Extract(ref.ID, snap.ID, dest); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"level.dat", "icon.png", "region/r.0.0.mca", "DIM-1/region/r.0.0.mca"} {
		want, _ := os.ReadFile(filepath.Join(world, f))
		got, err := os.ReadFile(filepath.Join(dest, f))
		if err != nil || string(got) != string(want) {
			t.Errorf("%s not restored correctly: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, "session.lock")); !os.IsNotExist(err) {
		t.Error("session.lock must not be archived")
	}
	if err := s.Extract(ref.ID, snap.ID, dest); err == nil {
		t.Error("extract over existing folder must fail")
	}
}

func TestPruneKeepsManual(t *testing.T) {
	s, root := newTestStore(t)
	world := testworld.Create(t, t.TempDir(), "w", testworld.Options{})
	ref := WorldRef{ID: "w"}

	var ids []string
	for _, k := range []Kind{KindManual, KindAuto, KindAuto, KindPreRestore, KindAuto} {
		snap, err := s.Create(ref, world, Meta{Kind: k})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, snap.ID)
	}

	removed, err := s.Prune("w", 2)
	if err != nil {
		t.Fatal(err)
	}
	// Newest first: auto(4), pre-restore(3) kept; auto(2), auto(1) removed; manual(0) kept.
	if len(removed) != 2 || removed[0] != ids[2] || removed[1] != ids[1] {
		t.Fatalf("removed = %v, ids = %v", removed, ids)
	}
	ix, _ := s.Get("w")
	if len(ix.Snapshots) != 3 || ix.Snapshots[2].Kind != KindManual {
		t.Fatalf("left = %+v", ix.Snapshots)
	}
	if _, err := os.Stat(filepath.Join(root, "w", ids[1]+".zip")); !os.IsNotExist(err) {
		t.Error("pruned archive still on disk")
	}
}

func TestDeleteAndAll(t *testing.T) {
	s, _ := newTestStore(t)
	world := testworld.Create(t, t.TempDir(), "w", testworld.Options{})
	a, _ := s.Create(WorldRef{ID: "a"}, world, Meta{Kind: KindManual})
	if _, err := s.Create(WorldRef{ID: "b"}, world, Meta{Kind: KindAuto}); err != nil {
		t.Fatal(err)
	}

	if err := s.Delete("a", a.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("a", a.ID); err != ErrNotFound {
		t.Errorf("second delete err = %v", err)
	}
	all, err := s.All()
	if err != nil || len(all) != 1 || all[0].World.ID != "b" {
		t.Fatalf("all = %+v, err %v", all, err)
	}
}

func TestRejectsUnsafeIDs(t *testing.T) {
	s, _ := newTestStore(t)
	for _, id := range []string{"", ".", "..", "../x", "a/b"} {
		if _, err := s.Get(id); err == nil {
			t.Errorf("Get(%q) accepted", id)
		}
	}
}

func TestExtractRejectsZipSlip(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "evil.zip")
	f, _ := os.Create(archive)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("../escaped.txt")
	w.Write([]byte("x"))
	zw.Close()
	f.Close()

	dest := t.TempDir()
	if err := extractZip(archive, dest); err == nil {
		t.Fatal("zip slip not rejected")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dest), "escaped.txt")); !os.IsNotExist(err) {
		t.Fatal("file escaped destination")
	}
}

func TestOpenReadsWorldInfoFromZip(t *testing.T) {
	s, _ := newTestStore(t)
	world := testworld.Create(t, t.TempDir(), "w", testworld.Options{Layout26: true})
	snap, err := s.Create(WorldRef{ID: "w"}, world, Meta{Kind: KindManual})
	if err != nil {
		t.Fatal(err)
	}
	fsys, closer, err := s.Open("w", snap.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()

	info, err := worldinfo.ReadFS(fsys, "w")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := worldinfo.Read(world)
	if info.Seed != want.Seed || info.Advancements != want.Advancements || info.Player == nil ||
		len(info.Dimensions) != len(want.Dimensions) || info.GameVersion != "26.3" {
		t.Errorf("from zip = %+v\nfrom dir = %+v", info, want)
	}
	if _, _, err := s.Open("w", "nope"); err != ErrNotFound {
		t.Errorf("unknown snapshot: %v", err)
	}
}
