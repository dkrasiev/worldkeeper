package restic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/snapshot"
)

// Snapshots are found by tags; free text is base64url-encoded because
// restic uses commas to separate tags.
const (
	tagApp         = "wk"
	tagWorld       = "world:"
	tagKind        = "kind:"
	tagLastPlayed  = "lp:"
	tagLabel       = "label:"
	tagNote        = "note:"
	tagVersion     = "ver:"
	tagName        = "name:"
	tagFolder      = "folder:"
	tagSource      = "src:"
	tagSourceLabel = "srcl:"
)

// cacheTTL bounds how stale the snapshot list may be. The UI polls every
// few seconds and listing a remote repository is not free.
const cacheTTL = 30 * time.Second

// Store implements snapshot.Backend on top of a restic repository.
type Store struct {
	r *Runner

	mu       sync.Mutex
	cached   []rsnap
	cachedAt time.Time
}

var _ snapshot.Backend = (*Store)(nil)

func NewStore(r *Runner) *Store { return &Store{r: r} }

type rsnap struct {
	ID      string    `json:"id"`
	Time    time.Time `json:"time"`
	Paths   []string  `json:"paths"`
	Tags    []string  `json:"tags"`
	Summary *struct {
		TotalBytesProcessed int64 `json:"total_bytes_processed"`
	} `json:"summary"`
}

// Status describes whether the repository is usable.
type Status struct {
	Version string `json:"version,omitempty"`
	// State: "ok", "not_installed", "missing", "wrong_password", "no_password", "error".
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
}

// Check verifies the restic binary and the repository.
func (s *Store) Check(ctx context.Context) Status {
	v, err := s.r.Version(ctx)
	if err != nil {
		state := "error"
		if errors.Is(err, ErrNotInstalled) {
			state = "not_installed"
		}
		return Status{Version: v, State: state, Message: err.Error()}
	}
	_, err = s.r.Run(ctx, "", "cat", "config")
	switch {
	case err == nil:
		return Status{Version: v, State: "ok"}
	case errors.Is(err, ErrRepoMissing):
		return Status{Version: v, State: "missing", Message: "Nothing is stored at this location yet. Initialize it to start backing up here."}
	case errors.Is(err, ErrWrongPassword):
		return Status{Version: v, State: "wrong_password", Message: "Wrong password for this repository."}
	case errors.Is(err, ErrNoPassword):
		return Status{Version: v, State: "no_password", Message: "Enter the repository password."}
	default:
		return Status{Version: v, State: "error", Message: err.Error()}
	}
}

// Init creates a new repository.
func (s *Store) Init(ctx context.Context) error {
	_, err := s.r.Run(ctx, "", "init")
	return err
}

// run retries once after removing stale locks, e.g. left by a crash or a
// NAS that went offline mid-backup. `restic unlock` only removes locks
// whose process is gone, so it is safe while another backup runs.
func (s *Store) run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return s.runLines(ctx, dir, nil, args...)
}

func (s *Store) runLines(ctx context.Context, dir string, onLine func([]byte), args ...string) ([]byte, error) {
	out, err := s.r.RunLines(ctx, dir, onLine, args...)
	if errors.Is(err, ErrLocked) {
		if _, uerr := s.r.Run(ctx, "", "unlock"); uerr == nil {
			out, err = s.r.RunLines(ctx, dir, onLine, args...)
		}
	}
	return out, err
}

// statusLines turns restic's --json status messages into progress reports.
// Backups count bytes_done, restores bytes_restored. A restore's summary
// has the same fields, so it reports the final state even when restic
// finished before printing any status.
func statusLines(progress snapshot.Progress) func([]byte) {
	if progress == nil {
		return nil
	}
	return func(line []byte) {
		var st struct {
			Type          string `json:"message_type"`
			TotalBytes    int64  `json:"total_bytes"`
			BytesDone     int64  `json:"bytes_done"`
			BytesRestored int64  `json:"bytes_restored"`
		}
		if json.Unmarshal(line, &st) != nil {
			return
		}
		if st.Type != "status" && (st.Type != "summary" || st.TotalBytes == 0) {
			return
		}
		progress(st.BytesDone+st.BytesRestored, st.TotalBytes)
	}
}

