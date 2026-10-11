package icon

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"testing"
)

func TestICO(t *testing.T) {
	ico := ICO(Scale(Block(), 2), Scale(Block(), 16))
	r := bytes.NewReader(ico)
	var hdr struct{ Reserved, Type, Count uint16 }
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		t.Fatal(err)
	}
	if hdr.Type != 1 || hdr.Count != 2 {
		t.Fatalf("ico header = %+v", hdr)
	}
	for i, want := range []int{32, 256} {
		var e struct {
			W, H, Colors, Res uint8
			Planes, BPP       uint16
			Size, Offset      uint32
		}
		if err := binary.Read(r, binary.LittleEndian, &e); err != nil {
			t.Fatal(err)
		}
		if int(e.W) != want%256 || e.H != e.W || e.BPP != 32 {
			t.Errorf("entry %d = %+v, want %d px", i, e, want)
		}
		img, err := png.Decode(bytes.NewReader(ico[e.Offset : e.Offset+e.Size]))
		if err != nil {
			t.Fatalf("entry %d: %v", i, err)
		}
		if img.Bounds().Dx() != want {
			t.Errorf("entry %d is %v", i, img.Bounds())
		}
	}
}
