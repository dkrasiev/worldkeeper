package restic

import (
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// currentPath is PATH as Windows has it now: the user's and the system's
// entries from the registry. A running process keeps the PATH it was
// started with, which misses what installers added since (winget adds
// restic's folder this way).
func currentPath() []string {
	var dirs []string
	for _, k := range []struct {
		root registry.Key
		path string
	}{
		{registry.CURRENT_USER, `Environment`},
		{registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`},
	} {
		key, err := registry.OpenKey(k.root, k.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		v, _, err := key.GetStringValue("Path")
		key.Close()
		if err != nil {
			continue
		}
		if expanded, err := registry.ExpandString(v); err == nil {
			v = expanded
		}
		dirs = append(dirs, filepath.SplitList(v)...)
	}
	return dirs
}
