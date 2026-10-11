package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

// devPage is a test build's release as GitHub returns it.
func devPage(tag, version string) string {
	return `{"tag_name":"` + tag + `","name":"Dev build ` + version + `","prerelease":true,
		"html_url":"https://github.com/dkrasiev/worldkeeper/releases/tag/` + tag + `",
		"assets":[{"name":"checksums.txt","browser_download_url":"https://example.test/checksums.txt"},
		{"name":"worldkeeper_windows_amd64.zip","browser_download_url":"https://example.test/` + tag + `.zip"}]}`
}

func TestDev(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Write([]byte(devPage("dev-feat-autostart", "0.3.0-feat-autostart.202610111530.abc1234")))
	}))
	defer srv.Close()

	rel, err := Checker{URL: srv.URL}.Dev(context.Background(), "0.3.0-develop.202610100000.0000000", "feat-autostart")
	if err != nil {
		t.Fatal(err)
	}
	want := Release{
		Version:  "0.3.0-feat-autostart.202610111530.abc1234",
		URL:      "https://github.com/dkrasiev/worldkeeper/releases/tag/dev-feat-autostart",
		Download: "https://example.test/dev-feat-autostart.zip",
		Dev:      true,
		Branch:   "feat-autostart",
	}
	if rel != want || path != "/tags/dev-feat-autostart" {
		t.Errorf("release = %+v from %s, want %+v", rel, path, want)
	}
	if DevTag(DevBranch) != "dev" {
		t.Errorf("develop's tag = %q", DevTag(DevBranch))
	}
}

func TestDevErrors(t *testing.T) {
	missing := httptest.NewServer(http.NotFoundHandler())
	defer missing.Close()
	if _, err := (Checker{URL: missing.URL}).Dev(context.Background(), "0.3.0", "develop"); !errors.Is(err, ErrNoDevBuild) {
		t.Errorf("no dev release: err = %v, want ErrNoDevBuild", err)
	}
	for name, page := range map[string]string{
		"no archive": `{"name":"Dev build 0.3.0-develop.202610111530.abc1234","html_url":"https://example.test/dev","assets":[]}`,
		"bad title":  `{"name":"nightly","html_url":"https://example.test/dev","assets":[]}`,
		"no time":    `{"name":"Dev build 0.3.0-dev.abc1234","html_url":"https://example.test/dev","assets":[]}`,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(page)) }))
		if _, err := (Checker{URL: srv.URL}).Dev(context.Background(), "0.3.0", "develop"); err == nil || errors.Is(err, ErrNoDevBuild) {
			t.Errorf("%s: err = %v", name, err)
		}
		srv.Close()
	}
}

func TestBuilds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[` +
			devPage("dev-fix-tray", "0.3.0-fix-tray.202610111200.1111111") + `,` +
			`{"tag_name":"v0.2.0","name":"Worldkeeper v0.2.0","prerelease":false,"html_url":"https://example.test/v0.2.0","assets":[]},` +
			devPage("dev", "0.3.0-develop.202610111300.2222222") + `,` +
			devPage("dev-0.3.0-develop.202610111300.2222222", "0.3.0-develop.202610111300.2222222") + `,` +
			devPage("dev-0.3.0-develop.202610101000.4444444", "0.3.0-develop.202610101000.4444444") + `,` +
			devPage("dev-0.3.0-develop.202610111000.5555555", "0.3.0-develop.202610111000.5555555") + `,` +
			devPage("dev-feat-autostart", "0.3.0-feat-autostart.202610111400.3333333") + `,` +
			`{"tag_name":"dev-broken","name":"Dev build ???","prerelease":true,"html_url":"https://example.test/x","assets":[]}` +
			`]`))
	}))
	defer srv.Close()

	builds, err := Checker{URL: srv.URL}.Builds(context.Background(), "0.3.0")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, b := range builds {
		got = append(got, b.Branch+"@"+b.Version[len(b.Version)-7:])
	}
	want := "develop@2222222,develop@5555555,develop@4444444,feat-autostart@3333333,fix-tray@1111111"
	if strings.Join(got, ",") != want {
		t.Errorf("builds = %s, want %s: develop newest first and once, then branches by name, releases and broken builds left out", strings.Join(got, ","), want)
	}
}

func TestOffer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"0.3.0-develop.202610120900.def5678", "0.3.0-develop.202610111530.abc1234", true},
		{"0.3.0-develop.202610111530.abc1234", "0.3.0-develop.202610120900.def5678", false},
		{"0.3.0-develop.202610111530.abc1234", "0.3.0-develop.202610111530.abc1234", false},
		// Another branch the user switched to, even with an older commit.
		{"0.3.0-feat-autostart.202610100000.abc1234", "0.3.0-develop.202610111530.def5678", true},
		// Not a test build of an older version, though.
		{"0.2.0-feat-old.202609010000.abc1234", "0.3.0-develop.202610111530.def5678", false},
		{"0.3.0", "0.3.0-feat-autostart.202610111530.abc1234", true},
		{"0.3.0-develop.202610111530.abc1234", "0.3.0", false},
		{"0.3.0-develop.202610111530.abc1234", "0.2.0", true},
	}
	for _, c := range cases {
		got, err := Offer(c.latest, c.current)
		if err != nil || got != c.want {
			t.Errorf("Offer(%q, %q) = %v, %v; want %v", c.latest, c.current, got, err, c.want)
		}
	}
}
