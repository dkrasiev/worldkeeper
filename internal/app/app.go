// Package app ties discovery, world info, locks and snapshots together.
package app

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/config"
	"github.com/dkrasiev/worldkeeper/internal/discovery"
	"github.com/dkrasiev/worldkeeper/internal/lock"
	"github.com/dkrasiev/worldkeeper/internal/restic"
	"github.com/dkrasiev/worldkeeper/internal/secrets"
	"github.com/dkrasiev/worldkeeper/internal/snapshot"
	"github.com/dkrasiev/worldkeeper/internal/worldinfo"
)

var (
	ErrWorldNotFound = errors.New("world not found")
	ErrInUse         = errors.New("world is open in the game")
	ErrUnchanged     = errors.New("world has not changed since the last snapshot")
)

type App struct {
	// Version is the running build's version ("0.1.0", "0.1.1-dev.abc1234",
	// or "dev" for go run / go build without release flags).
	Version string

	Config  *config.Store
	Secrets secrets.Store
	Env     discovery.Env
	Log     *slog.Logger

	zip *snapshot.Store

	resticMu    sync.Mutex
	resticKey   config.Restic
	resticStore *restic.Store

	eventsMu   sync.Mutex
	events     []Event
	eventsPath string // set by OpenEvents; empty keeps the feed in memory only

	snapCache  snapshotCache
	lastBackup lastBackupCache
	jobs       jobs
}

func New(cfg *config.Store, sec secrets.Store, env discovery.Env, log *slog.Logger) *App {
	return &App{
		Config:  cfg,
		Secrets: sec,
		Env:     env,
		Log:     log,
		zip:     snapshot.NewStore(func() string { return cfg.Get().StorageDir }),
	}
}

// Snapshots returns the storage backend selected in the config.
func (a *App) Snapshots() snapshot.Backend {
	if a.Config.Get().Engine == config.EngineRestic {
		return a.Restic()
	}
	return a.zip
}

// Restic returns the restic store for the current config. It is rebuilt
// when the restic settings change, so its snapshot cache stays valid.
func (a *App) Restic() *restic.Store {
	rc := a.Config.Get().Restic
	a.resticMu.Lock()
	defer a.resticMu.Unlock()
	if a.resticStore == nil || a.resticKey != rc {
		a.resticKey = rc
		a.resticStore = restic.NewStore(&restic.Runner{
			Binary:       rc.Binary,
			Repo:         rc.Repo,
			PasswordFile: rc.PasswordFile,
			Password:     func() (string, error) { return a.ResticPassword(rc.Repo) },
		})
	}
	return a.resticStore
}

// ResticPassword reads the password for repo from the credential store.
func (a *App) ResticPassword(repo string) (string, error) {
	pw, err := a.Secrets.Get(ResticSecretKey(repo))
	if errors.Is(err, secrets.ErrNotFound) {
		return "", restic.ErrNoPassword
	}
	return pw, err
}

// ResticSecretKey is the credential store key for a repository's password.
// Keying by repository means switching repositories never reuses a password.
func ResticSecretKey(repo string) string { return "restic:" + repo }

// WorldView is one row of the world list.
type WorldView struct {
	discovery.World
	worldinfo.Summary
	InUse      bool               `json:"inUse"`
	Error      string             `json:"error,omitempty"`
	Snapshots  int                `json:"snapshots"`
	LastBackup *snapshot.Snapshot `json:"lastBackup,omitempty"`
	Changed    bool               `json:"changed"` // played since the last snapshot
}

// Overview is everything the main screen needs.
type Overview struct {
	Worlds []WorldView `json:"worlds"`
	// Archived worlds have snapshots but no local folder, e.g. after reinstalling the OS.
	Archived []snapshot.Index `json:"archived"`
	// StorageError is set when the backup storage cannot be read; worlds
	// are still listed so the user sees what is at risk.
	StorageError string `json:"storageError,omitempty"`
}

func (a *App) scan() []discovery.World {
	return discovery.Scan(a.Env, a.Config.Get().ExtraSavesDirs)
}

