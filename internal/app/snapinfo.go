package app

import (
	"sync"

	"github.com/dkrasiev/worldkeeper/internal/worldinfo"
)

// SnapshotDetails is what a snapshot contained, read from the backup itself.
type SnapshotDetails struct {
	Info         worldinfo.Info `json:"info"`
	Advancements map[string]any `json:"-"`
}

// snapshotCache keeps parsed snapshots: they never change once written, and
// opening a restic snapshot runs restic twice.
type snapshotCache struct {
	mu sync.Mutex
	m  map[string]SnapshotDetails
}

const snapshotCacheSize = 64

func (c *snapshotCache) get(key string) (SnapshotDetails, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	d, ok := c.m[key]
	return d, ok
}

func (c *snapshotCache) put(key string, d SnapshotDetails) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil || len(c.m) >= snapshotCacheSize {
		c.m = map[string]SnapshotDetails{} // simple reset; entries are cheap to rebuild
	}
	c.m[key] = d
}

// SnapshotDetails reads world info and advancement progress from a snapshot
// without restoring it.
func (a *App) SnapshotDetails(worldID, snapID string) (SnapshotDetails, error) {
	store := a.Snapshots()
	key := a.Config.Get().Engine + "\x00" + worldID + "\x00" + snapID
	if d, ok := a.snapCache.get(key); ok {
		return d, nil
	}
	ix, err := store.Get(worldID)
	if err != nil {
		return SnapshotDetails{}, err
	}
	fsys, closer, err := store.Open(worldID, snapID)
	if err != nil {
		return SnapshotDetails{}, err
	}
	defer closer.Close()

	folder := ix.World.Folder
	if folder == "" {
		folder = worldID
	}
	info, err := worldinfo.ReadFS(fsys, folder)
	if err != nil {
		return SnapshotDetails{}, err
	}
	d := SnapshotDetails{Info: info, Advancements: worldinfo.AdvancementProgressFS(fsys)}
	a.snapCache.put(key, d)
	return d, nil
}
