//go:build unix

package lock

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// isLocked uses F_GETLK, which only queries the lock. Unlike a try-lock it
// can never make the game fail to open the world.
func isLocked(f *os.File) (bool, error) {
	lk := unix.Flock_t{Type: unix.F_WRLCK, Whence: io.SeekStart}
	if err := unix.FcntlFlock(f.Fd(), unix.F_GETLK, &lk); err != nil {
		return false, err
	}
	return lk.Type != unix.F_UNLCK, nil
}
