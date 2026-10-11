// Command macapp packs the macOS build into Worldkeeper.app and zips it for
// the release. GoReleaser runs it once the darwin builds are joined into a
// universal binary (see universal_binaries in .goreleaser.yaml):
//
//	go run ./tools/macapp -binary <universal binary> -version 0.3.0 \
//	  -app dist/Worldkeeper.app -zip dist/Worldkeeper_0.3.0_macOS.zip
//
// The bundle:
//
//	Worldkeeper.app/Contents/
//	  Info.plist                  bundle id, version; LSUIElement: menu bar only, no Dock icon
//	  MacOS/worldkeeper           the binary
//	  Resources/Worldkeeper.icns  the grass block from internal/icon, 16–1024 px
//
// On macOS the bundle is then ad-hoc signed (codesign --sign -), which seals
// Info.plist and the icon together with the binary. That is not a Developer
// ID signature (see issue #23): Gatekeeper still asks before the first
// start, but it does not call the app damaged.
package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/dkrasiev/worldkeeper/internal/icon"
)

const (
	bundleID   = "io.github.dkrasiev.worldkeeper"
	appName    = "Worldkeeper"
	executable = "worldkeeper"
	// minMacOS is the oldest macOS the Go toolchain in go.mod supports.
	minMacOS = "12.0"
)

func main() {
	bin := flag.String("binary", "", "the darwin binary to put in the bundle (universal: arm64 + x86_64)")
	version := flag.String("version", "", "release version, e.g. 0.3.0 or 0.3.0-dev.abc1234")
	app := flag.String("app", "dist/"+appName+".app", "bundle to create; replaced if it exists")
	zipPath := flag.String("zip", "", "zip of the bundle to create; none if empty")
	sign := flag.Bool("sign", runtime.GOOS == "darwin", "ad-hoc sign the bundle (needs codesign, macOS only)")
	flag.Parse()
	if *bin == "" || *version == "" || flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*bin, *version, *app, *zipPath, *sign); err != nil {
		fmt.Fprintln(os.Stderr, "macapp:", err)
		os.Exit(1)
	}
}

func run(bin, version, app, zipPath string, sign bool) error {
	if err := writeBundle(app, bin, version); err != nil {
		return err
	}
	if sign {
		cmd := exec.Command("codesign", "--force", "--sign", "-", "--timestamp=none", app)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("codesign: %w", err)
		}
	}
	if zipPath != "" {
		return zipBundle(app, zipPath)
	}
	return nil
}

