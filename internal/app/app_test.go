package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dkrasiev/worldkeeper/internal/config"
	"github.com/dkrasiev/worldkeeper/internal/discovery"
	"github.com/dkrasiev/worldkeeper/internal/secrets"
	"github.com/dkrasiev/worldkeeper/internal/snapshot"
	"github.com/dkrasiev/worldkeeper/internal/testworld"
)

type fixture struct {
	app   *App
	saves string
}

func setup(t *testing.T) fixture {
	t.Helper()
	home := t.TempDir()
	cfg, err := config.Open(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	storage := t.TempDir()
	if _, err := cfg.Update(func(c *config.Config) error { c.StorageDir = storage; c.KeepAuto = 3; return nil }); err != nil {
		t.Fatal(err)
	}
	env := discovery.Env{GOOS: "linux", Home: home}
	a := New(cfg, &secrets.Memory{}, env, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return fixture{app: a, saves: discovery.DefaultSavesDir(env)}
}

func TestAutoBackupSkipsUnchanged(t *testing.T) {
	f := setup(t)
	dir := testworld.Create(t, f.saves, "w", testworld.Options{LastPlayed: 1000})

	if _, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindAuto}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindAuto}); !errors.Is(err, ErrUnchanged) {
		t.Fatalf("second auto backup err = %v, want ErrUnchanged", err)
	}
	// Manual saves are always taken.
	if _, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindManual, Label: "x"}); err != nil {
		t.Fatal(err)
	}

	testworld.WriteLevel(t, dir, testworld.Options{Name: "w", LastPlayed: 2000}) // game saved
	if _, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindAuto}); err != nil {
		t.Fatalf("backup after change: %v", err)
	}

	ov, err := f.app.Overview()
	if err != nil {
		t.Fatal(err)
	}
	if len(ov.Worlds) != 1 || ov.Worlds[0].Snapshots != 3 || ov.Worlds[0].Changed {
		t.Fatalf("overview = %+v", ov.Worlds)
	}
}

func TestRestoreReplaceKeepsSafetySnapshot(t *testing.T) {
	f := setup(t)
	dir := testworld.Create(t, f.saves, "w", testworld.Options{})
	good, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindManual})
	if err != nil {
		t.Fatal(err)
	}

	// The world gets griefed.
	os.WriteFile(filepath.Join(dir, "region", "r.0.0.mca"), []byte("creeper was here"), 0o644)

	dest, err := f.app.Restore("minecraft--w", good.ID, RestoreReplace)
	if err != nil {
		t.Fatal(err)
	}
	if dest != dir {
		t.Errorf("dest = %s", dest)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "region", "r.0.0.mca")); string(b) != "overworld region" {
		t.Errorf("region not restored: %q", b)
	}
	if _, err := os.Stat(filepath.Join(f.saves, ".w.worldkeeper-old")); !os.IsNotExist(err) {
		t.Error("old copy left behind")
	}

	ix, _ := f.app.Snapshots().Get("minecraft--w")
	if len(ix.Snapshots) != 2 || ix.Snapshots[0].Kind != snapshot.KindPreRestore {
		t.Fatalf("snapshots = %+v", ix.Snapshots)
	}
	// Undo: the griefed state is recoverable.
	copyDest, err := f.app.Restore("minecraft--w", ix.Snapshots[0].ID, RestoreCopy)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(copyDest, "region", "r.0.0.mca")); string(b) != "creeper was here" {
		t.Errorf("safety snapshot content = %q", b)
	}
}

func TestResticEngineRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic not installed")
	}
	f := setup(t)
	repo := filepath.Join(t.TempDir(), "repo")
	f.app.Secrets.Set(ResticSecretKey(repo), "pw")
	f.app.Config.Update(func(c *config.Config) error {
		c.Engine = config.EngineRestic
		c.Restic.Repo = repo
		return nil
	})
	if err := f.app.Restic().Init(context.Background()); err != nil {
		t.Fatal(err)
	}

	dir := testworld.Create(t, f.saves, "w", testworld.Options{LastPlayed: 1000})
	good, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindManual, Label: "good"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindAuto}); !errors.Is(err, ErrUnchanged) {
		t.Fatalf("auto after manual: %v, want ErrUnchanged", err)
	}

	os.WriteFile(filepath.Join(dir, "region", "r.0.0.mca"), []byte("griefed"), 0o644)
	if _, err := f.app.Restore("minecraft--w", good.ID, RestoreReplace); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "region", "r.0.0.mca")); string(b) != "overworld region" {
		t.Errorf("not restored: %q", b)
	}
	ov, _ := f.app.Overview()
	if ov.StorageError != "" || len(ov.Worlds) != 1 || ov.Worlds[0].Snapshots != 2 {
		t.Fatalf("overview = %+v", ov)
	}
}

