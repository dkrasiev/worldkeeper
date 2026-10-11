package shell

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestShortcutFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Мои программы")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "World Keeper.lnk")
	want := shortcut{
		Target:      `C:\Program Files\Worldkeeper\worldkeeper.exe`,
		Args:        []string{"-config", `C:\wk "x"\config.json`},
		Dir:         `C:\Program Files\Worldkeeper`,
		Description: comment,
		AppID:       AppUserModelID,
	}
	if err := writeShortcut(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := readShortcut(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("read %+v\nwant %+v", got, want)
	}

	// Without arguments or an app ID.
	plain := shortcut{Target: `C:\wk\worldkeeper.exe`}
	if err := writeShortcut(path, plain); err != nil {
		t.Fatal(err)
	}
	if got, err := readShortcut(path); err != nil || !reflect.DeepEqual(got, plain) {
		t.Errorf("read %+v %v, want %+v", got, err, plain)
	}

	if _, err := readShortcut(filepath.Join(dir, "missing.lnk")); err == nil {
		t.Error("reading a missing shortcut succeeded")
	}
}

func TestStartMenu(t *testing.T) {
	tmp := t.TempDir()
	exe := filepath.Join(tmp, "app", "worldkeeper.exe")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	s := &StartMenu{Command: []string{exe}, path: filepath.Join(tmp, "Programs", AppName+".lnk")}

	if !s.Available() || s.Exists() {
		t.Fatalf("before Set: available=%v exists=%v", s.Available(), s.Exists())
	}
	if changed, err := s.Refresh(); changed || err != nil {
		t.Fatalf("Refresh without a shortcut: %v %v", changed, err)
	}
	if err := s.Set(true); err != nil {
		t.Fatal(err)
	}
	if !s.Exists() {
		t.Fatal("no shortcut after Set(true)")
	}
	got, err := readShortcut(s.path)
	if err != nil {
		t.Fatal(err)
	}
	// The shell writes the target's long path; TEMP can be a short one
	// (C:\Users\RUNNER~1\...).
	if !sameFile(got.Target, exe) || got.Args != nil || !sameFile(got.Dir, filepath.Dir(exe)) || got.AppID != AppUserModelID {
		t.Errorf("shortcut = %+v", got)
	}

	// A shortcut to a copy that still exists is kept.
	if changed, err := s.Refresh(); changed || err != nil {
		t.Fatalf("Refresh with existing target: %v %v", changed, err)
	}
	// One to a program that is gone follows this one.
	if err := writeShortcut(s.path, shortcut{Target: filepath.Join(tmp, "gone", "worldkeeper.exe")}); err != nil {
		t.Fatal(err)
	}
	if changed, err := s.Refresh(); !changed || err != nil {
		t.Fatalf("Refresh with missing target: %v %v", changed, err)
	}
	if got, _ := readShortcut(s.path); !sameFile(got.Target, exe) {
		t.Errorf("after Refresh the shortcut starts %q", got.Target)
	}

	if err := s.Set(false); err != nil {
		t.Fatal(err)
	}
	if s.Exists() {
		t.Fatal("shortcut kept after Set(false)")
	}
	if err := s.Set(false); err != nil {
		t.Fatalf("second Set(false): %v", err)
	}
}

func sameFile(a, b string) bool {
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	return err == nil && os.SameFile(fa, fb)
}
