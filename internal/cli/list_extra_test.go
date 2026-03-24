package cli

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/runner"
)

func TestPrintTable_Detailed(t *testing.T) {
	oldDetailed := lsDetailed
	lsDetailed = true
	defer func() { lsDetailed = oldDetailed }()

	commands := map[string]*runner.ResolvedCommand{
		"test": {Command: "go test ./...", Description: "Run tests", Service: "app"},
	}
	keys := []string{"test"}

	output := captureStdout(t, func() {
		printTable(commands, keys)
	})
	if !strings.Contains(output, "service:app") {
		t.Error("detailed output should contain service info")
	}
}

func TestPrintTable_NoDescription(t *testing.T) {
	commands := map[string]*runner.ResolvedCommand{
		"build": {Command: "make build"},
	}
	keys := []string{"build"}

	output := captureStdout(t, func() {
		printTable(commands, keys)
	})
	if !strings.Contains(output, "build") {
		t.Error("should contain command name")
	}
}

func TestPrintJSON_Output(t *testing.T) {
	commands := map[string]*runner.ResolvedCommand{
		"test": {Command: "go test", Description: "Run tests"},
	}
	keys := []string{"test"}

	output := captureStdout(t, func() {
		printJSON(commands, keys)
	})
	if !strings.Contains(output, "go test") {
		t.Error("JSON should contain command")
	}
	if !strings.Contains(output, "Run tests") {
		t.Error("JSON should contain description")
	}
}

func TestPrintYAML_Output(t *testing.T) {
	commands := map[string]*runner.ResolvedCommand{
		"test": {Command: "go test", Description: "Run tests"},
	}
	keys := []string{"test"}

	output := captureStdout(t, func() {
		printYAML(commands, keys)
	})
	if !strings.Contains(output, "go test") {
		t.Error("YAML should contain command")
	}
}

func TestPrintTable_DetailedWithPod(t *testing.T) {
	oldDetailed := lsDetailed
	lsDetailed = true
	defer func() { lsDetailed = oldDetailed }()

	commands := map[string]*runner.ResolvedCommand{
		"shell": {Command: "bash", Pod: "backend"},
	}
	keys := []string{"shell"}

	output := captureStdout(t, func() {
		printTable(commands, keys)
	})
	if !strings.Contains(output, "pod:backend") {
		t.Error("detailed output should contain pod info")
	}
}
