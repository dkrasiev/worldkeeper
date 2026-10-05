package tray

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"runtime"
)

// The icon is a pixel-art grass block drawn in code, so no image assets
// need to be shipped. The warning variant adds a red dot.
var (
	grass     = color.RGBA{0x5d, 0x9e, 0x3a, 0xff}
	grassDark = color.RGBA{0x4a, 0x80, 0x2c, 0xff}
	dirt      = color.RGBA{0x86, 0x60, 0x43, 0xff}
	dirtDark  = color.RGBA{0x6b, 0x4b, 0x33, 0xff}
	alert     = color.RGBA{0xe5, 0x39, 0x35, 0xff}
	outline   = color.RGBA{0xff, 0xff, 0xff, 0xff}
)

// block draws the 16×16 grass block.
func block() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			noise := (x*7 + y*13) % 5
			var c color.RGBA
			switch {
			case y < 4 || (y < 6 && (x*3+y)%4 == 0):
				c = grass
				if noise == 0 {
					c = grassDark
				}
			default:
				c = dirt
				if noise < 2 {
					c = dirtDark
				}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

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

// scale enlarges with nearest neighbour to keep pixel-art edges crisp.
func scale(src *image.RGBA, factor int) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()*factor, b.Dy()*factor))
	for y := 0; y < dst.Bounds().Dy(); y++ {
		for x := 0; x < dst.Bounds().Dx(); x++ {
			dst.SetRGBA(x, y, src.RGBAAt(x/factor, y/factor))
		}
	}
	return dst
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
	base := block()
	if runtime.GOOS == "windows" {
		return encodeICO(encodePNG(scale(base, 2)), 32), encodeICO(encodePNG(scale(withAlert(base), 2)), 32)
	}
	return encodePNG(scale(base, 4)), encodePNG(scale(withAlert(base), 4))
}
