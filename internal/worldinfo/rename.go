package worldinfo

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Tnze/go-mc/nbt"
)

// SetLevelName changes the world name shown in the game's world list. Only
// the LevelName tag is replaced: every other tag is copied as raw bytes, so
// data this package does not understand survives the rewrite. level.dat_old,
// the game's fallback copy, is renamed too when present.
func SetLevelName(dir, name string) error {
	if err := setLevelName(filepath.Join(dir, "level.dat"), name); err != nil {
		return err
	}
	err := setLevelName(filepath.Join(dir, "level.dat_old"), name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func setLevelName(path, name string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	gzipped := len(raw) > 0 && raw[0] == 0x1f
	var r io.Reader = bytes.NewReader(raw)
	if gzipped {
		gz, err := gzip.NewReader(r)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		defer gz.Close()
		r = gz
	}

	var root map[string]nbt.RawMessage
	rootName, err := nbt.NewDecoder(bufio.NewReader(r)).Decode(&root)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	dataMsg, ok := root["Data"]
	if !ok || dataMsg.Type != nbt.TagCompound {
		return fmt.Errorf("%s: no Data compound", path)
	}
	var data map[string]nbt.RawMessage
	if err := dataMsg.Unmarshal(&data); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	newData := map[string]any{}
	for k, v := range data {
		newData[k] = v
	}
	newData["LevelName"] = name
	newRoot := map[string]any{}
	for k, v := range root {
		newRoot[k] = v
	}
	newRoot["Data"] = newData

	var out bytes.Buffer
	var w io.Writer = &out
	var gz *gzip.Writer
	if gzipped {
		gz = gzip.NewWriter(&out)
		w = gz
	}
	if err := nbt.NewEncoder(w).Encode(newRoot, rootName); err != nil {
		return err
	}
	if gz != nil {
		if err := gz.Close(); err != nil {
			return err
		}
	}
	return writeFileAtomic(path, out.Bytes())
}

// writeFileAtomic replaces path only once the new content is fully on disk.
func writeFileAtomic(path string, b []byte) error {
	tmp := path + ".worldkeeper-tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
