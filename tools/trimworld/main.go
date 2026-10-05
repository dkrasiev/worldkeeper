// Command trimworld turns a Minecraft world into a small test fixture for
// internal/worldinfo: it keeps the files world info is read from, replaces
// region files with empty ones (so dimensions still count them), and
// replaces player UUIDs with fake ones so a fixture never identifies an
// account.
//
//	go run ./tools/trimworld <world folder> internal/worldinfo/testdata/worlds/<name>
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var uuidRe = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// keep reports whether a file (slash path relative to the world) belongs in
// a fixture.
func keep(p string) bool {
	switch {
	case p == "level.dat", p == "icon.png":
		return true
	case strings.HasSuffix(p, "_old"), path.Base(p) == "session.lock":
		return false
	}
	for _, dir := range []string{"data/", "players/", "playerdata/", "stats/", "advancements/"} {
		if strings.HasPrefix(p, dir) {
			return true
		}
	}
	return false
}

func isRegion(p string) bool {
	return path.Ext(p) == ".mca" && path.Base(path.Dir(p)) == "region"
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: trimworld <world folder> <fixture folder>")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(src, dst string) error {
	if _, err := os.Stat(filepath.Join(src, "level.dat")); err != nil {
		return fmt.Errorf("%s is not a world folder: %w", src, err)
	}
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("%s already exists", dst)
	}

	var files []string
	uuids := map[string]bool{}
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		rel = filepath.ToSlash(rel)
		if keep(rel) || isRegion(rel) {
			files = append(files, rel)
			for _, u := range uuidRe.FindAllString(path.Base(rel), -1) {
				uuids[u] = true
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Deterministic fake UUIDs: players in sorted order get ...0001, ...0002.
	var real []string
	for u := range uuids {
		real = append(real, u)
	}
	sort.Strings(real)
	fake := map[string]string{}
	for i, u := range real {
		fake[u] = fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
	}

	for _, rel := range files {
		out := filepath.Join(dst, filepath.FromSlash(anonText(rel, fake)))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if isRegion(rel) {
			if err := os.WriteFile(out, nil, 0o644); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		if data, err = anonymize(rel, data, fake); err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("wrote %d files to %s, %d player(s) anonymized\n", len(files), dst, len(fake))
	return nil
}

// anonymize replaces UUIDs in text files and in (gzipped) NBT, where a UUID
// is stored as 16 raw bytes (int array, 1.16+) or two 8-byte longs (older).
func anonymize(rel string, data []byte, fake map[string]string) ([]byte, error) {
	if path.Ext(rel) != ".dat" {
		return []byte(anonText(string(data), fake)), nil
	}
	raw, gz := data, false
	if len(data) > 1 && data[0] == 0x1f && data[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if raw, err = io.ReadAll(zr); err != nil {
			return nil, err
		}
		gz = true
	}
	for real, f := range fake {
		rb, fb := uuidBytes(real), uuidBytes(f)
		raw = bytes.ReplaceAll(raw, rb, fb)         // int array / 16 bytes
		raw = bytes.ReplaceAll(raw, rb[:8], fb[:8]) // UUIDMost
		raw = bytes.ReplaceAll(raw, rb[8:], fb[8:]) // UUIDLeast
		raw = bytes.ReplaceAll(raw, []byte(real), []byte(f))
	}
	if !gz {
		return raw, nil
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func anonText(s string, fake map[string]string) string {
	for real, f := range fake {
		s = strings.ReplaceAll(s, real, f)
	}
	return s
}

func uuidBytes(u string) []byte {
	b, _ := hex.DecodeString(strings.ReplaceAll(u, "-", ""))
	return b
}
