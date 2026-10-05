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
	mux.HandleFunc("POST /api/restic/check", s.resticCheck)
	mux.HandleFunc("POST /api/restic/init", s.resticInit)
	mux.HandleFunc("GET /api/worlds/{id}", s.worldInfo)
	mux.HandleFunc("GET /api/worlds/{id}/icon", s.icon)
	mux.HandleFunc("GET /api/worlds/{id}/advancements", s.advancements)
	mux.HandleFunc("GET /api/worlds/{id}/snapshots", s.snapshots)
	mux.HandleFunc("POST /api/worlds/{id}/snapshots", s.createSnapshot)
	mux.HandleFunc("GET /api/worlds/{id}/snapshots/{snap}/info", s.snapshotInfo)
	mux.HandleFunc("GET /api/worlds/{id}/snapshots/{snap}/advancements", s.snapshotAdvancements)
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

func (s *Server) advancements(w http.ResponseWriter, r *http.Request) {
	progress, err := s.app.Advancements(r.PathValue("id"))
	respond(w, progress, err)
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
	ix, err := s.app.Snapshots().Get(r.PathValue("id"))
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

func (s *Server) snapshotInfo(w http.ResponseWriter, r *http.Request) {
	d, err := s.app.SnapshotDetails(r.PathValue("id"), r.PathValue("snap"))
	respond(w, d.Info, err)
}

func (s *Server) snapshotAdvancements(w http.ResponseWriter, r *http.Request) {
	d, err := s.app.SnapshotDetails(r.PathValue("id"), r.PathValue("snap"))
	respond(w, d.Advancements, err)
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

// settings is the user-editable part of the config. The API token and
// the restic password never leave the server; only "is it set" does.
type settings struct {
	Engine         string        `json:"engine"`
	StorageDir     string        `json:"storageDir"`
	Restic         config.Restic `json:"restic"`
	ExtraSavesDirs []string      `json:"extraSavesDirs"`
	KeepAuto       int           `json:"keepAuto"`
	AutoBackup     bool          `json:"autoBackup"`
	PollSeconds    int           `json:"pollSeconds"`

	ResticPasswordSet bool `json:"resticPasswordSet"` // response only
	// ResticPassword is write-only: when non-empty it replaces the stored password.
	ResticPassword string `json:"resticPassword,omitempty"`
}

func (s *Server) toSettings(c config.Config) settings {
	pwSet := c.Restic.PasswordFile != ""
	if !pwSet && c.Restic.Repo != "" {
		_, err := s.app.ResticPassword(c.Restic.Repo)
		pwSet = err == nil
	}
	return settings{
		Engine:            c.Engine,
		StorageDir:        c.StorageDir,
		Restic:            c.Restic,
		ExtraSavesDirs:    c.ExtraSavesDirs,
		KeepAuto:          c.KeepAuto,
		AutoBackup:        c.AutoBackup,
		PollSeconds:       int(c.PollInterval.Seconds()),
		ResticPasswordSet: pwSet,
	}
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.toSettings(s.app.Config.Get()))
}

func (s *Server) resticCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.app.Restic().Check(r.Context()))
}

func (s *Server) resticInit(w http.ResponseWriter, r *http.Request) {
	store := s.app.Restic()
	if st := store.Check(r.Context()); st.State != "missing" {
		writeAPIError(w, &apiError{status: http.StatusConflict, code: "not_missing", key: "repo_not_missing",
			params: map[string]string{"state": st.State}, msg: "repository cannot be initialized: " + st.State + " " + st.Message})
		return
	}
	if err := store.Init(r.Context()); err != nil {
		respond(w, nil, err)
		return
	}
	writeJSON(w, http.StatusOK, store.Check(r.Context()))
}

