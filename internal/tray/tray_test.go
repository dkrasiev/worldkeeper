package tray

import (
	"bytes"
	"image/png"
	"testing"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/icon"
)

func TestStatusLine(t *testing.T) {
	now := time.Date(2026, 10, 5, 20, 0, 0, 0, time.Local)
	cases := []struct {
		locale string
		s      Status
		want   string
	}{
		{"en", Status{}, "No backups yet"},
		{"ru_RU", Status{LastBackup: now.Add(-time.Hour)}, "Последний бэкап: 19:00"},
		{"en-US", Status{LastBackup: now.AddDate(0, 0, -2)}, "Last backup: 2026-10-03 20:00"},
		{"ru-RU", Status{FailingWorld: "Мой мир", Error: "NAS offline"}, "⚠ Бэкап не удался: Мой мир"},
		{"de", Status{Error: "repository locked"}, "⚠ Backup storage is unavailable"}, // unknown language -> English
		{"ru", Status{}, "Бэкапов пока нет"},                                          // as the web UI sends it
		{"", Status{}, "No backups yet"},
	}
	for _, c := range cases {
		if got := statusLine(c.locale, c.s, now); got != c.want {
			t.Errorf("statusLine(%q, %+v) = %q, want %q", c.locale, c.s, got, c.want)
		}
	}
	if got := resultLine("ru", BackupResult{Saved: 2, Skipped: 1, Failed: 1}); got != "Готово: сохранено 2, пропущено 1, ошибок 1" {
		t.Errorf("resultLine = %q", got)
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

func TestIcons(t *testing.T) {
	base := icon.Block()
	img, err := png.Decode(bytes.NewReader(encodePNG(icon.Scale(withAlert(base), 4))))
	if err != nil || img.Bounds().Dx() != 64 {
		t.Fatalf("png: %v %v", err, img.Bounds())
	}
	if r, g, b, _ := img.At(48, 48).RGBA(); r>>8 != 0xe5 || g>>8 != 0x39 || b>>8 != 0x35 {
		t.Errorf("warning dot missing at (48,48): %x %x %x", r>>8, g>>8, b>>8)
	}
}
