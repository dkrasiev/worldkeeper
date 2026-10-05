package app

import (
	"errors"
	"os"
	"testing"

	"github.com/dkrasiev/worldkeeper/internal/snapshot"
	"github.com/dkrasiev/worldkeeper/internal/testworld"
	"github.com/dkrasiev/worldkeeper/internal/worldinfo"
)

func TestRenameKeepsFolderAndBackups(t *testing.T) {
	f := setup(t)
	dir := testworld.Create(t, f.saves, "w", testworld.Options{})
	if _, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindManual}); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Rename("minecraft--w", "  Новое имя "); err != nil {
		t.Fatal(err)
	}
	if s, _ := worldinfo.ReadSummary(dir); s.Name != "Новое имя" {
		t.Errorf("name = %q", s.Name)
	}
	ov, _ := f.app.Overview()
	if len(ov.Worlds) != 1 || ov.Worlds[0].ID != "minecraft--w" || ov.Worlds[0].Snapshots != 1 {
		t.Errorf("overview after rename = %+v", ov.Worlds)
	}
	if ev := f.app.Events(); ev[0].Code != CodeWorldRenamed || ev[0].Params["from"] != "w" || ev[0].Params["to"] != "Новое имя" {
		t.Errorf("event = %+v", ev[0])
	}
	if err := f.app.Rename("minecraft--w", " "); !errors.Is(err, ErrNameRequired) {
		t.Errorf("empty name err = %v", err)
	}
	if err := f.app.Rename("nope", "x"); !errors.Is(err, ErrWorldNotFound) {
		t.Errorf("unknown world err = %v", err)
	}
}

func TestDeleteSavesFirstAndCanBeUndone(t *testing.T) {
	f := setup(t)
	dir := testworld.Create(t, f.saves, "w", testworld.Options{LastPlayed: 1000})

	j, err := f.app.StartDelete("minecraft--w")
	if err != nil {
		t.Fatal(err)
	}
	if r := waitJob(t, f.app, j.ID); r.State != JobDone || r.Kind != JobDelete || r.Phase != PhaseDelete {
		t.Fatalf("job = %+v", r)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("world folder still there")
	}
	if entries, _ := os.ReadDir(f.saves); len(entries) != 0 {
		t.Errorf("left in saves: %v", entries)
	}

	// The world now shows up as a backup without a local world.
	ov, _ := f.app.Overview()
	if len(ov.Worlds) != 0 || len(ov.Archived) != 1 || ov.Archived[0].Snapshots[0].Kind != snapshot.KindPreDelete {
		t.Fatalf("overview after delete = %+v", ov)
	}
	if ev := f.app.Events(); ev[0].Code != CodeWorldDeleted {
		t.Errorf("event = %+v", ev[0])
	}

	dest, err := f.app.Restore("minecraft--w", ov.Archived[0].Snapshots[0].ID, RestoreReplace)
	if err != nil {
		t.Fatal(err)
	}
	if dest != dir {
		t.Errorf("restored to %s, want %s", dest, dir)
	}
}

func TestDeleteSkipsSnapshotWhenAlreadySaved(t *testing.T) {
	f := setup(t)
	testworld.Create(t, f.saves, "w", testworld.Options{LastPlayed: 1000})
	if _, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindAuto}); err != nil {
		t.Fatal(err)
	}
	j, err := f.app.StartDelete("minecraft--w")
	if err != nil {
		t.Fatal(err)
	}
	waitJob(t, f.app, j.ID)
	if ix, _ := f.app.Snapshots().Get("minecraft--w"); len(ix.Snapshots) != 1 || ix.Snapshots[0].Kind != snapshot.KindAuto {
		t.Errorf("snapshots = %+v, want only the existing one", ix.Snapshots)
	}
	if _, err := f.app.StartDelete("minecraft--w"); !errors.Is(err, ErrWorldNotFound) {
		t.Errorf("second delete err = %v", err)
	}
}

// A rename keeps LastPlayed, so the saved state alone would bring the
// world back under its old name.
func TestDeleteAfterRenameSavesNewName(t *testing.T) {
	f := setup(t)
	testworld.Create(t, f.saves, "w", testworld.Options{LastPlayed: 1000})
	if _, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindAuto}); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Rename("minecraft--w", "New name"); err != nil {
		t.Fatal(err)
	}
	j, err := f.app.StartDelete("minecraft--w")
	if err != nil {
		t.Fatal(err)
	}
	waitJob(t, f.app, j.ID)
	ix, _ := f.app.Snapshots().Get("minecraft--w")
	if len(ix.Snapshots) != 2 || ix.Snapshots[0].Kind != snapshot.KindPreDelete || ix.World.Name != "New name" {
		t.Errorf("index = %+v", ix)
	}
}

func TestRenameWhileBusy(t *testing.T) {
	f := setup(t)
	testworld.Create(t, f.saves, "w", testworld.Options{})
	j, _ := f.app.jobs.start(JobBackup, "minecraft--w", "w", false)
	if err := f.app.Rename("minecraft--w", "x"); !errors.Is(err, ErrBusy) {
		t.Errorf("rename during a job err = %v", err)
	}
	if _, err := f.app.StartDelete("minecraft--w"); !errors.Is(err, ErrBusy) {
		t.Errorf("delete during a job err = %v", err)
	}
	f.app.jobs.finish(j, nil, nil)
	if err := f.app.Rename("minecraft--w", "x"); err != nil {
		t.Error(err)
	}
}

func TestPruneKeepsPreDelete(t *testing.T) {
	f := setup(t)
	dir := testworld.Create(t, f.saves, "w", testworld.Options{LastPlayed: 1})
	if _, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindPreDelete}); err != nil {
		t.Fatal(err)
	}
	for i := int64(2); i < 7; i++ { // keepAuto is 3 in setup
		testworld.WriteLevel(t, dir, testworld.Options{Name: "w", LastPlayed: i})
		if _, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindAuto}); err != nil {
			t.Fatal(err)
		}
	}
	ix, _ := f.app.Snapshots().Get("minecraft--w")
	if n := len(ix.Snapshots); n != 4 || ix.Snapshots[n-1].Kind != snapshot.KindPreDelete {
		t.Errorf("snapshots = %+v", ix.Snapshots)
	}
}
