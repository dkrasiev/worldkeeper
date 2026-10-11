// Package update finds out whether a newer Worldkeeper release is published.
// It only reads the GitHub release list; installing stays with the user
// (download the archive, or winget upgrade).
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// LatestURL is GitHub's "latest release" endpoint. It skips drafts and
// pre-releases, so test builds never get offered.
const LatestURL = "https://api.github.com/repos/dkrasiev/worldkeeper/releases/latest"

// Release is a published version.
type Release struct {
	Version string `json:"version"` // without the leading "v", e.g. "0.3.0"
	URL     string `json:"url"`     // release notes and downloads
}

// Checker asks GitHub for the latest release.
type Checker struct {
	URL    string // defaults to LatestURL
	Client *http.Client
}

// Latest returns the newest published release.
func (c Checker) Latest(ctx context.Context, current string) (Release, error) {
	url := c.URL
	if url == "" {
		url = LatestURL
	}
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "worldkeeper/"+current) // GitHub rejects requests without one
	resp, err := client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Release{}, err
	}
	v := strings.TrimPrefix(body.TagName, "v")
	if _, ok := parse(v); !ok {
		return Release{}, fmt.Errorf("unexpected release tag %q", body.TagName)
	}
	if !strings.HasPrefix(body.HTMLURL, "https://") { // it is opened in the browser
		return Release{}, fmt.Errorf("unexpected release page %q", body.HTMLURL)
	}
	return Release{Version: v, URL: body.HTMLURL}, nil
}

// ErrUnversioned is returned for local builds ("dev"): there is nothing to
// compare a release with.
var ErrUnversioned = errors.New("this build has no version number")

// Newer reports whether latest is newer than current. A test build
// (0.2.0-dev.abc1234) is older than the 0.2.0 release it leads up to.
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
