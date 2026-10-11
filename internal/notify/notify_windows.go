package notify

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// appID names Worldkeeper as the sender of its toasts. Windows shows toasts
// only from registered app IDs; an unpackaged app registers its ID under
// HKCU\Software\Classes\AppUserModelId, no installer or shortcut needed.
const appID = "dkrasiev.Worldkeeper"

var registerOnce = sync.OnceValue(func() error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\AppUserModelId\`+appID, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue("DisplayName", "Worldkeeper")
})

// toastScript shows a toast through the WinRT API, which Windows PowerShell
// can call without cgo or a helper binary. The texts come in through the
// environment and are added as text nodes, so they need no escaping.
const toastScript = `$ErrorActionPreference = 'Stop'
$m = [Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime]
$xml = $m::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$text = $xml.GetElementsByTagName('text')
[void]$text.Item(0).AppendChild($xml.CreateTextNode($env:WK_TOAST_TITLE))
[void]$text.Item(1).AppendChild($xml.CreateTextNode($env:WK_TOAST_BODY))
$m::CreateToastNotifier($env:WK_TOAST_APP).Show([Windows.UI.Notifications.ToastNotification]::new($xml))
`

func native(title, body string) error {
	if err := registerOnce(); err != nil {
		return fmt.Errorf("register app id: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Windows PowerShell 5.1 by full path: it ships with Windows and,
	// unlike PowerShell 7, can load WinRT types.
	ps := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if os.Getenv("SystemRoot") == "" {
		ps = "powershell.exe"
	}
	cmd := exec.CommandContext(ctx, ps, "-NoProfile", "-NonInteractive", "-EncodedCommand", encode(toastScript))
	cmd.Env = append(os.Environ(), "WK_TOAST_APP="+appID, "WK_TOAST_TITLE="+title, "WK_TOAST_BODY="+body)
	// No console window flashing up from the GUI-subsystem release build.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("powershell: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// encode prepares a script for -EncodedCommand: base64 of UTF-16LE.
func encode(script string) string {
	u := utf16.Encode([]rune(script))
	b := make([]byte, 2*len(u))
	for i, c := range u {
		b[2*i], b[2*i+1] = byte(c), byte(c>>8)
	}
	return base64.StdEncoding.EncodeToString(b)
}