// writeBundle creates the .app folder around the binary.
func writeBundle(app, bin, version string) error {
	short, err := bundleVersion(version)
	if err != nil {
		return err
	}
	binData, err := os.ReadFile(bin)
	if err != nil {
		return err
	}
	icns, err := encodeICNS()
	if err != nil {
		return err
	}
	if err := os.RemoveAll(app); err != nil {
		return err
	}
	files := []struct {
		name string
		data []byte
		mode fs.FileMode
	}{
		{"Contents/Info.plist", infoPlist(short), 0o644},
		{"Contents/MacOS/" + executable, binData, 0o755},
		{"Contents/Resources/" + appName + ".icns", icns, 0o644},
	}
	for _, f := range files {
		p := filepath.Join(app, filepath.FromSlash(f.name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, f.data, f.mode); err != nil {
			return err
		}
	}
	return nil
}

var versionRe = regexp.MustCompile(`^v?(\d+\.\d+\.\d+)(?:[-+].*)?$`)

// bundleVersion is the numeric part of a release version: macOS wants
// CFBundleShortVersionString and CFBundleVersion to be plain numbers, so a
// dev build 0.3.0-develop.202610111530.abc1234 is 0.3.0 to Finder.
func bundleVersion(v string) (string, error) {
	m := versionRe.FindStringSubmatch(v)
	if m == nil {
		return "", fmt.Errorf("version %q is not like 1.2.3", v)
	}
	return m[1], nil
}

// infoPlist describes the app to macOS. LSUIElement keeps it out of the Dock
// and the app switcher: Worldkeeper lives in the menu bar.
func infoPlist(version string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key>
	<string>en</string>
	<key>CFBundleDisplayName</key>
	<string>` + appName + `</string>
	<key>CFBundleExecutable</key>
	<string>` + executable + `</string>
	<key>CFBundleIconFile</key>
	<string>` + appName + `.icns</string>
	<key>CFBundleIdentifier</key>
	<string>` + bundleID + `</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleName</key>
	<string>` + appName + `</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>` + version + `</string>
	<key>CFBundleVersion</key>
	<string>` + version + `</string>
	<key>LSApplicationCategoryType</key>
	<string>public.app-category.utilities</string>
	<key>LSMinimumSystemVersion</key>
	<string>` + minMacOS + `</string>
	<key>LSUIElement</key>
	<true/>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>NSHumanReadableCopyright</key>
	<string>© Dmitry Krasiev. MIT License.</string>
</dict>
</plist>
`)
}

// icnsImages are the icon sizes in Worldkeeper.icns, by their icns type
// (the PNG-based ones; @2x types hold the Retina version of a size).
var icnsImages = []struct {
	typ  string
	size int
}{
	{"icp4", 16},
	{"ic11", 32}, // 16@2x
	{"icp5", 32},
	{"ic12", 64}, // 32@2x
	{"ic07", 128},
	{"ic13", 256}, // 128@2x
	{"ic08", 256},
	{"ic14", 512}, // 256@2x
	{"ic09", 512},
	{"ic10", 1024}, // 512@2x
}

// encodeICNS builds the .icns: an "icns" header with the total length, then
// one entry per size (type, length including its 8-byte header, PNG data).
// All numbers are big-endian.
func encodeICNS() ([]byte, error) {
	var body bytes.Buffer
	for _, im := range icnsImages {
		var p bytes.Buffer
		if err := png.Encode(&p, appIcon(im.size)); err != nil {
			return nil, err
		}
		body.WriteString(im.typ)
		_ = binary.Write(&body, binary.BigEndian, uint32(8+p.Len()))
		body.Write(p.Bytes())
	}
	var out bytes.Buffer
	out.WriteString("icns")
	_ = binary.Write(&out, binary.BigEndian, uint32(8+body.Len()))
	out.Write(body.Bytes())
	return out.Bytes(), nil
}

// appIcon draws the grass block on a size×size canvas, scaled by a whole
// number to keep the pixels crisp. Like other Mac app icons it covers about
// 80% of the canvas, with a transparent margin around it; at 16 and 32 px
// that rounds up to the whole canvas.
func appIcon(size int) *image.RGBA {
	factor := max(1, int(math.Round(float64(size)*0.8/icon.Size)))
	block := icon.Scale(icon.Block(), factor)
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	off := (size - block.Bounds().Dx()) / 2
	draw.Draw(img, block.Bounds().Add(image.Pt(off, off)), block, image.Point{}, draw.Src)
	return img
}

// zipBundle zips the .app with the folder itself at the top, so unzipping
// gives Worldkeeper.app. Modes are stored as Unix modes (the executable
// keeps its x bit) and are set by role rather than read from disk, so a zip
// made on Windows comes out the same.
func zipBundle(app, zipPath string) error {
	out, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(out)
	root := filepath.Base(app)
	walkErr := filepath.WalkDir(app, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(app, p)
		if err != nil {
			return err
		}
		name := path.Join(root, filepath.ToSlash(rel))
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			h := &zip.FileHeader{Name: name + "/", Modified: info.ModTime()}
			h.SetMode(fs.ModeDir | 0o755)
			_, err = zw.CreateHeader(h)
			return err
		case !d.Type().IsRegular():
			return fmt.Errorf("%s: only files and folders can be zipped", p)
		}
		h := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: info.ModTime()}
		h.SetMode(fileMode(name))
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
	return errors.Join(walkErr, zw.Close(), out.Close())
}

// fileMode is 0755 for the programs in Contents/MacOS and 0644 for the rest.
func fileMode(name string) fs.FileMode {
	if strings.Contains(name, "/Contents/MacOS/") {
		return 0o755
	}
	return 0o644
}
