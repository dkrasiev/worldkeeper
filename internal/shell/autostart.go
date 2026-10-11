// Package shell integrates Worldkeeper with the desktop: starting it when
// the user logs in (a Run registry value on Windows, a LaunchAgent on macOS,
// an XDG autostart entry elsewhere), and its Start menu shortcut on
// Windows. The OS keeps the state, so the user can also change it in Task
// Manager, System Settings, the desktop's autostart settings or the Start
// menu folder, and Worldkeeper reads it back from there.
package shell

import (
	"errors"
	"os"
)

const (
	// AppName names the Run registry value, the autostart entry and the
	// Start menu shortcut.
	AppName = "Worldkeeper"
	// AppID is the reverse-DNS identifier: the LaunchAgent label and the
	// .desktop file name.
	AppID = "io.github.dkrasiev.worldkeeper"
	// AppUserModelID identifies Worldkeeper to Windows: its notifications
	// are sent under it (internal/notify), and the Start menu shortcut
	// carries it, so the two are shown as one app.
	AppUserModelID = "dkrasiev.Worldkeeper"
	// comment describes Worldkeeper in shortcuts and desktop entries.
	comment = "Automatic Minecraft world backups"
)

// entry is the platform's autostart record.
type entry interface {
	// read returns the command the entry starts, or nil when there is no
	// entry or the user turned it off.
	read() ([]string, error)
	write(cmd []string) error
	remove() error
}

// Autostart starts a command when the user logs in.
type Autostart struct {
	// Command is the executable and its arguments.
	Command []string
	entry   entry
}

// NewAutostart returns the autostart entry for this OS; cmd[0] must be an
// absolute path, usually os.Executable.
func NewAutostart(cmd []string) *Autostart {
	return &Autostart{Command: cmd, entry: platformEntry()}
}

// Enabled reports whether the entry exists and is turned on.
func (a *Autostart) Enabled() (bool, error) {
	cmd, err := a.entry.read()
	return cmd != nil, err
}

// Set creates the entry, pointing at Command, or removes it.
func (a *Autostart) Set(on bool) error {
	if !on {
		return a.entry.remove()
	}
	if len(a.Command) == 0 {
		return errors.New("autostart: no command")
	}
	return a.entry.write(a.Command)
}

// Refresh points an enabled entry at Command when the program it starts is
// gone, e.g. after the user moved worldkeeper to another folder. An entry
// for another copy that still exists is left alone, so running a second
// copy (or go run) does not take autostart over. It reports whether the
// entry was rewritten.
func (a *Autostart) Refresh() (bool, error) {
	cmd, err := a.entry.read()
	if err != nil || cmd == nil || len(a.Command) == 0 {
		return false, err
	}
	if _, err := os.Stat(cmd[0]); !errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return true, a.entry.write(a.Command)
}
