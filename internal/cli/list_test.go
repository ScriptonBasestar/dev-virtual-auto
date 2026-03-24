package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/runner"
)

func TestBuildCommandEntries_Basic(t *testing.T) {
	commands := map[string]*runner.ResolvedCommand{
		"test": {
			Description: "Run tests",
			Command:     "make test",
			Shell:       true,
		},
		"lint": {
			Description: "Run linter",
			Command:     "make lint",
			Service:     "app",
			Compose:     runner.ComposeOpts{Method: "exec"},
		},
	}
	keys := []string{"lint", "test"}

	entries := buildCommandEntries(commands, keys)

	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}

	testEntry := entries["test"].(map[string]any)
	if testEntry["command"] != "make test" {
		t.Errorf("test command = %v", testEntry["command"])
	}
	if testEntry["runner"] != "Local" {
		t.Errorf("test runner = %v, want 'local'", testEntry["runner"])
	}

	lintEntry := entries["lint"].(map[string]any)
	if lintEntry["service"] != "app" {
		t.Errorf("lint service = %v, want 'app'", lintEntry["service"])
	}
	if lintEntry["compose_method"] != "exec" {
		t.Errorf("lint compose_method = %v, want 'exec'", lintEntry["compose_method"])
	}
}

func TestBuildCommandEntries_Empty(t *testing.T) {
	entries := buildCommandEntries(nil, nil)
	if len(entries) != 0 {
		t.Errorf("entries = %d, want 0", len(entries))
	}
}

func TestPrintTable_Basic(t *testing.T) {
	commands := map[string]*runner.ResolvedCommand{
		"test": {
			Description: "Run tests",
			Command:     "make test",
		},
	}
	keys := []string{"test"}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printTable(commands, keys)

	w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])
	if !strings.Contains(output, "test") {
		t.Errorf("output should contain command name, got: %s", output)
	}
	if !strings.Contains(output, "Run tests") {
		t.Errorf("output should contain description, got: %s", output)
	}
}

func TestBuildCommandEntries_WithPod(t *testing.T) {
	commands := map[string]*runner.ResolvedCommand{
		"k8s-cmd": {
			Command: "kubectl exec",
			Pod:     "app-pod",
		},
	}
	keys := []string{"k8s-cmd"}

	entries := buildCommandEntries(commands, keys)
	entry := entries["k8s-cmd"].(map[string]any)
	if entry["pod"] != "app-pod" {
		t.Errorf("pod = %v, want 'app-pod'", entry["pod"])
	}
	if entry["runner"] != "Kubectl" {
		t.Errorf("runner = %v, want 'kubectl'", entry["runner"])
	}
}
