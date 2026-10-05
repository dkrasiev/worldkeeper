// Package snapshot stores world saves as plain zip files, so they can be
// restored with any archive tool even without worldkeeper.
//
// Layout:
//
//	<root>/<world-id>/<snapshot-id>.zip
//	<root>/<world-id>/index.json
package snapshot

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Kind string

const (
	KindAuto       Kind = "auto"
	KindManual     Kind = "manual"
	KindPreRestore Kind = "pre-restore" // taken automatically before a restore overwrites a world
)

// WorldRef remembers where a world came from, so its snapshots stay
// usable after the original folder is gone (e.g. after an OS reinstall).
type WorldRef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Folder      string `json:"folder"`
	Source      string `json:"source"`
	SourceLabel string `json:"sourceLabel"`
	Path        string `json:"path"`
}

type Snapshot struct {
	ID          string    `json:"id"`
	Kind        Kind      `json:"kind"`
	Label       string    `json:"label,omitempty"`
	Note        string    `json:"note,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	SizeBytes   int64     `json:"sizeBytes"`
	GameVersion string    `json:"gameVersion,omitempty"`
	LastPlayed  time.Time `json:"lastPlayed"`
}

type Index struct {
	World     WorldRef   `json:"world"`
	Snapshots []Snapshot `json:"snapshots"` // newest first
}

func (ix Index) Latest() *Snapshot {
	if len(ix.Snapshots) == 0 {
		return nil
	}
	return &ix.Snapshots[0]
}

var ErrNotFound = errors.New("snapshot not found")

// Store writes snapshots under a root directory that may change at runtime
// (the user can point it at another drive in settings).
type Store struct {
	root func() string
	mu   sync.Mutex // serializes all writes; backups are I/O bound anyway
	now  func() time.Time
}

func NewStore(root func() string) *Store {
	return &Store{root: root, now: time.Now}
}

// Meta describes a snapshot being created.
type Meta struct {
	Kind        Kind
	Label       string
	Note        string
	GameVersion string
	LastPlayed  time.Time
}

// Create archives worldDir. The zip is written to a temp file and renamed
// only when complete, so an interrupted backup never looks like a valid one.
func (s *Store) Create(ref WorldRef, worldDir string, m Meta) (Snapshot, error) {
	if err := checkName(ref.ID); err != nil {
		return Snapshot{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Join(s.root(), ref.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Snapshot{}, err
	}
	ix, err := s.readIndex(ref.ID)
	if err != nil {
		return Snapshot{}, err
	}

	now := s.now()
	id := uniqueID(ix, now.Format("20060102-150405")+"-"+string(m.Kind))
	final := filepath.Join(dir, id+".zip")
	tmp := final + ".partial"

	size, err := writeZipFile(tmp, worldDir)
	if err != nil {
		os.Remove(tmp)
		return Snapshot{}, fmt.Errorf("archive %s: %w", worldDir, err)
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return Snapshot{}, err
	}

	snap := Snapshot{
		ID:          id,
		Kind:        m.Kind,
		Label:       m.Label,
		Note:        m.Note,
		CreatedAt:   now.UTC(),
		SizeBytes:   size,
		GameVersion: m.GameVersion,
		LastPlayed:  m.LastPlayed,
	}
	ix.World = ref
	ix.Snapshots = append([]Snapshot{snap}, ix.Snapshots...)
	if err := s.writeIndex(ix); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

func writeZipFile(path, worldDir string) (int64, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, err
	}
	bw := bufio.NewWriterSize(f, 1<<20)
	if err := writeZip(bw, worldDir); err != nil {
		f.Close()
		return 0, err
	}
	if err := bw.Flush(); err != nil {
		f.Close()
		return 0, err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return 0, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return 0, err
	}
	return fi.Size(), f.Close()
}

func uniqueID(ix Index, base string) string {
	taken := map[string]bool{}
	for _, s := range ix.Snapshots {
		taken[s.ID] = true
	}
	id := base
	for i := 2; taken[id]; i++ {
		id = fmt.Sprintf("%s-%d", base, i)
	}
	return id
}

// Prune deletes the oldest automatic snapshots beyond keep.
// Manual snapshots are never deleted by rotation.
func (s *Store) Prune(worldID string, keep int) ([]string, error) {
	if err := checkName(worldID); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	ix, err := s.readIndex(worldID)
	if err != nil {
		return nil, err
	}
	var kept []Snapshot
	var removed []string
	autos := 0
	for _, snap := range ix.Snapshots {
		if snap.Kind != KindManual {
			autos++
			if autos > keep {
				removed = append(removed, snap.ID)
				continue
			}
		}
		kept = append(kept, snap)
	}
	if len(removed) == 0 {
		return nil, nil
	}
	ix.Snapshots = kept
	if err := s.writeIndex(ix); err != nil {
		return nil, err
	}
	for _, id := range removed {
		_ = os.Remove(s.archivePath(worldID, id))
	}
	return removed, nil
}

func (s *Store) Delete(worldID, snapID string) error {
	if err := checkName(worldID); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	ix, err := s.readIndex(worldID)
	if err != nil {
		return err
	}
	i := find(ix, snapID)
	if i < 0 {
		return ErrNotFound
	}
	ix.Snapshots = append(ix.Snapshots[:i], ix.Snapshots[i+1:]...)
	if err := s.writeIndex(ix); err != nil {
		return err
	}
	return os.Remove(s.archivePath(worldID, snapID))
}

// Get returns the index of one world. A world without snapshots yields an empty index.
func (s *Store) Get(worldID string) (Index, error) {
	if err := checkName(worldID); err != nil {
		return Index{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readIndex(worldID)
}

// All returns the indexes of every world that has snapshots.
func (s *Store) All() ([]Index, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.root())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Index
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		ix, err := s.readIndex(e.Name())
		if err != nil || len(ix.Snapshots) == 0 {
			continue
		}
		out = append(out, ix)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].World.ID < out[j].World.ID })
	return out, nil
}

// Extract unpacks a snapshot into dest, which must not exist yet.
// It extracts into a temp sibling first and renames at the end.
func (s *Store) Extract(worldID, snapID, dest string) error {
	if err := checkName(worldID); err != nil {
		return err
	}
	ix, err := s.Get(worldID)
	if err != nil {
		return err
	}
	if find(ix, snapID) < 0 {
		return ErrNotFound
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("%s already exists", dest)
	}
	tmp := filepath.Join(filepath.Dir(dest), "."+filepath.Base(dest)+".worldkeeper-restore")
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	if err := extractZip(s.archivePath(worldID, snapID), tmp); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	return nil
}

func (s *Store) archivePath(worldID, snapID string) string {
	return filepath.Join(s.root(), worldID, snapID+".zip")
}

func (s *Store) indexPath(worldID string) string {
	return filepath.Join(s.root(), worldID, "index.json")
}

func (s *Store) readIndex(worldID string) (Index, error) {
	b, err := os.ReadFile(s.indexPath(worldID))
	if errors.Is(err, os.ErrNotExist) {
		return Index{World: WorldRef{ID: worldID}}, nil
	}
	if err != nil {
		return Index{}, err
	}
	var ix Index
	if err := json.Unmarshal(b, &ix); err != nil {
		return Index{}, fmt.Errorf("%s: %w", s.indexPath(worldID), err)
	}
	return ix, nil
}

func (s *Store) writeIndex(ix Index) error {
	b, err := json.MarshalIndent(ix, "", "  ")
	if err != nil {
		return err
	}
	path := s.indexPath(ix.World.ID)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func find(ix Index, snapID string) int {
	for i, s := range ix.Snapshots {
		if s.ID == snapID {
			return i
		}
	}
	return -1
}

// checkName rejects ids that could escape the storage root.
func checkName(name string) error {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || !filepath.IsLocal(name) {
		return fmt.Errorf("invalid id %q", name)
	}
	return nil
}
