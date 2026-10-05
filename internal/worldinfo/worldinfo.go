// Package worldinfo reads everything we can learn about a Java Edition world
// straight from its files, without running the game.
package worldinfo

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Summary is cheap to compute: it only reads level.dat.
type Summary struct {
	Name        string    `json:"name"`
	GameVersion string    `json:"gameVersion"`
	DataVersion int64     `json:"dataVersion"`
	GameMode    string    `json:"gameMode"`
	Hardcore    bool      `json:"hardcore"`
	LastPlayed  time.Time `json:"lastPlayed"`
	HasIcon     bool      `json:"hasIcon"`
	Modded      bool      `json:"modded"`
}

// Info is the full picture: level.dat, player data, stats and disk usage.
type Info struct {
	Summary
	SnapshotVersion  bool              `json:"snapshotVersion"`
	Difficulty       string            `json:"difficulty"`
	DifficultyLocked bool              `json:"difficultyLocked"`
	Cheats           bool              `json:"cheats"`
	Seed             string            `json:"seed,omitempty"` // string: int64 does not fit a JS number
	Day              int64             `json:"day"`
	Weather          string            `json:"weather"`
	Spawn            []int64           `json:"spawn,omitempty"`
	Brands           []string          `json:"brands"`
	DataPacks        DataPacks         `json:"dataPacks"`
	GameRules        map[string]string `json:"gameRules"`
	Player           *Player           `json:"player,omitempty"`
	Stats            *Stats            `json:"stats,omitempty"`
	Advancements     int               `json:"advancements"`
	SizeBytes        int64             `json:"sizeBytes"`
	Dimensions       []Dimension       `json:"dimensions"`
}

type DataPacks struct {
	Enabled  []string `json:"enabled"`
	Disabled []string `json:"disabled"`
}

type Player struct {
	Dimension string    `json:"dimension"`
	Pos       []float64 `json:"pos,omitempty"`
	Health    float64   `json:"health"`
	Food      int64     `json:"food"`
	XPLevel   int64     `json:"xpLevel"`
}

type Stats struct {
	PlayTimeSeconds int64 `json:"playTimeSeconds"`
	Deaths          int64 `json:"deaths"`
	MobKills        int64 `json:"mobKills"`
	DistanceMeters  int64 `json:"distanceMeters"`
	Jumps           int64 `json:"jumps"`
}

type Dimension struct {
	ID          string `json:"id"`
	RegionFiles int    `json:"regionFiles"`
}

var gameModes = map[int64]string{0: "survival", 1: "creative", 2: "adventure", 3: "spectator"}
var difficulties = map[int64]string{0: "peaceful", 1: "easy", 2: "normal", 3: "hard"}

func readLevel(dir string) (map[string]any, error) {
	root, err := readNBT(filepath.Join(dir, "level.dat"))
	if err != nil {
		return nil, fmt.Errorf("level.dat: %w", err)
	}
	data := getMap(root, "Data")
	if data == nil {
		return nil, fmt.Errorf("level.dat: no Data compound")
	}
	return data, nil
}

// ReadSummary reads the fields needed for the world list.
func ReadSummary(dir string) (Summary, error) {
	data, err := readLevel(dir)
	if err != nil {
		return Summary{}, err
	}
	return summary(dir, data), nil
}

// LastPlayed is a fast change detector: it moves every time the world is saved.
func LastPlayed(dir string) (time.Time, error) {
	s, err := ReadSummary(dir)
	return s.LastPlayed, err
}

func summary(dir string, d map[string]any) Summary {
	s := Summary{
		Name:        getString(d, "LevelName"),
		GameVersion: getString(d, "Version.Name"),
		Hardcore:    getBool(d, "hardcore", "difficulty_settings.hardcore"),
		Modded:      getBool(d, "WasModded"),
	}
	if s.Name == "" {
		s.Name = filepath.Base(dir)
	}
	s.DataVersion, _ = getInt(d, "DataVersion", "Version.Id")
	if gm, ok := getInt(d, "GameType", "Player.playerGameType"); ok {
		s.GameMode = gameModes[gm]
	}
	if ms, ok := getInt(d, "LastPlayed"); ok && ms > 0 {
		s.LastPlayed = time.UnixMilli(ms).UTC()
	}
	if _, err := os.Stat(filepath.Join(dir, "icon.png")); err == nil {
		s.HasIcon = true
	}
	return s
}

