package worldinfo

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Tnze/go-mc/nbt"
)

// readNBT decodes a (usually gzipped) NBT file into generic Go values:
// compounds become map[string]any, lists []any, numbers keep their NBT width.
// Decoding generically keeps us tolerant of format changes between versions.
func readNBT(path string) (map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	br := bufio.NewReader(f)
	var r io.Reader = br
	if head, err := br.Peek(1); err == nil && head[0] == 0x1f {
		gz, err := gzip.NewReader(br)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	}

	var root any
	if _, err := nbt.NewDecoder(r).Decode(&root); err != nil {
		return nil, err
	}
	m, ok := root.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: root tag is not a compound", path)
	}
	return m, nil
}

// get walks a dotted path through nested compounds.
func get(m map[string]any, path string) (any, bool) {
	var cur any = m
	for _, key := range strings.Split(path, ".") {
		c, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = c[key]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// first returns the first path that exists.
func first(m map[string]any, paths ...string) (any, bool) {
	for _, p := range paths {
		if v, ok := get(m, p); ok {
			return v, true
		}
	}
	return nil, false
}

func toInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float32:
		return int64(n), true
	case float64:
		return int64(n), true
	}
	return 0, false
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float32:
		return float64(n), true
	case float64:
		return n, true
	}
	if i, ok := toInt(v); ok {
		return float64(i), true
	}
	return 0, false
}

func getInt(m map[string]any, paths ...string) (int64, bool) {
	v, ok := first(m, paths...)
	if !ok {
		return 0, false
	}
	return toInt(v)
}

func getBool(m map[string]any, paths ...string) bool {
	v, ok := first(m, paths...)
	if !ok {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	n, _ := toInt(v)
	return n != 0
}

func getString(m map[string]any, paths ...string) string {
	v, ok := first(m, paths...)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func getMap(m map[string]any, paths ...string) map[string]any {
	v, _ := first(m, paths...)
	c, _ := v.(map[string]any)
	return c
}

func getStrings(m map[string]any, paths ...string) []string {
	v, _ := first(m, paths...)
	list, _ := v.([]any)
	out := []string{}
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// getInts reads either an int array tag or a list of numbers.
func getInts(m map[string]any, paths ...string) []int64 {
	v, ok := first(m, paths...)
	if !ok {
		return nil
	}
	var out []int64
	switch a := v.(type) {
	case []int32:
		for _, n := range a {
			out = append(out, int64(n))
		}
	case []int64:
		out = append(out, a...)
	case []any:
		for _, item := range a {
			if n, ok := toInt(item); ok {
				out = append(out, n)
			}
		}
	}
	return out
}

func getFloats(m map[string]any, paths ...string) []float64 {
	v, _ := first(m, paths...)
	list, _ := v.([]any)
	var out []float64
	for _, item := range list {
		if f, ok := toFloat(item); ok {
			out = append(out, f)
		}
	}
	return out
}
