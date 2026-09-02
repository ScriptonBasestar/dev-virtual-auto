package cli

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestRunSubprojectCommand_NotFound(t *testing.T) {
	c := &config.Config{
		Subprojects: map[string]config.SubprojectConfig{
			"engine": {Path: "./engine"},
		},
	}
	err := runSubprojectCommand(c, "nonexistent", "test", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent subproject")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err.Error())
	}
	if !strings.Contains(err.Error(), "engine") {
		t.Errorf("error = %q, want to list available subproject 'engine'", err.Error())
	}
}

func TestRunSubprojectCommand_NoSubprojects(t *testing.T) {
	c := &config.Config{}
	err := runSubprojectCommand(c, "any", "test", nil)
	if err == nil {
		t.Fatal("expected error when no subprojects exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err.Error())
	}
}
