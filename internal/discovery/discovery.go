// Package discovery finds Minecraft Java Edition worlds in well-known
// launcher locations on Windows, macOS and Linux.
package discovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"
)

// Env describes the host. Tests substitute a fake one.
type Env struct {
	GOOS    string
	Home    string
	AppData string // Windows %APPDATA%
}

func HostEnv() Env {
	home, _ := os.UserHomeDir()
	return Env{GOOS: runtime.GOOS, Home: home, AppData: os.Getenv("APPDATA")}
}

// Root is a "saves" directory, possibly given as a glob over launcher instances.
type Root struct {
	Source string // stable machine id prefix: "minecraft", "prism", ...
	Label  string // human readable: "Minecraft Launcher", "Prism: Fabric 1.21"
	Saves  string // path or glob ending in the saves directory
}

// World is a discovered world folder.
type World struct {
	ID          string `json:"id"` // stable key, survives OS reinstall if the launcher layout is the same
	Folder      string `json:"folder"`
	Path        string `json:"path"`
	Source      string `json:"source"`
	SourceLabel string `json:"sourceLabel"`
	SavesDir    string `json:"savesDir"`
}

// KnownRoots returns the launcher locations to scan for the given host.
func KnownRoots(env Env) []Root {
	h := env.Home
	var roots []Root
	add := func(source, label, saves string) {
		roots = append(roots, Root{Source: source, Label: label, Saves: saves})
	}

	switch env.GOOS {
	case "windows":
		a := env.AppData
		if a == "" {
			a = filepath.Join(h, "AppData", "Roaming")
		}
		add("minecraft", "Minecraft Launcher", filepath.Join(a, ".minecraft", "saves"))
		add("prism", "Prism Launcher", filepath.Join(a, "PrismLauncher", "instances", "*", "minecraft", "saves"))
		add("prism", "Prism Launcher", filepath.Join(a, "PrismLauncher", "instances", "*", ".minecraft", "saves"))
		add("curseforge", "CurseForge", filepath.Join(h, "curseforge", "minecraft", "Instances", "*", "saves"))
		add("modrinth", "Modrinth App", filepath.Join(a, "ModrinthApp", "profiles", "*", "saves"))
		add("modrinth", "Modrinth App", filepath.Join(a, "com.modrinth.theseus", "profiles", "*", "saves"))
	case "darwin":
		as := filepath.Join(h, "Library", "Application Support")
		add("minecraft", "Minecraft Launcher", filepath.Join(as, "minecraft", "saves"))
		add("prism", "Prism Launcher", filepath.Join(as, "PrismLauncher", "instances", "*", "minecraft", "saves"))
		add("prism", "Prism Launcher", filepath.Join(as, "PrismLauncher", "instances", "*", ".minecraft", "saves"))
		add("curseforge", "CurseForge", filepath.Join(h, "Documents", "curseforge", "minecraft", "Instances", "*", "saves"))
		add("modrinth", "Modrinth App", filepath.Join(as, "ModrinthApp", "profiles", "*", "saves"))
		add("modrinth", "Modrinth App", filepath.Join(as, "com.modrinth.theseus", "profiles", "*", "saves"))
	default: // linux and other unixes
		share := filepath.Join(h, ".local", "share")
		add("minecraft", "Minecraft Launcher", filepath.Join(h, ".minecraft", "saves"))
		add("minecraft", "Minecraft Launcher (Flatpak)", filepath.Join(h, ".var", "app", "com.mojang.Minecraft", ".minecraft", "saves"))
		add("prism", "Prism Launcher", filepath.Join(share, "PrismLauncher", "instances", "*", "minecraft", "saves"))
		add("prism", "Prism Launcher", filepath.Join(share, "PrismLauncher", "instances", "*", ".minecraft", "saves"))
		add("prism", "Prism Launcher (Flatpak)", filepath.Join(h, ".var", "app", "org.prismlauncher.PrismLauncher", "data", "PrismLauncher", "instances", "*", "minecraft", "saves"))
		add("modrinth", "Modrinth App", filepath.Join(share, "ModrinthApp", "profiles", "*", "saves"))
		add("modrinth", "Modrinth App", filepath.Join(share, "com.modrinth.theseus", "profiles", "*", "saves"))
	}
	return roots
}

