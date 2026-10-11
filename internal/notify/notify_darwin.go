package notify

import (
	"fmt"
	"os/exec"
	"strings"
)

// native shows a Notification Center banner through AppleScript. The texts
// are passed as arguments, never pasted into the script.
func native(title, body string) error {
	out, err := exec.Command("osascript",
		"-e", "on run argv",
		"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
		"-e", "end run",
		title, body).CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
