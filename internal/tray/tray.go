// Package tray shows Worldkeeper in the system tray (Windows), menu bar
// (macOS) or StatusNotifier area (Linux). The macOS tray needs cgo; builds
// without it report Available == false and run headless.
package tray

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrUnavailable is returned by Run on builds without tray support.
var ErrUnavailable = errors.New("system tray is not available in this build")

// Controller is what the tray menu acts on.
type Controller interface {
	OpenUI()
	// BackupNow backs up every world that changed since its last save.
	BackupNow() BackupResult
	AutoBackup() bool
	SetAutoBackup(on bool) error
	Status() Status
}

type BackupResult struct {
	Saved, Skipped, Failed int
}

// Status drives the tray icon, tooltip and status line.
type Status struct {
	LastBackup time.Time // zero: no backups yet
	// FailingWorld is set while a world's latest backup attempt failed or
	// storage cannot be read ("" world with Error set means storage).
	FailingWorld string
	Error        string
}

func (s Status) Failing() bool { return s.Error != "" }

var messages = map[string]map[string]string{
	"en": {
		"status.none":    "No backups yet",
		"status.last":    "Last backup: %s",
		"status.failed":  "⚠ Backup failed: %s",
		"status.storage": "⚠ Backup storage is unavailable",
		"open":           "Open Worldkeeper",
		"backupNow":      "Back up now",
		"backingUp":      "Backing up…",
		"result":         "Done: %d saved, %d skipped",
		"resultFailed":   "Done: %d saved, %d skipped, %d failed",
		"auto":           "Back up when a world is closed",
		"quit":           "Quit Worldkeeper",
	},
	"ru": {
		"status.none":    "Бэкапов пока нет",
		"status.last":    "Последний бэкап: %s",
		"status.failed":  "⚠ Бэкап не удался: %s",
		"status.storage": "⚠ Хранилище бэкапов недоступно",
		"open":           "Открыть Worldkeeper",
		"backupNow":      "Сделать бэкап сейчас",
		"backingUp":      "Делаем бэкап…",
		"result":         "Готово: сохранено %d, пропущено %d",
		"resultFailed":   "Готово: сохранено %d, пропущено %d, ошибок %d",
		"auto":           "Бэкап при выходе из мира",
		"quit":           "Выйти из Worldkeeper",
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

// statusLine is the first (disabled) menu item and the tooltip.
func statusLine(locale string, s Status, now time.Time) string {
	switch {
	case s.Failing() && s.FailingWorld != "":
		return text(locale, "status.failed", s.FailingWorld)
	case s.Failing():
		return text(locale, "status.storage")
	case s.LastBackup.IsZero():
		return text(locale, "status.none")
	default:
		return text(locale, "status.last", shortTime(s.LastBackup, now))
	}
}

func shortTime(t, now time.Time) string {
	t = t.Local()
	if y, m, d := t.Date(); y == now.Year() && m == now.Month() && d == now.Day() {
		return t.Format("15:04")
	}
	return t.Format("2006-01-02 15:04")
}

func resultLine(locale string, r BackupResult) string {
	if r.Failed > 0 {
		return text(locale, "resultFailed", r.Saved, r.Skipped, r.Failed)
	}
	return text(locale, "result", r.Saved, r.Skipped)
}
