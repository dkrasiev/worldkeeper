package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/app"
	"github.com/dkrasiev/worldkeeper/internal/config"
	"github.com/dkrasiev/worldkeeper/internal/discovery"
	"github.com/dkrasiev/worldkeeper/internal/secrets"
	"github.com/dkrasiev/worldkeeper/internal/testworld"
)

func newServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	cfg, err := config.Open(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	storage := t.TempDir()
	cfg.Update(func(c *config.Config) error { c.StorageDir = storage; return nil })
	env := discovery.Env{GOOS: "linux", Home: t.TempDir()}
	testworld.Create(t, discovery.DefaultSavesDir(env), "w", testworld.Options{})
	a := app.New(cfg, &secrets.Memory{}, env, slog.New(slog.NewTextHandler(io.Discard, nil)))
	a.Version = "test-version"
	ui := fstest.MapFS{"index.html": {Data: []byte("<html>ui</html>")}}
	return New(a, ui), cfg.Get().Token
}

func do(h http.Handler, method, url, host, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Host = host
	if token != "" {
		req.Header.Set(tokenHeader, token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestGuard(t *testing.T) {
	h, token := newServer(t)
	cases := []struct {
		name, host, token string
		want              int
	}{
		{"ok", "127.0.0.1:25599", token, 200},
		{"localhost ok", "localhost:25599", token, 200},
		{"no token", "127.0.0.1:25599", "", 401},
		{"wrong token", "127.0.0.1:25599", "nope", 401},
		{"dns rebinding", "evil.example:25599", token, 403},
	}
	for _, c := range cases {
		if rec := do(h, "GET", "/api/overview", c.host, c.token, ""); rec.Code != c.want {
			t.Errorf("%s: status %d, want %d", c.name, rec.Code, c.want)
		}
	}
	if rec := do(h, "GET", "/api/worlds/minecraft--w/icon?token="+token, "127.0.0.1", "", ""); rec.Code != 200 {
		t.Errorf("icon via query token: %d", rec.Code)
	}
	if rec := do(h, "GET", "/some/client/route", "127.0.0.1", "", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), "ui") {
		t.Errorf("spa fallback: %d %q", rec.Code, rec.Body.String())
	}
}

func TestResticSettingsNeverEchoPassword(t *testing.T) {
	h, token := newServer(t)
	host := "127.0.0.1:25599"

	if rec := do(h, "PUT", "/api/config", host, token, `{"engine":"restic","restic":{"repo":""},"keepAuto":5,"pollSeconds":15}`); rec.Code != 400 {
		t.Errorf("restic without repo accepted: %d", rec.Code)
	}
	body := `{"engine":"restic","restic":{"repo":"/tmp/wk-repo"},"resticPassword":"s3cret","keepAuto":5,"pollSeconds":15}`
	rec := do(h, "PUT", "/api/config", host, token, body)
	if rec.Code != 200 {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	rec = do(h, "GET", "/api/config", host, token, "")
	if !strings.Contains(rec.Body.String(), `"resticPasswordSet":true`) || strings.Contains(rec.Body.String(), "s3cret") {
		t.Errorf("config = %s", rec.Body)
	}
}

func TestAbout(t *testing.T) {
	h, token := newServer(t)
	rec := do(h, "GET", "/api/about", "127.0.0.1:25599", token, "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"version":"test-version"`) {
		t.Fatalf("about: %d %s", rec.Code, rec.Body)
	}
}

func TestSnapshotFlow(t *testing.T) {
	h, token := newServer(t)
	host := "127.0.0.1:25599"

	rec := do(h, "POST", "/api/worlds/minecraft--w/snapshots", host, token, `{"label":"before dragon"}`)
	if rec.Code != 202 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	job := waitJob(t, h, token, rec)
	if job.State != app.JobDone || job.Snapshot == nil || job.Snapshot.Label != "before dragon" {
		t.Fatalf("backup job = %+v", job)
	}

	rec = do(h, "POST", "/api/worlds/minecraft--w/snapshots/"+job.Snapshot.ID+"/restore", host, token, `{"mode":"copy"}`)
	if rec.Code != 202 {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body)
	}
	if job := waitJob(t, h, token, rec); job.State != app.JobDone || !strings.Contains(job.Path, "(restored ") || job.Done != job.Total || job.Total == 0 {
		t.Fatalf("restore job = %+v", job)
	}
	// Mistakes the user must see right away are not deferred to the job.
	if rec := do(h, "POST", "/api/worlds/minecraft--w/snapshots/nope/restore", host, token, `{"mode":"copy"}`); rec.Code != 404 {
		t.Errorf("restore unknown snapshot: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "POST", "/api/worlds/nope/snapshots", host, token, `{}`); rec.Code != 404 {
		t.Errorf("save unknown world: %d %s", rec.Code, rec.Body)
	}
	rec = do(h, "GET", "/api/worlds/minecraft--w", host, token, "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"gameVersion":"1.21.4"`) {
		t.Fatalf("info: %d %s", rec.Code, rec.Body)
	}
	rec = do(h, "GET", "/api/worlds/minecraft--w/advancements", host, token, "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"minecraft:story/root":true`) {
		t.Fatalf("advancements: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "GET", "/api/worlds/nope", host, token, ""); rec.Code != 404 {
		t.Errorf("unknown world: %d", rec.Code)
	}
	// Regression: an empty list must be [], not null, or the settings page crashes.
	if rec := do(h, "GET", "/api/config", host, token, ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"extraSavesDirs":[]`) {
		t.Errorf("config: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "PUT", "/api/config", host, token, `{"engine":"zip","storageDir":"relative","keepAuto":5,"pollSeconds":15}`); rec.Code != 400 || !strings.Contains(rec.Body.String(), `"key":"storage_not_absolute"`) {
		t.Errorf("relative storage dir: %d %s", rec.Code, rec.Body)
	}
}

// waitJob polls /api/jobs until the job started by rec has finished.
func waitJob(t *testing.T, h http.Handler, token string, rec *httptest.ResponseRecorder) app.Job {
	t.Helper()
	var started app.Job
	if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil || started.ID == "" {
		t.Fatalf("start response %s: %v", rec.Body, err)
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		var jobs []app.Job
		json.Unmarshal(do(h, "GET", "/api/jobs", "127.0.0.1", token, "").Body.Bytes(), &jobs)
		for _, j := range jobs {
			if j.ID == started.ID && j.State != app.JobRunning {
				return j
			}
		}
	}
	t.Fatalf("job %s did not finish", started.ID)
	return app.Job{}
}

func TestRenameAndDeleteWorld(t *testing.T) {
	h, token := newServer(t)
	host := "127.0.0.1:25599"
	if rec := do(h, "PATCH", "/api/worlds/minecraft--w", host, token, `{"name":"  "}`); rec.Code != 400 || !strings.Contains(rec.Body.String(), `"key":"name_required"`) {
		t.Errorf("empty name: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "PATCH", "/api/worlds/minecraft--w", host, token, `{"name":"Renamed"}`); rec.Code != 200 {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "GET", "/api/worlds/minecraft--w", host, token, ""); !strings.Contains(rec.Body.String(), `"name":"Renamed"`) {
		t.Errorf("info after rename: %s", rec.Body)
	}
	rec := do(h, "DELETE", "/api/worlds/minecraft--w", host, token, "")
	if rec.Code != 202 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if job := waitJob(t, h, token, rec); job.State != app.JobDone {
		t.Fatalf("delete job = %+v", job)
	}
	if rec := do(h, "GET", "/api/worlds/minecraft--w", host, token, ""); rec.Code != 404 {
		t.Errorf("world after delete: %d", rec.Code)
	}
	if rec := do(h, "GET", "/api/overview", host, token, ""); !strings.Contains(rec.Body.String(), `"kind":"pre-delete"`) {
		t.Errorf("overview after delete: %s", rec.Body)
	}
}
