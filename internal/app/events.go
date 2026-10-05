package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

type EventKind string

const (
	EventBackup  EventKind = "backup"
	EventRestore EventKind = "restore"
	EventError   EventKind = "error"
	EventWorld   EventKind = "world" // renamed or deleted
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

// maxEvents bounds the feed in memory and on disk (a few dozen KB).
const maxEvents = 100

// Event codes, translated by the UI.
const (
	CodeBackupSaved   = "backup_saved"   // params: kind, snapshot
	CodeBackupFailed  = "backup_failed"  // params: error
	CodeRestoreDone   = "restore_done"   // params: snapshot, path
	CodeRestoreFailed = "restore_failed" // params: error
	CodeWorldRenamed  = "world_renamed"  // params: from, to
	CodeWorldDeleted  = "world_deleted"  // params: path
	CodeDeleteFailed  = "delete_failed"  // params: error
)

func (a *App) event(kind EventKind, worldID, world, code string, params map[string]string, msg string) {
	e := Event{Time: time.Now().UTC(), Kind: kind, WorldID: worldID, World: world, Message: msg, Code: code, Params: params}
	a.eventsMu.Lock()
	defer a.eventsMu.Unlock()

	// A retried failure repeats the same error every poll: refresh the
	// existing entry instead of flooding the feed and the log.
	// Only the time changes, so the file is not rewritten: a failing NAS
	// would otherwise cause a disk write on every poll.
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
	if a.eventsPath != "" {
		if err := writeEvents(a.eventsPath, a.events); err != nil {
			a.Log.Warn("cannot save activity log", "path", a.eventsPath, "err", err)
		}
	}
}

// OpenEvents loads the activity feed saved at path and saves every new
// event there, so failures from before a restart stay visible in the feed
// and the tray. A missing file is an empty feed. On a read error the feed
// starts empty and the file is replaced on the next event.
func (a *App) OpenEvents(path string) error {
	a.eventsMu.Lock()
	defer a.eventsMu.Unlock()
	a.eventsPath = path
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var events []Event
	if err := json.Unmarshal(b, &events); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if len(events) > maxEvents {
		events = events[:maxEvents]
	}
	a.events = events
	return nil
}

func writeEvents(path string, events []Event) error {
	b, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Events returns recent activity, newest first.
func (a *App) Events() []Event {
	a.eventsMu.Lock()
	defer a.eventsMu.Unlock()
	return append([]Event{}, a.events...)
}
