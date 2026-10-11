//go:build !windows

package shell

import "errors"

func startMenuPath() string { return "" }

func writeShortcut(path string, s shortcut) error { return errors.ErrUnsupported }

func readShortcut(path string) (shortcut, error) { return shortcut{}, errors.ErrUnsupported }