func TestOverviewSurvivesStorageError(t *testing.T) {
	f := setup(t)
	testworld.Create(t, f.saves, "w", testworld.Options{})
	f.app.Config.Update(func(c *config.Config) error {
		c.Engine = config.EngineRestic
		c.Restic.Repo = filepath.Join(t.TempDir(), "repo")
		c.Restic.Binary = filepath.Join(t.TempDir(), "no-restic")
		return nil
	})
	ov, err := f.app.Overview()
	if err != nil || len(ov.Worlds) != 1 || ov.StorageError == "" {
		t.Fatalf("overview = %+v, err %v", ov, err)
	}
}

func TestRestoreArchivedWorld(t *testing.T) {
	f := setup(t)
	dir := testworld.Create(t, f.saves, "w", testworld.Options{})
	snap, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindManual})
	if err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(dir) // "reinstalled Windows"

	ov, _ := f.app.Overview()
	if len(ov.Worlds) != 0 || len(ov.Archived) != 1 {
		t.Fatalf("overview = %+v", ov)
	}

	dest, err := f.app.Restore("minecraft--w", snap.ID, RestoreReplace)
	if err != nil {
		t.Fatal(err)
	}
	if dest != dir {
		t.Errorf("restored to %s, want original %s", dest, dir)
	}
	if _, err := os.Stat(filepath.Join(dest, "level.dat")); err != nil {
		t.Error(err)
	}
}

func TestWatcherCatchesUpAndRetries(t *testing.T) {
	f := setup(t)
	testworld.Create(t, f.saves, "w", testworld.Options{})

	// Storage is unreachable: a regular file where the directory should be.
	blocked := filepath.Join(t.TempDir(), "nas")
	os.WriteFile(blocked, nil, 0o644)
	f.app.Config.Update(func(c *config.Config) error { c.StorageDir = blocked; return nil })

	open, pending := map[string]bool{}, map[string]bool{}
	f.app.watchPass(open, pending, true)
	if !pending["minecraft--w"] {
		t.Fatal("failed backup not queued for retry")
	}
	if ev := f.app.Events(); len(ev) == 0 || ev[0].Kind != EventError {
		t.Fatalf("events = %+v", ev)
	}

	storage := t.TempDir()
	f.app.Config.Update(func(c *config.Config) error { c.StorageDir = storage; return nil })
	f.app.watchPass(open, pending, false)
	if pending["minecraft--w"] {
		t.Fatal("retry did not succeed")
	}
	ix, _ := f.app.Snapshots().Get("minecraft--w")
	if len(ix.Snapshots) != 1 {
		t.Fatalf("snapshots = %d", len(ix.Snapshots))
	}
}

func TestRepeatedErrorsAreCollapsed(t *testing.T) {
	f := setup(t)
	// NAS offline: every world fails on every poll, interleaved.
	for i := 0; i < 3; i++ {
		f.app.event(EventError, "w", "w", CodeBackupFailed, nil, "NAS offline")
		f.app.event(EventError, "x", "x", CodeBackupFailed, nil, "NAS offline")
	}
	if ev := f.app.Events(); len(ev) != 2 {
		t.Fatalf("events = %d, want one per world", len(ev))
	}
	// A success in between starts a new entry for the next failure.
	f.app.event(EventBackup, "w", "w", CodeBackupFailed, nil, "Saved")
	f.app.event(EventError, "w", "w", CodeBackupFailed, nil, "NAS offline")
	if ev := f.app.Events(); len(ev) != 4 {
		t.Fatalf("events = %d, want 4", len(ev))
	}
}