func (a *App) find(id string) (discovery.World, error) {
	for _, w := range a.scan() {
		if w.ID == id {
			return w, nil
		}
	}
	return discovery.World{}, ErrWorldNotFound
}

func (a *App) Overview() (Overview, error) {
	worlds := a.scan()
	ov := Overview{Worlds: []WorldView{}, Archived: []snapshot.Index{}}
	indexes, err := a.Snapshots().All()
	if err != nil {
		ov.StorageError = err.Error()
	}
	byID := map[string]snapshot.Index{}
	for _, ix := range indexes {
		byID[ix.World.ID] = ix
	}

	for _, w := range worlds {
		v := WorldView{World: w}
		if s, err := worldinfo.ReadSummary(w.Path); err != nil {
			v.Error = err.Error()
			v.Name = w.Folder
		} else {
			v.Summary = s
		}
		v.InUse, _ = lock.InUse(w.Path)
		v.Changed = true
		if ix, ok := byID[w.ID]; ok {
			v.Snapshots = len(ix.Snapshots)
			v.LastBackup = ix.Latest()
			v.Changed = !hasState(ix, v.LastPlayed)
			delete(byID, w.ID)
		}
		ov.Worlds = append(ov.Worlds, v)
	}
	for _, ix := range indexes {
		if _, orphan := byID[ix.World.ID]; orphan {
			ov.Archived = append(ov.Archived, ix)
		}
	}
	return ov, nil
}

// Details is the world page: where the world lives and everything read from it.
type Details struct {
	discovery.World
	worldinfo.Info
	InUse bool `json:"inUse"`
}

func (a *App) Info(id string) (Details, error) {
	w, err := a.find(id)
	if err != nil {
		return Details{}, err
	}
	info, err := worldinfo.Read(w.Path)
	if err != nil {
		return Details{}, err
	}
	inUse, _ := lock.InUse(w.Path)
	return Details{World: w, Info: info, InUse: inUse}, nil
}

// Advancements returns the player's advancement progress in the format the
// mcwidgets advancement viewer reads.
func (a *App) Advancements(id string) (map[string]any, error) {
	w, err := a.find(id)
	if err != nil {
		return nil, err
	}
	return worldinfo.AdvancementProgress(w.Path), nil
}

// IconPath returns the world's icon.png path.
func (a *App) IconPath(id string) (string, error) {
	w, err := a.find(id)
	if err != nil {
		return "", err
	}
	return filepath.Join(w.Path, "icon.png"), nil
}

type BackupOptions struct {
	Kind  snapshot.Kind
	Label string
	Note  string
	// Force backs up a world that is open in the game. The copy may be
	// inconsistent because the game writes region files while running.
	Force bool
	// Progress, if set, is told how far the backup is.
	Progress snapshot.Progress
}

// Backup snapshots a local world and waits for it; StartBackup is the
// background variant. Automatic backups are skipped when the world has not
// been saved since the last snapshot.
func (a *App) Backup(id string, o BackupOptions) (snapshot.Snapshot, error) {
	w, err := a.find(id)
	if err != nil {
		return snapshot.Snapshot{}, err
	}
	return a.backupJob(w, o)
}

// displayName is the world's name for jobs and messages.
func (a *App) displayName(w discovery.World) string {
	if s, err := worldinfo.ReadSummary(w.Path); err == nil && s.Name != "" {
		return s.Name
	}
	return w.Folder
}

func (a *App) checkInUse(w discovery.World, force bool) error {
	if inUse, err := lock.InUse(w.Path); err != nil {
		return err
	} else if inUse && !force {
		return ErrInUse
	}
	return nil
}

// backup records every real failure in the activity feed; "in use" and
// "unchanged" are expected outcomes, not failures.
func (a *App) backup(w discovery.World, o BackupOptions) (snapshot.Snapshot, error) {
	snap, err := a.doBackup(w, o)
	if err != nil && !errors.Is(err, ErrInUse) && !errors.Is(err, ErrUnchanged) {
		a.event(EventError, w.ID, w.Folder, CodeBackupFailed, map[string]string{"error": err.Error()}, "Backup failed: "+err.Error())
	}
	return snap, err
}

