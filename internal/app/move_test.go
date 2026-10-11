package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dkrasiev/worldkeeper/internal/snapshot"
	"github.com/dkrasiev/worldkeeper/internal/testworld"
	"github.com/dkrasiev/worldkeeper/internal/worldinfo"
)

// prismSaves creates a Prism instance's saves folder in the fixture's home.
func prismSaves(t *testing.T, f fixture, instance string) string {
	t.Helper()
	dir := filepath.Join(f.app.Env.Home, ".local", "share", "PrismLauncher", "instances", instance, "minecraft", "saves")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestMoveWorldTakesItsBackupsAlong(t *testing.T) {
	f := setup(t)
	src := testworld.Create(t, f.saves, "w", testworld.Options{})
	if _, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindManual}); err != nil {
		t.Fatal(err)
	}
	prism := prismSaves(t, f, "Inst")

	j, err := f.app.StartMove("minecraft--w", prism)
	if err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, f.app, j.ID)
	if done.State != JobDone || done.NewWorldID != "prism_Inst--w" || done.Path != filepath.Join(prism, "w") {
		t.Fatalf("job = %+v", done)
	}
	if exists(src) || !exists(filepath.Join(prism, "w", "level.dat")) {
		t.Error("world not moved")
	}
	ix, err := f.app.Snapshots().Get("prism_Inst--w")
	if err != nil || len(ix.Snapshots) != 1 || ix.World.SourceLabel != "Prism Launcher: Inst" {
		t.Fatalf("backups under the new id = %+v, err %v", ix, err)
	}
	if ix, _ := f.app.Snapshots().Get("minecraft--w"); len(ix.Snapshots) != 0 {
		t.Error("backups left under the old id")
	}
	ov, _ := f.app.Overview()
	if len(ov.Worlds) != 1 || ov.Worlds[0].ID != "prism_Inst--w" || ov.Worlds[0].Snapshots != 1 || len(ov.Archived) != 0 {
		t.Errorf("overview = %+v", ov)
	}
	if e := f.app.Events()[0]; e.Code != CodeWorldMoved || e.WorldID != "prism_Inst--w" {
		t.Errorf("event = %+v", e)
	}
}

func TestMoveWorldToANewFolderAcrossDrives(t *testing.T) {
	f := setup(t)
	testworld.Create(t, f.saves, "w", testworld.Options{})
	defer func(r func(string, string) error) { rename = r }(rename)
	rename = func(string, string) error { return errors.New("not the same device") }

	dest := filepath.Join(t.TempDir(), "my saves")
	j, err := f.app.StartMove("minecraft--w", dest)
	if err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, f.app, j.ID)
	if done.State != JobDone || done.NewWorldID != "custom--w" || done.Total == 0 || done.Done != done.Total {
		t.Fatalf("job = %+v", done)
	}
	// The world was saved first (it had no backups), and the backup moved too.
	if ix, _ := f.app.Snapshots().Get("custom--w"); len(ix.Snapshots) != 1 || ix.Snapshots[0].Kind != snapshot.KindPreMove {
		t.Errorf("backups = %+v", ix)
	}
	if _, err := worldinfo.ReadSummary(filepath.Join(dest, "w")); err != nil {
		t.Errorf("copied world unreadable: %v", err)
	}
	if exists(filepath.Join(f.saves, "w")) || exists(filepath.Join(dest, "w", "session.lock")) {
		t.Error("original left behind, or session.lock copied")
	}
	if dirs := f.app.Config.Get().ExtraSavesDirs; len(dirs) != 1 || dirs[0] != dest {
		t.Errorf("extra saves folders = %v, want the new folder", dirs)
	}
}

func TestMoveWorldRefusals(t *testing.T) {
	f := setup(t)
	world := testworld.Create(t, f.saves, "w", testworld.Options{})
	full := prismSaves(t, f, "Full") // already has a world folder "w"
	testworld.Create(t, full, "w", testworld.Options{})
	taken := prismSaves(t, f, "Taken") // another world's backups use prism_Taken--w
	if _, err := f.app.Snapshots().Create(snapshot.WorldRef{ID: "prism_Taken--w"}, world, snapshot.Meta{Kind: snapshot.KindManual}); err != nil {
		t.Fatal(err)
	}

	for name, c := range map[string]struct {
		saves string
		want  error
	}{
		"same folder":   {f.saves, ErrSameLocation},
		"relative path": {"saves", ErrNotAbsolute},
		"folder taken":  {full, ErrDestExists},
		"backups taken": {taken, snapshot.ErrIDTaken},
	} {
		if _, err := f.app.StartMove("minecraft--w", c.saves); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
	if !exists(world) {
		t.Error("a refused move touched the world")
	}
}

func TestRestoreIntoAnotherFolder(t *testing.T) {
	f := setup(t)
	testworld.Create(t, f.saves, "w", testworld.Options{Name: "My World"})
	snap, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindManual})
	if err != nil {
		t.Fatal(err)
	}
	prism := prismSaves(t, f, "Inst")

	// A copy in another location keeps the world's folder and name.
	dest, err := f.app.RestoreTo("minecraft--w", snap.ID, RestoreCopy, prism)
	if err != nil || dest != filepath.Join(prism, "w") {
		t.Fatalf("copy: %s, %v", dest, err)
	}
	if s, _ := worldinfo.ReadSummary(dest); s.Name != "My World" {
		t.Errorf("copy name = %q", s.Name)
	}
	if _, err := f.app.RestoreTo("minecraft--w", snap.ID, RestoreReplace, prism); !errors.Is(err, ErrLocationReplace) {
		t.Errorf("replace into a folder: err = %v", err)
	}

	// A world without a local folder, into a folder Worldkeeper did not know.
	if err := os.RemoveAll(filepath.Join(f.saves, "w")); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "elsewhere")
	dest, err = f.app.RestoreTo("minecraft--w", snap.ID, RestoreCopy, other)
	if err != nil || dest != filepath.Join(other, "w") {
		t.Fatalf("archived: %s, %v", dest, err)
	}
	if dirs := f.app.Config.Get().ExtraSavesDirs; len(dirs) != 1 || dirs[0] != other {
		t.Errorf("extra saves folders = %v", dirs)
	}
	// The backups stay where they were: another computer may still have the world there.
	if ix, _ := f.app.Snapshots().Get("minecraft--w"); len(ix.Snapshots) != 1 {
		t.Errorf("backups moved: %+v", ix)
	}
}