func TestSnapshotDetailsShowThePast(t *testing.T) {
	f := setup(t)
	dir := testworld.Create(t, f.saves, "w", testworld.Options{Name: "Old name", LastPlayed: 1000})
	old, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindManual})
	if err != nil {
		t.Fatal(err)
	}
	testworld.WriteLevel(t, dir, testworld.Options{Name: "New name", LastPlayed: 2000})

	d, err := f.app.SnapshotDetails("minecraft--w", old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Info.Name != "Old name" || d.Advancements["minecraft:story/root"] != true {
		t.Errorf("snapshot details = %q, %v", d.Info.Name, d.Advancements)
	}
	if cur, _ := f.app.Info("minecraft--w"); cur.Name != "New name" {
		t.Errorf("current = %q", cur.Name)
	}
	if _, err := f.app.SnapshotDetails("minecraft--w", "nope"); !errors.Is(err, snapshot.ErrNotFound) {
		t.Errorf("unknown snapshot: %v", err)
	}
}

func TestBackupChangedAndHealth(t *testing.T) {
	f := setup(t)
	testworld.Create(t, f.saves, "a", testworld.Options{})
	testworld.Create(t, f.saves, "b", testworld.Options{})

	if h := f.app.Health(); !h.LastBackup.IsZero() || h.Error != "" {
		t.Fatalf("fresh health = %+v", h)
	}
	if saved, skipped, failed := f.app.BackupChanged(); saved != 2 || skipped != 0 || failed != 0 {
		t.Fatalf("first run: %d %d %d", saved, skipped, failed)
	}
	if saved, skipped, failed := f.app.BackupChanged(); saved != 0 || skipped != 2 || failed != 0 {
		t.Fatalf("second run: %d %d %d", saved, skipped, failed)
	}
	if h := f.app.Health(); h.LastBackup.IsZero() || h.Error != "" {
		t.Fatalf("health after backup = %+v", h)
	}

	f.app.event(EventError, "minecraft--a", "a", CodeBackupFailed, map[string]string{"error": "NAS offline"}, "Backup failed: NAS offline")
	if h := f.app.Health(); h.FailingWorld != "a" {
		t.Fatalf("health with failure = %+v", h)
	}
}

func TestHealthReadsStorageOnlyOnce(t *testing.T) {
	f := setup(t)
	testworld.Create(t, f.saves, "w", testworld.Options{})
	storage := f.app.Config.Get().StorageDir

	if h := f.app.Health(); !h.LastBackup.IsZero() || h.Error != "" {
		t.Fatalf("empty storage: %+v", h)
	}
	snap, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindManual})
	if err != nil {
		t.Fatal(err)
	}
	if h := f.app.Health(); !h.LastBackup.Equal(snap.CreatedAt) {
		t.Fatalf("after backup: %+v, want %v", h, snap.CreatedAt)
	}

	// Storage gone (NAS asleep, unplugged): the tray keeps its answer
	// because it no longer reads storage on every poll.
	if err := os.RemoveAll(storage); err != nil {
		t.Fatal(err)
	}
	if h := f.app.Health(); !h.LastBackup.Equal(snap.CreatedAt) || h.Error != "" {
		t.Fatalf("cached health = %+v", h)
	}

	// New storage settings are read once. A restic engine whose binary is
	// missing is unreadable on every OS (a file in place of the zip folder
	// is not: Windows reports it as "not found", i.e. empty storage).
	f.app.Config.Update(func(c *config.Config) error {
		c.Engine = config.EngineRestic
		c.Restic.Repo = filepath.Join(t.TempDir(), "repo")
		c.Restic.Binary = filepath.Join(t.TempDir(), "no-restic")
		return nil
	})
	if h := f.app.Health(); h.Error == "" || !h.LastBackup.IsZero() {
		t.Fatalf("unreadable storage: %+v", h)
	}

	fresh := t.TempDir()
	f.app.Config.Update(func(c *config.Config) error { c.Engine = config.EngineZip; c.StorageDir = fresh; return nil })
	if h := f.app.Health(); h.Error != "" || !h.LastBackup.IsZero() {
		t.Fatalf("fresh storage: %+v", h)
	}
}

func TestHealthReloadsAfterDelete(t *testing.T) {
	f := setup(t)
	testworld.Create(t, f.saves, "w", testworld.Options{})
	first, _ := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindManual})
	second, _ := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindManual})
	if h := f.app.Health(); !h.LastBackup.Equal(second.CreatedAt) {
		t.Fatalf("health = %+v", h)
	}
	if err := f.app.DeleteSnapshot("minecraft--w", second.ID); err != nil {
		t.Fatal(err)
	}
	if h := f.app.Health(); !h.LastBackup.Equal(first.CreatedAt) {
		t.Fatalf("after deleting the newest: %+v, want %v", h, first.CreatedAt)
	}
}
