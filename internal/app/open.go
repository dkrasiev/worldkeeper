package app

import (
	"errors"
	"os/exec"

	"github.com/dkrasiev/worldkeeper/internal/shell"
)

// ErrNotPlayable means a world cannot be started from Worldkeeper. Only
// worlds in a Prism Launcher instance can, with Prism installed: the
// official launcher has no way to start a given world.
var ErrNotPlayable = errors.New("this world can only be started from its launcher")

// OpenFolder shows the world's folder in the file manager.
func (a *App) OpenFolder(id string) error {
	w, err := a.find(id)
	if err != nil {
		return err
	}
	return shell.OpenFolder(w.Path)
}

// Play starts the world in Prism Launcher: it launches the world's
// instance and joins the world.
func (a *App) Play(id string) error {
	w, err := a.find(id)
	if err != nil {
		return err
	}
	cmd, ok := shell.PrismCommand(w.SavesDir, w.Folder)
	if !ok {
		return ErrNotPlayable
	}
	if err := a.checkInUse(w, false); err != nil {
		return err
	}
	return shell.Start(exec.Command(cmd[0], cmd[1:]...))
}
