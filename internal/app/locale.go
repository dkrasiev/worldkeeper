package app

import (
	"regexp"
	"sync"

	"github.com/dkrasiev/worldkeeper/internal/config"
)

// languageTag accepts the tags the web UI sends ("en", "ru", "pt-BR").
var languageTag = regexp.MustCompile(`^[a-zA-Z]{2,3}([-_][a-zA-Z0-9]{1,8})*$`)

// ValidLanguage reports whether lang can be saved with SetLanguage; ""
// means "follow the OS language".
func ValidLanguage(lang string) bool {
	return lang == "" || (len(lang) <= 35 && languageTag.MatchString(lang))
}

type localeState struct {
	mu      sync.Mutex
	changed chan struct{} // closed and replaced when the language changes
}

// Locale is the language for text the server shows itself, such as the
// tray menu and desktop notifications: the one picked in the web UI, or
// SystemLocale until one is picked.
func (a *App) Locale() string {
	return resolveLocale(a.Config.Get().Language, a.SystemLocale)
}

func resolveLocale(picked, system string) string {
	switch {
	case picked != "":
		return picked
	case system != "":
		return system
	default:
		return "en"
	}
}

// SetLanguage saves the language picked in the web UI; "" goes back to the
// OS language. Listeners of LocaleChanged are woken when Locale changes.
func (a *App) SetLanguage(lang string) error {
	if a.Config.Get().Language == lang {
		return nil // the UI repeats its choice on every load
	}
	before := a.Locale()
	if _, err := a.Config.Update(func(c *config.Config) error { c.Language = lang; return nil }); err != nil {
		return err
	}
	if a.Locale() != before {
		a.locale.mu.Lock()
		if a.locale.changed != nil {
			close(a.locale.changed)
			a.locale.changed = nil
		}
		a.locale.mu.Unlock()
	}
	return nil
}

// LocaleChanged returns a channel that is closed the next time Locale
// changes. Call it again after each change, before reading Locale.
func (a *App) LocaleChanged() <-chan struct{} {
	a.locale.mu.Lock()
	defer a.locale.mu.Unlock()
	if a.locale.changed == nil {
		a.locale.changed = make(chan struct{})
	}
	return a.locale.changed
}
