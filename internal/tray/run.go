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
func Run(ctx context.Context, c Controller, log *slog.Logger) error {
	normal, warning := icons()

	onReady := func() {
		systray.SetIcon(normal)
		systray.SetTooltip("Worldkeeper")

		// Titles are set by retitle, in the current language.
		status := systray.AddMenuItem("", "")
		status.Disable()
		systray.AddSeparator()
		open := systray.AddMenuItem("", "")
		backup := systray.AddMenuItem("", "")
		auto := systray.AddMenuItemCheckbox("", "", c.AutoBackup())
		systray.AddSeparator()
		about := systray.AddMenuItem("Worldkeeper "+c.Version(), "")
		about.Disable()
		upd := systray.AddMenuItem("", "")
		upd.Hide() // until a newer release is found
		quit := systray.AddMenuItem("", "")

		// Menu state below is only touched by onReady and then the loop.
		localeChanged := c.LocaleChanged() // before Locale, so no change is missed
		locale := c.Locale()
		backingUp := false
		failing := false
		offered := "" // version the update item shows
		refresh := func() {
			if v := c.Update(); v != offered {
				offered = v
				if v != "" {
					upd.SetTitle(text(locale, "update", v))
					upd.Show()
				} else {
					upd.Hide()
				}
			}
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
		retitle := func() {
			open.SetTitle(text(locale, "open"))
			if backingUp {
				backup.SetTitle(text(locale, "backingUp"))
			} else {
				backup.SetTitle(text(locale, "backupNow"))
			}
			auto.SetTitle(text(locale, "auto"))
			if offered != "" {
				upd.SetTitle(text(locale, "update", offered))
			}
			quit.SetTitle(text(locale, "quit"))
			refresh()
		}
		retitle()

		backedUp := make(chan BackupResult, 1)
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
				case <-localeChanged:
					localeChanged = c.LocaleChanged()
					locale = c.Locale()
					retitle()
				case <-open.ClickedCh:
					c.OpenUI()
				case <-backup.ClickedCh:
					backup.Disable()
					backingUp = true
					backup.SetTitle(text(locale, "backingUp"))
					go func() { backedUp <- c.BackupNow() }()
				case r := <-backedUp:
					backingUp = false
					status.SetTitle(resultLine(locale, r))
					backup.SetTitle(text(locale, "backupNow"))
					backup.Enable()
				case <-auto.ClickedCh:
					if err := c.SetAutoBackup(!auto.Checked()); err != nil {
						log.Warn("toggle auto backup", "err", err)
					}
					refresh()
				case <-upd.ClickedCh:
					c.OpenUpdate()
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