func (a *App) doBackup(w discovery.World, o BackupOptions) (snapshot.Snapshot, error) {
	if err := a.checkInUse(w, o.Force); err != nil {
		return snapshot.Snapshot{}, err
	}
	sum, err := worldinfo.ReadSummary(w.Path)
	if err != nil {
		if o.Kind == snapshot.KindAuto {
			return snapshot.Snapshot{}, err
		}
		// A broken level.dat is exactly when people want to save or restore:
		// archive the folder anyway.
		sum = worldinfo.Summary{Name: w.Folder}
	}
	if o.Kind == snapshot.KindAuto {
		ix, err := a.Snapshots().Get(w.ID)
		if err != nil {
			return snapshot.Snapshot{}, err
		}
		if last := ix.Latest(); last != nil && last.LastPlayed.Equal(sum.LastPlayed) {
			return snapshot.Snapshot{}, ErrUnchanged
		}
	}

	ref := snapshot.WorldRef{
		ID: w.ID, Name: sum.Name, Folder: w.Folder,
		Source: w.Source, SourceLabel: w.SourceLabel, Path: w.Path,
	}
	store := a.Snapshots()
	snap, err := store.Create(ref, w.Path, snapshot.Meta{
		Kind: o.Kind, Label: o.Label, Note: o.Note,
		GameVersion: sum.GameVersion, LastPlayed: sum.LastPlayed,
		Progress: o.Progress,
	})
	if err != nil {
		return snapshot.Snapshot{}, err
	}
	a.lastBackup.noteBackup(a, snap.CreatedAt)
	a.event(EventBackup, w.ID, sum.Name, CodeBackupSaved, map[string]string{"kind": string(o.Kind), "snapshot": snap.ID},
		fmt.Sprintf("Saved %s snapshot %s", o.Kind, snap.ID))

	if removed, err := store.Prune(w.ID, a.Config.Get().KeepAuto); err != nil {
		a.Log.Warn("prune failed", "world", w.ID, "err", err)
	} else if len(removed) > 0 {
		a.Log.Info("pruned old snapshots", "world", w.ID, "removed", removed)
	}
	return snap, nil
}

type RestoreMode string

const (
	// RestoreReplace overwrites the world in place, after saving its current state.
	RestoreReplace RestoreMode = "replace"
	// RestoreCopy unpacks next to the original as a new world folder.
	RestoreCopy RestoreMode = "copy"
)

// Restore loads a snapshot, waits for it and returns the folder it was
// written to; StartRestore is the background variant. Worlds without a
// local folder are restored to their original path when possible,
// otherwise into the default saves folder.
func (a *App) Restore(worldID, snapID string, mode RestoreMode) (string, error) {
	ix, err := a.Snapshots().Get(worldID)
	if err != nil {
		return "", err
	}
	j, err := a.jobs.start(JobRestore, worldID, a.restoreName(ix), false)
	if err != nil {
		return "", err
	}
	dest, err := a.restore(worldID, snapID, mode, j)
	a.jobs.finish(j, err, func(j *Job) { j.Path = dest })
	return dest, err
}

func (a *App) restore(worldID, snapID string, mode RestoreMode, j *Job) (string, error) {
	ix, err := a.Snapshots().Get(worldID)
	if err != nil {
		return "", err
	}
	w, err := a.find(worldID)
	switch {
	case errors.Is(err, ErrWorldNotFound):
		dest := ix.World.Path
		if dest == "" || !dirExists(filepath.Dir(dest)) || exists(dest) {
			folder := ix.World.Folder
			if folder == "" {
				folder = worldID
			}
			saves := discovery.DefaultSavesDir(a.Env)
			if err := os.MkdirAll(saves, 0o755); err != nil {
				return "", err
			}
			dest = uniquePath(filepath.Join(saves, folder))
		}
		return dest, a.extract(ix, snapID, dest, a.jobs.progress(j, PhaseRestore))
	case err != nil:
		return "", err
	case mode == RestoreCopy:
		now := time.Now()
		dest := uniquePath(filepath.Join(w.SavesDir, w.Folder+" (restored "+now.Format("2006-01-02 15-04")+")"))
		if err := a.extract(ix, snapID, dest, a.jobs.progress(j, PhaseRestore)); err != nil {
			return "", err
		}
		a.renameCopy(dest, now)
		return dest, nil
	case mode == RestoreReplace:
		return w.Path, a.replace(w, ix, snapID, j)
	default:
		return "", fmt.Errorf("unknown restore mode %q", mode)
	}
}

