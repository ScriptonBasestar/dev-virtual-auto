package config

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// generatorCorpus are the files that teach an AI how to write dva.yml: the
// agent-mesh library and guardrails, the portable skills, and the reference
// text make generate embeds into the binary.
//
// They are a second copy of the schema written in prose and examples, and
// nothing compiles them. That is how services.{ports,related,hint} and
// env_file.{interpolate,priority} kept being generated into real user configs
// for months after schema.json stopped accepting them: the removal reached the
// schema, the structs and the docs, but not the prompt that produces configs.
func generatorCorpus() []string {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	return []string{
		filepath.Join(root, "agent-mesh-flows", "shared", "library"),
		filepath.Join(root, "agent-mesh-flows", "shared", "guardrails"),
		filepath.Join(root, "skills"),
		filepath.Join(root, "internal", "cli", "library_reference.txt"),
	}
}

// TestRemovedKeysAbsentFromGeneratorCorpus fails when the corpus still writes a
// removed key in YAML key position, which is how a template or example teaches
// it. Prose that names a key to forbid it is intentionally not matched — the
// guardrails have to be able to say "move ports: to endpoints:".
func TestRemovedKeysAbsentFromGeneratorCorpus(t *testing.T) {
	scanned := map[string]*regexp.Regexp{}
	for key := range removedSchemaKeys {
		scanned[key] = regexp.MustCompile(`^\s*` + regexp.QuoteMeta(key) + `\s*:`)
	}

	var files int
	for _, root := range generatorCorpus() {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			switch filepath.Ext(path) {
			case ".md", ".yml", ".yaml", ".txt":
			default:
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files++
			for i, line := range strings.Split(string(content), "\n") {
				for key, re := range scanned {
					if re.MatchString(line) {
						t.Errorf("%s:%d teaches removed key %q\n  %s\n  %s",
							path, i+1, key, strings.TrimSpace(line), removedSchemaKeys[key])
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}

	// A corpus that silently walked nothing would pass forever.
	if files == 0 {
		t.Fatal("generator corpus is empty — the paths moved and this test stopped guarding anything")
	}
}
