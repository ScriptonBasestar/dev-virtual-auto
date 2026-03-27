package lifecycle

import (
	"testing"
)

func TestNewPlugin_ValidTypes(t *testing.T) {
	for _, name := range []string{"compose", "process", "script"} {
		p, err := NewPlugin(name)
		if err != nil {
			t.Errorf("NewPlugin(%q) returned error: %v", name, err)
			continue
		}
		if p.Name() != name {
			t.Errorf("NewPlugin(%q).Name() = %q", name, p.Name())
		}
	}
}

func TestNewPlugin_Unknown(t *testing.T) {
	_, err := NewPlugin("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown plugin")
	}
}
