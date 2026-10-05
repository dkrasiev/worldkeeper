package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/config"
	"github.com/dkrasiev/worldkeeper/internal/snapshot"
	"github.com/dkrasiev/worldkeeper/internal/testworld"
)

func waitJob(t *testing.T, a *App, id string) Job {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		for _, j := range a.Jobs() {
			if j.ID == id && j.State != JobRunning {
				return j
			}
		}
	}
	t.Fatalf("job %s did not finish", id)
	return Job{}
}

func TestJobsReportProgressAndResult(t *testing.T) {
	f := setup(t)
	testworld.Create(t, f.saves, "w", testworld.Options{})

	j, err := f.app.StartBackup("minecraft--w", BackupOptions{Kind: snapshot.KindManual, Label: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if j.State != JobRunning || j.Kind != JobBackup || j.World != "w" {
		t.Errorf("started = %+v", j)
	}
	done := waitJob(t, f.app, j.ID)
	if done.State != JobDone || done.Snapshot == nil || done.Snapshot.Label != "x" || done.Total == 0 || done.Done != done.Total {
		t.Fatalf("backup job = %+v", done)
	}

	// Replacing takes a safety snapshot first, then restores.
	j, err = f.app.StartRestore("minecraft--w", done.Snapshot.ID, RestoreReplace)
	if err != nil {
		t.Fatal(err)
	}
	if r := waitJob(t, f.app, j.ID); r.State != JobDone || r.Phase != PhaseRestore || r.Path == "" || r.Done != r.Total {
		t.Fatalf("restore job = %+v", r)
	}
	if ix, _ := f.app.Snapshots().Get("minecraft--w"); len(ix.Snapshots) != 2 || ix.Snapshots[0].Kind != snapshot.KindPreRestore {
		t.Errorf("snapshots = %+v", ix.Snapshots)
	}
}

func TestJobsOnePerWorld(t *testing.T) {
	f := setup(t)
	testworld.Create(t, f.saves, "w", testworld.Options{})
	running, err := f.app.jobs.start(JobBackup, "minecraft--w", "w", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.StartBackup("minecraft--w", BackupOptions{Kind: snapshot.KindManual}); !errors.Is(err, ErrBusy) {
		t.Errorf("second backup err = %v, want ErrBusy", err)
	}
	// The watcher skips the world and tries again on its next pass.
	if saved, skipped, failed := f.app.BackupChanged(); saved != 0 || skipped != 1 || failed != 0 {
		t.Errorf("BackupChanged = %d saved, %d skipped, %d failed", saved, skipped, failed)
	}
	f.app.jobs.finish(running, nil, nil)
	if _, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindManual}); err != nil {
		t.Errorf("after the job finished: %v", err)
	}
}

func TestJobsFailWithCode(t *testing.T) {
	f := setup(t)
	testworld.Create(t, f.saves, "w", testworld.Options{})
	if _, err := f.app.StartRestore("minecraft--w", "nope", RestoreCopy); !errors.Is(err, snapshot.ErrNotFound) {
		t.Errorf("unknown snapshot err = %v", err)
	}

	// Storage inside a file cannot be created: the error comes from the
	// job, not from the request that started it.
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, nil, 0o644)
	f.app.Config.Update(func(c *config.Config) error { c.StorageDir = filepath.Join(file, "x"); return nil })
	j, err := f.app.StartBackup("minecraft--w", BackupOptions{Kind: snapshot.KindManual})
	if err != nil {
		t.Fatal(err)
	}
	if r := waitJob(t, f.app, j.ID); r.State != JobFailed || r.Error == nil || r.Error.Code == "" || r.Error.Message == "" {
		t.Fatalf("job = %+v", r)
	}
}