// renameCopy gives a restored copy its own name in the game's world list;
// otherwise it shows up twice under the original name. A failure only
// costs the nicer name, so it is logged rather than failing the restore.
func (a *App) renameCopy(dir string, at time.Time) {
	sum, err := worldinfo.ReadSummary(dir)
	if err == nil {
		err = worldinfo.SetLevelName(dir, sum.Name+" (restored "+at.Format("2006-01-02 15:04")+")")
	}
	if err != nil {
		a.Log.Warn("cannot rename restored copy", "path", dir, "err", err)
	}
}

func (a *App) replace(w discovery.World, ix snapshot.Index, snapID string, j *Job) error {
	// Save the current state first so a restore can always be undone.
	_, err := a.backup(w, BackupOptions{
		Kind: snapshot.KindPreRestore, Label: describe(ix, snapID),
		Progress: a.jobs.progress(j, PhaseSafety),
	})
	if err != nil {
		return fmt.Errorf("safety snapshot before restore: %w", err)
	}

	old := filepath.Join(w.SavesDir, "."+w.Folder+".worldkeeper-old")
	if err := os.RemoveAll(old); err != nil {
		return err
	}
	if err := os.Rename(w.Path, old); err != nil {
		return fmt.Errorf("move current world aside: %w", err)
	}
	if err := a.extract(ix, snapID, w.Path, a.jobs.progress(j, PhaseRestore)); err != nil {
		if rbErr := os.Rename(old, w.Path); rbErr != nil {
			return fmt.Errorf("%w; rollback failed, your world is at %s: %v", err, old, rbErr)
		}
		return err
	}
	return os.RemoveAll(old)
}

func (a *App) extract(ix snapshot.Index, snapID, dest string, progress snapshot.Progress) error {
	if err := a.Snapshots().Extract(ix.World.ID, snapID, dest, progress); err != nil {
		a.event(EventError, ix.World.ID, ix.World.Name, CodeRestoreFailed, map[string]string{"error": err.Error()}, "Restore failed: "+err.Error())
		return err
	}
	a.event(EventRestore, ix.World.ID, ix.World.Name, CodeRestoreDone, map[string]string{"snapshot": snapID, "path": dest},
		fmt.Sprintf("Restored %s to %s", snapID, dest))
	return nil
}

func (a *App) DeleteSnapshot(worldID, snapID string) error {
	defer a.lastBackup.invalidate() // the deleted save may have been the newest
	return a.Snapshots().Delete(worldID, snapID)
}

// hasState reports whether some snapshot already holds the world as last saved
// at lastPlayed (e.g. right after loading an older save).
func hasState(ix snapshot.Index, lastPlayed time.Time) bool {
	for _, s := range ix.Snapshots {
		if s.LastPlayed.Equal(lastPlayed) {
			return true
		}
	}
	return false
}

// describe names the snapshot a safety snapshot was taken before: its
// label, or when it was taken. It is stored as the safety snapshot's label
// and stays language-neutral; the UI words it as "Before loading <label>".
func describe(ix snapshot.Index, snapID string) string {
	for _, s := range ix.Snapshots {
		if s.ID == snapID {
			if s.Label != "" {
				return "“" + s.Label + "”"
			}
			return s.CreatedAt.Local().Format("2006-01-02 15:04")
		}
	}
	return snapID
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func uniquePath(p string) string {
	if !exists(p) {
		return p
	}
	for i := 2; ; i++ {
		c := fmt.Sprintf("%s (%d)", p, i)
		if !exists(c) {
			return c
		}
	}
}
