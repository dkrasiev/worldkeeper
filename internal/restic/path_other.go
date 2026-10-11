//go:build !windows

package restic

// currentPath adds nothing outside Windows: PATH lives in shell profiles
// there, and the usual install folders are checked by candidates.
func currentPath() []string { return nil }
