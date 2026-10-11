package app

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dkrasiev/worldkeeper/internal/config"
	"github.com/dkrasiev/worldkeeper/internal/discovery"
	"github.com/dkrasiev/worldkeeper/internal/snapshot"
)

var (
	ErrSameLocation = errors.New("the world is already in this folder")
	ErrDestExists   = errors.New("a world folder with this name is already there")
	ErrNotAbsolute  = errors.New("the folder must be a full path")
)

// Locations lists the saves folders worlds can be moved or restored into.
func (a *App) Locations() []discovery.Location {
	return discovery.Locations(a.Env, a.Config.Get().ExtraSavesDirs)
}

// locate returns the location of a saves folder that exists. known is
// false for a folder Worldkeeper does not look in yet; it becomes an extra
// saves folder (see addSavesDir) once a world is put there.
func (a *App) locate(saves string) (loc discovery.Location, known bool) {
	if l, ok := discovery.Locate(a.Env, a.Config.Get().ExtraSavesDirs, saves); ok {
		return l, true
	}
	return discovery.CustomLocation(saves), false
}

// addSavesDir adds a folder to the extra saves folders, so the world just
// put there is found.
func (a *App) addSavesDir(saves string) {
	_, err := a.Config.Update(func(c *config.Config) error {
		for _, d := range c.ExtraSavesDirs {
			if discovery.SamePath(a.Env, d, saves) {
				return nil
			}
		}
		c.ExtraSavesDirs = append(c.ExtraSavesDirs, saves)
		return nil
	})
	if err != nil {
		a.Log.Warn("cannot add saves folder", "path", saves, "err", err)
	}
}

// savesDir checks a saves folder the user typed or picked and creates it.
func savesDir(saves string) (string, error) {
	if !filepath.IsAbs(saves) {
		return "", ErrNotAbsolute
	}
	saves = filepath.Clean(saves)
	return saves, os.MkdirAll(saves, 0o755)
}

// StartMove moves a world into another saves folder in the background,
// e.g. from the official launcher into a Prism instance. It keeps its
// folder name. Its id follows the new location, and its backups are filed
// under the new id, so they stay with it. Unless a snapshot already holds
// its current state, the world is saved first.
func (a *App) StartMove(id, saves string) (Job, error) {
	w, err := a.find(id)
	if err != nil {
		return Job{}, err
	}
	if !filepath.IsAbs(saves) {
		return Job{}, ErrNotAbsolute
	}
	if discovery.SamePath(a.Env, saves, w.SavesDir) {
		return Job{}, ErrSameLocation
	}
	if exists(filepath.Join(saves, w.Folder)) {
		return Job{}, ErrDestExists
	}
	if err := a.checkInUse(w, false); err != nil {
		return Job{}, err
	}
	if saves, err = savesDir(saves); err != nil {
		return Job{}, err
	}
	loc, known := a.locate(saves)
	newID := discovery.WorldID(loc.Source, w.Folder)
	if newID != w.ID {
		// Two worlds would share an id: e.g. both in extra folders, whose
		// worlds are all "custom--<folder>".
		if _, err := a.find(newID); err == nil {
			return Job{}, snapshot.ErrIDTaken
		}
		ix, err := a.Snapshots().Get(newID)
		if err != nil {
			return Job{}, err
		}
		if len(ix.Snapshots) > 0 {
			return Job{}, snapshot.ErrIDTaken
		}
	}
	j, err := a.jobs.start(JobMove, w.ID, a.displayName(w), false)
	if err != nil {
		return Job{}, err
	}
	// The watcher must not back the world up under its new id before its
	// snapshots are filed there.
	release, err := a.jobs.claim(newID)
	if err != nil && newID != w.ID {
		a.jobs.finish(j, err, nil)
		return Job{}, err
	}
	go func() {
		if release != nil {
			defer release()
		}
		movedID, err := a.move(w, loc, known, j)
		a.jobs.finish(j, err, func(j *Job) {
			j.Path = filepath.Join(loc.Saves, w.Folder)
			j.NewWorldID = movedID
		})
	}()
	return a.jobCopy(j), nil
}

