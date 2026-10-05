package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// Release builds use the GUI subsystem so no console window sits next to
// the tray icon. When started from a terminal (worldkeeper list, backup),
// attach to that terminal so output still shows up.
func attachConsole() {
	const attachParentProcess = ^uint32(0) // ATTACH_PARENT_PROCESS = (DWORD)-1
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole")
	if r, _, _ := proc.Call(uintptr(attachParentProcess)); r == 0 {
		return // no parent console, or we already have one
	}
	if f := openConsole(); f != nil {
		os.Stdout, os.Stderr = f, f
	}
}

func openConsole() *os.File {
	name, err := windows.UTF16PtrFromString("CONOUT$")
	if err != nil {
		return nil
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil
	}
	return os.NewFile(uintptr(h), "CONOUT$")
}
