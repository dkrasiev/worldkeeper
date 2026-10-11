package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// OpenFolder shows a folder in the file manager: Explorer, Finder, or the
// desktop's default (xdg-open).
func OpenFolder(dir string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer.exe", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	return Start(cmd)
}

// Start runs a desktop program without waiting for it. Tests replace it.
var Start = func(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // reap it once it exits
	return nil
}

// PrismCommand is the command line that starts a world in Prism Launcher:
// it launches the world's instance and joins the world (Minecraft's quick
// play; versions before 1.20 open at the title screen instead). ok is
// false when the world is not in a Prism instance or Prism is not found.
func PrismCommand(savesDir, folder string) (cmd []string, ok bool) {
	data, inst, ok := prismInstance(savesDir)
	if !ok {
		return nil, false
	}
	launcher, ok := findPrism(data)
	if !ok {
		return nil, false
	}
	// Prism looks the instance up in its own data folder. A world from a
	// portable or moved Prism needs that folder named.
	if !sameDir(data, prismDefaultData()) && !flatpak(data) {
		launcher = append(launcher, "--dir", data)
	}
	return append(launcher, "--launch", inst, "--world", folder), true
}

// prismInstance splits <data>/instances/<instance>/(.)minecraft/saves.
func prismInstance(savesDir string) (data, instance string, ok bool) {
	game := filepath.Dir(filepath.Clean(savesDir))
	if b := strings.ToLower(filepath.Base(game)); b != "minecraft" && b != ".minecraft" {
		return "", "", false
	}
	inst := filepath.Dir(game)
	instances := filepath.Dir(inst)
	if !strings.EqualFold(filepath.Base(instances), "instances") {
		return "", "", false
	}
	return filepath.Dir(instances), filepath.Base(inst), true
}

// LookPath is exec.LookPath; tests replace it.
var LookPath = exec.LookPath

func findPrism(data string) ([]string, bool) {
	if flatpak(data) {
		if p, err := LookPath("flatpak"); err == nil {
			return []string{p, "run", "org.prismlauncher.PrismLauncher"}, true
		}
		return nil, false
	}
	exe := "prismlauncher"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	// A portable Prism keeps its instances next to the program.
	candidates := []string{filepath.Join(data, exe)}
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		candidates = append(candidates,
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "PrismLauncher", exe),
			filepath.Join(os.Getenv("ProgramFiles"), "PrismLauncher", exe),
			filepath.Join(home, "scoop", "apps", "prismlauncher", "current", exe),
		)
	case "darwin":
		candidates = append(candidates,
			"/Applications/Prism Launcher.app/Contents/MacOS/prismlauncher",
			filepath.Join(home, "Applications", "Prism Launcher.app", "Contents", "MacOS", "prismlauncher"),
		)
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return []string{c}, true
		}
	}
	for _, name := range []string{"prismlauncher", "PrismLauncher"} {
		if p, err := LookPath(name); err == nil {
			return []string{p}, true
		}
	}
	return nil, false
}

// prismDefaultData is where an installed Prism keeps its instances.
func prismDefaultData() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "PrismLauncher")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "PrismLauncher")
	}
	return filepath.Join(home, ".local", "share", "PrismLauncher")
}

func flatpak(data string) bool {
	return strings.Contains(filepath.ToSlash(data), "/.var/app/org.prismlauncher.PrismLauncher/")
}

func sameDir(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
