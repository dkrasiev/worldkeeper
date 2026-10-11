package main

import (
	"slices"
	"testing"
)

func TestDropProcessSerial(t *testing.T) {
	got := dropProcessSerial([]string{"/Applications/Worldkeeper.app/Contents/MacOS/worldkeeper", "-psn_0_1234567", "-no-browser"})
	want := []string{"/Applications/Worldkeeper.app/Contents/MacOS/worldkeeper", "-no-browser"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	args := []string{"worldkeeper", "list"}
	if got := dropProcessSerial(args); !slices.Equal(got, args) {
		t.Errorf("got %q, want %q unchanged", got, args)
	}
}

func TestWithPackageManagers(t *testing.T) {
	for in, want := range map[string]string{
		"/usr/bin:/bin:/usr/sbin:/sbin": "/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin:/usr/local/bin:/opt/local/bin",
		// A terminal's PATH keeps its order; only what is missing is added.
		"/opt/homebrew/bin:/usr/bin:/usr/local/bin": "/opt/homebrew/bin:/usr/bin:/usr/local/bin:/opt/local/bin",
		"": "/opt/homebrew/bin:/usr/local/bin:/opt/local/bin",
	} {
		if got := withPackageManagers(in); got != want {
			t.Errorf("withPackageManagers(%q) = %q, want %q", in, got, want)
		}
	}
}
