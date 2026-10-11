package tray

import (
	"bytes"
	"encoding/binary"
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

// encodeICO wraps a PNG in an .ico container, which Windows needs for tray
// icons. PNG-compressed ICO entries are supported since Windows Vista.
func encodeICO(pngData []byte, size int) []byte {
	var buf bytes.Buffer
	w := func(v any) { _ = binary.Write(&buf, binary.LittleEndian, v) }
	w(uint16(0)) // reserved
	w(uint16(1)) // type: icon
	w(uint16(1)) // image count
	dim := uint8(size)
	if size >= 256 {
		dim = 0 // 0 means 256
	}
	w(dim)                  // width
	w(dim)                  // height
	w(uint8(0))             // palette size
	w(uint8(0))             // reserved
	w(uint16(1))            // color planes
	w(uint16(32))           // bits per pixel
	w(uint32(len(pngData))) // image size
	w(uint32(6 + 16))       // image offset
	buf.Write(pngData)
	return buf.Bytes()
}

// icons returns the normal and warning icons in the format the platform's
// tray expects.
func icons() (normal, warning []byte) {
	base := icon.Block()
	if runtime.GOOS == "windows" {
		return encodeICO(encodePNG(icon.Scale(base, 2)), 32), encodeICO(encodePNG(icon.Scale(withAlert(base), 2)), 32)
	}
	return encodePNG(icon.Scale(base, 4)), encodePNG(icon.Scale(withAlert(base), 4))
}
