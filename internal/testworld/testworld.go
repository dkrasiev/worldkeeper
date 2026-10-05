// Package testworld builds small fake Minecraft worlds for tests.
package testworld

import (
	"compress/gzip"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/Tnze/go-mc/nbt"
)

type version struct {
	ID       int32  `nbt:"Id"`
	Name     string `nbt:"Name"`
	Snapshot int8   `nbt:"Snapshot"`
}

type worldGen struct {
	Seed int64 `nbt:"seed"`
}

type player struct {
	Pos       []float64 `nbt:"Pos"`
	Dimension string    `nbt:"Dimension"`
	Health    float32   `nbt:"Health"`
	FoodLevel int32     `nbt:"foodLevel"`
	XpLevel   int32     `nbt:"XpLevel"`
}

type data struct {
	LevelName        string            `nbt:"LevelName"`
	DataVersion      int32             `nbt:"DataVersion"`
	Version          version           `nbt:"Version"`
	GameType         int32             `nbt:"GameType"`
	Hardcore         int8              `nbt:"hardcore"`
	Difficulty       int8              `nbt:"Difficulty"`
	AllowCommands    int8              `nbt:"allowCommands"`
	LastPlayed       int64             `nbt:"LastPlayed"`
	DayTime          int64             `nbt:"DayTime"`
	Raining          int8              `nbt:"raining"`
	SpawnX           int32             `nbt:"SpawnX"`
	SpawnY           int32             `nbt:"SpawnY"`
	SpawnZ           int32             `nbt:"SpawnZ"`
	WasModded        int8              `nbt:"WasModded"`
	ServerBrands     []string          `nbt:"ServerBrands"`
	WorldGenSettings worldGen          `nbt:"WorldGenSettings"`
	GameRules        map[string]string `nbt:"GameRules"`
	Player           player            `nbt:"Player"`
}

type level struct {
	Data data `nbt:"Data"`
}

// Options tweak the generated world.
type Options struct {
	Name       string
	LastPlayed int64 // unix millis
	// Layout26 writes the 26.x layout: slim level.dat, global state in
	// data/minecraft/*.dat, per-player files under players/, and the
	// overworld under dimensions/minecraft/overworld/.
	Layout26 bool
}

// Create writes a world folder with level.dat, icon, stats, advancements
// and a couple of region files. It returns the world path.
func Create(t testing.TB, saves, folder string, o Options) string {
	t.Helper()
	dir := filepath.Join(saves, folder)
	if o.Layout26 {
		return create26(t, dir, o)
	}
	must(t, os.MkdirAll(filepath.Join(dir, "region"), 0o755))
	must(t, os.MkdirAll(filepath.Join(dir, "DIM-1", "region"), 0o755))
	must(t, os.MkdirAll(filepath.Join(dir, "stats"), 0o755))
	must(t, os.MkdirAll(filepath.Join(dir, "advancements"), 0o755))

	if o.Name == "" {
		o.Name = folder
	}
	if o.LastPlayed == 0 {
		o.LastPlayed = 1759680000000
	}
	WriteLevel(t, dir, o)

	files := map[string]string{
		"session.lock":           "☃",
		"region/r.0.0.mca":       "overworld region",
		"region/r.-1.0.mca":      "overworld region",
		"DIM-1/region/r.0.0.mca": "nether region",
		"stats/0000.json":        `{"stats":{"minecraft:custom":{"minecraft:play_time":72000,"minecraft:deaths":3,"minecraft:mob_kills":42,"minecraft:walk_one_cm":150000,"minecraft:sprint_one_cm":50000,"minecraft:jump":10}},"DataVersion":4189}`,
		"advancements/0000.json": `{"minecraft:story/root":{"done":true},"minecraft:story/mine_stone":{"done":true},"minecraft:recipes/misc/x":{"done":true},"minecraft:story/smelt_iron":{"criteria":{"iron":"2026-10-05 12:00:00 +0000"},"done":false},"DataVersion":4189}`,
	}
	for name, content := range files {
		must(t, os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(content), 0o644))
	}
	writeIcon(t, filepath.Join(dir, "icon.png"))
	return dir
}

// writeIcon draws a tiny grass block so the UI has a real PNG to show.
func writeIcon(t testing.TB, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			c := color.RGBA{0x86, 0x60, 0x43, 0xff}
			if y < 5 || (y < 7 && (x*7+y)%3 == 0) {
				c = color.RGBA{0x5d, 0x9e, 0x3a, 0xff}
			}
			img.Set(x, y, c)
		}
	}
	f, err := os.Create(path)
	must(t, err)
	must(t, png.Encode(f, img))
	must(t, f.Close())
}

