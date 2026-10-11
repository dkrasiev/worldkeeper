package snapshot

import (
	"errors"
	"io"
	"io/fs"
)

// Backend stores and restores world snapshots. Store (zip files) and
// restic.Store (a restic repository) implement it.
type Backend interface {
	// Create snapshots worldDir.
	Create(ref WorldRef, worldDir string, m Meta) (Snapshot, error)
	// Prune deletes the automatic snapshots that r does not keep and returns
	// their ids. Only kinds for which Kind.Rotates is true are pruned, and an
	// empty r prunes nothing.
	Prune(worldID string, r Retention) ([]string, error)
	Delete(worldID, snapID string) error
	// Get returns one world's snapshots, newest first.
	Get(worldID string) (Index, error)
	// All returns every world that has snapshots.
	All() ([]Index, error)
	// Extract unpacks a snapshot into dest, which must not exist yet.
	// progress may be nil.
	Extract(worldID, snapID, dest string, progress Progress) error
	// Open exposes a snapshot as a read-only file system rooted at the world
	// folder, for reading world info without restoring it. Close the closer
	// when done.
	Open(worldID, snapID string) (fs.FS, io.Closer, error)
	// Rekey files a world's snapshots under another id and records where
	// the world is now, after it was moved to another location. A world id
	// is derived from the location, so without this the snapshots would be
	// left behind. It fails with ErrIDTaken when to.ID already has
	// snapshots: they belong to another world.
	Rekey(from string, to WorldRef) error
}

// ErrIDTaken is returned by Rekey when the new id has snapshots of its own.
var ErrIDTaken = errors.New("another world's backups use this id")

// Progress reports how many bytes of a backup or restore are done so far.
// total is 0 while it is not known yet. Implementations call it often, so
// it must be cheap.
type Progress func(done, total int64)

// Report calls p if it is set.
func (p Progress) Report(done, total int64) {
	if p != nil {
		p(done, total)
	}
}

var _ Backend = (*Store)(nil)
