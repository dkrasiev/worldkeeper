package shell

import (
	"os"
	"path/filepath"
)

func platformEntry() entry {
	home, _ := os.UserHomeDir()
	return &launchAgent{path: filepath.Join(home, "Library", "LaunchAgents", AppID+".plist")}
}

// launchAgent is a plist in ~/Library/LaunchAgents that launchd runs at login.
type launchAgent struct{ path string }

func (e *launchAgent) read() ([]string, error) { return readFile(e.path, parseLaunchAgent) }

func (e *launchAgent) write(cmd []string) error {
	return writeFile(e.path, launchAgentPlist(AppID, cmd))
}

func (e *launchAgent) remove() error { return removeFile(e.path) }
