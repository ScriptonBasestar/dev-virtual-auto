package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestPrintComposeNameWarnings_Missing(t *testing.T) {
	warnings := []config.ComposeNameWarning{
		{File: "compose.yml", DvaName: "myproject", ComposeName: ""},
	}

	// Capture stderr
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	printComposeNameWarnings(warnings)

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "missing top-level") {
		t.Errorf("output should mention 'missing top-level', got: %s", output)
	}
	if !strings.Contains(output, "myproject") {
		t.Errorf("output should contain project name 'myproject', got: %s", output)
	}
}

func TestPrintComposeNameWarnings_Mismatch(t *testing.T) {
	warnings := []config.ComposeNameWarning{
		{File: "compose.yml", DvaName: "myproject", ComposeName: "old-name"},
	}

	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	printComposeNameWarnings(warnings)

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "differs from") {
		t.Errorf("output should mention 'differs from', got: %s", output)
	}
	if !strings.Contains(output, "old-name") {
		t.Errorf("output should contain old name, got: %s", output)
	}
}

func TestPrintComposeNameWarnings_Empty(t *testing.T) {
	// No warnings should produce no output
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	printComposeNameWarnings(nil)

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	if buf.Len() > 0 {
		t.Errorf("expected no output for empty warnings, got: %s", buf.String())
	}
}
