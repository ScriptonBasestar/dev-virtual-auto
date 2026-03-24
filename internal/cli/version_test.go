package cli

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestVersionText(t *testing.T) {
	output := captureStdout(t, func() {
		versionCmd.RunE(versionCmd, nil)
	})
	if !strings.Contains(output, "dva version") {
		t.Errorf("version output should contain 'dva version', got: %s", output)
	}
	if !strings.Contains(output, config.Version) {
		t.Errorf("version output should contain version %q, got: %s", config.Version, output)
	}
}

func TestVersionJSON(t *testing.T) {
	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	output := captureStdout(t, func() {
		versionCmd.RunE(versionCmd, nil)
	})
	if !strings.Contains(output, "version") {
		t.Errorf("JSON output should contain 'version' key, got: %s", output)
	}
	if !strings.Contains(output, config.Version) {
		t.Errorf("JSON output should contain version %q, got: %s", config.Version, output)
	}
}
