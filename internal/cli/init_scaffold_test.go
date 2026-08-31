package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestScaffoldDvaYml_ReturnsErrorWithoutCreatingConfig_whenComposeFileIsMissing(t *testing.T) {
	// Given
	dir := t.TempDir()

	// When
	created, err := scaffoldDvaYml(dir, "")

	// Then
	if created {
		t.Fatal("scaffoldDvaYml() created a config without a Compose file")
	}
	if !errors.Is(err, errComposeFileNotFound) {
		t.Fatalf("scaffoldDvaYml() error = %v, want %v", err, errComposeFileNotFound)
	}
	if !strings.Contains(err.Error(), "am run dva-discover") {
		t.Fatalf("missing Compose error should recommend discovery, got: %v", err)
	}
	if !strings.Contains(err.Error(), "am run dva-improve -p mode=rewrite") {
		t.Fatalf("missing Compose error should document explicit rewrite syntax, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, config.FileName)); !os.IsNotExist(statErr) {
		t.Fatalf("dva.yml was created or could not be checked: %v", statErr)
	}
}
