package app

import (
	"errors"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/snapshot"
)

// BackupChanged backs up every world that changed since its last snapshot.
// Worlds that are open in the game or unchanged are skipped.
func (a *App) BackupChanged() (saved, skipped, failed int) {
	for _, w := range a.scan() {
		_, err := a.backup(w, BackupOptions{Kind: snapshot.KindAuto})
		switch {
		case err == nil:
			saved++
		case errors.Is(err, ErrUnchanged), errors.Is(err, ErrInUse):
			skipped++
		default:
			failed++
		}
	}
	return saved, skipped, failed
}

// Health summarizes backup state for the tray icon.
type Health struct {
	LastBackup time.Time
	// FailingWorld is the name of a world whose latest backup attempt
	// failed; Error is set for that or when storage cannot be read.
	FailingWorld string
	Error        string
}

func (a *App) Health() Health {
	var h Health
	ov, _ := a.Overview()
	for _, w := range ov.Worlds {
		if w.LastBackup != nil && w.LastBackup.CreatedAt.After(h.LastBackup) {
			h.LastBackup = w.LastBackup.CreatedAt
		}
	}
	if ov.StorageError != "" {
		h.Error = ov.StorageError
		return h
	}
	// A world is failing while its most recent event is an error.
	seen := map[string]bool{}
	for _, e := range a.Events() {
		if seen[e.WorldID] {
			continue
		}
		seen[e.WorldID] = true
		if e.Kind == EventError {
			h.FailingWorld, h.Error = e.World, e.Message
			break
		}
	}
	return h
}
