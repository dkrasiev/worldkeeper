package notify

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestDesktopTexts(t *testing.T) {
	shown := make(chan [2]string, 1)
	locale := "ru_RU"
	d := &Desktop{
		Locale: func() string { return locale },
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		show:   func(title, body string) error { shown <- [2]string{title, body}; return nil },
	}
	next := func() [2]string {
		t.Helper()
		select {
		case n := <-shown:
			return n
		case <-time.After(5 * time.Second):
			t.Fatal("no notification")
			return [2]string{}
		}
	}

	d.BackupFailed("Мой мир", "NAS offline")
	if n := next(); n != [2]string{"Бэкап мира «Мой мир» не удался", "NAS offline"} {
		t.Errorf("failed = %q", n)
	}
	locale = "de-DE" // unknown language -> English
	d.BackupRecovered("Island")
	if n := next(); n != [2]string{"Backups work again", "“Island” is backed up."} {
		t.Errorf("recovered = %q", n)
	}
}

func TestMessagesComplete(t *testing.T) {
	for lang, m := range messages {
		for key := range messages["en"] {
			if m[key] == "" {
				t.Errorf("%s: missing %q", lang, key)
			}
		}
	}
}
