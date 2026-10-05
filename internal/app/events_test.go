package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestEventsSurviveRestart(t *testing.T) {
	f := setup(t)
	path := filepath.Join(t.TempDir(), "events.json")
	if err := f.app.OpenEvents(path); err != nil {
		t.Fatal(err)
	}
	f.app.event(EventBackup, "minecraft--a", "a", CodeBackupSaved, map[string]string{"kind": "auto", "snapshot": "s1"}, "Saved")
	f.app.event(EventError, "minecraft--b", "b", CodeBackupFailed, map[string]string{"error": "NAS offline"}, "Backup failed: NAS offline")

	restarted := setup(t).app
	if err := restarted.OpenEvents(path); err != nil {
		t.Fatal(err)
	}
	ev := restarted.Events()
	if len(ev) != 2 || ev[0].Code != CodeBackupFailed || ev[0].Params["error"] != "NAS offline" || ev[1].Params["snapshot"] != "s1" {
		t.Fatalf("events after restart = %+v", ev)
	}
	// The tray keeps warning about the failure after a restart.
	if h := restarted.Health(); h.FailingWorld != "b" {
		t.Errorf("health after restart = %+v", h)
	}
}

func TestEventsFileIsBounded(t *testing.T) {
	f := setup(t)
	path := filepath.Join(t.TempDir(), "events.json")
	f.app.OpenEvents(path)
	for i := range maxEvents + 20 {
		f.app.event(EventBackup, "w", "w", CodeBackupSaved, nil, fmt.Sprint("Saved ", i))
	}
	restarted := setup(t).app
	restarted.OpenEvents(path)
	if ev := restarted.Events(); len(ev) != maxEvents || ev[0].Message != fmt.Sprint("Saved ", maxEvents+19) {
		t.Fatalf("got %d events, newest %q", len(ev), ev[0].Message)
	}
}

func TestEventsCorruptFileStartsOver(t *testing.T) {
	f := setup(t)
	path := filepath.Join(t.TempDir(), "events.json")
	os.WriteFile(path, []byte("{not json"), 0o644)
	if err := f.app.OpenEvents(path); err == nil {
		t.Error("corrupt file accepted")
	}
	f.app.event(EventBackup, "w", "w", CodeBackupSaved, nil, "Saved")
	restarted := setup(t).app
	if err := restarted.OpenEvents(path); err != nil || len(restarted.Events()) != 1 {
		t.Fatalf("err %v, events %+v", err, restarted.Events())
	}
}
