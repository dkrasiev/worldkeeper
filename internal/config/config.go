// Package config loads and saves worldkeeper settings.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Engines decide how snapshots are stored.
const (
	EngineZip    = "zip"    // one zip file per snapshot in StorageDir
	EngineRestic = "restic" // a restic repository; restic must be installed
)

type Config struct {
	// Engine is EngineZip or EngineRestic.
	Engine string `json:"engine"`
	// Restic configures the restic engine. The password is kept in the OS
	// credential store, never in this file.
	Restic Restic `json:"restic"`
	// StorageDir is where zip snapshots are written: a local folder, a mounted
	// network share (\\NAS\backups) or a cloud-sync folder.
	StorageDir string `json:"storageDir"`
	// ExtraSavesDirs are additional "saves" folders to scan.
	ExtraSavesDirs []string `json:"extraSavesDirs"`
	// KeepAuto is how many automatic snapshots to keep per world.
	// Manual snapshots are never rotated.
	KeepAuto int `json:"keepAuto"`
	// AutoBackup enables the backup-on-world-close hook.
	AutoBackup bool `json:"autoBackup"`
	// PollInterval is how often the watcher checks session.lock files.
	PollInterval Duration `json:"pollInterval"`
	// Listen is the HTTP listen address. Must be a loopback address.
	Listen string `json:"listen"`
	// Token protects the local API from other websites and local processes.
	Token string `json:"token"`
}

type Restic struct {
	// Repo is anything restic accepts: a path, sftp:user@host:/path, rest:http://...
	Repo string `json:"repo"`
	// Binary is the restic executable; empty means "restic" from PATH.
	Binary string `json:"binary"`
	// PasswordFile, when set, is used instead of the OS credential store
	// (useful on Linux without a Secret Service).
	PasswordFile string `json:"passwordFile"`
}

type Duration struct{ time.Duration }

func (d Duration) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = v
	return nil
}

func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Engine:         EngineZip,
		StorageDir:     filepath.Join(home, "WorldkeeperBackups"),
		ExtraSavesDirs: []string{},
		KeepAuto:       20,
		AutoBackup:     true,
		PollInterval:   Duration{15 * time.Second},
		Listen:         "127.0.0.1:25599",
	}
}

// DefaultPath returns <user config dir>/worldkeeper/config.json.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "worldkeeper", "config.json"), nil
}

// Store holds the current config and persists changes.
type Store struct {
	path string
	mu   sync.RWMutex
	cfg  Config
}

// Open loads the config at path, creating it with defaults if missing.
func Open(path string) (*Store, error) {
	s := &Store{path: path, cfg: Default()}
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(b, &s.cfg); err != nil {
			return nil, err
		}
	}
	if s.cfg.Engine == "" {
		s.cfg.Engine = EngineZip // configs written before engines existed
	}
	if s.cfg.ExtraSavesDirs == nil {
		s.cfg.ExtraSavesDirs = []string{} // "null" in a hand-edited file
	}
	if s.cfg.Token == "" {
		s.cfg.Token = newToken()
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := s.cfg
	// Copy into a non-nil slice: a nil slice encodes as JSON null, not [].
	c.ExtraSavesDirs = append([]string{}, s.cfg.ExtraSavesDirs...)
	return c
}

// Update applies fn to a copy of the config and saves it.
func (s *Store) Update(fn func(*Config) error) (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cfg
	if err := fn(&next); err != nil {
		return s.cfg, err
	}
	prev := s.cfg
	s.cfg = next
	if err := s.save(); err != nil {
		s.cfg = prev
		return prev, err
	}
	return next, nil
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func newToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