func (s *Store) invalidate() {
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
}

func (s *Store) list(ctx context.Context) ([]rsnap, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != nil && time.Since(s.cachedAt) < cacheTTL {
		return s.cached, nil
	}
	out, err := s.run(ctx, "", "snapshots", "--json", "--no-lock", "--tag", tagApp)
	if err != nil {
		return nil, err
	}
	var snaps []rsnap
	if err := json.Unmarshal(out, &snaps); err != nil {
		return nil, fmt.Errorf("parse restic snapshots: %w", err)
	}
	if snaps == nil {
		snaps = []rsnap{}
	}
	s.cached, s.cachedAt = snaps, time.Now()
	return snaps, nil
}

func (s *Store) Create(ref snapshot.WorldRef, worldDir string, m snapshot.Meta) (snapshot.Snapshot, error) {
	ctx := context.Background()
	defer s.invalidate()

	args := []string{"backup", "--json", "--exclude", "session.lock",
		"--tag", tagApp,
		"--tag", tagWorld + ref.ID,
		"--tag", tagKind + string(m.Kind),
		"--tag", tagLastPlayed + strconv.FormatInt(m.LastPlayed.UnixMilli(), 10),
	}
	for prefix, v := range map[string]string{
		tagLabel: m.Label, tagNote: m.Note, tagVersion: m.GameVersion,
		tagName: ref.Name, tagFolder: ref.Folder, tagSource: ref.Source, tagSourceLabel: ref.SourceLabel,
	} {
		if v != "" {
			args = append(args, "--tag", prefix+enc(v))
		}
	}
	// Backing up "." from inside the world puts its files at the snapshot
	// root, so a restore unpacks them straight into the target folder.
	m.Progress.Report(0, 0)
	out, err := s.runLines(ctx, worldDir, statusLines(m.Progress), append(args, ".")...)
	if err != nil && !errors.Is(err, ErrIncomplete) {
		return snapshot.Snapshot{}, err
	}

	sum, perr := parseSummary(out)
	if perr != nil {
		return snapshot.Snapshot{}, perr
	}
	m.Progress.Report(sum.TotalBytesProcessed, sum.TotalBytesProcessed)
	snap := snapshot.Snapshot{
		ID:          sum.SnapshotID,
		Kind:        m.Kind,
		Label:       m.Label,
		Note:        m.Note,
		CreatedAt:   time.Now().UTC(),
		SizeBytes:   sum.TotalBytesProcessed,
		GameVersion: m.GameVersion,
		LastPlayed:  m.LastPlayed,
	}
	return snap, err // err is nil or ErrIncomplete
}

type summary struct {
	MessageType         string `json:"message_type"`
	SnapshotID          string `json:"snapshot_id"`
	TotalBytesProcessed int64  `json:"total_bytes_processed"`
}

func parseSummary(out []byte) (summary, error) {
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var s summary
		if json.Unmarshal(sc.Bytes(), &s) == nil && s.MessageType == "summary" && s.SnapshotID != "" {
			return s, nil
		}
	}
	return summary{}, errors.New("restic backup finished without a snapshot summary")
}

func (s *Store) Prune(worldID string, keep int) ([]string, error) {
	ctx := context.Background()
	w := tagApp + "," + tagWorld + worldID
	out, err := s.run(ctx, "", "forget", "--json", "--group-by", "",
		"--tag", w+","+tagKind+string(snapshot.KindAuto),
		"--tag", w+","+tagKind+string(snapshot.KindPreRestore),
		"--keep-last", strconv.Itoa(keep))
	if err != nil {
		return nil, err
	}
	var groups []struct {
		Remove []rsnap `json:"remove"`
	}
	if err := json.Unmarshal(out, &groups); err != nil {
		return nil, fmt.Errorf("parse restic forget: %w", err)
	}
	var removed []string
	for _, g := range groups {
		for _, r := range g.Remove {
			removed = append(removed, r.ID)
		}
	}
	if len(removed) == 0 {
		return nil, nil
	}
	s.invalidate()
	// Free the space now; forget alone only drops the snapshot records.
	if _, err := s.run(ctx, "", "prune"); err != nil {
		return removed, fmt.Errorf("restic prune: %w", err)
	}
	return removed, nil
}

