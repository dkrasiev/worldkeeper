//go:build windows

package lock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// isLocked tries to take a 1-byte lock and releases it immediately.
// Windows has no way to query a lock, so there is a microsecond window in
// which the game could fail to lock the file if it opens the world exactly
// at that moment. The watcher polls rarely, so this is acceptable.
func isLocked(f *os.File) (bool, error) {
	h := windows.Handle(f.Fd())
	ol := new(windows.Overlapped)
	err := windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, windows.UnlockFileEx(h, 0, 1, 0, ol)
}
