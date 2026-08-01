package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustInventory(t *testing.T, root string, paths ...string) []InventoryEntry {
	t.Helper()
	out := make([]InventoryEntry, 0, len(paths))
	for _, p := range paths {
		full := filepath.Join(root, filepath.FromSlash(p))
		if _, err := os.Stat(full); err != nil {
			t.Fatalf("missing fixture %s: %v", p, err)
		}
		out = append(out, InventoryEntry{Path: p, Mode: modeRegular})
	}
	return out
}

func containsAny(errs []string, needles ...string) bool {
	for _, e := range errs {
		low := strings.ToLower(e)
		for _, n := range needles {
			if strings.Contains(low, n) {
				return true
			}
		}
	}
	return false
}
