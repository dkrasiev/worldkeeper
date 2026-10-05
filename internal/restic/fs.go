package restic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// metadataPaths are the small files world info is read from. Region files
// can be gigabytes, so Open restores only these and takes everything else
// (names, sizes) from `restic ls`.
var metadataPaths = []string{
	"/level.dat", "/icon.png", "/data", "/players",
	"/playerdata", "/stats", "/advancements", // before 26.x
}

// Open lists the snapshot and restores its metadata files into a temp dir.
// The returned closer removes the temp dir.
func (s *Store) Open(worldID, snapID string) (fs.FS, io.Closer, error) {
	if _, err := s.find(worldID, snapID); err != nil {
		return nil, nil, err
	}
	ctx := context.Background()
	out, err := s.run(ctx, "", "ls", "--json", "--no-lock", snapID)
	if err != nil {
		return nil, nil, err
	}
	tree, err := parseListing(out)
	if err != nil {
		return nil, nil, err
	}

	tmp, err := os.MkdirTemp("", "worldkeeper-snapshot-")
	if err != nil {
		return nil, nil, err
	}
	args := []string{"restore", snapID, "--target", tmp}
	for _, p := range metadataPaths {
		args = append(args, "--include", p)
	}
	if _, err := s.run(ctx, "", args...); err != nil {
		os.RemoveAll(tmp)
		return nil, nil, err
	}
	tree.root = tmp
	return tree, removeDir(tmp), nil
}

type removeDir string

func (d removeDir) Close() error { return os.RemoveAll(string(d)) }

// listFS is a read-only fs.FS whose tree and sizes come from `restic ls`
// and whose file contents come from the partially restored root. Files that
// were not restored can be listed and stat'ed but not read.
type listFS struct {
	root  string
	nodes map[string]*node // by slash path relative to the world, "." is the root
}

type node struct {
	name     string
	dir      bool
	size     int64
	mtime    time.Time
	children []string
}

func parseListing(out []byte) (*listFS, error) {
	t := &listFS{nodes: map[string]*node{".": {name: ".", dir: true}}}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var n struct {
			Type        string    `json:"type"`
			Path        string    `json:"path"`
			Size        int64     `json:"size"`
			Mtime       time.Time `json:"mtime"`
			MessageType string    `json:"message_type"`
			StructType  string    `json:"struct_type"`
		}
		if json.Unmarshal(sc.Bytes(), &n) != nil || (n.MessageType != "node" && n.StructType != "node") {
			continue
		}
		p := strings.TrimPrefix(path.Clean(n.Path), "/")
		if p == "" || p == "." {
			continue
		}
		t.add(p, &node{name: path.Base(p), dir: n.Type == "dir", size: n.Size, mtime: n.Mtime})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("parse restic ls: %w", err)
	}
	for _, n := range t.nodes {
		sort.Strings(n.children)
	}
	return t, nil
}

// add inserts n and any missing parent directories.
func (t *listFS) add(p string, n *node) {
	if existing, ok := t.nodes[p]; ok {
		n.children = existing.children
	}
	t.nodes[p] = n
	for {
		parent := path.Dir(p)
		pn, ok := t.nodes[parent]
		if !ok {
			pn = &node{name: path.Base(parent), dir: true}
			t.nodes[parent] = pn
		}
		if !contains(pn.children, n.name) {
			pn.children = append(pn.children, n.name)
		}
		if ok || parent == "." {
			return
		}
		p, n = parent, pn
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (t *listFS) lookup(op, name string) (*node, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	n, ok := t.nodes[name]
	if !ok {
		return nil, &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
	}
	return n, nil
}

func (t *listFS) Open(name string) (fs.File, error) {
	n, err := t.lookup("open", name)
	if err != nil {
		return nil, err
	}
	if n.dir {
		return &dirFile{info: info{n}, fsys: t, path: name}, nil
	}
	if f, err := os.Open(filepath.Join(t.root, filepath.FromSlash(name))); err == nil {
		return f, nil
	}
	return &unreadFile{info: info{n}}, nil
}

func (t *listFS) Stat(name string) (fs.FileInfo, error) {
	n, err := t.lookup("stat", name)
	if err != nil {
		return nil, err
	}
	return info{n}, nil
}

func (t *listFS) ReadDir(name string) ([]fs.DirEntry, error) {
	n, err := t.lookup("readdir", name)
	if err != nil {
		return nil, err
	}
	if !n.dir {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: errors.New("not a directory")}
	}
	entries := make([]fs.DirEntry, 0, len(n.children))
	for _, c := range n.children {
		entries = append(entries, fs.FileInfoToDirEntry(info{t.nodes[path.Join(name, c)]}))
	}
	return entries, nil
}

type info struct{ n *node }

func (i info) Name() string       { return i.n.name }
func (i info) Size() int64        { return i.n.size }
func (i info) ModTime() time.Time { return i.n.mtime }
func (i info) IsDir() bool        { return i.n.dir }
func (i info) Sys() any           { return nil }
func (i info) Mode() fs.FileMode {
	if i.n.dir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}

type dirFile struct {
	info
	fsys    *listFS
	path    string
	entries []fs.DirEntry
	read    bool
}

func (d *dirFile) Stat() (fs.FileInfo, error) { return d.info, nil }
func (d *dirFile) Read([]byte) (int, error)   { return 0, errors.New("is a directory") }
func (d *dirFile) Close() error               { return nil }

func (d *dirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if !d.read {
		d.entries, _ = d.fsys.ReadDir(d.path)
		d.read = true
	}
	if n <= 0 {
		out := d.entries
		d.entries = nil
		return out, nil
	}
	if len(d.entries) == 0 {
		return nil, io.EOF
	}
	n = min(n, len(d.entries))
	out := d.entries[:n]
	d.entries = d.entries[n:]
	return out, nil
}

// unreadFile stands in for a file that was listed but not restored.
type unreadFile struct{ info }

func (f *unreadFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *unreadFile) Read([]byte) (int, error) {
	return 0, errors.New("file content not restored for preview")
}
func (f *unreadFile) Close() error { return nil }
