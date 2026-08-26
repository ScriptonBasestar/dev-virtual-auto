package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveStaleCursorRules(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".cursor", "rules")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"config.mdc": "---\ndescription: old generated rule\n---\n\n" +
			"<!-- GENERATED from skills/config/SKILL.md by tools/skillgen — do not edit; edit the canonical skill and run `make generate` -->\n",
		"dva-config.mdc": "---\ndescription: current generated rule\n---\n\n" +
			"<!-- GENERATED from skills/dva-config/SKILL.md by tools/skillgen — do not edit; edit the canonical skill and run `make generate` -->\n",
		"manual.mdc": "# hand-authored rule\n\n" +
			"Example markers: <!-- GENERATED from skills/example and by tools/skillgen -->\n",
		"notes.txt": "---\ndescription: generated marker in a different extension\n---\n\n" +
			"<!-- GENERATED from skills/config/SKILL.md by tools/skillgen — do not edit; edit the canonical skill and run `make generate` -->\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	skills := []skill{{fm: frontmatter{Name: "dva-config"}}}
	if err := removeStaleCursorRules(root, skills, ".cursor/rules/", ".mdc"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "config.mdc")); !os.IsNotExist(err) {
		t.Fatalf("stale generated rule still exists: %v", err)
	}
	for _, name := range []string{"dva-config.mdc", "manual.mdc", "notes.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s to be preserved: %v", name, err)
		}
	}
}
