package cli

import (
	"strings"
	"testing"
)

func TestConsoleStartScript_ContainsRequiredParts(t *testing.T) {
	script := consoleStartScript("/usr/local/bin/dva")

	required := []string{
		"DVA_SHELL=1",
		config.EnvPromptTextKey,
		"dva_clear",
		"dva_inject",
		"dva_reload",
		"/usr/local/bin/dva console inject",
	}

	for _, part := range required {
		if !strings.Contains(script, part) {
			t.Errorf("script missing required part: %q", part)
		}
	}
}

func TestConsoleStartScript_BinPathInjected(t *testing.T) {
	path := "/custom/path/to/dva"
	script := consoleStartScript(path)

	if !strings.Contains(script, path) {
		t.Errorf("script should contain bin path %q", path)
	}
}

func TestConsoleStartScript_ChpwdHook(t *testing.T) {
	script := consoleStartScript("/usr/local/bin/dva")

	if !strings.Contains(script, "chpwd_functions") {
		t.Error("script should set up chpwd hook for directory change detection")
	}
	if !strings.Contains(script, "__zsh_like_cd") {
		t.Error("script should contain __zsh_like_cd for bash compatibility")
	}
}

func TestShellBuiltins_ContainsKnownBuiltins(t *testing.T) {
	expected := []string{"test", "echo", "cd", "export", "eval", "exec"}
	for _, name := range expected {
		if !shellBuiltins[name] {
			t.Errorf("shellBuiltins missing %q", name)
		}
	}
}

func TestShellBuiltins_DoesNotContainNonBuiltins(t *testing.T) {
	nonBuiltins := []string{"docker", "git", "make", "npm"}
	for _, name := range nonBuiltins {
		if shellBuiltins[name] {
			t.Errorf("shellBuiltins should not contain %q", name)
		}
	}
}
