package tray

import (
	"os/exec"
	"strings"
)

// Apps started from Finder or at login get no LANG, so ask the system.
func osLocale() string {
	out, err := exec.Command("defaults", "read", "-g", "AppleLocale").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out)) // e.g. "ru_RU"
}
