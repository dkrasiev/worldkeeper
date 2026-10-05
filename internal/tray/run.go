//go:build !darwin || cgo

package tray

import (
	"context"
	"log/slog"
	"time"

	"fyne.io/systray"
)

// Available reports whether this build can show a tray icon.
const Available = true

// refreshEvery is how often the status line and icon are updated.
const refreshEvery = 15 * time.Second

// Run shows the tray icon and blocks until the user picks Quit or ctx is
// cancelled. On macOS it must be called from the main goroutine.
func Run(ctx context.Context, c Controller, locale string, log *slog.Logger) error {
	normal, warning := icons()

	onReady := func() {
		systray.SetIcon(normal)
		systray.SetTooltip("Worldkeeper")

		status := systray.AddMenuItem("", "")
		status.Disable()
		systray.AddSeparator()
		open := systray.AddMenuItem(text(locale, "open"), "")
		backup := systray.AddMenuItem(text(locale, "backupNow"), "")
		auto := systray.AddMenuItemCheckbox(text(locale, "auto"), "", c.AutoBackup())
		systray.AddSeparator()
		quit := systray.AddMenuItem(text(locale, "quit"), "")

		failing := false
		refresh := func() {
			s := c.Status()
			line := statusLine(locale, s, time.Now())
			status.SetTitle(line)
			systray.SetTooltip("Worldkeeper — " + line)
			if s.Failing() != failing {
				failing = s.Failing()
				if failing {
					systray.SetIcon(warning)
				} else {
					systray.SetIcon(normal)
				}
			}
			if c.AutoBackup() != auto.Checked() {
				if c.AutoBackup() {
					auto.Check()
				} else {
					auto.Uncheck()
				}
			}
		}
		refresh()

		go func() {
			ticker := time.NewTicker(refreshEvery)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					systray.Quit()
					return
				case <-ticker.C:
					refresh()
				case <-open.ClickedCh:
					c.OpenUI()
				case <-backup.ClickedCh:
					backup.Disable()
					backup.SetTitle(text(locale, "backingUp"))
					go func() {
						r := c.BackupNow()
						status.SetTitle(resultLine(locale, r))
						backup.SetTitle(text(locale, "backupNow"))
						backup.Enable()
					}()
				case <-auto.ClickedCh:
					if err := c.SetAutoBackup(!auto.Checked()); err != nil {
						log.Warn("toggle auto backup", "err", err)
					}
					refresh()
				case <-quit.ClickedCh:
					systray.Quit()
					return
				}
			}
		}()
	}

	systray.Run(onReady, func() {})
	return nil
}