func (s *Server) putConfig(w http.ResponseWriter, r *http.Request) {
	var in settings
	if !decode(w, r, &in) {
		return
	}
	in.Restic.Repo = strings.TrimSpace(in.Restic.Repo)
	in.Restic.Binary = strings.TrimSpace(in.Restic.Binary)
	in.Restic.PasswordFile = strings.TrimSpace(in.Restic.PasswordFile)
	if in.Engine != config.EngineZip && in.Engine != config.EngineRestic {
		writeAPIError(w, badRequest("engine_unknown", "unknown storage engine"))
		return
	}
	if in.Engine == config.EngineRestic && in.Restic.Repo == "" {
		writeAPIError(w, badRequest("restic_repo_required", "enter the restic repository"))
		return
	}
	if in.ResticPassword != "" {
		if in.Restic.Repo == "" {
			writeAPIError(w, badRequest("restic_repo_before_password", "enter the restic repository before its password"))
			return
		}
		if err := s.app.Secrets.Set(app.ResticSecretKey(in.Restic.Repo), in.ResticPassword); err != nil {
			writeAPIError(w, &apiError{status: http.StatusInternalServerError, code: "internal", key: "keychain_failed",
				params: map[string]string{"error": err.Error()},
				msg:    "cannot save the password in the system keychain: " + err.Error() + ". Use a password file instead."})
			return
		}
	}
	cfg, err := s.app.Config.Update(func(c *config.Config) error {
		if in.Engine == config.EngineZip && !filepath.IsAbs(in.StorageDir) {
			return badRequest("storage_not_absolute", "storage folder must be an absolute path")
		}
		if in.KeepAuto < 1 {
			return badRequest("keep_auto_min", "keep at least one automatic snapshot")
		}
		if in.PollSeconds < 5 {
			return badRequest("poll_min", "check interval must be at least 5 seconds")
		}
		dirs := []string{}
		for _, d := range in.ExtraSavesDirs {
			d = strings.TrimSpace(d)
			if d == "" {
				continue
			}
			if !filepath.IsAbs(d) {
				return badRequest("extra_not_absolute", "extra folder must be an absolute path: "+d, "path", d)
			}
			dirs = append(dirs, d)
		}
		if in.Engine == config.EngineZip {
			if err := os.MkdirAll(in.StorageDir, 0o755); err != nil {
				return badRequest("storage_unusable", "cannot use storage folder: "+err.Error(), "error", err.Error())
			}
			c.StorageDir = filepath.Clean(in.StorageDir)
		}
		c.Engine = in.Engine
		c.Restic = in.Restic
		c.ExtraSavesDirs = dirs
		c.KeepAuto = in.KeepAuto
		c.AutoBackup = in.AutoBackup
		c.PollInterval = config.Duration{Duration: time.Duration(in.PollSeconds) * time.Second}
		return nil
	})
	respond(w, s.toSettings(cfg), err)
}

// apiError is a client-facing error. key and params let the UI show it in
// the user's language; msg is the English text for logs and old clients.
type apiError struct {
	status int
	code   string // error category, e.g. "bad_request"
	key    string // translation key, e.g. "poll_min"
	params map[string]string
	msg    string
}

func (e *apiError) Error() string { return e.msg }

// badRequest builds a 400 error; kv are alternating param names and values.
func badRequest(key, msg string, kv ...string) *apiError {
	e := &apiError{status: http.StatusBadRequest, code: "bad_request", key: key, msg: msg}
	if len(kv) > 0 {
		e.params = map[string]string{}
		for i := 0; i+1 < len(kv); i += 2 {
			e.params[kv[i]] = kv[i+1]
		}
	}
	return e
}

func writeAPIError(w http.ResponseWriter, e *apiError) {
	body := map[string]any{"error": e.code, "message": e.msg}
	if e.key != "" {
		body["key"] = e.key
	}
	if len(e.params) > 0 {
		body["params"] = e.params
	}
	writeJSON(w, e.status, body)
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeAPIError(w, badRequest("invalid_json", "invalid JSON: "+err.Error(), "error", err.Error()))
		return false
	}
	return true
}

func respond(w http.ResponseWriter, v any, err error) {
	var ae *apiError
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, v)
	case errors.As(err, &ae):
		writeAPIError(w, ae)
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
