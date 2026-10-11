package worldinfo

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dkrasiev/worldkeeper/internal/testworld"
)

func TestRead(t *testing.T) {
	dir := testworld.Create(t, t.TempDir(), "w1", testworld.Options{Name: "Мой мир", LastPlayed: 1759680000000})

	info, err := Read(dir, "")
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
	p := AdvancementProgress(dir, "")
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

	info, err := Read(dir, "")
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

	p := AdvancementProgress(dir, "")
	if p["minecraft:story/root"] != true {
		t.Errorf("progress = %v", p)
	}
	if _, ok := p["minecraft:adventure/adventuring_time"].(map[string]any); !ok {
		t.Errorf("partial progress missing: %v", p)
	}
}

// In a world several people played (LAN, a server world, a shared save)
// every player section shows the same player: the owner by default.
func TestPlayers(t *testing.T) {
	const owner, guest, other = "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", "00000000-0000-4000-8000-000000000003"
	players := []testworld.Player{
		{UUID: owner, PlayTicks: 20 * 600, XpLevel: 5, Advancements: []string{"minecraft:story/root"}},
		{UUID: guest, PlayTicks: 20 * 3600, XpLevel: 20, Advancements: []string{"minecraft:story/root", "minecraft:story/mine_stone", "minecraft:story/smelt_iron"}},
		{UUID: other, PlayTicks: 20 * 60, XpLevel: 1},
	}
	for _, layout26 := range []bool{false, true} {
		dir := testworld.Create(t, t.TempDir(), "w", testworld.Options{Layout26: layout26, Players: players, Owner: owner})
		check := func(player, wantUUID string, wantXP, wantPlay int64, wantAdv int) {
			t.Helper()
			info, err := Read(dir, player)
			if err != nil {
				t.Fatal(err)
			}
			if info.PlayerUUID != wantUUID || info.Player == nil || info.Player.XPLevel != wantXP ||
				info.Stats == nil || info.Stats.PlayTimeSeconds != wantPlay || info.Advancements != wantAdv {
				t.Errorf("26.x %v, player %q: got %s player=%+v stats=%+v adv=%d", layout26, player, info.PlayerUUID, info.Player, info.Stats, info.Advancements)
			}
			if p := AdvancementProgress(dir, player); len(p) != wantAdv {
				t.Errorf("26.x %v, player %q: progress = %v", layout26, player, p)
			}
			want := []PlayerRef{{UUID: owner, Owner: true, PlayTimeSeconds: 600}, {UUID: guest, PlayTimeSeconds: 3600}, {UUID: other, PlayTimeSeconds: 60}}
			if !reflect.DeepEqual(info.Players, want) {
				t.Errorf("26.x %v: players = %+v", layout26, info.Players)
			}
		}
		check("", owner, 5, 600, 1)
		check(guest, guest, 20, 3600, 3)
		check(strings.ToUpper(guest), guest, 20, 3600, 3)
		check("../level", owner, 5, 600, 1) // not in the world: the main player
	}

	// Without a known owner, the one who played the most.
	dir := testworld.Create(t, t.TempDir(), "server", testworld.Options{Layout26: true, Players: players})
	info, err := Read(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if info.PlayerUUID != guest || info.Players[0].UUID != guest || info.Players[0].Owner {
		t.Errorf("no owner: main = %s, players = %+v", info.PlayerUUID, info.Players)
	}
}

func TestReadUUID(t *testing.T) {
	const notch = "069a79f4-44e9-4726-a5be-fca90e38aaf5"
	for _, m := range []map[string]any{
		{"UUID": []int32{0x069a79f4, 0x44e94726, -0x5a410357, 0x0e38aaf5}},               // 1.16+
		{"UUIDMost": int64(0x069a79f444e94726), "UUIDLeast": int64(-0x5a410356f1c7550b)}, // before 1.16
	} {
		if got := readUUID(m, "UUID"); got != notch {
			t.Errorf("readUUID(%v) = %q, want %q", m, got, notch)
		}
	}
	if got := readUUID(map[string]any{}, "UUID"); got != "" {
		t.Errorf("missing UUID = %q", got)
	}
}
