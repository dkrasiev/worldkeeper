package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
)

// ICO packs square images, at most 256 px each, into an .ico file, which
// Windows needs for tray and app icons. Each image is stored as a PNG,
// which ICO files support since Windows Vista.
func ICO(images ...*image.RGBA) []byte {
	pngs := make([][]byte, len(images))
	for i, img := range images {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			panic(err) // encoding an in-memory RGBA image cannot fail
		}
		pngs[i] = buf.Bytes()
	}

	var buf bytes.Buffer
	w := func(v any) { _ = binary.Write(&buf, binary.LittleEndian, v) }
	w(uint16(0))           // reserved
	w(uint16(1))           // type: icon
	w(uint16(len(images))) // image count
	offset := 6 + 16*len(images)
	for i, img := range images {
		size := img.Bounds().Dx()
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
		w(uint32(len(pngs[i]))) // image size
		w(uint32(offset))       // image offset
		offset += len(pngs[i])
	}
	for _, p := range pngs {
		buf.Write(p)
	}
	return buf.Bytes()
}
