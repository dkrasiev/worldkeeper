package app

// Notifier shows desktop notifications about backups, so a failure is
// noticed even when neither the UI nor the tray menu is open. Calls must
// return quickly: they are made from the backup path.
type Notifier interface {
	// BackupFailed is called on the first failed backup of a world, not on
	// every retry.
	BackupFailed(world, err string)
	// BackupRecovered is called when a world that was failing is backed up again.
	BackupRecovered(world string)
}

// backupFailing reports whether the world's latest backup attempt in the
// activity feed failed. The feed survives restarts, so a failure announced
// before a restart is not announced again, and its recovery still is.
func (a *App) backupFailing(worldID string) bool {
	a.eventsMu.Lock()
	defer a.eventsMu.Unlock()
	for _, e := range a.events {
		if e.WorldID != worldID {
			continue
		}
		switch e.Code {
		case CodeBackupFailed:
			return true
		case CodeBackupSaved:
			return false
		}
	}
	return false
}

func (a *App) notifyFailed(world, err string) {
	if a.Notifier != nil && a.Config.Get().Notifications {
		a.Notifier.BackupFailed(world, err)
	}
}

func (a *App) notifyRecovered(world string) {
	if a.Notifier != nil && a.Config.Get().Notifications {
		a.Notifier.BackupRecovered(world)
	}
}
