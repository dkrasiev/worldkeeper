package app

import (
	"context"
	"sync"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/update"
)

const (
	// updateEvery is how often GitHub is asked for a new release.
	updateEvery = 24 * time.Hour
	// updateRetry is how soon a failed check (offline, rate limited) is retried.
	updateRetry = time.Hour
	// updatePoll is how often WatchUpdates looks whether a check is due, so
	// turning the setting on or waking from sleep is noticed soon.
	updatePoll = 10 * time.Minute
)

// UpdateStatus is what the UI and the tray show about new releases.
type UpdateStatus struct {
	Current string `json:"current"`
	// Versioned is false for local builds ("dev"), which are never checked.
	Versioned bool            `json:"versioned"`
	Latest    *update.Release `json:"latest,omitempty"`
	// Available is set when Latest is newer than the running build.
	Available bool      `json:"available"`
	CheckedAt time.Time `json:"checkedAt,omitzero"` // last successful check
	Error     string    `json:"error,omitempty"`    // last check failed
}

type updateState struct {
	mu        sync.Mutex
	latest    *update.Release
	checkedAt time.Time
	triedAt   time.Time
	err       string
}

// UpdateStatus returns the result of the last check without checking again.
func (a *App) UpdateStatus() UpdateStatus {
	a.updates.mu.Lock()
	defer a.updates.mu.Unlock()
	st := UpdateStatus{
		Current:   a.Version,
		Versioned: update.Versioned(a.Version),
		Latest:    a.updates.latest,
		CheckedAt: a.updates.checkedAt,
		Error:     a.updates.err,
	}
	if st.Latest != nil {
		st.Available, _ = update.Newer(st.Latest.Version, a.Version)
	}
	return st
}

// CheckUpdate asks GitHub for the latest release now.
func (a *App) CheckUpdate(ctx context.Context) UpdateStatus {
	if !update.Versioned(a.Version) {
		return a.UpdateStatus()
	}
	rel, err := a.UpdateChecker.Latest(ctx, a.Version)

	a.updates.mu.Lock()
	a.updates.triedAt = time.Now()
	if err != nil {
		a.updates.err = err.Error()
	} else {
		known := a.updates.latest != nil && a.updates.latest.Version == rel.Version
		a.updates.latest, a.updates.checkedAt, a.updates.err = &rel, a.updates.triedAt, ""
		if newer, _ := update.Newer(rel.Version, a.Version); newer && !known {
			a.Log.Info("a new version is available", "version", rel.Version, "url", rel.URL)
		}
	}
	a.updates.mu.Unlock()

	if err != nil {
		a.Log.Warn("update check failed", "err", err)
	}
	return a.UpdateStatus()
}

// WatchUpdates checks for a new release on start and then once a day,
// while the setting is on. Local builds are never checked.
func (a *App) WatchUpdates(ctx context.Context) {
	if !update.Versioned(a.Version) {
		return
	}
	for {
		if a.updateDue(time.Now()) {
			a.CheckUpdate(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(updatePoll):
		}
	}
}

func (a *App) updateDue(now time.Time) bool {
	if !a.Config.Get().CheckUpdates {
		return false
	}
	a.updates.mu.Lock()
	defer a.updates.mu.Unlock()
	if a.updates.triedAt.IsZero() {
		return true
	}
	wait := updateEvery
	if a.updates.err != "" {
		wait = updateRetry
	}
	return now.Sub(a.updates.triedAt) >= wait
}
