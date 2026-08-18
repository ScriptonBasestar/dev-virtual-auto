// Command flowcheck: hold agent-mesh flow definitions to their decision paths.
//
// The guided pipeline decides whether to keep going by reading JSON an LLM wrote, and
// every gate on that path once passed on garbage: `.dva_needed // true` made the stop
// branch unreachable, `jq -e .` waved through a concatenated stream, `exit_if_empty`
// turned a missing CLI into a successful run, `dva app ls` rendered its own error text
// as a finding, and `enum: [true, false]` type-failed the schema. Those are all
// mechanically detectable, and none of them fail loudly at runtime — which is exactly
// why they survived. This program fails the build instead.
//
// Rules are checked only against fields the runtime hands to /bin/sh. Prompt bodies are
// prose about dva and scanning them produces noise, not findings.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/ScriptonBasestar/dva/internal/config"
	"gopkg.in/yaml.v3"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	dir := filepath.Join(root, "agent-mesh-flows")

	files, err := flowFiles(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flowcheck: %v\n", err)
		os.Exit(2)
	}
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "flowcheck: no flow files under %s\n", dir)
		os.Exit(2)
	}

	reserved := config.ReservedCommands()
	total := scan{}
	failed := false

	for _, path := range files {
		s, err := checkFile(path, reserved)
		if err != nil {
			fmt.Fprintf(os.Stderr, "flowcheck: %s: %v\n", path, err)
			os.Exit(2)
		}
		total.dvaCalls += s.dvaCalls
		total.reportReads += s.reportReads
		total.shells = append(total.shells, s.shells...)
		for _, f := range s.findings {
			failed = true
			fmt.Fprintf(os.Stderr, "%s:%d: [%s] %s\n", path, f.line, f.rule, f.msg)
		}
	}

	// Print what was actually inspected. A rule that silently matches nothing reads
	// exactly like a rule that passed, and that is how a scan rots into decoration.
	fmt.Printf("flowcheck: %d flow file(s), %d shell field(s), %d dva invocation(s), %d report read(s), %d built-in command(s)\n",
		len(files), len(total.shells), total.dvaCalls, total.reportReads, len(reserved))
	if failed {
		os.Exit(1)
	}
	fmt.Println("flowcheck: OK — no decision-path defects")
}

// flowFiles lists the YAML under dir. schemas/ holds JSON Schema, not flows.
func flowFiles(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "schemas" {
				return fs.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(path); ext == ".yaml" || ext == ".yml" {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

func checkFile(path string, reserved map[string]bool) (*scan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return checkBytes(data, reserved)
}

func checkBytes(data []byte, reserved map[string]bool) (*scan, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	s := &scan{}
	walk(&doc, "", s)
	for _, f := range s.shells {
		checkShell(f, reserved, s)
	}
	sort.SliceStable(s.findings, func(i, j int) bool { return s.findings[i].line < s.findings[j].line })
	return s, nil
}
