package tray

import (
	"os"
	"strings"
)

// Locale returns the OS language as a BCP 47-ish tag ("ru-RU", "en").
// POSIX environment variables win when set; otherwise the OS setting is used.
// It is the default; a language picked in the web UI replaces it (see
// Controller.Locale).
func Locale() string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(name); v != "" && v != "C" && v != "POSIX" && !strings.HasPrefix(v, "C.") {
			return strings.SplitN(v, ".", 2)[0] // ru_RU.UTF-8 -> ru_RU
		}
	}
	if l := osLocale(); l != "" {
		return l
	}
	return "en"
}
