// Package update finds out whether a newer Worldkeeper release is published.
// It only reads the GitHub release list; installing stays with the user.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// ReleasesURL is the GitHub API for the repository's releases.
	ReleasesURL = "https://api.github.com/repos/dkrasiev/worldkeeper/releases"

	// DevBranch is the branch every feature lands in. Its test builds are the
	// "dev" pre-release; a pull request's branch gets "dev-<branch>" (see
	// .github/workflows/dev-build.yml).
	DevBranch = "develop"
	devTag    = "dev"
	// devTitle prefixes a test build's release title; the version follows.
	devTitle = "Dev build "
	// DevAsset is the archive in a test build's pre-release. Test builds are
	// made for Windows x64 only.
	DevAsset = "worldkeeper_windows_amd64.zip"
)

// DevBuilds reports whether test builds are made for this platform.
var DevBuilds = runtime.GOOS == "windows" && runtime.GOARCH == "amd64"

// ErrNoDevBuild is returned by Dev when the branch has no test build
// published, e.g. after its pull request was merged or closed.
var ErrNoDevBuild = errors.New("no test build is published")

// Release is a published version.
type Release struct {
	Version string `json:"version"` // without the leading "v", e.g. "0.3.0"
	URL     string `json:"url"`     // release notes and downloads
	// Download links straight to the archive for this platform; set for
	// test builds, which have just one.
	Download string `json:"download,omitempty"`
	// Dev marks a test build; Branch is the branch it was built from.
	Dev    bool   `json:"dev,omitempty"`
	Branch string `json:"branch,omitempty"`
}

// Checker asks GitHub for releases.
type Checker struct {
	URL    string // defaults to ReleasesURL
	Client *http.Client
}

// Latest returns the newest published release. GitHub's "latest" skips
// drafts and pre-releases, so test builds never come from it.
func (c Checker) Latest(ctx context.Context, current string) (Release, error) {
	var body struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if _, err := c.get(ctx, "/latest", current, &body); err != nil {
		return Release{}, err
	}
	v := strings.TrimPrefix(body.TagName, "v")
	if _, ok := parse(v); !ok {
		return Release{}, fmt.Errorf("unexpected release tag %q", body.TagName)
	}
	if !https(body.HTMLURL) {
		return Release{}, fmt.Errorf("unexpected release page %q", body.HTMLURL)
	}
	return Release{Version: v, URL: body.HTMLURL}, nil
}

