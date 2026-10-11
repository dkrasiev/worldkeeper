package discovery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mkWorld(t *testing.T, saves, name string) {
	t.Helper()
	dir := filepath.Join(saves, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "level.dat"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanLinux(t *testing.T) {
	home := t.TempDir()
	env := Env{GOOS: "linux", Home: home}

	official := filepath.Join(home, ".minecraft", "saves")
	mkWorld(t, official, "New World")
	mkWorld(t, official, "Мой мир")
	if err := os.MkdirAll(filepath.Join(official, "not a world"), 0o755); err != nil {
		t.Fatal(err)
	}

	prism := filepath.Join(home, ".local", "share", "PrismLauncher", "instances", "Fabric 1.21", "minecraft", "saves")
	mkWorld(t, prism, "New World")

	custom := filepath.Join(home, "elsewhere")
	mkWorld(t, custom, "Imported")

	worlds := Scan(env, []string{custom, official}) // official listed twice on purpose
	got := map[string]World{}
	for _, w := range worlds {
		got[w.ID] = w
	}
	if len(worlds) != 4 {
		t.Fatalf("want 4 worlds, got %d: %+v", len(worlds), worlds)
	}
	for _, id := range []string{
		"minecraft--New_World",
		"minecraft--Мой_мир",
		"prism_Fabric_1.21--New_World",
		"custom--Imported",
	} {
		if _, ok := got[id]; !ok {
			t.Errorf("missing world %q; have %v", id, keys(got))
		}
	}
	if w := got["prism_Fabric_1.21--New_World"]; w.SourceLabel != "Prism Launcher: Fabric 1.21" {
		t.Errorf("label = %q", w.SourceLabel)
	}
}

func TestLauncherProfileGameDir(t *testing.T) {
	home := t.TempDir()
	env := Env{GOOS: "linux", Home: home}
	mc := filepath.Join(home, ".minecraft")
	gameDir := filepath.Join(home, "modded")
	mkWorld(t, filepath.Join(gameDir, "saves"), "Skyblock")
	profiles := `{"profiles":{"abc":{"name":"Modded","gameDir":"` + filepath.ToSlash(gameDir) + `"}}}`
	if err := os.MkdirAll(mc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mc, "launcher_profiles.json"), []byte(profiles), 0o644); err != nil {
		t.Fatal(err)
	}

	worlds := Scan(env, nil)
	if len(worlds) != 1 || worlds[0].ID != "minecraft_Modded--Skyblock" {
		t.Fatalf("got %+v", worlds)
	}
}

func TestWindowsRoots(t *testing.T) {
	roots := KnownRoots(Env{GOOS: "windows", Home: `C:\Users\steve`, AppData: `C:\Users\steve\AppData\Roaming`})
	if roots[0].Saves != filepath.Join(`C:\Users\steve\AppData\Roaming`, ".minecraft", "saves") {
		t.Errorf("official saves = %q", roots[0].Saves)
	}
}

func TestPlayerNames(t *testing.T) {
	home := t.TempDir()
	env := Env{GOOS: "linux", Home: home}
	write := func(dir, content string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "usercache.json"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	instance := filepath.Join(home, ".local", "share", "PrismLauncher", "instances", "Fabric", "minecraft")
	write(instance, `[{"name":"Alex","uuid":"00000000-0000-4000-8000-00000000000A","expiresOn":"2026-11-11 10:00:00 +0300"}]`)
	write(filepath.Join(home, ".minecraft"), `[{"name":"OldAlex","uuid":"00000000-0000-4000-8000-00000000000a"},{"name":"Steve","uuid":"00000000-0000-4000-8000-000000000001"}]`)

	got := PlayerNames(env, filepath.Join(instance, "saves"))
	want := map[string]string{
		"00000000-0000-4000-8000-00000000000a": "Alex", // the instance's own cache wins
		"00000000-0000-4000-8000-000000000001": "Steve",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for id, name := range want {
		if got[id] != name {
			t.Errorf("%s = %q, want %q", id, got[id], name)
		}
	}
	if n := PlayerNames(Env{GOOS: "linux", Home: t.TempDir()}, ""); len(n) != 0 {
		t.Errorf("no caches: got %v", n)
	}
}

func keys(m map[string]World) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestLocations(t *testing.T) {
	home := t.TempDir()
	env := Env{GOOS: "linux", Home: home}
	official := filepath.Join(home, ".minecraft", "saves")
	prism := filepath.Join(home, ".local", "share", "PrismLauncher", "instances", "Fabric 1.21", "minecraft", "saves")
	empty := filepath.Join(home, "empty saves") // a location without worlds yet
	for _, dir := range []string{official, prism, empty} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	missing := filepath.Join(home, "gone")

	locs := Locations(env, []string{empty, official, missing}) // official listed twice on purpose
	var got []string
	for _, l := range locs {
		got = append(got, l.Source+"|"+l.Label)
	}
	want := []string{
		"minecraft|Minecraft Launcher",
		"prism:Fabric 1.21|Prism Launcher: Fabric 1.21",
		"custom|Folder: empty saves",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("locations = %v, want %v", got, want)
	}

	if l, ok := Locate(env, nil, prism+string(filepath.Separator)); !ok || l.Source != "prism:Fabric 1.21" {
		t.Errorf("Locate(prism) = %+v, %v", l, ok)
	}
	if _, ok := Locate(env, nil, empty); ok {
		t.Error("a folder not in the settings is not a location")
	}
	if !SamePath(Env{GOOS: "windows"}, `C:\Games\Saves`, `c:\games\saves\`) || SamePath(env, "/a/Saves", "/a/saves") {
		t.Error("SamePath must ignore case on Windows only (and macOS), not on Linux")
	}
}
