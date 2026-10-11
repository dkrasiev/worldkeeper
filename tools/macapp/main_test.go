package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestBundleVersion(t *testing.T) {
	for in, want := range map[string]string{
		"0.3.0":                              "0.3.0",
		"v1.2.3":                             "1.2.3",
		"0.3.0-dev.abc1234":                  "0.3.0",
		"0.3.0-develop.202610111530.abc1234": "0.3.0",
		"10.20.30+meta":                      "10.20.30",
	} {
		if got, err := bundleVersion(in); err != nil || got != want {
			t.Errorf("bundleVersion(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "dev", "1.2", "1.2.3.4", "x1.2.3"} {
		if got, err := bundleVersion(bad); err == nil {
			t.Errorf("bundleVersion(%q) = %q, want an error", bad, got)
		}
	}
}

// plistDict reads the top-level <dict> of a property list into key → value,
// with <true/> and <false/> as "true" and "false".
func plistDict(t *testing.T, data []byte) map[string]string {
	t.Helper()
	var doc struct {
		Dict struct {
			Items []struct {
				XMLName xml.Name
				Value   string `xml:",chardata"`
			} `xml:",any"`
		} `xml:"dict"`
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("Info.plist: %v", err)
	}
	m := map[string]string{}
	items := doc.Dict.Items
	for i := 0; i+1 < len(items); i += 2 {
		if items[i].XMLName.Local != "key" {
			t.Fatalf("Info.plist: expected <key>, got <%s>", items[i].XMLName.Local)
		}
		v := items[i+1]
		switch v.XMLName.Local {
		case "true", "false":
			m[items[i].Value] = v.XMLName.Local
		default:
			m[items[i].Value] = v.Value
		}
	}
	return m
}

func TestInfoPlist(t *testing.T) {
	m := plistDict(t, infoPlist("0.3.0"))
	for key, want := range map[string]string{
		"CFBundleIdentifier":         "io.github.dkrasiev.worldkeeper",
		"CFBundleExecutable":         "worldkeeper",
		"CFBundleIconFile":           "Worldkeeper.icns",
		"CFBundleName":               "Worldkeeper",
		"CFBundlePackageType":        "APPL",
		"CFBundleShortVersionString": "0.3.0",
		"CFBundleVersion":            "0.3.0",
		"LSUIElement":                "true",
	} {
		if m[key] != want {
			t.Errorf("%s = %q, want %q", key, m[key], want)
		}
	}
}

func TestICNS(t *testing.T) {
	data, err := encodeICNS()
	if err != nil {
		t.Fatal(err)
	}
	if string(data[:4]) != "icns" || int(binary.BigEndian.Uint32(data[4:8])) != len(data) {
		t.Fatalf("bad icns header %q %d (len %d)", data[:4], binary.BigEndian.Uint32(data[4:8]), len(data))
	}
	sizes := map[string]int{}
	for rest := data[8:]; len(rest) > 0; {
		if len(rest) < 8 {
			t.Fatalf("truncated entry: %d bytes left", len(rest))
		}
		typ, n := string(rest[:4]), int(binary.BigEndian.Uint32(rest[4:8]))
		if n < 8 || n > len(rest) {
			t.Fatalf("%s: bad length %d", typ, n)
		}
		img, err := png.Decode(bytes.NewReader(rest[8:n]))
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if b := img.Bounds(); b.Dx() != b.Dy() {
			t.Errorf("%s is %v, not square", typ, b)
		}
		sizes[typ] = img.Bounds().Dx()
		rest = rest[n:]
	}
	for _, im := range icnsImages {
		if sizes[im.typ] != im.size {
			t.Errorf("%s is %d px, want %d", im.typ, sizes[im.typ], im.size)
		}
	}
	if sizes["icp4"] != 16 || sizes["ic10"] != 1024 {
		t.Errorf("want 16 to 1024 px, got %v", sizes)
	}
}

func TestAppIcon(t *testing.T) {
	// Small sizes fill the canvas; large ones keep a transparent margin.
	if _, _, _, a := appIcon(16).At(0, 0).RGBA(); a == 0 {
		t.Error("16 px icon has a margin")
	}
	big := appIcon(1024)
	if _, _, _, a := big.At(50, 50).RGBA(); a != 0 {
		t.Error("1024 px icon has no margin")
	}
	if _, _, _, a := big.At(512, 512).RGBA(); a == 0 {
		t.Error("1024 px icon is empty in the middle")
	}
	// The margin is even on both sides.
	b := big.Bounds()
	first, last := -1, -1
	for x := 0; x < b.Dx(); x++ {
		if _, _, _, a := big.At(x, 512).RGBA(); a != 0 {
			if first < 0 {
				first = x
			}
			last = x
		}
	}
	if first != b.Dx()-1-last {
		t.Errorf("block spans %d..%d, not centred", first, last)
	}
}

func TestBundleAndZip(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "worldkeeper")
	binData := []byte("\xca\xfe\xba\xbe fake universal binary")
	if err := os.WriteFile(bin, binData, 0o755); err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(dir, "dist", "Worldkeeper.app")
	// A leftover from an earlier run is replaced.
	if err := os.MkdirAll(filepath.Join(app, "Contents", "stale"), 0o755); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(dir, "dist", "Worldkeeper_0.3.0-dev.abc1234_macOS.zip")
	if err := run(bin, "0.3.0-dev.abc1234", app, zipPath, false); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	entries := map[string]*zip.File{}
	for _, f := range zr.File {
		entries[f.Name] = f
	}
	for _, d := range []string{"Worldkeeper.app/", "Worldkeeper.app/Contents/", "Worldkeeper.app/Contents/MacOS/", "Worldkeeper.app/Contents/Resources/"} {
		if f := entries[d]; f == nil || !f.Mode().IsDir() || f.Mode().Perm() != 0o755 {
			t.Errorf("folder %s missing or not 0755: %v", d, f)
		}
	}
	if entries["Worldkeeper.app/Contents/stale/"] != nil {
		t.Error("leftover folder was not removed")
	}
	read := func(name string) []byte {
		f := entries[name]
		if f == nil {
			t.Fatalf("%s missing from the zip; have %v", name, zr.File)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}

	exe := entries["Worldkeeper.app/Contents/MacOS/worldkeeper"]
	if exe == nil || exe.Mode() != 0o755 || exe.CreatorVersion>>8 != 3 {
		t.Fatalf("executable missing, not 0755 or not stored with Unix modes: %+v", exe)
	}
	if got := read(exe.Name); !bytes.Equal(got, binData) {
		t.Errorf("executable = %q, want the binary", got)
	}
	if m := plistDict(t, read("Worldkeeper.app/Contents/Info.plist")); m["CFBundleShortVersionString"] != "0.3.0" {
		t.Errorf("Info.plist version = %q, want 0.3.0", m["CFBundleShortVersionString"])
	}
	icns := read("Worldkeeper.app/Contents/Resources/Worldkeeper.icns")
	if want, _ := encodeICNS(); !bytes.Equal(icns, want) {
		t.Error("icns in the zip differs from encodeICNS")
	}
	for _, name := range []string{"Worldkeeper.app/Contents/Info.plist", "Worldkeeper.app/Contents/Resources/Worldkeeper.icns"} {
		if m := entries[name].Mode(); m != fs.FileMode(0o644) {
			t.Errorf("%s mode = %v, want 0644", name, m)
		}
	}
}
