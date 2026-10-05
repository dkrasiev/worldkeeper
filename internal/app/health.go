package app

import (
	"errors"
	"sync"
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

// Health is polled by the tray every few seconds, so it must not touch the
// backup storage: reading a NAS share or running restic that often keeps
// network disks awake around the clock. The time of the last backup is
// read from storage once and then kept up to date as backups happen.
func (a *App) Health() Health {
	var h Health
	h.LastBackup, h.Error = a.lastBackup.get(a)
	if h.Error != "" {
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

// lastBackupCache remembers the newest snapshot time for the configured
// storage. It reloads only when the storage settings change, after a
// snapshot is deleted, or (with backoff) after the storage was unreadable.
type lastBackupCache struct {
	mu       sync.Mutex
	key      string // storage identity the value belongs to
	loaded   bool
	last     time.Time
	err      string
	failedAt time.Time
}

// retryUnreadable is how long to wait before reading storage again after
// it failed, e.g. while a NAS is offline.
const retryUnreadable = time.Minute

func storageKey(a *App) string {
	c := a.Config.Get()
	return c.Engine + "\x00" + c.StorageDir + "\x00" + c.Restic.Repo
}

func (c *lastBackupCache) get(a *App) (time.Time, string) {
	key := storageKey(a)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.key != key { // storage settings changed: start over
		c.key, c.loaded, c.last, c.err, c.failedAt = key, false, time.Time{}, "", time.Time{}
	}
	if c.loaded || (c.err != "" && time.Since(c.failedAt) < retryUnreadable) {
		return c.last, c.err
	}

	indexes, err := a.Snapshots().All()
	if err != nil {
		c.err, c.failedAt = err.Error(), time.Now()
		return c.last, c.err
	}
	c.last, c.err, c.loaded = time.Time{}, "", true
	for _, ix := range indexes {
		if s := ix.Latest(); s != nil && s.CreatedAt.After(c.last) {
			c.last = s.CreatedAt
		}
	}
	return c.last, ""
}

// noteBackup records a snapshot just written, without reading storage.
func (c *lastBackupCache) noteBackup(a *App, at time.Time) {
	key := storageKey(a)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.key != key || !c.loaded {
		return // the next get loads the full picture anyway
	}
	if at.After(c.last) {
		c.last = at
	}
}

// invalidate forces the next get to read storage, e.g. after a delete.
func (c *lastBackupCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loaded = false
}
