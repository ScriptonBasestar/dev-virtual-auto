package cli

import (
	"strings"
	"testing"
)

func TestDevcontainerYAMLSection_Default(t *testing.T) {
	got := devcontainerYAMLSection("")
	if !strings.Contains(got, "service: app") {
		t.Errorf("expected default service 'app', got: %s", got)
	}
	if !strings.Contains(got, "enabled: true") {
		t.Errorf("expected enabled: true, got: %s", got)
	}
}

func TestDevcontainerYAMLSection_CustomService(t *testing.T) {
	got := devcontainerYAMLSection("backend")
	if !strings.Contains(got, "service: backend") {
		t.Errorf("expected service 'backend', got: %s", got)
	}
}
