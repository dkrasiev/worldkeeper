package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// scratchKey is an HKCU key the tests use instead of the real Run key.
func scratchKey(t *testing.T) string {
	base := fmt.Sprintf(`Software\WorldkeeperTest%d-%s`, os.Getpid(), t.Name())
	t.Cleanup(func() {
		registry.DeleteKey(registry.CURRENT_USER, base+`\Run`)
		registry.DeleteKey(registry.CURRENT_USER, base+`\Approved`)
		registry.DeleteKey(registry.CURRENT_USER, base)
	})
	return base
}

func TestRunEntry(t *testing.T) {
	base := scratchKey(t)
	e := &runEntry{run: base + `\Run`, approved: base + `\Approved`, name: AppName}
	cmd := []string{`C:\Program Files\Worldkeeper\worldkeeper.exe`, "-no-browser", "-config", `C:\wk "x"\config.json`}

	if got, err := e.read(); got != nil || err != nil {
		t.Fatalf("read before write: %q %v", got, err)
	}
	if err := e.remove(); err != nil {
		t.Fatalf("remove without entry: %v", err)
	}
	if err := e.write(cmd); err != nil {
		t.Fatal(err)
	}
	if got, err := e.read(); !reflect.DeepEqual(got, cmd) || err != nil {
		t.Fatalf("read = %q %v, want %q", got, err, cmd)
	}

	// Turned off in Task Manager: the Run value stays, StartupApproved says 3.
	k, _, err := registry.CreateKey(registry.CURRENT_USER, e.approved, registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	err = k.SetBinaryValue(AppName, []byte{3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	k.Close()
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := e.read(); got != nil {
		t.Fatalf("entry turned off in Task Manager reads as %q", got)
	}
	// Turning it on again here clears Task Manager's record.
	if err := e.write(cmd); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.read(); !reflect.DeepEqual(got, cmd) {
		t.Fatalf("after rewrite: %q", got)
	}

	if err := e.remove(); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.read(); got != nil {
		t.Fatalf("read after remove: %q", got)
	}
}

// TestStartupShortcut covers a shortcut put in the Startup folder by hand.
func TestStartupShortcut(t *testing.T) {
	base := scratchKey(t)
	e := &runEntry{run: base + `\Run`, approved: base + `\Approved`, name: AppName, startup: t.TempDir()}
	link := filepath.Join(e.startup, "worldkeeper.exe - Shortcut.lnk")
	other := filepath.Join(e.startup, "Telegram.lnk")
	for _, f := range []string{link, other} {
		if err := os.WriteFile(f, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if got, _ := e.read(); !reflect.DeepEqual(got, []string{link}) {
		t.Fatalf("read = %q, want the shortcut", got)
	}
	// Turning it on writes the Run value instead, so it does not start twice.
	cmd := []string{`C:\wk\worldkeeper.exe`, "-no-browser"}
	if err := e.write(cmd); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.read(); !reflect.DeepEqual(got, cmd) {
		t.Fatalf("read after write = %q", got)
	}
	if _, err := os.Stat(link); !os.IsNotExist(err) {
		t.Errorf("shortcut kept next to the Run value: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("another app's shortcut removed: %v", err)
	}

	// Turning it off removes a shortcut too.
	if err := os.WriteFile(link, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.remove(); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.read(); got != nil {
		t.Fatalf("read after remove = %q", got)
	}
}
