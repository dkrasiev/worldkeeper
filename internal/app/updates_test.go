package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/config"
	"github.com/dkrasiev/worldkeeper/internal/update"
)

func TestCheckUpdate(t *testing.T) {
	f := setup(t)
	var fail atomic.Bool
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if fail.Load() {
			http.Error(w, "offline", http.StatusBadGateway)
			return
		}
		w.Write([]byte(`{"tag_name":"v0.3.0","html_url":"https://example.test/v0.3.0"}`))
	}))
	defer srv.Close()
	f.app.UpdateChecker = update.Checker{URL: srv.URL}
	f.app.Version = "0.2.0"
	now := time.Now()

	if !f.app.updateDue(now) {
		t.Fatal("first check is not due")
	}
	st := f.app.CheckUpdate(context.Background())
	if !st.Available || st.Latest == nil || st.Latest.Version != "0.3.0" || st.CheckedAt.IsZero() || st.Error != "" {
		t.Fatalf("status = %+v", st)
	}
	if f.app.updateDue(now.Add(time.Hour)) || !f.app.updateDue(now.Add(25*time.Hour)) {
		t.Error("next check should be due a day later")
	}

	// A failed check keeps the last known release and retries sooner.
	fail.Store(true)
	st = f.app.CheckUpdate(context.Background())
	if st.Error == "" || !st.Available || st.Latest.Version != "0.3.0" {
		t.Fatalf("after failure: %+v", st)
	}
	if !f.app.updateDue(time.Now().Add(2 * time.Hour)) {
		t.Error("a failed check should be retried within hours")
	}

	f.app.Config.Update(func(c *config.Config) error { c.CheckUpdates = false; return nil })
	if f.app.updateDue(time.Now().Add(48 * time.Hour)) {
		t.Error("check is due although turned off")
	}

	// Local builds never ask GitHub.
	f.app.Version = "dev"
	before := requests.Load()
	if st := f.app.CheckUpdate(context.Background()); st.Versioned || st.Available {
		t.Errorf("dev build status = %+v", st)
	}
	if requests.Load() != before {
		t.Error("dev build asked GitHub")
	}
}

func TestUpdateChannel(t *testing.T) {
	f := setup(t)
	defer func(was bool) { update.DevBuilds = was }(update.DevBuilds)
	update.DevBuilds = true

	stable := "v0.2.0"
	var devTitle atomic.Value
	devTitle.Store("Dev build 0.3.0-develop.202610111530.abc1234")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dev" {
			w.Write([]byte(`{"name":"` + devTitle.Load().(string) + `","html_url":"https://example.test/dev",
				"assets":[{"name":"worldkeeper_windows_amd64.zip","browser_download_url":"https://example.test/dev.zip"}]}`))
			return
		}
		w.Write([]byte(`{"tag_name":"` + stable + `","html_url":"https://example.test/` + stable + `"}`))
	}))
	defer srv.Close()
	f.app.UpdateChecker = update.Checker{URL: srv.URL + "/latest", DevURL: srv.URL + "/dev"}
	f.app.Version = "0.2.0"
	setChannel := func(ch string) {
		f.app.Config.Update(func(c *config.Config) error { c.UpdateChannel = ch; return nil })
	}

	// Stable: the test build is not offered.
	if st := f.app.CheckUpdate(context.Background()); st.Available || st.Latest.Dev {
		t.Fatalf("stable channel: %+v", st.Latest)
	}

	// Switching channels makes a check due at once and hides the old result.
	setChannel(config.ChannelDev)
	if !f.app.updateDue(time.Now()) || f.app.UpdateStatus().Latest != nil {
		t.Fatal("channel switch: check not due, or stale result shown")
	}
	st := f.app.CheckUpdate(context.Background())
	if !st.Available || !st.Latest.Dev || st.Latest.Download != "https://example.test/dev.zip" {
		t.Fatalf("dev channel: %+v", st.Latest)
	}

	// A test build of the same version is offered when its commit is newer.
	f.app.Version = "0.3.0-develop.202610100000.0000000"
	if st := f.app.UpdateStatus(); !st.Available {
		t.Error("newer test build not offered")
	}
	// A release newer than the test build wins.
	stable = "v0.3.0"
	if st := f.app.CheckUpdate(context.Background()); !st.Available || st.Latest.Dev || st.Latest.Version != "0.3.0" {
		t.Errorf("newer release: %+v", st.Latest)
	}

	// Where no test builds are made, the dev channel acts as stable.
	update.DevBuilds = false
	if f.app.updateChannel() != config.ChannelStable {
		t.Error("dev channel without test builds for this platform")
	}
}