// DefaultSavesDir is the official launcher's saves folder, used when
// restoring a world that no longer exists locally.
func DefaultSavesDir(env Env) string {
	return KnownRoots(env)[0].Saves
}

// Scan finds all worlds under the known roots, custom launcher profiles
// and extra directories. Duplicate paths are reported once.
func Scan(env Env, extra []string) []World {
	roots := KnownRoots(env)
	roots = append(roots, profileRoots(filepath.Dir(DefaultSavesDir(env)))...)
	for _, dir := range extra {
		roots = append(roots, Root{Source: "custom", Label: "Folder: " + friendlyName(dir), Saves: dir})
	}

	seen := map[string]bool{}
	var worlds []World
	for _, r := range roots {
		dirs, _ := filepath.Glob(r.Saves)
		for _, saves := range dirs {
			source, label := r.Source, r.Label
			if inst := instanceName(r, saves); inst != "" {
				source += ":" + inst
				label += ": " + inst
			}
			for _, w := range scanSaves(saves, source, label) {
				key := filepath.Clean(w.Path)
				if env.GOOS == "windows" || env.GOOS == "darwin" {
					key = strings.ToLower(key) // case-insensitive file systems
				}
				if seen[key] {
					continue
				}
				seen[key] = true
				worlds = append(worlds, w)
			}
		}
	}
	sort.Slice(worlds, func(i, j int) bool { return worlds[i].ID < worlds[j].ID })
	return worlds
}

func scanSaves(saves, source, label string) []World {
	entries, err := os.ReadDir(saves)
	if err != nil {
		return nil
	}
	var out []World
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p := filepath.Join(saves, e.Name())
		if _, err := os.Stat(filepath.Join(p, "level.dat")); err != nil {
			continue
		}
		out = append(out, World{
			ID:          WorldID(source, e.Name()),
			Folder:      e.Name(),
			Path:        p,
			Source:      source,
			SourceLabel: label,
			SavesDir:    saves,
		})
	}
	return out
}

// instanceName extracts the launcher instance from a glob match, e.g.
// ".../instances/Fabric 1.21/minecraft/saves" -> "Fabric 1.21".
func instanceName(r Root, saves string) string {
	if !strings.Contains(r.Saves, "*") {
		return ""
	}
	pattern := strings.Split(filepath.ToSlash(r.Saves), "/")
	actual := strings.Split(filepath.ToSlash(saves), "/")
	if len(pattern) != len(actual) {
		return ""
	}
	for i, part := range pattern {
		if part == "*" {
			return actual[i]
		}
	}
	return ""
}

// friendlyName shortens a saves path for display: ".../My Pack/saves" -> "My Pack".
func friendlyName(dir string) string {
	dir = filepath.Clean(dir)
	if strings.EqualFold(filepath.Base(dir), "saves") {
		dir = filepath.Dir(dir)
	}
	if strings.EqualFold(filepath.Base(dir), ".minecraft") || strings.EqualFold(filepath.Base(dir), "minecraft") {
		dir = filepath.Dir(dir)
	}
	return filepath.Base(dir)
}

// profileRoots reads custom game directories from the official launcher's
// launcher_profiles.json.
func profileRoots(mcDir string) []Root {
	b, err := os.ReadFile(filepath.Join(mcDir, "launcher_profiles.json"))
	if err != nil {
		return nil
	}
	var lp struct {
		Profiles map[string]struct {
			Name    string `json:"name"`
			GameDir string `json:"gameDir"`
		} `json:"profiles"`
	}
	if json.Unmarshal(b, &lp) != nil {
		return nil
	}
	var roots []Root
	for id, p := range lp.Profiles {
		if p.GameDir == "" {
			continue
		}
		name := p.Name
		if name == "" {
			name = id
		}
		roots = append(roots, Root{
			Source: "minecraft:" + name,
			Label:  "Minecraft Launcher: " + name,
			Saves:  filepath.Join(p.GameDir, "saves"),
		})
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Source < roots[j].Source })
	return roots
}

// WorldID builds a URL- and filesystem-safe key from the source and world
// folder. Unicode letters are kept so that non-Latin world names stay distinct.
func WorldID(source, folder string) string {
	return sanitize(source) + "--" + sanitize(folder)
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), ".")
	if out == "" {
		out = "_"
	}
	return out
}
