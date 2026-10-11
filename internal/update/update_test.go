package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"0.2.0", "0.1.0", true},
		{"0.1.1", "0.1.0", true},
		{"1.0.0", "0.9.9", true},
		{"0.10.0", "0.9.0", true},
		{"0.1.0", "0.1.0", false},
		{"0.1.0", "0.2.0", false},
		{"0.2.0", "0.2.0-dev.abc1234", true}, // the release a test build leads up to
		{"0.3.0", "0.3.0-develop.202610111530.abc1234", true},
		{"0.3.0-develop.202610120900.def5678", "0.3.0-develop.202610111530.abc1234", true}, // later commit
		{"0.3.0-develop.202610111530.abc1234", "0.3.0-develop.202610120900.def5678", false},
		{"0.3.0-develop.202610111530.abc1234", "0.2.0", true},
		{"0.1.0", "0.2.0-dev.abc1234", false},
		{"0.2.0-rc.2", "0.2.0-rc.1", true},
		{"0.2.0-rc.10", "0.2.0-rc.9", true},
		{"0.2.0-rc.1", "0.2.0-rc", true},
		{"v0.2.0", "0.1.0", true},
		{"0.2.0+build.5", "0.2.0", false},
	}
	for _, c := range cases {
		got, err := Newer(c.latest, c.current)
		if err != nil || got != c.want {
			t.Errorf("Newer(%q, %q) = %v, %v; want %v", c.latest, c.current, got, err, c.want)
		}
	}
	if _, err := Newer("0.2.0", "dev"); !errors.Is(err, ErrUnversioned) {
		t.Errorf("dev build: err = %v, want ErrUnversioned", err)
	}
	if _, err := Newer("latest", "0.1.0"); err == nil {
		t.Error("invalid latest version: no error")
	}
}

func TestLatest(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Write([]byte(`{"tag_name":"v0.3.0","html_url":"https://github.com/dkrasiev/worldkeeper/releases/tag/v0.3.0","body":"…"}`))
	}))
	defer srv.Close()

	rel, err := Checker{URL: srv.URL}.Latest(context.Background(), "0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "0.3.0" || rel.URL != "https://github.com/dkrasiev/worldkeeper/releases/tag/v0.3.0" {
		t.Errorf("release = %+v", rel)
	}
	if gotUA != "worldkeeper/0.2.0" {
		t.Errorf("User-Agent = %q", gotUA)
	}
}

func TestLatestErrors(t *testing.T) {
	for name, h := range map[string]http.HandlerFunc{
		"rate limited": func(w http.ResponseWriter, r *http.Request) { http.Error(w, "slow down", http.StatusForbidden) },
		"bad tag":      func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"tag_name":"nightly"}`)) },
		"not json":     func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`<html>`)) },
	} {
		srv := httptest.NewServer(h)
		if _, err := (Checker{URL: srv.URL}).Latest(context.Background(), "0.1.0"); err == nil {
			t.Errorf("%s: no error", name)
		}
		srv.Close()
	}
}

func TestDev(t *testing.T) {
	const page = `{"name":"Dev build 0.3.0-develop.202610111530.abc1234","html_url":"https://github.com/dkrasiev/worldkeeper/releases/tag/dev",
		"assets":[{"name":"checksums.txt","browser_download_url":"https://example.test/checksums.txt"},
		{"name":"worldkeeper_windows_amd64.zip","browser_download_url":"https://example.test/worldkeeper_windows_amd64.zip"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(page)) }))
	defer srv.Close()

	rel, err := Checker{DevURL: srv.URL}.Dev(context.Background(), "0.3.0-develop.202610100000.0000000")
	if err != nil {
		t.Fatal(err)
	}
	want := Release{
		Version:  "0.3.0-develop.202610111530.abc1234",
		URL:      "https://github.com/dkrasiev/worldkeeper/releases/tag/dev",
		Download: "https://example.test/worldkeeper_windows_amd64.zip",
		Dev:      true,
	}
	if rel != want {
		t.Errorf("release = %+v, want %+v", rel, want)
	}
}

func TestDevErrors(t *testing.T) {
	missing := httptest.NewServer(http.NotFoundHandler())
	defer missing.Close()
	if _, err := (Checker{DevURL: missing.URL}).Dev(context.Background(), "0.3.0"); !errors.Is(err, ErrNoDevBuild) {
		t.Errorf("no dev release: err = %v, want ErrNoDevBuild", err)
	}
	for name, page := range map[string]string{
		"no archive": `{"name":"Dev build 0.3.0-develop.1.abc","html_url":"https://example.test/dev","assets":[]}`,
		"bad title":  `{"name":"nightly","html_url":"https://example.test/dev","assets":[]}`,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(page)) }))
		if _, err := (Checker{DevURL: srv.URL}).Dev(context.Background(), "0.3.0"); err == nil || errors.Is(err, ErrNoDevBuild) {
			t.Errorf("%s: err = %v", name, err)
		}
		srv.Close()
	}
}
