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
}

// Create writes a world folder with level.dat, icon, stats, advancements
// and a couple of region files. It returns the world path.
func Create(t testing.TB, saves, folder string, o Options) string {
	t.Helper()
	dir := filepath.Join(saves, folder)
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
