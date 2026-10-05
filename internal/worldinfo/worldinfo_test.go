package worldinfo

import (
	"testing"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/testworld"
)

func TestRead(t *testing.T) {
	dir := testworld.Create(t, t.TempDir(), "w1", testworld.Options{Name: "Мой мир", LastPlayed: 1759680000000})

	info, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}

	checks := []struct {
		name      string
		got, want any
	}{
		{"name", info.Name, "Мой мир"},
		{"version", info.GameVersion, "1.21.4"},
		{"dataVersion", info.DataVersion, int64(4189)},
		{"mode", info.GameMode, "survival"},
		{"difficulty", info.Difficulty, "normal"},
		{"cheats", info.Cheats, true},
		{"seed", info.Seed, "-4172144997902289642"},
		{"day", info.Day, int64(12)},
		{"weather", info.Weather, "rain"},
		{"lastPlayed", info.LastPlayed, time.UnixMilli(1759680000000).UTC()},
		{"icon", info.HasIcon, true},
		{"modded", info.Modded, true},
		{"brands", len(info.Brands), 1},
		{"gamerule", info.GameRules["keepInventory"], "true"},
		{"player dim", info.Player.Dimension, "minecraft:overworld"},
		{"player xp", info.Player.XPLevel, int64(30)},
		{"player health", info.Player.Health, float64(18)},
		{"playtime", info.Stats.PlayTimeSeconds, int64(3600)},
		{"deaths", info.Stats.Deaths, int64(3)},
		{"distance", info.Stats.DistanceMeters, int64(2000)},
		{"advancements", info.Advancements, 2},
		{"dimensions", len(info.Dimensions), 2},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if len(info.Spawn) != 3 || info.Spawn[2] != -20 {
		t.Errorf("spawn = %v", info.Spawn)
	}
	for _, d := range info.Dimensions {
		if d.ID == "minecraft:overworld" && d.RegionFiles != 2 {
			t.Errorf("overworld regions = %d", d.RegionFiles)
		}
	}
	if info.SizeBytes == 0 {
		t.Error("size is zero")
	}
}

func TestDimensionID(t *testing.T) {
	for rel, want := range map[string]string{
		".":                            "minecraft:overworld",
		"DIM1":                         "minecraft:the_end",
		"dimensions/aether/the_aether": "aether:the_aether",
	} {
		if got := dimensionID(rel); got != want {
			t.Errorf("dimensionID(%q) = %q, want %q", rel, got, want)
		}
	}
}

func TestAdvancementProgress(t *testing.T) {
	dir := testworld.Create(t, t.TempDir(), "w", testworld.Options{})
	p := AdvancementProgress(dir)
	if p["minecraft:story/root"] != true || p["minecraft:story/mine_stone"] != true {
		t.Errorf("done advancements missing: %v", p)
	}
	if _, ok := p["minecraft:recipes/misc/x"]; ok {
		t.Error("recipes must be excluded")
	}
	if _, ok := p["DataVersion"]; ok {
		t.Error("DataVersion must be excluded")
	}
	partial, ok := p["minecraft:story/smelt_iron"].(map[string]any)
	if !ok || partial["criteria"].(map[string]bool)["iron"] != true {
		t.Errorf("partial progress = %#v", p["minecraft:story/smelt_iron"])
	}
}

// Minecraft 26.x moved most data out of level.dat (see testworld.Layout26).
func TestRead26Layout(t *testing.T) {
	dir := testworld.Create(t, t.TempDir(), "New World", testworld.Options{Layout26: true})

	info, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name      string
		got, want any
	}{
		{"version", info.GameVersion, "26.3"},
		{"difficulty", info.Difficulty, "hard"},
		{"hardcore", info.Hardcore, true},
		{"seed", info.Seed, "2762118087573245665"},
		{"weather", info.Weather, "thunder"},
		{"day", info.Day, int64(3)},
		{"gamerule", info.GameRules["minecraft:keep_inventory"], "1"},
		{"advancements", info.Advancements, 1},
		{"player xp", info.Player != nil && info.Player.XPLevel == 7, true},
		{"stats", info.Stats != nil && info.Stats.PlayTimeSeconds == 1800, true},
		{"dimensions", len(info.Dimensions) == 1 && info.Dimensions[0].ID == "minecraft:overworld", true},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if len(info.Spawn) != 3 || info.Spawn[0] != 224 {
		t.Errorf("spawn = %v", info.Spawn)
	}

	p := AdvancementProgress(dir)
	if p["minecraft:story/root"] != true {
		t.Errorf("progress = %v", p)
	}
	if _, ok := p["minecraft:adventure/adventuring_time"].(map[string]any); !ok {
		t.Errorf("partial progress missing: %v", p)
	}
}
