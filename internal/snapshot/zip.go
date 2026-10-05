package snapshot

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// skipFiles are never archived. session.lock is held by the game and
// must not be restored over a running instance's lock.
var skipFiles = map[string]bool{"session.lock": true}

// walkWorld calls fn for every directory and regular file of dir that is
// archived, with its path relative to dir.
func walkWorld(dir string, fn func(path, rel string, e fs.DirEntry) error) error {
	return filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil || rel == "." {
			return err
		}
		if skipFiles[rel] {
			return nil
		}
		if !e.IsDir() && !e.Type().IsRegular() {
			return nil // symlinks, devices
		}
		return fn(path, rel, e)
	})
}

// writeZip archives the contents of dir (without the folder itself) to w.
func writeZip(w io.Writer, dir string, progress Progress) error {
	// A quick first pass sizes the world, so progress can show a percentage.
	var total int64
	if progress != nil {
		err := walkWorld(dir, func(_, _ string, e fs.DirEntry) error {
			if fi, err := e.Info(); err == nil && !e.IsDir() {
				total += fi.Size()
			}
			return nil
		})
		if err != nil {
			return err
		}
		progress(0, total)
	}
	counter := &progressWriter{progress: progress, total: total}

	zw := zip.NewWriter(w)
	err := walkWorld(dir, func(path, rel string, e fs.DirEntry) error {
		fi, err := e.Info()
		if err != nil {
			return err
		}
		h, err := zip.FileInfoHeader(fi)
		if err != nil {
			return err
		}
		h.Name = filepath.ToSlash(rel)
		if e.IsDir() {
			h.Name += "/"
			_, err := zw.CreateHeader(h)
			return err
		}
		// Region files are already zlib-compressed chunk by chunk.
		if ext := filepath.Ext(path); ext == ".mca" || ext == ".mcc" {
			h.Method = zip.Store
		} else {
			h.Method = zip.Deflate
		}
		dst, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		counter.w = dst
		_, err = io.Copy(counter, src)
		return err
	})
	if err != nil {
		zw.Close()
		return err
	}
	return zw.Close()
}

// extractZip unpacks archive into dest, which must already exist.
func extractZip(archive, dest string, progress Progress) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()

	var total int64
	for _, f := range zr.File {
		total += int64(f.UncompressedSize64)
	}
	progress.Report(0, total)
	counter := &progressWriter{progress: progress, total: total}

	for _, f := range zr.File {
		name := strings.TrimSuffix(f.Name, "/")
		if name == "" {
			continue
		}
		if !filepath.IsLocal(filepath.FromSlash(name)) {
			return fmt.Errorf("unsafe path in archive: %q", f.Name)
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := extractFile(f, target, counter); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(f *zip.File, target string, counter *progressWriter) error {
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	counter.w = dst
	if _, err := io.Copy(counter, src); err != nil {
		dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	return os.Chtimes(target, f.Modified, f.Modified)
}

// progressWriter forwards writes to w and reports the running byte count.
type progressWriter struct {
	w        io.Writer
	progress Progress
	done     int64
	total    int64
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.done += int64(n)
	p.progress.Report(p.done, p.total)
	return n, err
}
