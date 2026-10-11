package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dkrasiev/worldkeeper/internal/shell"
	"github.com/dkrasiev/worldkeeper/internal/testworld"
)

// started records what would have been started instead of starting it.
func started(t *testing.T) *[][]string {
	t.Helper()
	var got [][]string
	start, look := shell.Start, shell.LookPath
	t.Cleanup(func() { shell.Start, shell.LookPath = start, look })
	shell.Start = func(cmd *exec.Cmd) error { got = append(got, cmd.Args); return nil }
	shell.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	empty := t.TempDir()
	for _, v := range []string{"LOCALAPPDATA", "ProgramFiles"} {
		t.Setenv(v, empty)
	}
	return &got
}

func TestOpenAndPlay(t *testing.T) {
	f := setup(t)
	got := started(t)
	official := testworld.Create(t, f.saves, "w", testworld.Options{})
	data := filepath.Join(f.app.Env.Home, ".local", "share", "PrismLauncher")
	testworld.Create(t, filepath.Join(data, "instances", "Fabric 1.21", "minecraft", "saves"), "p", testworld.Options{})

	if err := f.app.OpenFolder("minecraft--w"); err != nil || len(*got) != 1 || (*got)[0][len((*got)[0])-1] != official {
		t.Fatalf("open folder: %v, started %q", err, *got)
	}
	if err := f.app.Play("minecraft--w"); !errors.Is(err, ErrNotPlayable) {
		t.Errorf("play from the official launcher: err = %v", err)
	}
	if d, _ := f.app.Info("prism_Fabric_1.21--p", ""); d.Playable {
		t.Error("playable without Prism installed")
	}

	exe := "prismlauncher"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if err := os.WriteFile(filepath.Join(data, exe), nil, 0o755); err != nil { // a portable Prism
		t.Fatal(err)
	}
	if d, _ := f.app.Info("prism_Fabric_1.21--p", ""); !d.Playable {
		t.Error("not playable with Prism next to its instances")
	}
	if err := f.app.Play("prism_Fabric_1.21--p"); err != nil {
		t.Fatal(err)
	}
	args := (*got)[len(*got)-1]
	if n := len(args); n < 4 || args[n-4] != "--launch" || args[n-3] != "Fabric 1.21" || args[n-2] != "--world" || args[n-1] != "p" {
		t.Errorf("play started %q", args)
	}
}