func (s *Store) Delete(worldID, snapID string) error {
	if _, err := s.find(worldID, snapID); err != nil {
		return err
	}
	ctx := context.Background()
	defer s.invalidate()
	if _, err := s.run(ctx, "", "forget", snapID); err != nil {
		return err
	}
	_, err := s.run(ctx, "", "prune")
	return err
}

func (s *Store) Get(worldID string) (snapshot.Index, error) {
	all, err := s.All()
	if err != nil {
		return snapshot.Index{}, err
	}
	for _, ix := range all {
		if ix.World.ID == worldID {
			return ix, nil
		}
	}
	return snapshot.Index{World: snapshot.WorldRef{ID: worldID}}, nil
}

func (s *Store) All() ([]snapshot.Index, error) {
	snaps, err := s.list(context.Background())
	if err != nil {
		return nil, err
	}
	byWorld := map[string]*snapshot.Index{}
	sorted := append([]rsnap(nil), snaps...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Time.After(sorted[j].Time) })
	for _, rs := range sorted {
		t := parseTags(rs.Tags)
		id := t[tagWorld]
		if id == "" {
			continue
		}
		ix, ok := byWorld[id]
		if !ok {
			// Newest snapshot first, so the ref reflects the latest location.
			ix = &snapshot.Index{World: snapshot.WorldRef{
				ID:          id,
				Name:        dec(t[tagName]),
				Folder:      dec(t[tagFolder]),
				Source:      dec(t[tagSource]),
				SourceLabel: dec(t[tagSourceLabel]),
			}}
			if len(rs.Paths) > 0 {
				ix.World.Path = rs.Paths[0]
			}
			byWorld[id] = ix
		}
		ix.Snapshots = append(ix.Snapshots, toSnapshot(rs, t))
	}

	out := make([]snapshot.Index, 0, len(byWorld))
	for _, ix := range byWorld {
		out = append(out, *ix)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].World.ID < out[j].World.ID })
	return out, nil
}

func toSnapshot(rs rsnap, t map[string]string) snapshot.Snapshot {
	snap := snapshot.Snapshot{
		ID:          rs.ID,
		Kind:        snapshot.Kind(t[tagKind]),
		Label:       dec(t[tagLabel]),
		Note:        dec(t[tagNote]),
		CreatedAt:   rs.Time.UTC(),
		GameVersion: dec(t[tagVersion]),
	}
	if snap.Kind == "" {
		snap.Kind = snapshot.KindManual // never rotate snapshots we cannot classify
	}
	if ms, err := strconv.ParseInt(t[tagLastPlayed], 10, 64); err == nil && ms > 0 {
		snap.LastPlayed = time.UnixMilli(ms).UTC()
	}
	if rs.Summary != nil {
		snap.SizeBytes = rs.Summary.TotalBytesProcessed
	}
	return snap
}

func (s *Store) find(worldID, snapID string) (snapshot.Snapshot, error) {
	ix, err := s.Get(worldID)
	if err != nil {
		return snapshot.Snapshot{}, err
	}
	for _, snap := range ix.Snapshots {
		if snap.ID == snapID {
			return snap, nil
		}
	}
	return snapshot.Snapshot{}, snapshot.ErrNotFound
}

func (s *Store) Extract(worldID, snapID, dest string, progress snapshot.Progress) error {
	if _, err := s.find(worldID, snapID); err != nil {
		return err
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("%s already exists", dest)
	}
	tmp := filepath.Join(filepath.Dir(dest), "."+filepath.Base(dest)+".worldkeeper-restore")
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	progress.Report(0, 0)
	if _, err := s.runLines(context.Background(), "", statusLines(progress), "restore", "--json", snapID, "--target", tmp); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	return nil
}

func parseTags(tags []string) map[string]string {
	m := map[string]string{}
	for _, t := range tags {
		if k, v, ok := strings.Cut(t, ":"); ok {
			m[k+":"] = v
		}
	}
	return m
}

func enc(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

func dec(s string) string {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return ""
	}
	return string(b)
}
