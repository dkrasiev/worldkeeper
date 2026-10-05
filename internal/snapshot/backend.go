package snapshot

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
}

var _ Backend = (*Store)(nil)
