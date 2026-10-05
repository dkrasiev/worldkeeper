package discovery

import (
	"os"
	"path/filepath"
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

func keys(m map[string]World) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
