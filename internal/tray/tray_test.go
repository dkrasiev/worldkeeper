package tray

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"testing"
	"time"
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
	base := block()
	img, err := png.Decode(bytes.NewReader(encodePNG(scale(withAlert(base), 4))))
	if err != nil || img.Bounds().Dx() != 64 {
		t.Fatalf("png: %v %v", err, img.Bounds())
	}
	if r, g, b, _ := img.At(48, 48).RGBA(); r>>8 != 0xe5 || g>>8 != 0x39 || b>>8 != 0x35 {
		t.Errorf("warning dot missing at (48,48): %x %x %x", r>>8, g>>8, b>>8)
	}

	pngData := encodePNG(scale(base, 2))
	ico := encodeICO(pngData, 32)
	var hdr struct {
		Reserved, Type, Count uint16
		W, H, Colors, Res     uint8
		Planes, BPP           uint16
		Size, Offset          uint32
	}
	if err := binary.Read(bytes.NewReader(ico), binary.LittleEndian, &hdr); err != nil {
		t.Fatal(err)
	}
	if hdr.Type != 1 || hdr.Count != 1 || hdr.W != 32 || int(hdr.Size) != len(pngData) || hdr.Offset != 22 {
		t.Errorf("ico header = %+v", hdr)
	}
	if !bytes.Equal(ico[22:], pngData) {
		t.Error("ico payload is not the png")
	}
}
