package app

import "time"

type EventKind string

const (
	EventBackup  EventKind = "backup"
	EventRestore EventKind = "restore"
	EventError   EventKind = "error"
)

// Event is a line in the activity feed. Failures must be visible:
// a backup tool that fails silently is worse than none.
type Event struct {
	Time    time.Time `json:"time"`
	Kind    EventKind `json:"kind"`
	WorldID string    `json:"worldId"`
	World   string    `json:"world"`
	Message string    `json:"message"`
}

const maxEvents = 100

func (a *App) event(kind EventKind, worldID, world, msg string) {
	e := Event{Time: time.Now().UTC(), Kind: kind, WorldID: worldID, World: world, Message: msg}
	a.eventsMu.Lock()
	defer a.eventsMu.Unlock()

	// A retried failure repeats the same error every poll: refresh the
	// existing entry instead of flooding the feed and the log.
	for i, prev := range a.events {
		if prev.WorldID != worldID {
			continue
		}
		if prev.Kind == kind && prev.Message == msg && kind == EventError {
			a.events[i].Time = e.Time
			return
		}
		break
	}

	if kind == EventError {
		a.Log.Error(msg, "world", worldID)
	} else {
		a.Log.Info(msg, "world", worldID)
	}
	a.events = append([]Event{e}, a.events...)
	if len(a.events) > maxEvents {
		a.events = a.events[:maxEvents]
	}
}

// Events returns recent activity, newest first.
func (a *App) Events() []Event {
	a.eventsMu.Lock()
	defer a.eventsMu.Unlock()
	return append([]Event{}, a.events...)
}
