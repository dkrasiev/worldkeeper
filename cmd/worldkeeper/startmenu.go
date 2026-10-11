package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/dkrasiev/worldkeeper/internal/config"
	"github.com/dkrasiev/worldkeeper/internal/shell"
)

// startMenu returns the Start menu shortcut (Windows only; nil when this
// executable cannot be found). The first start creates it, once: the config
// remembers that, so a shortcut the user removed is not brought back. A
// shortcut to a program that is gone is pointed at this one.
func startMenu(cfg *config.Store, cfgPath, defaultCfg string, log *slog.Logger) *shell.StartMenu {
	cmd, err := launchCommand(cfgPath, defaultCfg)
	if err != nil {
		log.Warn("start menu shortcut is unavailable", "err", err)
		return nil
	}
	sm := shell.NewStartMenu(cmd)
	switch {
	case !sm.Available():
	case cfg.Get().StartMenuOffered:
		if moved, err := sm.Refresh(); err != nil {
			log.Warn("cannot update the start menu shortcut", "err", err)
		} else if moved {
			log.Info("start menu shortcut now points to this copy", "exe", cmd[0])
		}
	// A copy with another config is a second one and leaves the Start menu
	// to the main one. A program in the temporary folder (go run, or an exe
	// opened inside the zip in Explorer) would leave a broken shortcut.
	case cfgPath != defaultCfg || inTempDir(cmd[0]):
	default:
		if err := sm.Set(true); err != nil {
			log.Warn("cannot create the start menu shortcut", "err", err)
			break // try again on the next start
		}
		if _, err := cfg.Update(func(c *config.Config) error { c.StartMenuOffered = true; return nil }); err != nil {
			log.Warn("cannot save the config", "err", err)
		}
		log.Info("added worldkeeper to the start menu")
	}
	return sm
}

// inTempDir reports whether path is inside the temporary folder.
func inTempDir(path string) bool {
	// Both resolved: TEMP can be a short 8.3 path (C:\Users\DMITRY~1\...).
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return false
	}
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(tmp, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
