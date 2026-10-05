package worldinfo

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/worlds/*.json from the fixtures")

// TestGoldenWorlds parses every real world in testdata/worlds/<name>/ and
// compares the result with testdata/worlds/<name>.json. Fixtures come from
// actual game saves trimmed by tools/trimworld; see testdata/worlds/README.md
// for adding a Minecraft version.
func TestGoldenWorlds(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("testdata", "worlds"))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n++
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join("testdata", "worlds", name)
			info, err := Read(dir)
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.MarshalIndent(struct {
				Info         Info           `json:"info"`
				Advancements map[string]any `json:"advancementProgress"`
			}{info, AdvancementProgress(dir)}, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')

			golden := dir + ".json"
			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (run: go test ./internal/worldinfo -run TestGoldenWorlds -update)", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("%s differs from %s; if the change is intended, rerun with -update and review the diff.\ngot:\n%s", name, golden, got)
			}
		})
	}
	if n == 0 {
		t.Fatal("no fixtures in testdata/worlds")
	}
}
