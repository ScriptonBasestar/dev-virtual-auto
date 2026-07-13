package config

import (
	"io/fs"
	"path/filepath"
	"sort"
	"testing"
)

func TestExamplesValidateAgainstSchema(t *testing.T) {
	var paths []string
	if err := filepath.WalkDir(examplesDir(), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".yml" {
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		t.Fatalf("walk examples dir: %v", err)
	}
	sort.Strings(paths)

	for _, path := range paths {
		rel, err := filepath.Rel(examplesDir(), path)
		if err != nil {
			t.Fatalf("relative example path: %v", err)
		}
		t.Run(rel, func(t *testing.T) {
			validateExampleSchema(t, path)
		})
	}
}

func validateExampleSchema(t *testing.T, path string) {
	t.Helper()
	cfg, err := loadFile(path)
	if err != nil {
		t.Fatalf("load example: %v", err)
	}
	cfg.filePath = path
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate example: %v", err)
	}
}
