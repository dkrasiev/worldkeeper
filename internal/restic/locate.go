package restic

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// Locate finds the restic executable. PATH comes first, but it is often
// stale or minimal for a tray app: winget adds its Links folder to PATH
// only for processes started after the install, and apps launched from
// Finder or at login on macOS do not see Homebrew's bin folder. So the
// usual install locations are checked as well. Nothing is cached, so a
// restic installed while Worldkeeper runs is found on the next call.
func Locate() string {
	if p, err := exec.LookPath("restic"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	if p := firstFile(candidates(runtime.GOOS, os.Getenv, home)); p != "" {
		return p
	}
	return "restic" // not found; running it reports ErrNotInstalled
}

// firstFile returns a regular file for the first pattern that matches any.
// When a pattern matches several (e.g. an old and a new winget version), the
// one with the highest version in its name wins.
func firstFile(patterns []string) string {
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		best, bestVer := "", ""
		for _, m := range matches {
			if fi, err := os.Stat(m); err != nil || fi.IsDir() {
				continue
			}
			v := versionInName.FindString(filepath.Base(m))
			if best == "" || newer(v, bestVer) {
				best, bestVer = m, v
			}
		}
		if best != "" {
			return best
		}
	}
	return ""
}

var versionInName = regexp.MustCompile(`\d+\.\d+\.\d+`)

// newer compares dotted versions numerically ("0.19.1" > "0.9.0").
func newer(a, b string) bool {
	if a == "" || b == "" {
		return a != "" && b == ""
	}
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := range 3 {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			return x > y
		}
	}
	return false
}

// candidates lists where package managers put restic, as glob patterns.
func candidates(goos string, getenv func(string) string, home string) []string {
	join := filepath.Join
	switch goos {
	case "windows":
		local := getenv("LOCALAPPDATA")
		if local == "" {
			local = join(home, "AppData", "Local")
		}
		programData := getenv("ProgramData")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		// winget's restic package has no command alias, so the executable
		// keeps its release name, e.g. restic_0.19.1_windows_amd64.exe.
		winget := func(root string) []string {
			return []string{
				join(root, "Links", "restic.exe"),
				join(root, "Links", "restic_*.exe"),
				join(root, "Packages", "restic.restic_*", "restic*.exe"),
			}
		}
		out := winget(join(local, "Microsoft", "WinGet"))                    // user scope
		out = append(out, winget(join(getenv("ProgramFiles"), "WinGet"))...) // machine scope
		return append(out,
			join(home, "scoop", "shims", "restic.exe"),           // Scoop
			join(programData, "chocolatey", "bin", "restic.exe"), // Chocolatey
		)
	case "darwin":
		return []string{
			"/opt/homebrew/bin/restic", // Homebrew, Apple Silicon
			"/usr/local/bin/restic",    // Homebrew, Intel / manual
			"/opt/local/bin/restic",    // MacPorts
			join(home, ".local", "bin", "restic"),
		}
	default:
		return []string{
			"/usr/bin/restic",
			"/usr/local/bin/restic",
			"/snap/bin/restic",
			"/home/linuxbrew/.linuxbrew/bin/restic",
			join(home, ".local", "bin", "restic"),
			join(home, "bin", "restic"),
		}
	}
}
