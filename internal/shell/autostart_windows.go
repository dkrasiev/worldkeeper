package shell

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
	// approvedKey is where Task Manager's Startup apps page records that the
	// user turned a Run entry off; the Run value itself stays.
	approvedKey = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`
)

func platformEntry() entry {
	startup, _ := windows.KnownFolderPath(windows.FOLDERID_Startup, 0)
	return &runEntry{run: runKey, approved: approvedKey, name: AppName, startup: startup}
}

// runEntry is a value under HKCU\...\Run holding the command line.
type runEntry struct {
	run, approved, name string // key paths under HKEY_CURRENT_USER, value name
	// startup is the Startup folder, where the README used to tell users to
	// put a shortcut by hand. Such a shortcut counts as the entry, so it is
	// not doubled by a Run value and turning autostart off removes it.
	startup string
}

func (e *runEntry) read() ([]string, error) {
	cmd, err := e.readRun()
	if cmd != nil || err != nil {
		return cmd, err
	}
	if links := e.shortcuts(); len(links) > 0 {
		// The shortcut's target is not parsed; the shortcut itself exists,
		// so Refresh leaves it alone.
		return []string{links[0]}, nil
	}
	return nil, nil
}

func (e *runEntry) readRun() ([]string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, e.run, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer k.Close()
	line, _, err := k.GetStringValue(e.name)
	if errors.Is(err, registry.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if e.turnedOff() {
		return nil, nil
	}
	cmd, err := windows.DecomposeCommandLine(line)
	if err != nil || len(cmd) == 0 {
		return []string{""}, nil // there is an entry, but not one we can follow
	}
	return cmd, nil
}

// turnedOff reports whether the user disabled the entry in Task Manager:
// the first byte of its StartupApproved value is odd (2 on, 3 off).
func (e *runEntry) turnedOff() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, e.approved, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetBinaryValue(e.name)
	return err == nil && len(v) > 0 && v[0]&1 == 1
}

// shortcuts lists hand-made Startup shortcuts: Worldkeeper.lnk, or
// "worldkeeper.exe - Shortcut.lnk" as Explorer names them.
func (e *runEntry) shortcuts() []string {
	if e.startup == "" {
		return nil
	}
	links, _ := filepath.Glob(filepath.Join(e.startup, "*.lnk"))
	var ours []string
	for _, l := range links {
		if strings.HasPrefix(strings.ToLower(filepath.Base(l)), "worldkeeper") {
			ours = append(ours, l)
		}
	}
	return ours
}

func (e *runEntry) write(cmd []string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, e.run, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.SetStringValue(e.name, windows.ComposeCommandLine(cmd)); err != nil {
		return err
	}
	if err := e.forgetApproval(); err != nil {
		return err
	}
	return e.removeShortcuts() // the Run value replaces them
}

func (e *runEntry) remove() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, e.run, registry.SET_VALUE)
	if err == nil {
		err = k.DeleteValue(e.name)
		k.Close()
	}
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	if err := e.forgetApproval(); err != nil {
		return err
	}
	return e.removeShortcuts()
}

func (e *runEntry) removeShortcuts() error {
	for _, l := range e.shortcuts() {
		if err := os.Remove(l); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// forgetApproval drops Task Manager's on/off record, so a newly written
// entry starts out on.
func (e *runEntry) forgetApproval() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, e.approved, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.DeleteValue(e.name); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}
