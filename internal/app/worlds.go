package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dkrasiev/worldkeeper/internal/discovery"
	"github.com/dkrasiev/worldkeeper/internal/snapshot"
	"github.com/dkrasiev/worldkeeper/internal/worldinfo"
)

// ErrNameRequired means a rename was asked for with an empty name.
var ErrNameRequired = errors.New("enter a name for the world")

// Rename changes the name the game shows in its world list. The folder
// keeps its name, so the world keeps its id and stays linked to its backups.
func (a *App) Rename(id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrNameRequired
	}
	w, err := a.find(id)
	if err != nil {
		return err
	}
	release, err := a.jobs.claim(w.ID)
	if err != nil {
		return err
	}
	defer release()
	if err := a.checkInUse(w, false); err != nil {
		return err
	}
	old := a.displayName(w)
	if err := worldinfo.SetLevelName(w.Path, name); err != nil {
		return err
	}
	a.event(EventWorld, w.ID, name, CodeWorldRenamed, map[string]string{"from": old, "to": name},
		fmt.Sprintf("Renamed %q to %q", old, name))
	return nil
}

// StartDelete deletes a world in the background. Unless a snapshot already
// holds its current state, the world is saved first, so it can be loaded
// back from "Backups without a local world".
func (a *App) StartDelete(id string) (Job, error) {
	w, err := a.find(id)
	if err != nil {
		return Job{}, err
	}
	if err := a.checkInUse(w, false); err != nil {
		return Job{}, err
	}
	j, err := a.jobs.start(JobDelete, w.ID, a.displayName(w), false)
	if err != nil {
		return Job{}, err
	}
	go func() {
		err := a.deleteWorld(w, j)
		a.jobs.finish(j, err, func(j *Job) { j.Path = w.Path })
	}()
	return a.jobCopy(j), nil
}

func (a *App) deleteWorld(w discovery.World, j *Job) error {
	name := a.displayName(w)
	if err := a.saveBeforeDelete(w, j); err != nil {
		err = fmt.Errorf("safety snapshot before deleting: %w", err)
		a.event(EventError, w.ID, name, CodeDeleteFailed, map[string]string{"error": err.Error()}, "Delete failed: "+err.Error())
		return err
	}
	a.jobs.progress(j, PhaseDelete)
	if err := a.checkInUse(w, false); err != nil { // opened during the safety snapshot
		return err
	}
	// Move the folder aside first: the game never sees a half-deleted world.
	trash := filepath.Join(w.SavesDir, "."+w.Folder+".worldkeeper-deleted")
	if err := os.RemoveAll(trash); err != nil {
		return err
	}
	if err := os.Rename(w.Path, trash); err != nil {
		err = fmt.Errorf("move world aside: %w", err)
		a.event(EventError, w.ID, name, CodeDeleteFailed, map[string]string{"error": err.Error()}, "Delete failed: "+err.Error())
		return err
	}
	if err := os.RemoveAll(trash); err != nil {
		// The world is gone from the game's list; only disk space is left behind.
		a.Log.Warn("cannot remove deleted world", "path", trash, "err", err)
	}
	a.event(EventWorld, w.ID, name, CodeWorldDeleted, map[string]string{"path": w.Path}, "Deleted "+w.Path)
	return nil
}

// saveBeforeDelete snapshots the world unless a snapshot already holds the
// state it was last saved in. A rename does not move LastPlayed, so the
// name is compared too; otherwise the world would come back under its old name.
func (a *App) saveBeforeDelete(w discovery.World, j *Job) error {
	if sum, err := worldinfo.ReadSummary(w.Path); err == nil {
		ix, err := a.Snapshots().Get(w.ID)
		if err != nil {
			return err
		}
		if hasState(ix, sum.LastPlayed) && ix.World.Name == sum.Name {
			return nil
		}
	}
	_, err := a.backup(w, BackupOptions{Kind: snapshot.KindPreDelete, Progress: a.jobs.progress(j, PhaseSafety)})
	return err
}
