package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"runtime"

	"github.com/dkrasiev/worldkeeper/internal/icon"
)

// The tray icon is the grass block from internal/icon. The warning variant
// adds a red dot.
var (
	alert   = color.RGBA{0xe5, 0x39, 0x35, 0xff}
	outline = color.RGBA{0xff, 0xff, 0xff, 0xff}
)

// withAlert puts a red dot with a light rim in the bottom-right corner.
func withAlert(src *image.RGBA) *image.RGBA {
	img := image.NewRGBA(src.Bounds())
	copy(img.Pix, src.Pix)
	const cx, cy, r = 12.0, 12.0, 4.0
	for y := 7; y < 16; y++ {
		for x := 7; x < 16; x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			d := dx*dx + dy*dy
			switch {
			case d <= (r-1)*(r-1):
				img.SetRGBA(x, y, alert)
			case d <= r*r:
				img.SetRGBA(x, y, outline)
			}
		}
	}
	return img
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err) // encoding an in-memory RGBA image cannot fail
	}
	return buf.Bytes()
}

// icons returns the normal and warning icons in the format the platform's
// tray expects: .ico on Windows, PNG elsewhere.
func icons() (normal, warning []byte) {
	base := icon.Block()
	if runtime.GOOS == "windows" {
		return icon.ICO(icon.Scale(base, 2)), icon.ICO(icon.Scale(withAlert(base), 2))
	}
	return encodePNG(icon.Scale(base, 4)), encodePNG(icon.Scale(withAlert(base), 4))
}
