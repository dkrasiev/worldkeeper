// Package api serves the local HTTP API and the embedded web UI.
package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/app"
	"github.com/dkrasiev/worldkeeper/internal/config"
	"github.com/dkrasiev/worldkeeper/internal/snapshot"
)

const tokenHeader = "X-Worldkeeper-Token"

type Server struct {
	app *app.App
	ui  fs.FS
}

// New returns the HTTP handler. ui is the built web app (index.html at its root).
func New(a *app.App, ui fs.FS) http.Handler {
	s := &Server{app: a, ui: ui}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/overview", s.overview)
	mux.HandleFunc("GET /api/events", s.events)
	mux.HandleFunc("GET /api/config", s.getConfig)
	mux.HandleFunc("PUT /api/config", s.putConfig)
	mux.HandleFunc("GET /api/worlds/{id}", s.worldInfo)
	mux.HandleFunc("GET /api/worlds/{id}/icon", s.icon)
	mux.HandleFunc("GET /api/worlds/{id}/snapshots", s.snapshots)
	mux.HandleFunc("POST /api/worlds/{id}/snapshots", s.createSnapshot)
	mux.HandleFunc("POST /api/worlds/{id}/snapshots/{snap}/restore", s.restore)
	mux.HandleFunc("DELETE /api/worlds/{id}/snapshots/{snap}", s.deleteSnapshot)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "unknown endpoint")
	})
	mux.Handle("/", spa(ui))

	return s.guard(mux)
}

// guard protects the API from the two realistic attacks on a localhost
// service: DNS rebinding (checked via Host) and cross-site requests from
// any open web page (checked via the secret token).
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopbackHost(r.Host) {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			token := r.Header.Get(tokenHeader)
			if token == "" && r.Method == http.MethodGet {
				token = r.URL.Query().Get("token") // <img src> cannot send headers
			}
			want := s.app.Config.Get().Token
			if subtle.ConstantTimeCompare([]byte(token), []byte(want)) != 1 {
				writeError(w, http.StatusUnauthorized, "unauthorized", "missing or wrong token; open worldkeeper from its own link")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func loopbackHost(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	ov, err := s.app.Overview()
	respond(w, ov, err)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.app.Events())
}

func (s *Server) worldInfo(w http.ResponseWriter, r *http.Request) {
	info, err := s.app.Info(r.PathValue("id"))
	respond(w, info, err)
}

func (s *Server) icon(w http.ResponseWriter, r *http.Request) {
	path, err := s.app.IconPath(r.PathValue("id"))
	if err != nil {
		respond(w, nil, err)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "image/png")
	http.ServeFile(w, r, path)
}

func (s *Server) snapshots(w http.ResponseWriter, r *http.Request) {
	ix, err := s.app.Store.Get(r.PathValue("id"))
	respond(w, ix, err)
}

func (s *Server) createSnapshot(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Label string `json:"label"`
		Note  string `json:"note"`
		Force bool   `json:"force"`
	}
	if !decode(w, r, &body) {
		return
	}
	snap, err := s.app.Backup(r.PathValue("id"), app.BackupOptions{
		Kind:  snapshot.KindManual,
		Label: strings.TrimSpace(body.Label),
		Note:  strings.TrimSpace(body.Note),
		Force: body.Force,
	})
	respond(w, snap, err)
}

func (s *Server) restore(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode app.RestoreMode `json:"mode"`
	}
	if !decode(w, r, &body) {
		return
	}
	dest, err := s.app.Restore(r.PathValue("id"), r.PathValue("snap"), body.Mode)
	respond(w, map[string]string{"path": dest}, err)
}

func (s *Server) deleteSnapshot(w http.ResponseWriter, r *http.Request) {
	err := s.app.DeleteSnapshot(r.PathValue("id"), r.PathValue("snap"))
	respond(w, map[string]bool{"ok": true}, err)
}

// settings is the user-editable part of the config; the token stays server side.
type settings struct {
	StorageDir     string   `json:"storageDir"`
	ExtraSavesDirs []string `json:"extraSavesDirs"`
	KeepAuto       int      `json:"keepAuto"`
	AutoBackup     bool     `json:"autoBackup"`
	PollSeconds    int      `json:"pollSeconds"`
}

func toSettings(c config.Config) settings {
	return settings{
		StorageDir:     c.StorageDir,
		ExtraSavesDirs: c.ExtraSavesDirs,
		KeepAuto:       c.KeepAuto,
		AutoBackup:     c.AutoBackup,
		PollSeconds:    int(c.PollInterval.Seconds()),
	}
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, toSettings(s.app.Config.Get()))
}

func (s *Server) putConfig(w http.ResponseWriter, r *http.Request) {
	var in settings
	if !decode(w, r, &in) {
		return
	}
	cfg, err := s.app.Config.Update(func(c *config.Config) error {
		if !filepath.IsAbs(in.StorageDir) {
			return badRequest("storage folder must be an absolute path")
		}
		if in.KeepAuto < 1 {
			return badRequest("keep at least one automatic snapshot")
		}
		if in.PollSeconds < 5 {
			return badRequest("check interval must be at least 5 seconds")
		}
		dirs := []string{}
		for _, d := range in.ExtraSavesDirs {
			d = strings.TrimSpace(d)
			if d == "" {
				continue
			}
			if !filepath.IsAbs(d) {
				return badRequest("extra folder must be an absolute path: " + d)
			}
			dirs = append(dirs, d)
		}
		if err := os.MkdirAll(in.StorageDir, 0o755); err != nil {
			return badRequest("cannot use storage folder: " + err.Error())
		}
		c.StorageDir = filepath.Clean(in.StorageDir)
		c.ExtraSavesDirs = dirs
		c.KeepAuto = in.KeepAuto
		c.AutoBackup = in.AutoBackup
		c.PollInterval = config.Duration{Duration: time.Duration(in.PollSeconds) * time.Second}
		return nil
	})
	respond(w, toSettings(cfg), err)
}

type badRequest string

func (b badRequest) Error() string { return string(b) }

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON: "+err.Error())
		return false
	}
	return true
}

func respond(w http.ResponseWriter, v any, err error) {
	var br badRequest
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, v)
	case errors.As(err, &br):
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
	case errors.Is(err, app.ErrWorldNotFound), errors.Is(err, snapshot.ErrNotFound), errors.Is(err, fs.ErrNotExist):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, app.ErrInUse):
		writeError(w, http.StatusConflict, "in_use", err.Error())
	case errors.Is(err, app.ErrUnchanged):
		writeError(w, http.StatusConflict, "unchanged", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": code, "message": msg})
}

// spa serves static files and falls back to index.html for client routes.
func spa(ui fs.FS) http.Handler {
	files := http.FileServerFS(ui)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name != "" {
			if _, err := fs.Stat(ui, name); err == nil {
				files.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(ui, "index.html")
		if err != nil {
			http.Error(w, "web UI is not built; run `make web`", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}
