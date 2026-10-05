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
	Message string    `json:"message"` // English, for logs
	// Code and Params let the UI render the event in the user's language.
	Code   string            `json:"code"`
	Params map[string]string `json:"params,omitempty"`
}

const maxEvents = 100

// Event codes, translated by the UI.
const (
	CodeBackupSaved   = "backup_saved"   // params: kind, snapshot
	CodeBackupFailed  = "backup_failed"  // params: error
	CodeRestoreDone   = "restore_done"   // params: snapshot, path
	CodeRestoreFailed = "restore_failed" // params: error
)

func (a *App) event(kind EventKind, worldID, world, code string, params map[string]string, msg string) {
	e := Event{Time: time.Now().UTC(), Kind: kind, WorldID: worldID, World: world, Message: msg, Code: code, Params: params}
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
