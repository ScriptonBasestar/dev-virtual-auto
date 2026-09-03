package config

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
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
// names it declares. isDvaSample below decides membership from this set.
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

// isDvaSample reports whether a markdown-embedded YAML mapping is a dva.yml sample (whole
// file or a section fragment) rather than some other tool's config caught in the same fence.
//
// It decides by majority: more top-level keys the root schema declares than keys it does not.
// The obvious rule — every key must be known — cannot be used, because the root schema sets
// additionalProperties:false, so an unknown root key IS the defect class this test exists to
// catch. Under "all keys known", a sample whose only fault is `plan:` for `plans:` is
// reclassified as a foreign block and skipped silently, and the test passes by not looking.
//
// Majority keeps the foreign blocks out for the reason they were out before: examples/
// DISCOURSE.md's docker-compose fragment is `services:` plus service names, none of them root
// keys, and even compose's own `version:` — the one name the two schemas share — leaves such a
// block in the minority.
func isDvaSample(top map[string]any, rootKeys map[string]bool) bool {
	known := 0
	for key := range top {
		if rootKeys[key] {
			known++
		}
	}
	return known*2 > len(top)
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

			if !isDvaSample(top, rootKeys) {
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

var (
	strictBinaryOnce sync.Once
	strictBinaryPath string
	strictBinaryErr  error
)

// strictValidateBinary builds cmd/dva once and returns the binary path.
//
// The compose-drift check that "dva config validate --strict" reports lives in
// internal/cli (detectConfigDriftWarnings), which this package cannot import — cli already
// imports config, and importing it back would cycle. The built binary is the only way to
// exercise that real code path from a config package test, matching the pattern already
// used by internal/integration's dvaBinary helper.
func strictValidateBinary(t *testing.T) string {
	t.Helper()
	strictBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "dva-strict-bin-")
		if err != nil {
			strictBinaryErr = err
			return
		}
		strictBinaryPath = filepath.Join(dir, "dva")
		cmd := exec.Command("go", "build", "-o", strictBinaryPath, "./cmd/dva")
		cmd.Dir = filepath.Dir(examplesDir())
		if out, buildErr := cmd.CombinedOutput(); buildErr != nil {
			strictBinaryErr = fmt.Errorf("go build ./cmd/dva: %w\n%s", buildErr, out)
		}
	})
	if strictBinaryErr != nil {
		t.Fatalf("building cmd/dva: %v", strictBinaryErr)
	}
	return strictBinaryPath
}

// composeAbsenceWarningRE matches the two config-drift warnings TASK-276's ruling exempts
// for this corpus: no example under examples/ has ever shipped the compose file it
// references, and examples/README.md documents that the reader supplies it, so the warning
// is a property of a fragment corpus rather than a defect in one. Any other [warn] line
// fails the test.
var composeAbsenceWarningRE = regexp.MustCompile(
	`compose\.files is .* but detected root compose files are \(none\)` +
		`|compose file ".*" is configured by dva\.yml but does not exist`,
)

// TestExamplesStrictCleanExceptComposeAbsence proves every examples/*.yml file is clean
// under `dva config validate --strict` except for the compose-absence warnings the corpus is
// exempted for. service-orchestration.yml carried one further warning here — the
// overlay-split warning firing on its compose "service subsets" shape — until TASK-288
// narrowed the warning's predicate to stop treating a disjoint services: split as an
// overlay; no exemption is needed for it anymore, and its reappearance here would mean that
// fix regressed. The remaining exemption lives here, in the corpus gate, not in --strict
// itself — see TestStrictStillWarnsOnMissingComposeOutsideCorpus for the other half of that
// boundary.
func TestExamplesStrictCleanExceptComposeAbsence(t *testing.T) {
	bin := strictValidateBinary(t)

	// Top-level examples/*.yml only, matching the glob the completion criterion names and
	// the corpus the ruling measured (16 files) — not the recursive examples/modules/ tree,
	// which demonstrates the module-import feature rather than standing in for a project's
	// own dva.yml.
	entries, err := os.ReadDir(examplesDir())
	if err != nil {
		t.Fatalf("read examples dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yml" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, rel := range names {
		path := filepath.Join(examplesDir(), rel)
		t.Run(rel, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", rel, err)
			}
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "dva.yml"), data, 0o644); err != nil {
				t.Fatalf("write scratch dva.yml: %v", err)
			}

			cmd := exec.Command(bin, "config", "validate", "--strict")
			cmd.Dir = dir
			out, _ := cmd.CombinedOutput()

			for line := range strings.SplitSeq(string(out), "\n") {
				if !strings.HasPrefix(line, "[warn]") {
					continue
				}
				if composeAbsenceWarningRE.MatchString(line) {
					continue
				}
				t.Errorf("unexpected strict warning: %s", line)
			}
		})
	}
}
