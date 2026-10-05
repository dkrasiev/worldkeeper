//go:build windows

package lock

import (
	"errors"
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// isLocked reads the first byte of session.lock. Java's tryLock takes a
// mandatory exclusive lock on Windows, so reading a locked byte fails with
// ERROR_LOCK_VIOLATION. Reading never takes a lock itself, so the probe can
// never make the game fail to open the world.
func isLocked(f *os.File) (bool, error) {
	buf := make([]byte, 1)
	_, err := f.ReadAt(buf, 0)
	switch {
	case err == nil:
		return false, nil
	case errors.Is(err, windows.ERROR_LOCK_VIOLATION):
		return true, nil
	case errors.Is(err, io.EOF):
		// An empty file has no byte to read; Minecraft always writes one
		// before locking, but fall back to a momentary lock attempt.
		return tryLock(f)
	default:
		return false, err
	}
}

// tryLock takes a 1-byte lock and releases it immediately. It has a tiny
// window in which the game could fail to lock the file, so it is only the
// fallback for empty lock files.
func tryLock(f *os.File) (bool, error) {
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
