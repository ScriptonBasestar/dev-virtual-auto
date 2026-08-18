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
// `when:` expressions joined the list once every gate in the corpus turned out to be
// inert: an unquoted `true`/`false` operand compares a rendered string against a YAML
// boolean, and a filter inside the reference defeats the comparison outright. Both let
// the step run unconditionally and still exit 0.
//
// What happens once a gate closes is checked too. am skips a gated step's dependents only
// when they carry a `when:` of their own, and a skipped step's key renders as the literal
// text `{{step.key}}` rather than as empty -- so an ungated reader runs holding a template
// string, and if that reader is a prompt the model answers around it. Both were measured
// on a probe flow; both exit 0.
//
// Comments inside those fields are checked too, because am does not ignore them the way
// /bin/sh does: it drops a comment's plain words but still extracts backtick and `$(...)`
// spans out of it and blocks the step on the first command it does not allow. Three
// shipped steps were blocked that way by the prose explaining the code beneath it. Worse,
// whether a span is extracted at all depends on the apostrophe parity of the whole field
// -- am's quote tracking crosses lines and `#` does not end a quote -- so "don't" in one
// comment hides a span three lines below, and deleting that word arms it. That is why the
// rule reports every span rather than the ones measured to block today.
//
// Rules are checked against fields the runtime hands to /bin/sh, plus `when:` operands.
// Prompt bodies are prose about dva and scanning them produces noise, not findings.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
		total.reportFields += s.reportFields
		total.skippableRefs += s.skippableRefs
		total.shells = append(total.shells, s.shells...)
		total.gates = append(total.gates, s.gates...)
		for _, f := range s.findings {
			failed = true
			fmt.Fprintf(os.Stderr, "%s:%d: [%s] %s\n", path, f.line, f.rule, f.msg)
		}
	}

	// Print what was actually inspected. A rule that silently matches nothing reads
	// exactly like a rule that passed, and that is how a scan rots into decoration.
	fmt.Printf("flowcheck: %d flow file(s), %d shell field(s), %d when-gate(s), %d dva invocation(s), %d report-reading field(s), %d skippable reference(s), %d built-in command(s)\n",
		len(files), len(total.shells), len(total.gates), total.dvaCalls, total.reportFields, total.skippableRefs, len(reserved))
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

// checkBytes scans every YAML document in the stream. yaml.Unmarshal decodes only the
// first, which would have made a `---`-separated flow report clean on the strength of a
// prefix — the same shape of silent partial success the rules themselves exist to catch.
func checkBytes(data []byte, reserved map[string]bool) (*scan, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	s := &scan{}
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("parse: %w", err)
		}
		walk(&doc, "", s)
	}
	for _, f := range s.shells {
		checkShell(f, reserved, s)
	}
	for _, f := range s.gates {
		checkGate(f, s)
	}
	checkSkipPropagation(s)
	sort.SliceStable(s.findings, func(i, j int) bool { return s.findings[i].line < s.findings[j].line })
	return s, nil
}
