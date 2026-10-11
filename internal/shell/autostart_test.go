package shell

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fakeEntry struct{ cmd []string }

func (e *fakeEntry) read() ([]string, error)  { return e.cmd, nil }
func (e *fakeEntry) write(cmd []string) error { e.cmd = cmd; return nil }
func (e *fakeEntry) remove() error            { e.cmd = nil; return nil }

func TestAutostart(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "worldkeeper")
	if err := os.WriteFile(exe, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	e := &fakeEntry{}
	a := &Autostart{Command: []string{exe, "-no-browser"}, entry: e}

	if on, _ := a.Enabled(); on {
		t.Fatal("enabled before Set")
	}
	if err := a.Set(true); err != nil {
		t.Fatal(err)
	}
	if on, _ := a.Enabled(); !on || !reflect.DeepEqual(e.cmd, a.Command) {
		t.Fatalf("after Set(true): on=%v cmd=%q", on, e.cmd)
	}

	// Another copy that still exists keeps the entry.
	other := []string{exe, "-config", "other.json"}
	e.cmd = other
	if changed, err := a.Refresh(); changed || err != nil || !reflect.DeepEqual(e.cmd, other) {
		t.Fatalf("Refresh with existing target: changed=%v err=%v cmd=%q", changed, err, e.cmd)
	}
	// A moved program is followed.
	e.cmd = []string{filepath.Join(t.TempDir(), "gone", "worldkeeper"), "-no-browser"}
	if changed, err := a.Refresh(); !changed || err != nil || !reflect.DeepEqual(e.cmd, a.Command) {
		t.Fatalf("Refresh with missing target: changed=%v err=%v cmd=%q", changed, err, e.cmd)
	}

	if err := a.Set(false); err != nil {
		t.Fatal(err)
	}
	if on, _ := a.Enabled(); on {
		t.Fatal("enabled after Set(false)")
	}
	if changed, _ := a.Refresh(); changed || e.cmd != nil {
		t.Fatal("Refresh recreated a removed entry")
	}
}

func TestLaunchAgent(t *testing.T) {
	cmd := []string{"/Applications/World & Keeper/worldkeeper", "-no-browser", "-config", "/Users/me/<wk>.json"}
	plist := launchAgentPlist(AppID, cmd)
	for _, want := range []string{
		"<string>io.github.dkrasiev.worldkeeper</string>",
		"<string>/Applications/World &amp; Keeper/worldkeeper</string>",
		"<string>/Users/me/&lt;wk&gt;.json</string>",
		"<key>RunAtLoad</key>\n\t<true/>",
	} {
		if !strings.Contains(plist, want) {
			t.Errorf("plist lacks %q:\n%s", want, plist)
		}
	}
	if strings.Contains(plist, "KeepAlive") {
		t.Error("plist must not restart worldkeeper after Quit")
	}
	if got := parseLaunchAgent([]byte(plist)); !reflect.DeepEqual(got, cmd) {
		t.Errorf("parse = %q, want %q", got, cmd)
	}

	disabled := strings.Replace(plist, "<key>RunAtLoad</key>", "<key>Disabled</key>\n\t<true/>\n\t<key>RunAtLoad</key>", 1)
	if got := parseLaunchAgent([]byte(disabled)); got != nil {
		t.Errorf("disabled agent parsed as %q", got)
	}
	if got := parseLaunchAgent([]byte("not a plist")); !reflect.DeepEqual(got, []string{""}) {
		t.Errorf("garbage parsed as %q", got)
	}
}

func TestDesktopEntry(t *testing.T) {
	cmd := []string{"/home/me/My Apps/worldkeeper", "-no-browser", "-config", `/home/me/100% "wk"\$x.json`}
	file := desktopFile(AppName, cmd)
	wantExec := `Exec="/home/me/My Apps/worldkeeper" -no-browser -config "/home/me/100%% \\"wk\\"\\\\\\$x.json"` + "\n"
	if !strings.Contains(file, wantExec) {
		t.Errorf("desktop file lacks %q:\n%s", wantExec, file)
	}
	if !strings.HasPrefix(file, "[Desktop Entry]\nType=Application\nName=Worldkeeper\n") {
		t.Errorf("desktop file header:\n%s", file)
	}
	if got := parseDesktopEntry([]byte(file)); !reflect.DeepEqual(got, cmd) {
		t.Errorf("parse = %q, want %q", got, cmd)
	}
	if got := parseDesktopEntry([]byte(desktopFile(AppName, []string{"/usr/bin/worldkeeper"}))); !reflect.DeepEqual(got, []string{"/usr/bin/worldkeeper"}) {
		t.Errorf("plain parse = %q", got)
	}

	for _, off := range []string{"Hidden=true", "X-GNOME-Autostart-enabled=false"} {
		f := strings.Replace(file, "X-GNOME-Autostart-enabled=true", off, 1)
		if got := parseDesktopEntry([]byte(f)); got != nil {
			t.Errorf("%s parsed as %q", off, got)
		}
	}
	// Keys of other groups are not the entry's.
	other := "[Desktop Entry]\nName=Worldkeeper\n[Desktop Action x]\nExec=/bin/false\n"
	if got := parseDesktopEntry([]byte(other)); !reflect.DeepEqual(got, []string{""}) {
		t.Errorf("entry without Exec parsed as %q", got)
	}
}

func TestFileEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "autostart", AppID+".desktop")
	cmd := []string{"/opt/worldkeeper", "-no-browser"}
	if got, err := readFile(path, parseDesktopEntry); got != nil || err != nil {
		t.Fatalf("missing file: %q %v", got, err)
	}
	if err := writeFile(path, desktopFile(AppName, cmd)); err != nil {
		t.Fatal(err)
	}
	if got, err := readFile(path, parseDesktopEntry); !reflect.DeepEqual(got, cmd) || err != nil {
		t.Fatalf("read = %q %v", got, err)
	}
	if err := removeFile(path); err != nil {
		t.Fatal(err)
	}
	if err := removeFile(path); err != nil {
		t.Fatalf("second remove: %v", err)
	}
}
