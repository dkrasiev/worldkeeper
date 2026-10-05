package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"
)

func TestAnonymizeNBTAndText(t *testing.T) {
	real := "2ad83172-cc63-4bc9-b6d9-93913ff6eb21"
	fake := map[string]string{real: "00000000-0000-4000-8000-000000000001"}
	rb := uuidBytes(real)

	// NBT-like payload: int array UUID, separate most/least longs, and text.
	raw := append(append(append([]byte("x"), rb...), rb[:8]...), rb[8:]...)
	raw = append(raw, []byte(" owner="+real)...)
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(raw)
	zw.Close()

	out, err := anonymize("players/data/x.dat", buf.Bytes(), fake)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal("output must stay gzipped:", err)
	}
	got, _ := io.ReadAll(zr)
	for _, needle := range [][]byte{rb, rb[:8], rb[8:], []byte(real)} {
		if bytes.Contains(got, needle) {
			t.Errorf("real UUID bytes %x still present", needle)
		}
	}
	if len(got) != len(raw) {
		t.Errorf("length changed: %d -> %d", len(raw), len(got))
	}

	js, _ := anonymize("players/stats/"+real+".json", []byte(`{"by":"`+real+`"}`), fake)
	if bytes.Contains(js, []byte(real)) {
		t.Error("UUID left in JSON")
	}
	if anonText("players/stats/"+real+".json", fake) != "players/stats/00000000-0000-4000-8000-000000000001.json" {
		t.Error("file name not anonymized")
	}
}

func TestKeep(t *testing.T) {
	for p, want := range map[string]bool{
		"level.dat":                  true,
		"level.dat_old":              false,
		"session.lock":               false,
		"players/data/u.dat":         true,
		"players/data/u.dat_old":     false,
		"playerdata/u.dat":           true,
		"data/minecraft/weather.dat": true,
		"region/r.0.0.mca":           false, // copied empty via isRegion instead
		"entities/r.0.0.mca":         false,
	} {
		if keep(p) != want {
			t.Errorf("keep(%q) = %v", p, !want)
		}
	}
}
