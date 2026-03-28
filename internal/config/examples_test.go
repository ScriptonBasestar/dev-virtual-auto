package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/yaml.v3"
)

// examplesDir returns the absolute path to the project examples/ directory.
func examplesDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "examples")
}

func TestExamplesParseSuccessfully(t *testing.T) {
	dir := examplesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read examples dir: %v", err)
	}

	var count int
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yml" {
			continue
		}
		count++
		t.Run(e.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatalf("read file: %v", err)
			}
			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatalf("parse YAML: %v", err)
			}
			if cfg.Version == "" {
				t.Error("example should have a version field")
			}
		})
	}
	if count == 0 {
		t.Fatal("no example YAML files found")
	}
	t.Logf("validated %d example files", count)
}

func TestExampleModulesParseSuccessfully(t *testing.T) {
	dir := examplesDir()

	// Main module config
	t.Run("modules/main.yml", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(dir, "modules", "main.yml"))
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("parse YAML: %v", err)
		}
		if len(cfg.Modules) == 0 {
			t.Error("modules/main.yml should declare modules")
		}
	})

	// Sub-module files (.dva/)
	dvaDir := filepath.Join(dir, "modules", ".dva")
	entries, err := os.ReadDir(dvaDir)
	if err != nil {
		t.Fatalf("read .dva dir: %v", err)
	}

	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".yml" {
			continue
		}
		t.Run(".dva/"+e.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dvaDir, e.Name()))
			if err != nil {
				t.Fatalf("read file: %v", err)
			}
			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatalf("parse YAML: %v", err)
			}
			if len(cfg.Interaction) == 0 {
				t.Error("module should define interaction commands")
			}
		})
	}
}
