package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestExamplesAvoidPlainReservedInteractionCommands(t *testing.T) {
	dir := examplesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read examples dir: %v", err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yml" {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			// Given: an example DVA config.
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatalf("read file: %v", err)
			}
			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatalf("parse YAML: %v", err)
			}

			// When: interaction keys are checked against built-in command names.
			conflicts := ValidateReservedCommands(cfg.Interaction)

			// Then: examples use hooks or project-specific names instead.
			if len(conflicts) > 0 {
				names := make([]string, 0, len(conflicts))
				for _, conflict := range conflicts {
					names = append(names, conflict.Name)
				}
				sort.Strings(names)
				t.Fatalf("plain reserved interaction command(s): %s", strings.Join(names, ", "))
			}
		})
	}
}
