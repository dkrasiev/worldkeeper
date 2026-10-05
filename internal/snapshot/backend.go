package snapshot

import (
	"io"
	"io/fs"
)

// Backend stores and restores world snapshots. Store (zip files) and
// restic.Store (a restic repository) implement it.
type Backend interface {
	// Create snapshots worldDir.
	Create(ref WorldRef, worldDir string, m Meta) (Snapshot, error)
	// Prune deletes the oldest automatic snapshots beyond keep and returns their ids.
	// Manual snapshots are never pruned.
	Prune(worldID string, keep int) ([]string, error)
	Delete(worldID, snapID string) error
	// Get returns one world's snapshots, newest first.
	Get(worldID string) (Index, error)
	// All returns every world that has snapshots.
	All() ([]Index, error)
	// Extract unpacks a snapshot into dest, which must not exist yet.
	Extract(worldID, snapID, dest string) error
	// Open exposes a snapshot as a read-only file system rooted at the world
	// folder, for reading world info without restoring it. Close the closer
	// when done.
	Open(worldID, snapID string) (fs.FS, io.Closer, error)
}

var _ Backend = (*Store)(nil)
