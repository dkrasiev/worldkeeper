// Package lock tells whether Minecraft currently has a world open.
//
// While a world is loaded, the game holds an OS-level lock on
// <world>/session.lock (Java FileChannel.tryLock: fcntl on Unix,
// LockFileEx on Windows). We probe that lock without disturbing it.
package lock

import (
	"errors"
	"os"
	"path/filepath"
)

// InUse reports whether the world in dir is open in a running game.
// A missing session.lock means the world is not in use.
func InUse(dir string) (bool, error) {
	f, err := os.Open(filepath.Join(dir, "session.lock"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	return isLocked(f)
}
