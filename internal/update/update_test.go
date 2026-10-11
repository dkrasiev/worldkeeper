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
