package config

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"gopkg.in/yaml.v3"
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

// yamlFenceRE matches ```yaml ... ``` fenced code blocks in markdown. Non-greedy so each
// match stops at the first closing fence rather than swallowing the rest of the file.
var yamlFenceRE = regexp.MustCompile("(?s)```yaml\\r?\\n(.*?)```")

// rootSchemaPropertyKeys reads the embedded schema.json and returns the top-level property
// names it declares. A markdown-embedded YAML block whose top-level keys are all members of
// this set is a dva.yml sample (whole file or a section fragment); a block with a foreign
// top-level key (e.g. docker-compose's "services") is some other tool's config caught in the
// same fence and is not a dva.yml sample to validate.
func rootSchemaPropertyKeys(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := embeddedSchema.ReadFile("schema.json")
	if err != nil {
		t.Fatalf("read embedded schema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema has no root properties")
	}
	keys := make(map[string]bool, len(props))
	for k := range props {
		keys[k] = true
	}
	return keys
}

// TestExampleMarkdownValidateAgainstSchema extracts every ```yaml fenced block from
// examples/*.md and validates the dva.yml samples among them against the same schema/
// semantic path used for examples/*.yml (TestExamplesValidateAgainstSchema above). Without
// this, a config-shape defect embedded only in markdown (e.g. a `provision:` block that does
// not unmarshal) has no test that ever looks inside a fenced block to catch it (TASK-276).
func TestExampleMarkdownValidateAgainstSchema(t *testing.T) {
	var paths []string
	if err := filepath.WalkDir(examplesDir(), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		t.Fatalf("walk examples dir: %v", err)
	}
	sort.Strings(paths)

	rootKeys := rootSchemaPropertyKeys(t)

	for _, path := range paths {
		rel, err := filepath.Rel(examplesDir(), path)
		if err != nil {
			t.Fatalf("relative example path: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}

		for i, match := range yamlFenceRE.FindAllSubmatch(data, -1) {
			block := match[1]

			var top map[string]any
			if err := yaml.Unmarshal(block, &top); err != nil {
				t.Fatalf("%s block #%d: invalid YAML: %v", rel, i, err)
			}
			if top == nil {
				// Not a mapping (empty or scalar/sequence document) — not a dva.yml sample.
				continue
			}

			isDvaSample := true
			for key := range top {
				if !rootKeys[key] {
					isDvaSample = false
					break
				}
			}
			if !isDvaSample {
				continue
			}

			t.Run(fmt.Sprintf("%s#%d", rel, i), func(t *testing.T) {
				dir := t.TempDir()
				blockPath := filepath.Join(dir, "dva.yml")
				if err := os.WriteFile(blockPath, block, 0o644); err != nil {
					t.Fatalf("write extracted block: %v", err)
				}
				validateExampleSchema(t, blockPath)
			})
		}
	}
}
