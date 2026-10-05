package worldinfo

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dkrasiev/worldkeeper/internal/testworld"
)

// TestSetLevelNameKeepsOtherTags round-trips a level.dat written by the real
// game: apart from LevelName, every tag must decode to the same value.
func TestSetLevelNameKeepsOtherTags(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "worlds", "26.3-singleplayer", "level.dat"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "level.dat"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := readNBT(os.DirFS(dir), "level.dat")
	if err != nil {
		t.Fatal(err)
	}

	const name = "Мой мир (restored 2026-10-05 18:30)"
	if err := SetLevelName(dir, name); err != nil {
		t.Fatal(err)
	}
	after, err := readNBT(os.DirFS(dir), "level.dat")
	if err != nil {
		t.Fatal(err)
	}
	if got := getString(after, "Data.LevelName"); got != name {
		t.Fatalf("LevelName = %q, want %q", got, name)
	}
	before["Data"].(map[string]any)["LevelName"] = name
	if !reflect.DeepEqual(before, after) {
		t.Error("tags other than LevelName changed")
	}
	if raw, _ := os.ReadFile(filepath.Join(dir, "level.dat")); raw[0] != 0x1f {
		t.Error("level.dat is no longer gzipped")
	}
	if _, err := os.Stat(filepath.Join(dir, "level.dat_old")); !os.IsNotExist(err) {
		t.Error("level.dat_old created")
	}
}

func TestSetLevelNameRenamesFallbackCopy(t *testing.T) {
	dir := testworld.Create(t, t.TempDir(), "w", testworld.Options{})
	level, _ := os.ReadFile(filepath.Join(dir, "level.dat"))
	if err := os.WriteFile(filepath.Join(dir, "level.dat_old"), level, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetLevelName(dir, "Copy"); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"level.dat", "level.dat_old"} {
		m, err := readNBT(os.DirFS(dir), f)
		if err != nil {
			t.Fatal(err)
		}
		if got := getString(m, "Data.LevelName"); got != "Copy" {
			t.Errorf("%s: LevelName = %q", f, got)
		}
	}
	if s, _ := ReadSummary(dir); s.Name != "Copy" {
		t.Errorf("summary name = %q", s.Name)
	}
}

func TestSetLevelNameMissingLevel(t *testing.T) {
	if err := SetLevelName(t.TempDir(), "x"); !os.IsNotExist(err) {
		t.Errorf("err = %v, want not exist", err)
	}
}
