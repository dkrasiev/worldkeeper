package app

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/discovery"
	"github.com/dkrasiev/worldkeeper/internal/snapshot"
)

// ErrBusy means another backup or restore of the same world is running.
var ErrBusy = errors.New("world is being saved or restored right now")

type JobKind string

const (
	JobBackup  JobKind = "backup"
	JobRestore JobKind = "restore"
	JobDelete  JobKind = "delete"
)

type JobState string

const (
	JobRunning JobState = "running"
	JobDone    JobState = "done"
	JobFailed  JobState = "failed"
)

// Job phases. A restore over an existing world first takes a safety
// snapshot, so it shows two progress runs.
const (
	PhaseBackup  = "backup"
	PhaseSafety  = "safety"
	PhaseRestore = "restore"
	PhaseDelete  = "delete"
)

// Job is a backup or restore running in the background, so the HTTP
// request returns at once and the UI can poll its progress.
type Job struct {
	ID         string     `json:"id"`
	Kind       JobKind    `json:"kind"`
	WorldID    string     `json:"worldId"`
	World      string     `json:"world"`
	Auto       bool       `json:"auto,omitempty"` // started by the watcher
	State      JobState   `json:"state"`
	Phase      string     `json:"phase"`
	Done       int64      `json:"done"`  // bytes
	Total      int64      `json:"total"` // bytes; 0 while unknown
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`

	Snapshot *snapshot.Snapshot `json:"snapshot,omitempty"` // backup result
	Path     string             `json:"path,omitempty"`     // restore destination
	Error    *JobError          `json:"error,omitempty"`
}

// JobError has the same shape as API errors, so the UI words it the same way.
type JobError struct {
	Code    string `json:"error"`
	Message string `json:"message"`
}

// keepFinished is how long finished jobs stay listed, so a page opened
// later still sees how its job ended.
const keepFinished = 10 * time.Minute

type jobs struct {
	mu   sync.Mutex
	seq  int
	byID map[string]*Job
	busy map[string]string // world id -> running job id
}

// start registers a running job for a world, or fails with ErrBusy.
func (js *jobs) start(kind JobKind, worldID, world string, auto bool) (*Job, error) {
	js.mu.Lock()
	defer js.mu.Unlock()
	js.init()
	if _, ok := js.busy[worldID]; ok {
		return nil, ErrBusy
	}
	js.prune()
	js.seq++
	phase := map[JobKind]string{JobBackup: PhaseBackup, JobRestore: PhaseRestore, JobDelete: PhaseSafety}[kind]
	j := &Job{
		ID: fmt.Sprintf("%d", js.seq), Kind: kind, WorldID: worldID, World: world, Auto: auto,
		State: JobRunning, Phase: phase, StartedAt: time.Now().UTC(),
	}
	js.byID[j.ID] = j
	js.busy[worldID] = j.ID
	return j, nil
}

func (js *jobs) init() {
	if js.byID == nil {
		js.byID, js.busy = map[string]*Job{}, map[string]string{}
	}
}

// claim reserves a world for a quick change that is not listed as a job,
// such as a rename. Call release when done.
func (js *jobs) claim(worldID string) (release func(), err error) {
	js.mu.Lock()
	defer js.mu.Unlock()
	js.init()
	if _, ok := js.busy[worldID]; ok {
		return nil, ErrBusy
	}
	js.busy[worldID] = ""
	return func() {
		js.mu.Lock()
		delete(js.busy, worldID)
		js.mu.Unlock()
	}, nil
}

func (js *jobs) prune() {
	for id, j := range js.byID {
		if j.FinishedAt != nil && time.Since(*j.FinishedAt) > keepFinished {
			delete(js.byID, id)
		}
	}
}

// progress returns the callback a backend reports to while j is in phase.
func (js *jobs) progress(j *Job, phase string) snapshot.Progress {
	if j == nil {
		return nil
	}
	js.mu.Lock()
	j.Phase, j.Done, j.Total = phase, 0, 0
	js.mu.Unlock()
	return func(done, total int64) {
		js.mu.Lock()
		j.Done, j.Total = done, total
		js.mu.Unlock()
	}
}

func (js *jobs) finish(j *Job, err error, result func(*Job)) {
	js.mu.Lock()
	defer js.mu.Unlock()
	delete(js.busy, j.WorldID)
	// An automatic backup that found nothing to do is not worth listing.
	if j.Auto && (errors.Is(err, ErrUnchanged) || errors.Is(err, ErrInUse)) {
		delete(js.byID, j.ID)
		return
	}
	now := time.Now().UTC()
	j.FinishedAt = &now
	if err != nil {
		j.State = JobFailed
		j.Error = &JobError{Code: ErrorCode(err), Message: err.Error()}
	} else {
		j.State = JobDone
		if result != nil {
			result(j)
		}
	}
}

func (js *jobs) list() []Job {
	js.mu.Lock()
	defer js.mu.Unlock()
	js.prune()
	out := make([]Job, 0, len(js.byID))
	for _, j := range js.byID {
		out = append(out, *j)
	}
	sort.Slice(out, func(i, k int) bool { return out[i].StartedAt.After(out[k].StartedAt) })
	return out
}

// Jobs returns running and recently finished jobs, newest first.
func (a *App) Jobs() []Job { return a.jobs.list() }

// ErrorCode is the API error code for err.
func ErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrWorldNotFound), errors.Is(err, snapshot.ErrNotFound), errors.Is(err, fs.ErrNotExist):
		return "not_found"
	case errors.Is(err, ErrInUse):
		return "in_use"
	case errors.Is(err, ErrUnchanged):
		return "unchanged"
	case errors.Is(err, ErrBusy):
		return "busy"
	default:
		return "internal"
	}
}

// StartBackup starts a backup in the background. The checks that need an
// answer from the user (world not found, open in the game, already busy)
// fail right away; everything else is reported by the job.
func (a *App) StartBackup(id string, o BackupOptions) (Job, error) {
	w, err := a.find(id)
	if err != nil {
		return Job{}, err
	}
	if err := a.checkInUse(w, o.Force); err != nil {
		return Job{}, err
	}
	j, err := a.jobs.start(JobBackup, w.ID, a.displayName(w), o.Kind == snapshot.KindAuto)
	if err != nil {
		return Job{}, err
	}
	go a.runBackupJob(j, w, o)
	return a.jobCopy(j), nil
}

// backupJob runs a backup as a job in the caller's goroutine, so automatic
// backups show progress too.
func (a *App) backupJob(w discovery.World, o BackupOptions) (snapshot.Snapshot, error) {
	j, err := a.jobs.start(JobBackup, w.ID, a.displayName(w), o.Kind == snapshot.KindAuto)
	if err != nil {
		return snapshot.Snapshot{}, err
	}
	return a.runBackupJob(j, w, o)
}

func (a *App) runBackupJob(j *Job, w discovery.World, o BackupOptions) (snapshot.Snapshot, error) {
	o.Progress = a.jobs.progress(j, PhaseBackup)
	snap, err := a.backup(w, o)
	a.jobs.finish(j, err, func(j *Job) { j.Snapshot = &snap })
	return snap, err
}

// StartRestore starts a restore in the background; see StartBackup.
func (a *App) StartRestore(worldID, snapID string, mode RestoreMode) (Job, error) {
	ix, err := a.Snapshots().Get(worldID)
	if err != nil {
		return Job{}, err
	}
	if !slices.ContainsFunc(ix.Snapshots, func(s snapshot.Snapshot) bool { return s.ID == snapID }) {
		return Job{}, snapshot.ErrNotFound
	}
	if mode != RestoreReplace && mode != RestoreCopy {
		return Job{}, fmt.Errorf("unknown restore mode %q", mode)
	}
	if w, err := a.find(worldID); err == nil && mode == RestoreReplace {
		if err := a.checkInUse(w, false); err != nil {
			return Job{}, err
		}
	}
	j, err := a.jobs.start(JobRestore, worldID, a.restoreName(ix), false)
	if err != nil {
		return Job{}, err
	}
	go func() {
		dest, err := a.restore(worldID, snapID, mode, j)
		a.jobs.finish(j, err, func(j *Job) { j.Path = dest })
	}()
	return a.jobCopy(j), nil
}

// restoreName names a restore job after the local world, or after the
// snapshots' record of it when the world is gone.
func (a *App) restoreName(ix snapshot.Index) string {
	if w, err := a.find(ix.World.ID); err == nil {
		return a.displayName(w)
	}
	if ix.World.Name != "" {
		return ix.World.Name
	}
	return ix.World.ID
}

func (a *App) jobCopy(j *Job) Job {
	a.jobs.mu.Lock()
	defer a.jobs.mu.Unlock()
	return *j
}
