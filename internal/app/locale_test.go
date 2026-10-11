package app

import (
	"strings"
	"testing"
)

func TestResolveLocale(t *testing.T) {
	cases := []struct{ picked, system, want string }{
		{"", "ru-RU", "ru-RU"}, // nothing picked: the OS language
		{"en", "ru-RU", "en"},  // the web UI's choice wins
		{"ru", "en_US", "ru"},
		{"", "", "en"},
	}
	for _, c := range cases {
		if got := resolveLocale(c.picked, c.system); got != c.want {
			t.Errorf("resolveLocale(%q, %q) = %q, want %q", c.picked, c.system, got, c.want)
		}
	}
}

func TestSetLanguage(t *testing.T) {
	f := setup(t)
	f.app.SystemLocale = "ru-RU"
	if got := f.app.Locale(); got != "ru-RU" {
		t.Fatalf("Locale before a pick = %q, want the OS language", got)
	}

	changed := f.app.LocaleChanged()
	if err := f.app.SetLanguage("en"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changed:
	default:
		t.Fatal("LocaleChanged not closed after the language changed")
	}
	if got := f.app.Locale(); got != "en" {
		t.Errorf("Locale = %q, want en", got)
	}
	if got := f.app.Config.Get().Language; got != "en" {
		t.Errorf("saved language = %q, want en", got)
	}

	// Repeating the choice changes nothing.
	changed = f.app.LocaleChanged()
	if err := f.app.SetLanguage("en"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changed:
		t.Error("LocaleChanged closed although the language stayed the same")
	default:
	}

	// "" goes back to the OS language.
	if err := f.app.SetLanguage(""); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changed:
	default:
		t.Error("LocaleChanged not closed after going back to the OS language")
	}
	if got := f.app.Locale(); got != "ru-RU" {
		t.Errorf("Locale after reset = %q, want ru-RU", got)
	}
}

func TestValidLanguage(t *testing.T) {
	for _, s := range []string{"", "en", "ru", "pt-BR", "zh-Hant-TW", "ru_RU"} {
		if !ValidLanguage(s) {
			t.Errorf("ValidLanguage(%q) = false", s)
		}
	}
	for _, s := range []string{"e", "english!", "ru-", "../etc", "ru RU", "en" + strings.Repeat("-abcdefgh", 4)} {
		if ValidLanguage(s) {
			t.Errorf("ValidLanguage(%q) = true", s)
		}
	}
}
