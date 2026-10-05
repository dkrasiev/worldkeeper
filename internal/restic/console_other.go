//go:build !windows

package restic

import "os/exec"

func hideConsole(*exec.Cmd) {}
