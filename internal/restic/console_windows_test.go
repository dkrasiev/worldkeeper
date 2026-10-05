package restic

import (
	"context"
	"testing"

	"golang.org/x/sys/windows"
)

func TestResticRunsWithoutConsoleWindow(t *testing.T) {
	r := &Runner{Binary: "restic"}
	cmd := r.command(context.Background(), "version")
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 || !cmd.SysProcAttr.HideWindow {
		t.Fatalf("restic would open a console window: %+v", cmd.SysProcAttr)
	}
}
