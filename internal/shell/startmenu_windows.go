package shell

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

// startMenuPath is Worldkeeper.lnk in the user's Start menu Programs folder,
// %APPDATA%\Microsoft\Windows\Start Menu\Programs.
func startMenuPath() string {
	programs, err := windows.KnownFolderPath(windows.FOLDERID_Programs, 0)
	if err != nil {
		return ""
	}
	return filepath.Join(programs, AppName+".lnk")
}