// Read gathers the full world info. Missing optional files are not errors.
func Read(dir string) (Info, error) {
	d, err := readLevel(dir)
	if err != nil {
		return Info{}, err
	}
	info := Info{
		Summary:          summary(dir, d),
		SnapshotVersion:  getBool(d, "Version.Snapshot"),
		DifficultyLocked: getBool(d, "DifficultyLocked", "difficulty_settings.locked"),
		Cheats:           getBool(d, "allowCommands"),
		Brands:           getStrings(d, "ServerBrands"),
		DataPacks: DataPacks{
			Enabled:  getStrings(d, "DataPacks.Enabled"),
			Disabled: getStrings(d, "DataPacks.Disabled"),
		},
		GameRules: map[string]string{},
	}

	if v, ok := first(d, "Difficulty", "difficulty_settings.difficulty"); ok {
		if s, ok := v.(string); ok {
			info.Difficulty = s
		} else if n, ok := toInt(v); ok {
			info.Difficulty = difficulties[n]
		}
	}
	// Since 26.x most global state lives in data/minecraft/*.dat instead of
	// level.dat; read whichever exists.
	if seed, ok := getInt(d, "WorldGenSettings.seed", "RandomSeed"); ok {
		info.Seed = strconv.FormatInt(seed, 10)
	} else if seed, ok := getInt(readData(dir, "world_gen_settings"), "seed"); ok {
		info.Seed = strconv.FormatInt(seed, 10)
	}
	if t, ok := getInt(d, "DayTime", "Time"); ok {
		info.Day = t / 24000
	}
	weather := d
	if _, ok := first(d, "raining", "thundering"); !ok {
		weather = readData(dir, "weather")
	}
	switch {
	case getBool(weather, "thundering"):
		info.Weather = "thunder"
	case getBool(weather, "raining"):
		info.Weather = "rain"
	default:
		info.Weather = "clear"
	}
	if pos := getInts(d, "spawn.pos"); len(pos) == 3 {
		info.Spawn = pos
	} else if x, ok := getInt(d, "SpawnX"); ok {
		y, _ := getInt(d, "SpawnY")
		z, _ := getInt(d, "SpawnZ")
		info.Spawn = []int64{x, y, z}
	}
	rules := getMap(d, "GameRules", "game_rules")
	if rules == nil {
		rules = readData(dir, "game_rules")
	}
	for k, v := range rules {
		if k != "DataVersion" {
			info.GameRules[k] = fmt.Sprint(v)
		}
	}

	info.Player = readPlayer(dir, getMap(d, "Player"))
	info.Stats = readStats(dir)
	info.Advancements = countAdvancements(dir)
	info.SizeBytes, info.Dimensions = scanDisk(dir)
	return info, nil
}

func readPlayer(dir string, p map[string]any) *Player {
	if p == nil {
		// Newer versions may keep the singleplayer player only in playerdata/.
		files := playerFiles(dir, "data", "*.dat")
		if len(files) == 0 {
			return nil
		}
		root, err := readNBT(files[0])
		if err != nil {
			return nil
		}
		p = root
	}
	pl := &Player{Pos: getFloats(p, "Pos")}
	if v, ok := first(p, "Dimension"); ok {
		switch dim := v.(type) {
		case string:
			pl.Dimension = dim
		default: // pre-1.16 numeric ids
			n, _ := toInt(dim)
			pl.Dimension = map[int64]string{0: "minecraft:overworld", -1: "minecraft:the_nether", 1: "minecraft:the_end"}[n]
		}
	}
	if v, ok := first(p, "Health"); ok {
		pl.Health, _ = toFloat(v)
	}
	pl.Food, _ = getInt(p, "foodLevel")
	pl.XPLevel, _ = getInt(p, "XpLevel")
	return pl
}

// playerFiles finds per-player files in both world layouts: 26.x keeps
// them under players/{data,stats,advancements}/, older versions under
// playerdata/, stats/ and advancements/ at the world root.
func playerFiles(dir, kind, pattern string) []string {
	legacy := kind
	if kind == "data" {
		legacy = "playerdata"
	}
	files, _ := filepath.Glob(filepath.Join(dir, "players", kind, pattern))
	old, _ := filepath.Glob(filepath.Join(dir, legacy, pattern))
	return append(files, old...)
}

// readData reads the "data" compound of data/minecraft/<name>.dat (26.x).
// A missing file yields nil, which the getters treat as "no value".
func readData(dir, name string) map[string]any {
	root, err := readNBT(filepath.Join(dir, "data", "minecraft", name+".dat"))
	if err != nil {
		return nil
	}
	return getMap(root, "data")
}

