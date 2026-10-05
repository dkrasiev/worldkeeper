package app

import (
	"context"
	"errors"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/lock"
	"github.com/dkrasiev/worldkeeper/internal/snapshot"
)

// Watch polls session.lock files and backs a world up when the game closes it.
//
// On the first pass it also catches up on worlds played while worldkeeper
// was not running. Failed backups (e.g. NAS offline) are retried every pass.
func (a *App) Watch(ctx context.Context) {
	open := map[string]bool{}    // world id -> was in use on the previous pass
	pending := map[string]bool{} // world id -> backup still owed
	first := true

	for {
		if a.Config.Get().AutoBackup {
			a.watchPass(open, pending, first)
			first = false
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(a.Config.Get().PollInterval.Duration):
		}
	}
}

func (a *App) watchPass(open, pending map[string]bool, first bool) {
	for _, w := range a.scan() {
		inUse, err := lock.InUse(w.Path)
		if err != nil {
			a.Log.Warn("lock check failed", "world", w.ID, "err", err)
			continue
		}
		wasOpen := open[w.ID]
		open[w.ID] = inUse
		if inUse {
			continue
		}
		if !(first || wasOpen || pending[w.ID]) {
			continue
		}

		_, err = a.backupJob(w, BackupOptions{Kind: snapshot.KindAuto})
		switch {
		case err == nil, errors.Is(err, ErrUnchanged):
			delete(pending, w.ID)
		case errors.Is(err, ErrInUse):
			// Reopened between the check and the backup; next close will trigger again.
		default:
			// Includes ErrBusy: a manual save or restore is running, try again next pass.
			pending[w.ID] = true
		}
	}
}
