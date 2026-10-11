package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInTempDir(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "worldkeeper.exe")
	if err := os.WriteFile(exe, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if !inTempDir(exe) {
		t.Errorf("%s is not seen as temporary", exe)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if inTempDir(filepath.Join(wd, "main.go")) {
		t.Error("the source folder is seen as temporary")
	}
}