// move returns the world's new id, also when only its backups could not
// follow it.
func (a *App) move(w discovery.World, loc discovery.Location, known bool, j *Job) (string, error) {
	name := a.displayName(w)
	fail := func(err error) error {
		a.event(EventError, w.ID, name, CodeMoveFailed, map[string]string{"error": err.Error()}, "Move failed: "+err.Error())
		return err
	}
	if err := a.saveUnlessSaved(w, snapshot.KindPreMove, j); err != nil {
		return "", fail(fmt.Errorf("safety snapshot before moving: %w", err))
	}
	progress := a.jobs.progress(j, PhaseMove)
	if err := a.checkInUse(w, false); err != nil { // opened during the safety snapshot
		return "", err
	}
	dest := filepath.Join(loc.Saves, w.Folder)
	if err := moveDir(w.Path, dest, progress); err != nil {
		return "", fail(err)
	}
	if !known {
		a.addSavesDir(loc.Saves)
	}
	newID := discovery.WorldID(loc.Source, w.Folder)
	if newID != w.ID {
		ref := snapshot.WorldRef{ID: newID, Name: name, Folder: w.Folder, Source: loc.Source, SourceLabel: loc.Label, Path: dest}
		if err := a.Snapshots().Rekey(w.ID, ref); err != nil {
			// The world is in place; its backups are listed under
			// "Backups without a local world" and can still be loaded.
			err = fmt.Errorf("the world was moved to %s, but its backups stay under the old location: %w", dest, err)
			a.event(EventError, newID, name, CodeMoveFailed, map[string]string{"error": err.Error()}, err.Error())
			return newID, err
		}
	}
	a.event(EventWorld, newID, name, CodeWorldMoved, map[string]string{"from": w.SavesDir, "to": loc.Saves},
		fmt.Sprintf("Moved from %s to %s", w.SavesDir, loc.Saves))
	return newID, nil
}

// rename is os.Rename; tests replace it to take the copy path.
var rename = os.Rename

// moveDir moves a folder. When that is not possible in one step (another
// drive), it copies the folder to a hidden sibling of dst, renames that
// into place once complete, so the game never sees half a world, and only
// then deletes the original.
func moveDir(src, dst string, progress snapshot.Progress) error {
	if err := rename(src, dst); err == nil {
		return nil
	}
	tmp := filepath.Join(filepath.Dir(dst), "."+filepath.Base(dst)+".worldkeeper-move")
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	if err := copyDir(src, tmp, progress); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	// Hide the original from the game first, as a delete does.
	trash := filepath.Join(filepath.Dir(src), "."+filepath.Base(src)+".worldkeeper-moved")
	if err := os.RemoveAll(trash); err == nil {
		if err := os.Rename(src, trash); err == nil {
			src = trash
		}
	}
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("copied to %s, but cannot remove %s: %w", dst, src, err)
	}
	return nil
}

// copyDir copies a world folder, leaving out session.lock (the game makes
// a new one).
func copyDir(src, dst string, progress snapshot.Progress) error {
	var total, done int64
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() && p != filepath.Join(src, "session.lock") {
			if fi, err := d.Info(); err == nil {
				total += fi.Size()
			}
		}
		return err
	})
	if err != nil {
		return err
	}
	progress.Report(0, total)
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case rel == "session.lock" || !d.Type().IsRegular():
			return nil
		}
		n, err := copyFile(p, target)
		done += n
		progress.Report(done, total)
		return err
	})
}

func copyFile(src, dst string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return n, err
}

// extractInto restores a snapshot into a folder the user picked, and adds
// that folder to the extra saves folders if Worldkeeper does not look
// there yet, so the restored world is found.
func (a *App) extractInto(ix snapshot.Index, snapID, dest string, j *Job) error {
	if err := a.extract(ix, snapID, dest, a.jobs.progress(j, PhaseRestore)); err != nil {
		return err
	}
	if _, known := a.locate(filepath.Dir(dest)); !known {
		a.addSavesDir(filepath.Dir(dest))
	}
	return nil
}
