package main

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "winres", "icon.ico")
	if err := write(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r := bytes.NewReader(data)
	var hdr struct{ Reserved, Type, Count uint16 }
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		t.Fatal(err)
	}
	if hdr.Type != 1 || int(hdr.Count) != len(sizes) {
		t.Fatalf("ico header = %+v", hdr)
	}
	for _, want := range sizes {
		var e struct {
			W, H, Colors, Res uint8
			Planes, BPP       uint16
			Size, Offset      uint32
		}
		if err := binary.Read(r, binary.LittleEndian, &e); err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data[e.Offset : e.Offset+e.Size]))
		if err != nil {
			t.Fatalf("%d px: %v", want, err)
		}
		if b := img.Bounds(); b.Dx() != want || b.Dy() != want || int(e.W) != want%256 {
			t.Errorf("%d px entry: %v, width byte %d", want, b, e.W)
		}
		// The block fills the canvas: no transparent margin.
		if _, _, _, a := img.At(0, 0).RGBA(); a != 0xffff {
			t.Errorf("%d px: corner is transparent", want)
		}
	}
}
