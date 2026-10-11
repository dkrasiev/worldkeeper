package shell

import (
	"errors"
	"os"
	"path/filepath"
)

// StartMenu is Worldkeeper's Start menu entry on Windows: a Worldkeeper.lnk
// shortcut in the user's Start menu Programs folder, which can be started,
// searched for and pinned like any app's. Other systems have no such entry
// (macOS lists Worldkeeper.app in Launchpad by itself), and Available is
// false there.
type StartMenu struct {
	// Command is the executable and its arguments.
	Command []string
	path    string // the shortcut; "" where there is no Start menu
}

// shortcut is what a .lnk file holds.
type shortcut struct {
	Target      string   // the program
	Args        []string // its arguments
	Dir         string   // working directory
	Description string   // the tooltip
	// AppID is the shortcut's AppUserModelID. Windows shows notifications
	// sent under it with the shortcut's name and icon.
	AppID string
}

// NewStartMenu returns the Start menu entry for this OS; cmd[0] must be an
// absolute path, usually os.Executable.
func NewStartMenu(cmd []string) *StartMenu {
	return &StartMenu{Command: cmd, path: startMenuPath()}
}

// Available reports whether the OS has a Start menu entry.
func (s *StartMenu) Available() bool { return s.path != "" }

// Exists reports whether the shortcut is there.
func (s *StartMenu) Exists() bool {
	if s.path == "" {
		return false
	}
	_, err := os.Stat(s.path)
	return err == nil
}

// Set creates the shortcut, pointing at Command, or removes it.
func (s *StartMenu) Set(on bool) error {
	switch {
	case s.path == "":
		return errors.ErrUnsupported
	case !on:
		return removeFile(s.path)
	case len(s.Command) == 0:
		return errors.New("start menu: no command")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	return writeShortcut(s.path, shortcut{
		Target:      s.Command[0],
		Args:        s.Command[1:],
		Dir:         filepath.Dir(s.Command[0]),
		Description: comment,
		AppID:       AppUserModelID,
	})
}

// Refresh points the shortcut at Command when the program it starts is
// gone, like Autostart.Refresh: a moved worldkeeper is followed, another
// copy that still exists keeps the shortcut. It reports whether the
// shortcut was rewritten.
func (s *StartMenu) Refresh() (bool, error) {
	if !s.Exists() || len(s.Command) == 0 {
		return false, nil
	}
	sc, err := readShortcut(s.path)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(sc.Target); sc.Target == "" || !errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return true, s.Set(true)
}