// readStats picks the stats file with the most play time (in singleplayer
// there is normally just one).
func readStats(dir string) *Stats {
	files := playerFiles(dir, "stats", "*.json")
	var best *Stats
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var raw struct {
			Stats map[string]map[string]int64 `json:"stats"`
		}
		if json.Unmarshal(b, &raw) != nil {
			continue
		}
		c := raw.Stats["minecraft:custom"]
		ticks := c["minecraft:play_time"]
		if ticks == 0 {
			ticks = c["minecraft:play_one_minute"] // pre-1.17 name, still ticks
		}
		s := &Stats{
			PlayTimeSeconds: ticks / 20,
			Deaths:          c["minecraft:deaths"],
			MobKills:        c["minecraft:mob_kills"],
			Jumps:           c["minecraft:jump"],
		}
		var cm int64
		for k, v := range c {
			if strings.HasSuffix(k, "_one_cm") {
				cm += v
			}
		}
		s.DistanceMeters = cm / 100
		if best == nil || s.PlayTimeSeconds > best.PlayTimeSeconds {
			best = s
		}
	}
	return best
}

type advancement struct {
	Criteria map[string]string `json:"criteria"` // criterion -> completion time
	Done     bool              `json:"done"`
}

// readAdvancements returns the advancements file of the player with the most
// completed advancements (in singleplayer there is normally just one).
// Recipe unlocks are left out: the game tracks them as advancements too.
func readAdvancements(dir string) map[string]advancement {
	files := playerFiles(dir, "advancements", "*.json")
	var best map[string]advancement
	bestDone := -1
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var raw map[string]json.RawMessage
		if json.Unmarshal(b, &raw) != nil {
			continue
		}
		advs := map[string]advancement{}
		done := 0
		for k, v := range raw {
			if k == "DataVersion" || strings.Contains(k, ":recipes/") {
				continue
			}
			var a advancement
			if json.Unmarshal(v, &a) != nil {
				continue
			}
			advs[k] = a
			if a.Done {
				done++
			}
		}
		if done > bestDone {
			best, bestDone = advs, done
		}
	}
	return best
}

func countAdvancements(dir string) int {
	n := 0
	for _, a := range readAdvancements(dir) {
		if a.Done {
			n++
		}
	}
	return n
}

// AdvancementProgress converts the player's advancements into the progress
// file format of the mcwidgets advancement viewer: id -> true when done,
// otherwise id -> {"criteria": {name: true}} for partial progress.
func AdvancementProgress(dir string) map[string]any {
	out := map[string]any{}
	for id, a := range readAdvancements(dir) {
		switch {
		case a.Done:
			out[id] = true
		case len(a.Criteria) > 0:
			criteria := map[string]bool{}
			for name := range a.Criteria {
				criteria[name] = true
			}
			out[id] = map[string]any{"criteria": criteria}
		}
	}
	return out
}

// scanDisk sums file sizes and counts region files per dimension.
func scanDisk(dir string) (int64, []Dimension) {
	var size int64
	regions := map[string]int{}
	_ = filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return nil
		}
		if fi, err := e.Info(); err == nil {
			size += fi.Size()
		}
		if filepath.Ext(path) != ".mca" || filepath.Base(filepath.Dir(path)) != "region" {
			return nil
		}
		rel, err := filepath.Rel(dir, filepath.Dir(filepath.Dir(path)))
		if err != nil {
			return nil
		}
		regions[dimensionID(filepath.ToSlash(rel))]++
		return nil
	})

	dims := make([]Dimension, 0, len(regions))
	for id, n := range regions {
		dims = append(dims, Dimension{ID: id, RegionFiles: n})
	}
	sort.Slice(dims, func(i, j int) bool { return dims[i].ID < dims[j].ID })
	return size, dims
}

// dimensionID maps a folder relative to the world root to a dimension id.
func dimensionID(rel string) string {
	switch rel {
	case ".":
		return "minecraft:overworld"
	case "DIM-1":
		return "minecraft:the_nether"
	case "DIM1":
		return "minecraft:the_end"
	}
	// dimensions/<namespace>/<path...>: custom dimensions from datapacks and mods (1.16+)
	if rest, ok := strings.CutPrefix(rel, "dimensions/"); ok {
		if ns, path, ok := strings.Cut(rest, "/"); ok {
			return ns + ":" + path
		}
	}
	return rel
}
