package cli

import (
	"errors"
	"os"
	"path/filepath"
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
	if _, statErr := os.Stat(filepath.Join(dir, config.FileName)); !os.IsNotExist(statErr) {
		t.Fatalf("dva.yml was created or could not be checked: %v", statErr)
	}
}
