package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/dkrasiev/worldkeeper/internal/config"
	"github.com/dkrasiev/worldkeeper/internal/snapshot"
	"github.com/dkrasiev/worldkeeper/internal/testworld"
)

type fakeNotifier struct{ got []string }

func (n *fakeNotifier) BackupFailed(world, err string) { n.got = append(n.got, "failed "+world) }
func (n *fakeNotifier) BackupRecovered(world string)   { n.got = append(n.got, "recovered "+world) }

// blockStorage makes the backup folder unusable, like a NAS that is offline.
func blockStorage(t *testing.T, f fixture) {
	t.Helper()
	blocked := filepath.Join(t.TempDir(), "nas")
	if err := os.WriteFile(blocked, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	f.app.Config.Update(func(c *config.Config) error { c.StorageDir = blocked; return nil })
}

func TestNotifyFirstFailureAndRecovery(t *testing.T) {
	f := setup(t)
	n := &fakeNotifier{}
	f.app.Notifier = n
	dir := testworld.Create(t, f.saves, "w", testworld.Options{Name: "My World", LastPlayed: 1000})

	blockStorage(t, f)
	open, pending := map[string]bool{}, map[string]bool{}
	for i := 0; i < 3; i++ { // the watcher retries every pass
		f.app.watchPass(open, pending, i == 0)
	}
	if want := []string{"failed My World"}; !slices.Equal(n.got, want) {
		t.Fatalf("while failing: %q, want %q", n.got, want)
	}

	storage := t.TempDir()
	f.app.Config.Update(func(c *config.Config) error { c.StorageDir = storage; return nil })
	f.app.watchPass(open, pending, false)
	testworld.WriteLevel(t, dir, testworld.Options{Name: "My World", LastPlayed: 2000})
	if _, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindAuto}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"failed My World", "recovered My World"}; !slices.Equal(n.got, want) {
		t.Fatalf("after recovery: %q, want %q", n.got, want)
	}

	// The next outage is a new first failure.
	blockStorage(t, f)
	testworld.WriteLevel(t, dir, testworld.Options{Name: "My World", LastPlayed: 3000})
	f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindAuto})
	if len(n.got) != 3 || n.got[2] != "failed My World" {
		t.Fatalf("second outage: %q", n.got)
	}
}

func TestNotifyRemembersFailuresAcrossRestarts(t *testing.T) {
	f := setup(t)
	path := filepath.Join(t.TempDir(), "events.json")
	if err := f.app.OpenEvents(path); err != nil {
		t.Fatal(err)
	}
	testworld.Create(t, f.saves, "w", testworld.Options{Name: "My World"})
	blockStorage(t, f)
	f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindAuto})

	// Restarted: the failure was announced before, its recovery was not.
	restarted := New(f.app.Config, f.app.Secrets, f.app.Env, f.app.Log)
	if err := restarted.OpenEvents(path); err != nil {
		t.Fatal(err)
	}
	n := &fakeNotifier{}
	restarted.Notifier = n
	restarted.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindAuto})
	if len(n.got) != 0 {
		t.Fatalf("failure announced again after restart: %q", n.got)
	}
	storage := t.TempDir()
	restarted.Config.Update(func(c *config.Config) error { c.StorageDir = storage; return nil })
	if _, err := restarted.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindAuto}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"recovered My World"}; !slices.Equal(n.got, want) {
		t.Fatalf("after restart: %q, want %q", n.got, want)
	}
}

func TestNotificationsCanBeTurnedOff(t *testing.T) {
	f := setup(t)
	n := &fakeNotifier{}
	f.app.Notifier = n
	f.app.Config.Update(func(c *config.Config) error { c.Notifications = false; return nil })
	testworld.Create(t, f.saves, "w", testworld.Options{})
	blockStorage(t, f)
	f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindAuto})
	if len(n.got) != 0 {
		t.Fatalf("notified with notifications off: %q", n.got)
	}
	if ev := f.app.Events(); len(ev) != 1 || ev[0].Code != CodeBackupFailed {
		t.Fatalf("the failure must still be in the feed: %+v", ev)
	}
}
