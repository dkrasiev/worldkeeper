package worldinfo

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// PlayerRef is one player with data in the world.
type PlayerRef struct {
	UUID string `json:"uuid"`
	// Name comes from the launcher's usercache.json; worldinfo leaves it
	// empty because the world files only know UUIDs.
	Name            string `json:"name,omitempty"`
	Owner           bool   `json:"owner,omitempty"`
	PlayTimeSeconds int64  `json:"playTimeSeconds"`
}

// playerFileSet is where one player's data, stats and advancements live.
type playerFileSet struct {
	data, stats, advancements string
}

// roster is everyone with files in the world. The player data, stats and
// advancements shown together always belong to one of them.
type roster struct {
	list  []PlayerRef // the main player first
	files map[string]*playerFileSet
	stats map[string]*Stats
}

// readRoster finds the players in both world layouts. The main player is
// the world's owner (the one who created it in singleplayer), otherwise
// the one with the most play time, e.g. on a server world.
func readRoster(fsys fs.FS, level map[string]any) roster {
	r := roster{files: map[string]*playerFileSet{}, stats: map[string]*Stats{}}
	owner := readUUID(level, "singleplayer_uuid") // 26.x
	if owner == "" {
		owner = readUUID(getMap(level, "Player"), "UUID")
	}
	set := func(id string) *playerFileSet {
		if r.files[id] == nil {
			r.files[id] = &playerFileSet{}
		}
		return r.files[id]
	}
	// File names are the players' UUIDs. playerFiles lists the 26.x layout
	// first, so its files win over leftovers of an upgraded world.
	add := func(kind, ext string, field func(*playerFileSet) *string) {
		for _, f := range playerFiles(fsys, kind, "*"+ext) {
			p := field(set(strings.ToLower(strings.TrimSuffix(path.Base(f), ext))))
			if *p == "" {
				*p = f
			}
		}
	}
	add("data", ".dat", func(s *playerFileSet) *string { return &s.data })
	add("stats", ".json", func(s *playerFileSet) *string { return &s.stats })
	add("advancements", ".json", func(s *playerFileSet) *string { return &s.advancements })
	if owner != "" {
		set(owner)
	}

	for id, files := range r.files {
		p := PlayerRef{UUID: id, Owner: id == owner}
		if files.stats != "" {
			if s := readStats(fsys, files.stats); s != nil {
				r.stats[id] = s
				p.PlayTimeSeconds = s.PlayTimeSeconds
			}
		}
		r.list = append(r.list, p)
	}
	sort.Slice(r.list, func(i, j int) bool {
		a, b := r.list[i], r.list[j]
		if a.Owner != b.Owner {
			return a.Owner
		}
		if a.PlayTimeSeconds != b.PlayTimeSeconds {
			return a.PlayTimeSeconds > b.PlayTimeSeconds
		}
		return a.UUID < b.UUID
	})
	return r
}

// pick returns the player to show: want when the world has them, otherwise
// the main player. It is "" only when the world has no player files.
func (r roster) pick(want string) string {
	want = strings.ToLower(want)
	if _, ok := r.files[want]; ok && want != "" {
		return want
	}
	if len(r.list) == 0 {
		return ""
	}
	return r.list[0].UUID
}

// playerData returns the player compound of id. Before 26.x level.dat
// holds the owner's copy, which the game writes together with playerdata/.
// Very old worlds do not record whose it is; it is then the main player's
// when they have no file of their own.
func (r roster) playerData(fsys fs.FS, level map[string]any, id string) map[string]any {
	files := r.files[id]
	if p := getMap(level, "Player"); p != nil {
		switch owner := readUUID(p, "UUID"); {
		case owner == id:
			return p
		case owner == "" && id == r.pick("") && (files == nil || files.data == ""):
			return p
		}
	}
	if files == nil || files.data == "" {
		return nil
	}
	root, err := readNBT(fsys, files.data)
	if err != nil {
		return nil
	}
	return root
}

// readUUID reads a UUID stored as an int array (1.16+) or as the pair of
// longs <key>Most and <key>Least (before 1.16). It returns "" if absent.
func readUUID(m map[string]any, key string) string {
	if v, ok := get(m, key); ok {
		if a, ok := v.([]int32); ok && len(a) == 4 {
			hi := uint64(uint32(a[0]))<<32 | uint64(uint32(a[1]))
			lo := uint64(uint32(a[2]))<<32 | uint64(uint32(a[3]))
			return formatUUID(hi, lo)
		}
	}
	most, ok1 := getInt(m, key+"Most")
	least, ok2 := getInt(m, key+"Least")
	if ok1 && ok2 {
		return formatUUID(uint64(most), uint64(least))
	}
	return ""
}

// formatUUID writes a UUID the way the game names player files.
func formatUUID(hi, lo uint64) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", hi>>32, hi>>16&0xffff, hi&0xffff, lo>>48, lo&0xffffffffffff)
}
