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
	for _, needle := range []string{"scoop", "chocolatey", "restic.restic_*"} {
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

func TestResolve(t *testing.T) {
	dir := t.TempDir()
	name := "restic_0.19.1_windows_amd64.exe" // the file winget installs
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := inDirs([]string{filepath.Join(dir, "missing"), "", dir}, name); got != filepath.Join(dir, name) {
		t.Errorf("inDirs = %q", got)
	}
	// A path from the settings is used as given, found or not.
	if p := filepath.Join(dir, "x", "restic"); Resolve(p) != p {
		t.Errorf("Resolve(path) = %q", Resolve(p))
	}
	// A bare name: on PATH first; unknown names are passed through.
	t.Setenv("PATH", dir)
	if got := Resolve(name); !strings.EqualFold(got, filepath.Join(dir, name)) {
		t.Errorf("Resolve(%q) with it on PATH = %q", name, got)
	}
	if got := Resolve("no-such-restic"); got != "no-such-restic" {
		t.Errorf("Resolve(unknown) = %q", got)
	}
}

// The case from a Windows machine: the settings name winget's restic file,
// and Worldkeeper's own PATH (started from an archive, before an install)
// does not have its folder.
func TestResolveWithoutPATH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Logf("Resolve(restic_0.19.1_windows_amd64.exe) without PATH: %s", Resolve("restic_0.19.1_windows_amd64.exe"))
}
