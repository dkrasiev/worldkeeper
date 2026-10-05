//go:build !darwin && !windows

package tray

// Elsewhere the POSIX environment variables are the system setting.
func osLocale() string { return "" }
