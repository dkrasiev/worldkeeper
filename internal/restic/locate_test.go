package restic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCandidatesWindows(t *testing.T) {
	env := map[string]string{"LOCALAPPDATA": `C:\Users\steve\AppData\Local`, "ProgramData": `C:\ProgramData`, "ProgramFiles": `C:\Program Files`}
	got := candidates("windows", func(k string) string { return env[k] }, `C:\Users\steve`)
	want := filepath.Join(`C:\Users\steve\AppData\Local`, "Microsoft", "WinGet", "Links", "restic.exe")
	if len(got) == 0 || got[0] != want {
		t.Fatalf("first candidate = %v, want winget links %q", got, want)
	}
	for _, needle := range []string{"scoop", "chocolatey", "restic.restic_*", filepath.Join("Links", "restic_*.exe")} {
		if !strings.Contains(strings.Join(got, "|"), needle) {
			t.Errorf("no %s candidate in %v", needle, got)
		}
	}
}

func TestCandidatesMacIncludeHomebrew(t *testing.T) {
	got := strings.Join(candidates("darwin", os.Getenv, "/Users/steve"), "|")
	if !strings.Contains(got, "/opt/homebrew/bin/restic") || !strings.Contains(got, "/usr/local/bin/restic") {
		t.Errorf("candidates = %s", got)
	}
}

func TestFirstFilePrefersNewestVersion(t *testing.T) {
	links := t.TempDir()
	for _, name := range []string{"restic_0.9.6_windows_amd64.exe", "restic_0.19.1_windows_amd64.exe", "restic_0.18.0_windows_amd64.exe"} {
		if err := os.WriteFile(filepath.Join(links, name), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	patterns := []string{filepath.Join(links, "restic.exe"), filepath.Join(links, "restic_*.exe")}
	if got := firstFile(patterns); filepath.Base(got) != "restic_0.19.1_windows_amd64.exe" {
		t.Errorf("firstFile = %q, want the 0.19.1 executable", got)
	}
	// An exact name in an earlier pattern still wins.
	if err := os.WriteFile(filepath.Join(links, "restic.exe"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := firstFile(patterns); filepath.Base(got) != "restic.exe" {
		t.Errorf("firstFile = %q, want restic.exe", got)
	}
}

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"0.19.1", "0.9.6", true}, {"0.9.6", "0.19.1", false}, {"1.0.0", "1.0.0", false}, {"0.1.0", "", true}, {"", "0.1.0", false}} {
		if got := newer(c.a, c.b); got != c.want {
			t.Errorf("newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func TestFirstFile(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "Packages", "restic.restic_Microsoft.Winget.Source_8wekyb3d8bbwe")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(pkg, "restic_0.18.1_windows_amd64.exe")
	if err := os.WriteFile(exe, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	patterns := []string{
		filepath.Join(dir, "Links", "restic.exe"), // missing
		filepath.Join(dir, "Packages"),            // a directory, not a file
		filepath.Join(dir, "Packages", "restic.restic_*", "restic*.exe"),
	}
	if got := firstFile(patterns); got != exe {
		t.Errorf("firstFile = %q, want %q", got, exe)
	}
	if got := firstFile([]string{filepath.Join(dir, "nope")}); got != "" {
		t.Errorf("firstFile(missing) = %q", got)
	}
}

// With PATH empty, Locate still finds restic where package managers put it
// (Homebrew on this Mac, the system package on Linux CI), or reports
// "restic" so running it fails as not installed.
func TestLocateWithoutPATH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	got := Locate()
	if got != "restic" {
		if _, err := os.Stat(got); err != nil {
			t.Fatalf("Locate returned %q which does not exist", got)
		}
	}
	t.Logf("Locate without PATH: %s", got)
}