// ghRelease is the part of GitHub's release JSON a test build is read from.
type ghRelease struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	HTMLURL    string `json:"html_url"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// Dev returns the newest test build of branch, or ErrNoDevBuild.
func (c Checker) Dev(ctx context.Context, current, branch string) (Release, error) {
	var body ghRelease
	status, err := c.get(ctx, "/tags/"+DevTag(branch), current, &body)
	if status == http.StatusNotFound {
		return Release{}, ErrNoDevBuild // also briefly while a build replaces it
	}
	if err != nil {
		return Release{}, err
	}
	return devRelease(body)
}

// Builds lists the published test builds, develop's first and then those of
// open pull requests by branch name.
func (c Checker) Builds(ctx context.Context, current string) ([]Release, error) {
	var page []ghRelease
	if _, err := c.get(ctx, "?per_page=100", current, &page); err != nil {
		return nil, err
	}
	builds := []Release{}
	for _, r := range page {
		if !r.Prerelease || (r.TagName != devTag && !strings.HasPrefix(r.TagName, devTag+"-")) {
			continue
		}
		if rel, err := devRelease(r); err == nil {
			builds = append(builds, rel)
		}
	}
	sort.Slice(builds, func(i, j int) bool {
		a, b := builds[i].Branch, builds[j].Branch
		if a == DevBranch || b == DevBranch {
			return a == DevBranch && b != DevBranch
		}
		return a < b
	})
	return builds, nil
}

// DevTag is the pre-release tag of branch's test builds.
func DevTag(branch string) string {
	if branch == DevBranch {
		return devTag
	}
	return devTag + "-" + branch
}

func devRelease(r ghRelease) (Release, error) {
	v := strings.TrimPrefix(r.Name, devTitle)
	b, ok := testBuild(v)
	if !ok || v == r.Name {
		return Release{}, fmt.Errorf("unexpected test build title %q", r.Name)
	}
	rel := Release{Version: v, URL: r.HTMLURL, Dev: true, Branch: b.branch}
	for _, a := range r.Assets {
		if a.Name == DevAsset {
			rel.Download = a.URL
		}
	}
	if !https(rel.URL) || !https(rel.Download) {
		return Release{}, fmt.Errorf("test build %s has no %s", v, DevAsset)
	}
	return rel, nil
}

// get fetches the releases API at path into v and returns the HTTP status.
func (c Checker) get(ctx context.Context, path, current string, v any) (int, error) {
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	base := c.URL
	if base == "" {
		base = ReleasesURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "worldkeeper/"+current) // GitHub rejects requests without one
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	return resp.StatusCode, json.NewDecoder(resp.Body).Decode(v)
}

// https reports whether u is safe to open in the browser.
func https(u string) bool { return strings.HasPrefix(u, "https://") }

// ErrUnversioned is returned for local builds ("dev"): there is nothing to
// compare a release with.
var ErrUnversioned = errors.New("this build has no version number")

// Newer reports whether latest is newer than current. A test build
// (0.3.0-develop.202610111530.abc1234) is older than the 0.3.0 release it
// leads up to, and builds of one branch are ordered by commit time.
func Newer(latest, current string) (bool, error) {
	c, ok := parse(current)
	if !ok {
		return false, ErrUnversioned
	}
	l, ok := parse(latest)
	if !ok {
		return false, fmt.Errorf("invalid version %q", latest)
	}
	return compare(l, c) > 0, nil
}

// Offer reports whether latest should be offered to a build running
// current. It is Newer, except that a test build of another branch of the
// same version is offered even if its commit is older: the user picked
// that branch.
func Offer(latest, current string) (bool, error) {
	newer, err := Newer(latest, current)
	if err != nil {
		return false, err
	}
	lb, lok := testBuild(latest)
	cb, cok := testBuild(current)
	if lok && cok && lb.branch != cb.branch {
		l, _ := parse(latest)
		c, _ := parse(current)
		return newer || l.core == c.core, nil
	}
	return newer, nil
}

// build is what a test build's version says about it.
type build struct {
	branch string
	time   string // commit time, yyyymmddhhmm UTC
}

// testBuild reads a test build's version: <x.y.z>-<branch>.<commit time>.<commit>.
func testBuild(v string) (build, bool) {
	p, ok := parse(v)
	if !ok || len(p.pre) != 3 || len(p.pre[1]) != 12 {
		return build{}, false
	}
	if _, err := strconv.ParseUint(p.pre[1], 10, 64); err != nil {
		return build{}, false
	}
	return build{branch: p.pre[0], time: p.pre[1]}, true
}

// Versioned reports whether v is a version number releases can be compared
// with; local builds ("dev") are not.
func Versioned(v string) bool {
	_, ok := parse(v)
	return ok
}

type version struct {
	core [3]int
	pre  []string // empty for a release
}

// parse reads a semantic version such as 1.2.3 or 1.2.3-dev.abc1234;
// build metadata (+...) is ignored.
func parse(s string) (version, bool) {
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	var v version
	core, pre, hasPre := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v.core[i] = n
	}
	if hasPre {
		if pre == "" {
			return v, false
		}
		v.pre = strings.Split(pre, ".")
	}
	return v, true
}

// compare orders versions by semver precedence.
func compare(a, b version) int {
	for i := range a.core {
		if a.core[i] != b.core[i] {
			return cmpInt(a.core[i], b.core[i])
		}
	}
	switch {
	case len(a.pre) == 0 && len(b.pre) == 0:
		return 0
	case len(a.pre) == 0:
		return 1 // a release outranks its pre-releases
	case len(b.pre) == 0:
		return -1
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		x, y := a.pre[i], b.pre[i]
		xn, xErr := strconv.Atoi(x)
		yn, yErr := strconv.Atoi(y)
		switch {
		case xErr == nil && yErr == nil:
			if xn != yn {
				return cmpInt(xn, yn)
			}
		case xErr == nil:
			return -1 // numeric identifiers sort first
		case yErr == nil:
			return 1
		case x != y:
			return strings.Compare(x, y)
		}
	}
	return cmpInt(len(a.pre), len(b.pre))
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
