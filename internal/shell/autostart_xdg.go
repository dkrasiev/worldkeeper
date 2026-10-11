//go:build !windows && !darwin

package shell

import (
	"os"
	"path/filepath"
)

func platformEntry() entry {
	dir, _ := os.UserConfigDir() // $XDG_CONFIG_HOME or ~/.config
	return &desktopEntry{path: filepath.Join(dir, "autostart", AppID+".desktop")}
}

// desktopEntry is an XDG autostart file, started by the desktop at login.
type desktopEntry struct{ path string }

func (e *desktopEntry) read() ([]string, error) { return readFile(e.path, parseDesktopEntry) }

func (e *desktopEntry) write(cmd []string) error {
	return writeFile(e.path, desktopFile(AppName, cmd))
}

func (e *desktopEntry) remove() error { return removeFile(e.path) }
