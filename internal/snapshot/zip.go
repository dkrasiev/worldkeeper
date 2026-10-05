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

// writeZip archives the contents of dir (without the folder itself) to w.
func writeZip(w io.Writer, dir string) error {
	zw := zip.NewWriter(w)
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
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
		_, err = io.Copy(dst, src)
		return err
	})
	if err != nil {
		zw.Close()
		return err
	}
	return zw.Close()
}

// extractZip unpacks archive into dest, which must already exist.
func extractZip(archive, dest string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()

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
		if err := extractFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(f *zip.File, target string) error {
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	return os.Chtimes(target, f.Modified, f.Modified)
}
