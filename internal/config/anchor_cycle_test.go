package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// cyclicAnchorYAML is the shape that used to end the process in a stack overflow:
// an anchored interaction command whose own subcommand aliases it back.
const cyclicAnchorYAML = `version: "0.1.0"
interaction:
  loop: &loop
    command: echo hi
    subcommands:
      self: *loop
`

// A self-referencing anchor must not reach the decoder. Loading it in-process is
// itself the assertion: before the pre-decode scan this call did not return a bad
// value, it took the test binary down with a runtime fatal error.
func TestLoadFileRejectsCyclicAnchor(t *testing.T) {
	path := writeConfigFile(t, cyclicAnchorYAML)

	_, err := loadFile(path)
	if err == nil {
		t.Fatal("loadFile accepted a self-referencing anchor")
	}
	// The message has to be actionable: which anchor, and where it closes the loop.
	for _, want := range []string{"loop", "interaction.loop.subcommands.self"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// Reaching one node from several places is ordinary YAML, not a cycle. Each of
// these decodes into a usable config and must survive the scan untouched.
func TestLoadFileAcceptsAcyclicAliases(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "anchor shared by siblings",
			yaml: `version: "0.1.0"
interaction:
  alpha: &shared
    command: echo shared
  beta: *shared
  gamma: *shared
`,
		},
		{
			name: "merge key",
			yaml: `version: "0.1.0"
interaction:
  alpha: &base
    command: echo base
    tags: [ci]
  beta:
    <<: *base
    description: beta
`,
		},
		{
			name: "anchor aliased inside its own sibling subtree",
			yaml: `version: "0.1.0"
interaction:
  alpha: &leaf
    command: echo leaf
  beta:
    command: echo beta
    subcommands:
      one: *leaf
      two: *leaf
`,
		},
		{
			name: "empty document",
			yaml: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := loadFile(writeConfigFile(t, tc.yaml)); err != nil {
				t.Fatalf("loadFile: %v", err)
			}
		})
	}
}

// Deep nesting is a warning, not a load failure — the scan must not turn the
// config the depth check exists for into an error before that check can run.
func TestLoadFileAcceptsDeepAcyclicNesting(t *testing.T) {
	var b strings.Builder
	b.WriteString("version: \"0.1.0\"\ninteraction:\n  alpha:\n    command: echo 0\n")
	indent := "    "
	for i := 1; i <= MaxSubcommandDepth+1; i++ {
		fmt.Fprintf(&b, "%ssubcommands:\n", indent)
		indent += "  "
		fmt.Fprintf(&b, "%slevel%d:\n", indent, i)
		indent += "  "
		fmt.Fprintf(&b, "%scommand: echo %d\n", indent, i)
	}

	cfg, err := loadFile(writeConfigFile(t, b.String()))
	if err != nil {
		t.Fatalf("loadFile: %v", err)
	}

	warnings := cfg.warnDeepSubcommandNesting()
	if len(warnings) != 1 {
		t.Fatalf("warnDeepSubcommandNesting() = %v, want exactly one warning", warnings)
	}
	if !strings.Contains(warnings[0], "interaction.alpha") {
		t.Errorf("warning %q does not name the nested command", warnings[0])
	}
}

// loadFile is not the only path that decodes user bytes into config types.
// `dva config migrate` verifies its rewrite in memory, and a cyclic anchor
// passes through the rewrite untouched, so that check has to survive it too.
func TestVerifyMigratedRejectsCyclicAnchor(t *testing.T) {
	err := VerifyMigrated([]byte(cyclicAnchorYAML))
	if err == nil {
		t.Fatal("VerifyMigrated accepted a self-referencing anchor")
	}
	if !strings.Contains(err.Error(), "contains itself") {
		t.Errorf("error %q does not report the cycle", err)
	}
}

// The remaining YAML reads in this package do not go through decodeConfig, so
// nothing stops a cyclic document from reaching them. Each is survivable for a
// reason that belongs to its destination type — a Node tree is walked rather
// than decoded, or the type carries no custom unmarshaler, leaving yaml.v3's
// own alias guard intact across the walk. Those are properties of types that
// can be changed by someone who never reads this file, so what is pinned here
// is the only property that matters: none of them ends the process.
//
// A failure here does not fail an assertion, it takes the test binary down.
func TestCyclicAnchorSurvivesTheDecodesThatBypassDecodeConfig(t *testing.T) {
	configPath := writeConfigFile(t, cyclicAnchorYAML)

	t.Run("MigrateLegacyCompose", func(t *testing.T) {
		// The cycle sits inside a legacy compose entry, so the rewrite has to
		// re-encode the node that carries it rather than pass it through.
		src := `version: "0.1.0"
stack:
  compose: &entry
    order: 10
    files: [compose.yml]
    project_name: app
    self: *entry
`
		if _, _, err := MigrateLegacyCompose([]byte(src)); err != nil {
			t.Logf("MigrateLegacyCompose: %v", err)
		}
	})

	t.Run("Config.Validate", func(t *testing.T) {
		// Decodes the file into `any` for the JSON Schema pass.
		cfg := &Config{filePath: configPath}
		// yaml.v3's own alias guard is what fires here, and it can, because `any`
		// carries no custom unmarshaler to reset the decoder it lives on.
		if err := cfg.Validate(); err == nil {
			t.Error("Validate accepted a self-referencing anchor")
		}
	})

	t.Run("validateCanonicalOrder", func(t *testing.T) {
		validateCanonicalOrder(configPath)
	})

	t.Run("readComposeNameKey", func(t *testing.T) {
		// Reads a compose file, which never passes through decodeConfig at all.
		if _, err := readComposeNameKey(writeConfigFile(t, cyclicAnchorYAML)); err != nil {
			t.Logf("readComposeNameKey: %v", err)
		}
	})
}

