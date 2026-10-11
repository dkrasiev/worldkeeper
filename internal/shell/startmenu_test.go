package shell

import (
	"errors"
	"testing"
)

// TestStartMenuUnavailable covers systems without a Start menu.
func TestStartMenuUnavailable(t *testing.T) {
	s := &StartMenu{Command: []string{"/usr/bin/worldkeeper"}}
	if s.Available() || s.Exists() {
		t.Fatalf("available=%v exists=%v", s.Available(), s.Exists())
	}
	if err := s.Set(true); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("Set(true) = %v", err)
	}
	if changed, err := s.Refresh(); changed || err != nil {
		t.Errorf("Refresh = %v %v", changed, err)
	}
}
