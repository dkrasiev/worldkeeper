package main

import (
	"os"
	"runtime"
	"strings"
)

// prepareMacLaunch makes Worldkeeper.app work when it is started from
// Finder, Launchpad or Login Items rather than from a terminal.
func prepareMacLaunch() {
	if runtime.GOOS != "darwin" {
		return
	}
	os.Args = dropProcessSerial(os.Args)
	_ = os.Setenv("PATH", withPackageManagers(os.Getenv("PATH")))
}

// dropProcessSerial removes the -psn_<n>_<n> argument that macOS can pass
// to an app opened from Finder (notably on the first start of a downloaded
// app). The flag package would reject it as an unknown flag.
func dropProcessSerial(args []string) []string {
	out := args[:0:0]
	for i, a := range args {
		if i > 0 && strings.HasPrefix(a, "-psn_") {
			continue
		}
		out = append(out, a)
	}
	return out
}

// packageManagerDirs are where Homebrew (Apple Silicon, then Intel) and
// MacPorts install programs such as restic.
var packageManagerDirs = []string{"/opt/homebrew/bin", "/usr/local/bin", "/opt/local/bin"}

// withPackageManagers appends packageManagerDirs to a PATH that lacks them.
// Apps started outside a terminal get only /usr/bin:/bin:/usr/sbin:/sbin,
// so restic installed with `brew install restic` would not be found.
func withPackageManagers(path string) string {
	have := map[string]bool{}
	var dirs []string
	if path != "" {
		dirs = strings.Split(path, ":")
	}
	for _, d := range dirs {
		have[d] = true
	}
	for _, d := range packageManagerDirs {
		if !have[d] {
			dirs = append(dirs, d)
		}
	}
	return strings.Join(dirs, ":")
}