// Aliasing one anchor from many places multiplies the expanded document without
// adding nodes. The walk must stay proportional to the nodes, not to the expansion:
// re-walking every alias would make this document take longer than the run it is
// checking. Removing that memoisation does not fail this test, it hangs it.
func TestCheckAnchorCyclesHandlesRepeatedAliases(t *testing.T) {
	var b strings.Builder
	b.WriteString("a0: &a0 lol\n")
	const levels, fanout = 12, 9
	for i := 1; i <= levels; i++ {
		fmt.Fprintf(&b, "a%d: &a%d [", i, i)
		for j := range fanout {
			if j > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "*a%d", i-1)
		}
		b.WriteString("]\n")
	}

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(b.String()), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := checkAnchorCycles(&doc); err != nil {
		t.Fatalf("checkAnchorCycles rejected an acyclic document: %v", err)
	}
}

// A cycle is reported wherever it sits, not only under interaction:.
func TestCheckAnchorCyclesReportsPath(t *testing.T) {
	cases := []struct {
		name     string
		yaml     string
		wantPath string
	}{
		{
			name: "mapping value",
			yaml: `vars: &vars
  nested: *vars
`,
			wantPath: "vars.nested",
		},
		{
			name: "sequence element",
			yaml: `items: &items
  - first
  - *items
`,
			wantPath: "items[1]",
		},
		{
			name: "root document",
			yaml: `&root
key: *root
`,
			wantPath: "key",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tc.yaml), &doc); err != nil {
				t.Fatalf("parse: %v", err)
			}
			err := checkAnchorCycles(&doc)
			if err == nil {
				t.Fatal("checkAnchorCycles accepted a cyclic document")
			}
			if !strings.Contains(err.Error(), tc.wantPath) {
				t.Errorf("error %q does not report path %q", err, tc.wantPath)
			}
		})
	}
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// TestMergeKeyInheritsCommand covers TASK-162. InteractionCommand.UnmarshalYAML recovered the
// polymorphic `command:` by hand-scanning the mapping node for the literal key "command". A merge
// key (`<<: *base`) is not a literal key, so a command inherited through a merge was dropped while
// every other field (description, service, …) merged normally — the run then exited 0 having done
// nothing. Moving command onto the Decode path (a custom type with its own UnmarshalYAML) makes it
// honour `<<:` like its neighbours.
func TestMergeKeyInheritsCommand(t *testing.T) {
	dir := t.TempDir()
	content := `version: "0.1.44"
interaction:
  base-scalar: &base-scalar
    command: echo hello
    description: from-base
  via-merge-scalar:
    <<: *base-scalar
  base-list: &base-list
    command: ["echo one", "echo two"]
    description: list-base
  via-merge-list:
    <<: *base-list
  override-after:
    <<: *base-scalar
    command: echo local
  override-before:
    command: echo local
    <<: *base-scalar
`
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Scalar form: command inherited through <<: must populate Command.
	scalar := cfg.Interaction["via-merge-scalar"]
	if scalar.Command != "echo hello" {
		t.Errorf("via-merge-scalar.Command = %q, want \"echo hello\" (the merge key dropped it)", scalar.Command)
	}
	if scalar.Description != "from-base" {
		t.Errorf("via-merge-scalar.Description = %q, want \"from-base\" (control: the merge itself worked)", scalar.Description)
	}

	// Sequence form: command inherited as a list must populate CommandLines too, not just Command.
	list := cfg.Interaction["via-merge-list"]
	if !list.HasMultiCommand() || len(list.CommandLines) != 2 {
		t.Errorf("via-merge-list.CommandLines = %v, want 2 lines inherited through the merge", list.CommandLines)
	}
	if list.Command != "echo one" {
		t.Errorf("via-merge-list.Command = %q, want first line \"echo one\"", list.Command)
	}

	// A local command overrides an inherited one, regardless of key order: merge key before the
	// local key, and local key before the merge key. YAML 1.1 makes an explicit key win over a
	// merge either way, and Decode honours that.
	override := cfg.Interaction["override-after"]
	if override.Command != "echo local" {
		t.Errorf("override-after.Command = %q, want \"echo local\" (local must win over the merge)", override.Command)
	}
	overrideBefore := cfg.Interaction["override-before"]
	if overrideBefore.Command != "echo local" {
		t.Errorf("override-before.Command = %q, want \"echo local\" (local must win regardless of order)", overrideBefore.Command)
	}
}
