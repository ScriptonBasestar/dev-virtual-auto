package cli

import (
	"testing"
)

func TestIsFlag(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"--debug", true},
		{"-d", true},
		{"run", false},
		{"", false},
		{"up", false},
		{"-", true},
	}
	for _, tt := range tests {
		if got := isFlag(tt.input); got != tt.want {
			t.Errorf("isFlag(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsTopLevelCommand(t *testing.T) {
	// Known built-in commands should return true
	builtins := []string{"run", "up", "down", "stop", "status", "show", "init", "version", "compose", "logs", "build", "restart", "clean", "validate", "manifest", "provision", "ssh", "console", "ls"}
	for _, cmd := range builtins {
		if !isTopLevelCommand(cmd) {
			t.Errorf("isTopLevelCommand(%q) = false, want true", cmd)
		}
	}

	// Unknown commands should return false
	unknowns := []string{"mycommand", "test", "deploy", "foo"}
	for _, cmd := range unknowns {
		if isTopLevelCommand(cmd) {
			t.Errorf("isTopLevelCommand(%q) = true, want false", cmd)
		}
	}
}

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"up", "up", 0},
		{"up", "pu", 2},
		{"run", "rn", 1},
		{"status", "stats", 1},
		{"abc", "def", 3},
	}
	for _, tt := range tests {
		if got := levenshtein(tt.a, tt.b); got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSuggestCommands_KnownSimilar(t *testing.T) {
	// "urn" is 2 edits from "run"
	suggestions := suggestCommands("urn")
	found := false
	for _, s := range suggestions {
		if s == "run" {
			found = true
		}
	}
	if !found {
		t.Errorf("suggestCommands(%q) = %v, want to contain 'run'", "urn", suggestions)
	}
}

func TestSuggestCommands_NoMatch(t *testing.T) {
	suggestions := suggestCommands("zzzzzzzzz")
	if len(suggestions) != 0 {
		t.Errorf("suggestCommands(%q) = %v, want empty", "zzzzzzzzz", suggestions)
	}
}

func TestSuggestCommands_ExactMatch(t *testing.T) {
	suggestions := suggestCommands("up")
	found := false
	for _, s := range suggestions {
		if s == "up" {
			found = true
		}
	}
	if !found {
		t.Errorf("suggestCommands(%q) should include 'up'", "up")
	}
}