// WriteLevel (re)writes level.dat, e.g. to simulate the game saving.
func WriteLevel(t testing.TB, dir string, o Options) {
	t.Helper()
	l := level{Data: data{
		LevelName:        o.Name,
		DataVersion:      4189,
		Version:          version{ID: 4189, Name: "1.21.4"},
		GameType:         0,
		Hardcore:         0,
		Difficulty:       2,
		AllowCommands:    1,
		LastPlayed:       o.LastPlayed,
		DayTime:          24000*12 + 500,
		Raining:          1,
		SpawnX:           10,
		SpawnY:           64,
		SpawnZ:           -20,
		WasModded:        1,
		ServerBrands:     []string{"fabric"},
		WorldGenSettings: worldGen{Seed: -4172144997902289642},
		GameRules:        map[string]string{"keepInventory": "true"},
		Player: player{
			Pos:       []float64{1.5, 70, -3.25},
			Dimension: "minecraft:overworld",
			Health:    18,
			FoodLevel: 17,
			XpLevel:   30,
		},
	}}

	f, err := os.Create(filepath.Join(dir, "level.dat"))
	must(t, err)
	gz := gzip.NewWriter(f)
	must(t, nbt.NewEncoder(gz).Encode(l, ""))
	must(t, gz.Close())
	must(t, f.Close())
}

func must(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

type difficulty26 struct {
	Difficulty string `nbt:"difficulty"`
	Hardcore   int8   `nbt:"hardcore"`
	Locked     int8   `nbt:"locked"`
}

type spawn26 struct {
	Dimension string  `nbt:"dimension"`
	Pos       []int32 `nbt:"pos,int_array"`
}

type data26 struct {
	LevelName          string       `nbt:"LevelName"`
	DataVersion        int32        `nbt:"DataVersion"`
	Version            version      `nbt:"Version"`
	GameType           int32        `nbt:"GameType"`
	AllowCommands      int8         `nbt:"allowCommands"`
	LastPlayed         int64        `nbt:"LastPlayed"`
	Time               int64        `nbt:"Time"`
	DifficultySettings difficulty26 `nbt:"difficulty_settings"`
	Spawn              spawn26      `nbt:"spawn"`
	ServerBrands       []string     `nbt:"ServerBrands"`
}

type wrapped[T any] struct {
	DataVersion int32 `nbt:"DataVersion"`
	Data        T     `nbt:"data"`
}

type weather26 struct {
	Raining    int8 `nbt:"raining"`
	Thundering int8 `nbt:"thundering"`
}

// create26 mirrors a world saved by Minecraft 26.3.
func create26(t testing.TB, dir string, o Options) string {
	t.Helper()
	if o.Name == "" {
		o.Name = filepath.Base(dir)
	}
	if o.LastPlayed == 0 {
		o.LastPlayed = 1791218913021
	}
	for _, d := range []string{"data/minecraft", "players/data", "players/stats", "players/advancements", "dimensions/minecraft/overworld/region"} {
		must(t, os.MkdirAll(filepath.Join(dir, filepath.FromSlash(d)), 0o755))
	}
	writeNBT(t, filepath.Join(dir, "level.dat"), level26{Data: data26{
		LevelName:          o.Name,
		DataVersion:        5023,
		Version:            version{ID: 5023, Name: "26.3"},
		LastPlayed:         o.LastPlayed,
		Time:               24000*3 + 12,
		DifficultySettings: difficulty26{Difficulty: "hard", Hardcore: 1},
		Spawn:              spawn26{Dimension: "minecraft:overworld", Pos: []int32{224, 66, -160}},
		ServerBrands:       []string{"vanilla"},
	}})
	writeNBT(t, filepath.Join(dir, "data", "minecraft", "world_gen_settings.dat"), wrapped[worldGen]{5023, worldGen{Seed: 2762118087573245665}})
	writeNBT(t, filepath.Join(dir, "data", "minecraft", "game_rules.dat"), wrapped[map[string]int8]{5023, map[string]int8{"minecraft:keep_inventory": 1}})
	writeNBT(t, filepath.Join(dir, "data", "minecraft", "weather.dat"), wrapped[weather26]{5023, weather26{Thundering: 1}})
	writeNBT(t, filepath.Join(dir, "players", "data", "0000.dat"), player{
		Pos: []float64{184.7, 64, -66.3}, Dimension: "minecraft:overworld", Health: 20, FoodLevel: 20, XpLevel: 7,
	})

	files := map[string]string{
		"session.lock": "\u2603",
		"dimensions/minecraft/overworld/region/r.0.0.mca": "overworld region",
		"players/stats/0000.json":                         `{"stats":{"minecraft:custom":{"minecraft:play_time":36000,"minecraft:deaths":1}},"DataVersion":5023}`,
		"players/advancements/0000.json":                  `{"minecraft:recipes/decorations/crafting_table":{"criteria":{"unlock_right_away":"2026-10-05 19:39:06 +0300"},"done":true},"minecraft:story/root":{"criteria":{"crafting_table":"2026-10-05 19:40:00 +0300"},"done":true},"minecraft:adventure/adventuring_time":{"criteria":{"minecraft:beach":"2026-10-05 19:46:33 +0300","minecraft:forest":"2026-10-05 19:40:08 +0300"},"done":false},"DataVersion":5023}`,
	}
	for name, content := range files {
		must(t, os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(content), 0o644))
	}
	writeIcon(t, filepath.Join(dir, "icon.png"))
	return dir
}

type level26 struct {
	Data data26 `nbt:"Data"`
}

func writeNBT(t testing.TB, path string, v any) {
	t.Helper()
	f, err := os.Create(path)
	must(t, err)
	gz := gzip.NewWriter(f)
	must(t, nbt.NewEncoder(gz).Encode(v, ""))
	must(t, gz.Close())
	must(t, f.Close())
}
