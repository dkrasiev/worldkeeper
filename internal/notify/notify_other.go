//go:build !darwin && !windows

package notify

import (
	"strings"

	"github.com/godbus/dbus/v5"
)

// native asks the desktop's notification service over the D-Bus session bus
// (org.freedesktop.Notifications), which GNOME, KDE and most others provide.
func native(title, body string) error {
	conn, err := dbus.SessionBus() // shared; never closed
	if err != nil {
		return err
	}
	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	return obj.Call("org.freedesktop.Notifications.Notify", 0,
		"Worldkeeper", // app name
		uint32(0),     // replaces no earlier notification
		"",            // icon
		title,
		markupEscaper.Replace(body),
		[]string{},                // actions
		map[string]dbus.Variant{}, // hints
		int32(-1),                 // server's default timeout
	).Err
}

// Servers may render the body as simple markup, so an error message with
// "<" or "&" in it must be escaped.
var markupEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
