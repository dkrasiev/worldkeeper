// Command winicon writes the Windows app icon, the grass block from
// internal/icon as an .ico file. GoReleaser runs it before the build, and
// go-winres then puts it into worldkeeper.exe together with the version
// info (see before in .goreleaser.yaml and cmd/worldkeeper/winres):
//
//	go run ./tools/winicon -out cmd/worldkeeper/winres/icon.ico
package main

import (
	"flag"
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/dkrasiev/worldkeeper/internal/icon"
)

// sizes are the icon sizes in the file, whole multiples of the 16 px block
// so the pixels stay crisp. Explorer, the Start menu and the taskbar pick
// the nearest one for the display scale.
var sizes = []int{16, 32, 48, 64, 96, 128, 256}

func main() {
	out := flag.String("out", "cmd/worldkeeper/winres/icon.ico", "icon file to write")
	flag.Parse()
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := write(*out); err != nil {
		fmt.Fprintln(os.Stderr, "winicon:", err)
		os.Exit(1)
	}
}

func write(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, ico(), 0o644)
}

// ico draws the block at every size, filling the whole canvas: Windows app
// icons have no margin of their own.
func ico() []byte {
	images := make([]*image.RGBA, len(sizes))
	for i, size := range sizes {
		images[i] = icon.Scale(icon.Block(), size/icon.Size)
	}
	return icon.ICO(images...)
}
