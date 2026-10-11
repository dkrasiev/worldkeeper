package app

import (
	"slices"
	"testing"

	"github.com/dkrasiev/worldkeeper/internal/config"
	"github.com/dkrasiev/worldkeeper/internal/snapshot"
	"github.com/dkrasiev/worldkeeper/internal/testworld"
)

// A world saved before the storage went away is not played again: once
// the storage is back, its retried backup finds nothing new to save. That
// must still clear the failure, in the tray and in notifications.
func TestFailureClearsWhenTheWorldWasSavedAlready(t *testing.T) {
	f := setup(t)
	n := &fakeNotifier{}
	f.app.Notifier = n
	testworld.Create(t, f.saves, "w", testworld.Options{Name: "My World", LastPlayed: 1000})
	if _, err := f.app.Backup("minecraft--w", BackupOptions{Kind: snapshot.KindAuto}); err != nil {
		t.Fatal(err)
	}
	storage := f.app.Config.Get().StorageDir

	blockStorage(t, f)
	open, pending := map[string]bool{}, map[string]bool{}
	f.app.watchPass(open, pending, true) // the first pass after a start checks every world
	if h := f.app.Health(); h.FailingWorld != "w" {
		t.Fatalf("while the storage is away: %+v", h)
	}

	f.app.Config.Update(func(c *config.Config) error { c.StorageDir = storage; return nil })
	f.app.watchPass(open, pending, false)
	if h := f.app.Health(); h.Error != "" || h.FailingWorld != "" {
		t.Errorf("after the storage is back: %+v", h)
	}
	if e := f.app.Events()[0]; e.Code != CodeBackupResumed || e.Kind != EventBackup {
		t.Errorf("latest event = %+v", e)
	}
	if want := []string{"failed My World", "recovered My World"}; !slices.Equal(n.got, want) {
		t.Errorf("notifications = %q, want %q", n.got, want)
	}

	// Later unchanged passes add nothing.
	before := len(f.app.Events())
	f.app.watchPass(open, pending, true)
	if len(f.app.Events()) != before {
		t.Error("an unchanged world that is not failing got an event")
	}
}
