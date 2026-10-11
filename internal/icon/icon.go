// Package icon draws Worldkeeper's icon, a pixel-art grass block, in code,
// so no image assets need to be shipped. The tray icon (internal/tray) and
// the macOS app icon (tools/macapp) are both made from it.
package icon

import (
	"image"
	"image/color"
)

var (
	grass     = color.RGBA{0x5d, 0x9e, 0x3a, 0xff}
	grassDark = color.RGBA{0x4a, 0x80, 0x2c, 0xff}
	dirt      = color.RGBA{0x86, 0x60, 0x43, 0xff}
	dirtDark  = color.RGBA{0x6b, 0x4b, 0x33, 0xff}
)

// Size is the width and height of Block in pixels.
const Size = 16

// Block draws the 16×16 grass block.
func Block() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, Size, Size))
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
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

// Scale enlarges with nearest neighbour to keep pixel-art edges crisp.
func Scale(src *image.RGBA, factor int) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()*factor, b.Dy()*factor))
	for y := 0; y < dst.Bounds().Dy(); y++ {
		for x := 0; x < dst.Bounds().Dx(); x++ {
			dst.SetRGBA(x, y, src.RGBAAt(b.Min.X+x/factor, b.Min.Y+y/factor))
		}
	}
	return dst
}
