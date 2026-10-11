package shell

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// The file formats below are used by the macOS and XDG entries; they build
// everywhere so their tests run on every OS.

// readFile parses an autostart file; a missing file means no entry.
func readFile(path string, parse func([]byte) []string) ([]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parse(data), nil
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func removeFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// launchAgentPlist is a LaunchAgent that starts cmd once at login. Without
// KeepAlive, quitting from the tray stays quit.
func launchAgentPlist(label string, cmd []string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + xmlText(label) + `</string>
	<key>ProgramArguments</key>
	<array>
`)
	for _, arg := range cmd {
		b.WriteString("\t\t<string>" + xmlText(arg) + "</string>\n")
	}
	b.WriteString(`	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>ProcessType</key>
	<string>Interactive</string>
</dict>
</plist>
`)
	return b.String()
}

func xmlText(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// parseLaunchAgent returns the ProgramArguments of a LaunchAgent plist, nil
// when it has <key>Disabled</key><true/>, or [""] when it cannot be read.
func parseLaunchAgent(data []byte) []string {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	var (
		key      string // the last <key> of the top-level dict
		inKey    bool
		depth    int // <plist> is 1, its <dict> 2, the dict entries 3
		args     []string
		found    bool
		disabled bool
	)
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return []string{""}
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			inKey = t.Name.Local == "key" && depth == 3
			if depth == 3 && key == "Disabled" && t.Name.Local == "true" {
				disabled = true
			}
			if depth == 4 && key == "ProgramArguments" && t.Name.Local == "string" {
				var s string
				if err := d.DecodeElement(&s, &t); err != nil {
					return []string{""}
				}
				args = append(args, s)
				found = true
				depth--
			}
		case xml.EndElement:
			depth--
			inKey = false
		case xml.CharData:
			if inKey {
				key = string(t)
			}
		}
	}
	if disabled {
		return nil
	}
	if !found {
		return []string{""}
	}
	return args
}

// desktopFile is an XDG autostart entry that starts cmd.
func desktopFile(name string, cmd []string) string {
	return "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=" + name + "\n" +
		"Comment=" + comment + "\n" +
		"Exec=" + desktopExec(cmd) + "\n" +
		"Terminal=false\n" +
		"X-GNOME-Autostart-enabled=true\n"
}

// desktopReserved are the characters that make an Exec argument need quotes.
const desktopReserved = " \t\n\"'\\><~|&;$*?#()`"

// desktopExec quotes cmd for the Exec key: arguments with reserved
// characters go in double quotes with ", `, $ and \ escaped, then the value
// gets the general string escapes and % is doubled.
func desktopExec(cmd []string) string {
	parts := make([]string, len(cmd))
	for i, arg := range cmd {
		if arg != "" && !strings.ContainsAny(arg, desktopReserved) {
			parts[i] = arg
			continue
		}
		var b strings.Builder
		b.WriteByte('"')
		for _, r := range arg {
			if strings.ContainsRune("\"`$\\", r) {
				b.WriteByte('\\')
			}
			b.WriteRune(r)
		}
		b.WriteByte('"')
		parts[i] = b.String()
	}
	line := strings.Join(parts, " ")
	line = strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\t", `\t`, "\r", `\r`, "%", "%%").Replace(line)
	return line
}

// parseDesktopEntry returns the Exec command of a desktop file, nil when it
// is hidden or turned off in GNOME, or [""] when it has no Exec.
func parseDesktopEntry(data []byte) []string {
	var exec string
	inEntry := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			inEntry = line == "[Desktop Entry]"
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !inEntry || !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "Exec":
			exec = strings.TrimSpace(v)
		case "Hidden":
			if strings.TrimSpace(v) == "true" {
				return nil
			}
		case "X-GNOME-Autostart-enabled":
			if strings.TrimSpace(v) == "false" {
				return nil
			}
		}
	}
	if cmd := splitDesktopExec(exec); len(cmd) > 0 {
		return cmd
	}
	return []string{""}
}

// splitDesktopExec reverses desktopExec.
func splitDesktopExec(line string) []string {
	line = strings.NewReplacer(`\\`, `\`, `\s`, " ", `\n`, "\n", `\t`, "\t", `\r`, "\r").Replace(line)
	var (
		args    []string
		cur     strings.Builder
		inArg   bool
		quoted  bool
		escaped bool
	)
	for _, r := range line {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case quoted && r == '\\':
			escaped = true
		case r == '"':
			quoted = !quoted
			inArg = true
		case !quoted && (r == ' ' || r == '\t' || r == '\n'):
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteRune(r)
			inArg = true
		}
	}
	if inArg {
		args = append(args, cur.String())
	}
	for i, a := range args {
		args[i] = strings.ReplaceAll(a, "%%", "%")
	}
	return args
}
