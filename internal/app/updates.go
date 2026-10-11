package app

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/config"
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
	Versioned bool `json:"versioned"`
	// DevBuilds is set when test builds are made for this platform.
	DevBuilds bool            `json:"devBuilds"`
	Latest    *update.Release `json:"latest,omitempty"`
	// Available is set when Latest is newer than the running build.
	Available bool      `json:"available"`
	CheckedAt time.Time `json:"checkedAt,omitzero"` // last successful check
	Error     string    `json:"error,omitempty"`    // last check failed
}

type updateState struct {
	mu        sync.Mutex
	latest    *update.Release
	found     string // channel latest was found on
	tried     string // channel of the last check
	checkedAt time.Time
	triedAt   time.Time
	err       string
}

// updateChannel is the configured channel; stable where no test builds are made.
func (a *App) updateChannel() string {
	if a.Config.Get().UpdateChannel == config.ChannelDev && update.DevBuilds {
		return config.ChannelDev
	}
	return config.ChannelStable
}

// UpdateStatus returns the result of the last check without checking again.
func (a *App) UpdateStatus() UpdateStatus {
	channel := a.updateChannel()
	a.updates.mu.Lock()
	defer a.updates.mu.Unlock()
	st := UpdateStatus{
		Current:   a.Version,
		Versioned: update.Versioned(a.Version),
		DevBuilds: update.DevBuilds,
		CheckedAt: a.updates.checkedAt,
		Error:     a.updates.err,
	}
	if a.updates.found == channel { // a result from the other channel is stale
		st.Latest = a.updates.latest
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
	channel := a.updateChannel()
	rel, err := a.latestOn(ctx, channel)

	a.updates.mu.Lock()
	a.updates.triedAt, a.updates.tried = time.Now(), channel
	if err != nil {
		a.updates.err = err.Error()
	} else {
		known := a.updates.latest != nil && a.updates.found == channel && a.updates.latest.Version == rel.Version
		a.updates.latest, a.updates.found, a.updates.checkedAt, a.updates.err = &rel, channel, a.updates.triedAt, ""
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

// latestOn returns the newest version on channel: the latest release, or on
// the dev channel whichever is newer of it and the newest test build.
func (a *App) latestOn(ctx context.Context, channel string) (update.Release, error) {
	rel, err := a.UpdateChecker.Latest(ctx, a.Version)
	if channel != config.ChannelDev {
		return rel, err
	}
	dev, devErr := a.UpdateChecker.Dev(ctx, a.Version)
	switch {
	case errors.Is(devErr, update.ErrNoDevBuild):
		return rel, err
	case devErr != nil && err != nil:
		return rel, errors.Join(err, devErr)
	case devErr != nil:
		a.Log.Warn("cannot read the latest test build", "err", devErr)
		return rel, nil
	case err != nil:
		return dev, nil
	}
	if newer, _ := update.Newer(dev.Version, rel.Version); newer {
		return dev, nil
	}
	return rel, nil
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
	channel := a.updateChannel()
	a.updates.mu.Lock()
	defer a.updates.mu.Unlock()
	if a.updates.triedAt.IsZero() || a.updates.tried != channel {
		return true
	}
	wait := updateEvery
	if a.updates.err != "" {
		wait = updateRetry
	}
	return now.Sub(a.updates.triedAt) >= wait
}
