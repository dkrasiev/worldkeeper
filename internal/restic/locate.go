package restic

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// firstFile returns the first existing regular file matching the patterns.
func firstFile(patterns []string) string {
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, m := range matches {
			if fi, err := os.Stat(m); err == nil && !fi.IsDir() {
				return m
			}
		}
	}
	return ""
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
		return []string{
			join(local, "Microsoft", "WinGet", "Links", "restic.exe"),                        // winget, user scope
			join(local, "Microsoft", "WinGet", "Packages", "restic.restic_*", "restic*.exe"), // winget, the package itself
			join(getenv("ProgramFiles"), "WinGet", "Links", "restic.exe"),                    // winget, machine scope
			join(home, "scoop", "shims", "restic.exe"),                                       // Scoop
			join(programData, "chocolatey", "bin", "restic.exe"),                             // Chocolatey
		}
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
