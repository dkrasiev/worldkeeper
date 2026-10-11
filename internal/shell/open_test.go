package shell

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestPrismInstance(t *testing.T) {
	for _, c := range []struct {
		saves, data, inst string
		ok                bool
	}{
		{filepath.Join("P", "instances", "Fabric 1.21", "minecraft", "saves"), "P", "Fabric 1.21", true},
		{filepath.Join("P", "instances", "Old", ".minecraft", "saves"), "P", "Old", true},
		{filepath.Join("home", ".minecraft", "saves"), "", "", false},
		{filepath.Join("curseforge", "minecraft", "Instances", "Pack", "saves"), "", "", false},
	} {
		data, inst, ok := prismInstance(c.saves)
		if data != c.data || inst != c.inst || ok != c.ok {
			t.Errorf("prismInstance(%q) = %q, %q, %v", c.saves, data, inst, ok)
		}
	}
}

// noPrism hides any Prism Launcher installed on this computer.
func noPrism(t *testing.T) {
	t.Helper()
	empty := t.TempDir()
	for _, v := range []string{"LOCALAPPDATA", "ProgramFiles", "APPDATA", "HOME", "USERPROFILE"} {
		t.Setenv(v, empty)
	}
	was := LookPath
	t.Cleanup(func() { LookPath = was })
	LookPath = func(string) (string, error) { return "", errors.New("not found") }
}

func TestPrismCommand(t *testing.T) {
	noPrism(t)
	exe := "prismlauncher"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}

	// A portable Prism: the program sits in its data folder, which Prism
	// is told about because it is not the default one.
	data := t.TempDir()
	saves := filepath.Join(data, "instances", "Fabric 1.21", "minecraft", "saves")
	if _, ok := PrismCommand(saves, "My World"); ok {
		t.Error("a command without Prism installed")
	}
	if err := os.WriteFile(filepath.Join(data, exe), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd, ok := PrismCommand(saves, "My World")
	want := []string{filepath.Join(data, exe), "--dir", data, "--launch", "Fabric 1.21", "--world", "My World"}
	if !ok || !reflect.DeepEqual(cmd, want) {
		t.Errorf("portable: %q, %v; want %q", cmd, ok, want)
	}

	// The Flatpak runs through flatpak and knows its own folder.
	LookPath = func(name string) (string, error) {
		if name == "flatpak" {
			return "/usr/bin/flatpak", nil
		}
		return "", errors.New("not found")
	}
	flat := filepath.Join("/home/u/.var/app/org.prismlauncher.PrismLauncher/data/PrismLauncher", "instances", "I", "minecraft", "saves")
	cmd, ok = PrismCommand(flat, "W")
	want = []string{"/usr/bin/flatpak", "run", "org.prismlauncher.PrismLauncher", "--launch", "I", "--world", "W"}
	if !ok || !reflect.DeepEqual(cmd, want) {
		t.Errorf("flatpak: %q, %v; want %q", cmd, ok, want)
	}

	if _, ok := PrismCommand(filepath.Join(data, ".minecraft", "saves"), "W"); ok {
		t.Error("a command for a world outside Prism")
	}
}
