// Package notify shows desktop notifications: a toast on Windows, a
// Notification Center banner on macOS and the freedesktop notification
// service (D-Bus) elsewhere. None of them needs cgo.
package notify

import (
	"fmt"
	"log/slog"
	"strings"
)

// Desktop announces failed and recovered backups in the user's language.
// It implements app.Notifier.
type Desktop struct {
	// Locale returns the language to write in ("ru-RU", "en"). It is asked
	// for every notification, so a language change applies to the next one.
	Locale func() string
	Log    *slog.Logger

	// show displays a notification; nil means the system's own.
	show func(title, body string) error
}

func (d *Desktop) BackupFailed(world, err string) {
	d.send(text(d.Locale(), "failed", world), err)
}

func (d *Desktop) BackupRecovered(world string) {
	l := d.Locale()
	d.send(text(l, "recovered"), text(l, "recoveredBody", world))
}

// send shows the notification in the background: starting PowerShell or
// osascript takes a moment, and the backup should not wait for it.
func (d *Desktop) send(title, body string) {
	show := d.show
	if show == nil {
		show = native
	}
	go func() {
		if err := show(title, body); err != nil {
			d.Log.Warn("cannot show notification", "err", err)
		}
	}()
}

var messages = map[string]map[string]string{
	"en": {
		"failed":        "Backup of “%s” failed",
		"recovered":     "Backups work again",
		"recoveredBody": "“%s” is backed up.",
	},
	"ru": {
		"failed":        "Бэкап мира «%s» не удался",
		"recovered":     "Бэкапы снова работают",
		"recoveredBody": "Мир «%s» сохранён.",
	},
}

// text returns the message for the locale's language, falling back to English.
func text(locale, key string, args ...any) string {
	lang := strings.ToLower(locale)
	if i := strings.IndexAny(lang, "-_"); i >= 0 {
		lang = lang[:i]
	}
	m, ok := messages[lang]
	if !ok {
		m = messages["en"]
	}
	if len(args) == 0 {
		return m[key]
	}
	return fmt.Sprintf(m[key], args...)
}
