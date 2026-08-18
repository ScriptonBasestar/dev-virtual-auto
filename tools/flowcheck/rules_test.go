package main

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// reserved mirrors the built-in set closely enough for the rule under test. The command
// runs against the live config.ReservedCommands(); see TestReservedSetIsLive.
var reserved = map[string]bool{"config": true, "up": true, "ls": true, "run": true, "doctor": true}

// find runs the checker over a flow fragment and returns the rules that fired, in order.
func find(t *testing.T, doc string) []finding {
	t.Helper()
	s, err := checkBytes([]byte(doc), reserved)
	if err != nil {
		t.Fatalf("checkBytes: %v", err)
	}
	return s.findings
}

func rules(fs []finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.rule)
	}
	return out
}

func TestRules(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want []string
	}{{
		// The gate that was unreachable for every possible input: jq's `//` substitutes
		// for false as well as null, so `dva_needed: false` read back as true.
		name: "jq boolean default is a dead gate",
		doc:  "steps:\n  - name: gate\n    action: |\n      NEED=$(jq -r '.dva_needed // true' r.json)\n",
		want: []string{"dead-gate"},
	}, {
		// The fix for a defect explains that defect in a comment right above the code.
		// A scanner that reads comments reports the explanation as the bug.
		// It is written without backticks for a separate reason -- see TestCommentSubstitution.
		name: "the same text in a comment is not a defect",
		doc:  "steps:\n  - name: gate\n    action: |\n      # .dva_needed // true cannot express this gate.\n      jq -e 'has(\"dva_needed\")' r.json\n",
		want: nil,
	}, {
		name: "exit_if_empty is prohibited",
		doc:  "steps:\n  - name: probe\n    exit_if_empty: dva_available\n    action: echo hi\n",
		want: []string{"exit-if-empty"},
	}, {
		// `dva app` was removed in docs/43; its error text rendered into the summary
		// report as the false sentence "no applications configured".
		name: "phantom dva command in command position",
		doc:  "steps:\n  - name: s\n    action: |\n      cd x && dva app ls -f json\n",
		want: []string{"phantom-command"},
	}, {
		name: "dva in prose is not an invocation",
		doc:  "steps:\n  - name: s\n    action: |\n      echo \"ERROR: dva is not on PATH\" >&2\n      echo \"Run 'dva init' first.\"\n",
		want: nil,
	}, {
		name: "namespaced subproject command is not a phantom",
		doc:  "steps:\n  - name: s\n    action: |\n      dva compose:ps\n",
		want: nil,
	}, {
		name: "built-in commands pass",
		doc:  "steps:\n  - name: s\n    action: |\n      cd x && dva config validate\n      $(dva ls -f json)\n",
		want: nil,
	}, {
		// `jq -e .` accepts a stream, so `[1][2]{...}` validates and a later `jq -r`
		// reads a plausible value out of the trailing object.
		name: "report read without the single-object guard",
		doc:  "steps:\n  - name: s\n    action: |\n      jq -e . tmp/x/report.json >/dev/null || exit 1\n      jq -r '.setup_track' tmp/x/report.json\n",
		want: []string{"unguarded-report"},
	}, {
		name: "guarded report read passes",
		doc:  "steps:\n  - name: s\n    action: |\n      jq -e -s 'length == 1' tmp/x/report.json >/dev/null || exit 1\n      jq -r '.setup_track' tmp/x/report.json\n",
		want: nil,
	}, {
		name: "jq on command output is not a report read",
		doc:  "steps:\n  - name: s\n    action: |\n      dva config show -f json | jq -r '.name'\n",
		want: nil,
	}, {
		// YAML resolves a bare true to a boolean, so this type-failed flow.schema.json
		// while reading as perfectly reasonable.
		// This corpus writes parameters as a sequence of maps. An earlier version of
		// this test invented a mapping-keyed shape, passed against it, and missed the
		// live defect entirely.
		name: "unquoted booleans in a parameter enum (sequence shape)",
		doc:  "parameters:\n  - name: skip_execute\n    default: \"false\"\n    enum: [true, false]\n",
		want: []string{"param-type", "param-type"},
	}, {
		name: "unquoted booleans in a parameter enum (mapping shape)",
		doc:  "parameters:\n  interactive:\n    enum: [true, false]\n",
		want: []string{"param-type", "param-type"},
	}, {
		name: "quoted enum values pass",
		doc:  "parameters:\n  - name: skip_execute\n    default: \"false\"\n    enum: [\"true\", \"false\"]\n",
		want: nil,
	}, {
		name: "non-string parameter default",
		doc:  "parameters:\n  - name: depth\n    default: 3\n",
		want: []string{"param-type"},
	}, {
		name: "context values are shell too",
		doc:  "steps:\n  - name: s\n    context:\n      track: \"dva app ls\"\n",
		want: []string{"phantom-command"},
	}, {
		// Prompt bodies are prose about dva. Scanning them is what made an earlier hand
		// audit report `dva repo` and `dva commands` as real commands.
		name: "prompt bodies are not scanned",
		doc:  "steps:\n  - name: s\n    prompt: |\n      Run `dva app ls` and `dva repo status`.\n",
		want: nil,
	}, {
		// Measured: `{{ref | trim}} == 'true'` and `{{ref | trim}} == 'false'` both ran.
		name: "a filter defeats the gate",
		doc:  "steps:\n  - name: s\n    when: \"{{c.k | trim}} == 'true'\"\n    action: echo hi\n",
		want: []string{"gate-filter"},
	}, {
		// Measured: the template renders to a string, the bare operand parses as a
		// boolean, and the two are never equal.
		name: "an unquoted boolean operand never compares equal",
		doc:  "steps:\n  - name: s\n    when: \"{{c.k}} != true\"\n    action: echo hi\n",
		want: []string{"gate-operand"},
	}, {
		name: "both halves of the defect on one gate",
		doc:  "steps:\n  - name: s\n    when: \"{{c.k | trim}} != true\"\n    action: echo hi\n",
		want: []string{"gate-filter", "gate-operand"},
	}, {
		name: "a quoted operand with no filter is the correct form",
		doc:  "steps:\n  - name: s\n    when: \"{{c.k}} != 'true'\"\n    action: echo hi\n",
		want: nil,
	}, {
		// A literal comparison carries no template, and the evaluator handles it. Flagging
		// it would report working code, which is how a checker stops being believed.
		name: "a literal comparison is not a template gate",
		doc:  "steps:\n  - name: s\n    when: \"true == true\"\n    action: echo hi\n",
		want: nil,
	}, {
		// Only `true`/`false` are coerced. `{{param.a}} == same` was measured to gate
		// correctly unquoted, so the rule stops where the evidence stops.
		name: "an unquoted non-boolean operand is not the defect",
		doc:  "steps:\n  - name: s\n    when: \"{{c.k}} == ready\"\n    action: echo hi\n",
		want: nil,
	}, {
		// Measured: am does not trim, so `echo true` renders as "true\n" and the quoted
		// operand it is compared against can never match. A correctly written gate is
		// still inert if its producer appends a newline.
		name: "an echo producer defeats a correctly written gate",
		doc: `steps:
  - id: c
    type: context
    context:
      k: |
        test -f 'x' && echo "true" || echo "false"
  - id: s
    when: "{{c.k}} != 'true'"
    action: echo hi
`,
		want: []string{"gate-producer-newline"},
	}, {
		name: "a printf producer is the correct form",
		doc: `steps:
  - id: c
    type: context
    context:
      k: |
        test -f 'x' && printf true || printf false
  - id: s
    when: "{{c.k}} != 'true'"
    action: echo hi
`,
		want: nil,
	}, {
		// The newline only matters where something compares the value. A prompt that
		// interpolates it does not care, and flagging that would report working code.
		name: "an echo producer no gate reads is not a defect",
		doc: `steps:
  - id: c
    type: context
    context:
      k: |
        echo "true"
  - id: s
    action: echo hi
`,
		want: nil,
	}, {
		// `validate_pass1.is_valid` feeds three gates in the real corpus. The defect is
		// in the producer, and one fix closes all of them.
		name: "one producer feeding two gates is reported once",
		doc: `steps:
  - id: c
    type: context
    context:
      k: |
        echo "true"
  - id: s1
    when: "{{c.k}} != 'true'"
    action: echo hi
  - id: s2
    when: "{{c.k}} == 'true'"
    action: echo bye
`,
		want: []string{"gate-producer-newline"},
	}, {
		// `param.*` is supplied by the runner, not by a shell in this file. Measured:
		// a param compares correctly in both polarities, so there is nothing to report.
		name: "a param reference has no producer to check",
		doc: `steps:
  - id: s
    when: "{{param.flag}} != 'true'"
    action: echo hi
`,
		want: nil,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := rules(find(t, tc.doc))
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("rules fired = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestFindingLine pins the offset arithmetic across scalar styles. An earlier version
// covered only the block scalar and so passed while every finding on a single-line
// `context:` entry — the common shape in this corpus — was reported one line late.
func TestFindingLine(t *testing.T) {
	t.Run("flow scalar under context", func(t *testing.T) {
		doc := "steps:\n" + // 1
			"  - name: s\n" + // 2
			"    context:\n" + // 3
			"      app_ls: \"dva app ls\"\n" // 4
		fs := find(t, doc)
		if len(fs) != 1 {
			t.Fatalf("findings = %d, want 1", len(fs))
		}
		if fs[0].line != 4 {
			t.Errorf("line = %d, want 4", fs[0].line)
		}
	})
	t.Run("plain scalar action", func(t *testing.T) {
		doc := "steps:\n" + // 1
			"  - name: s\n" + // 2
			"    action: dva app ls\n" // 3
		fs := find(t, doc)
		if len(fs) != 1 {
			t.Fatalf("findings = %d, want 1", len(fs))
		}
		if fs[0].line != 3 {
			t.Errorf("line = %d, want 3", fs[0].line)
		}
	})
	t.Run("block scalar action", func(t *testing.T) {
		doc := "steps:\n" + // 1
			"  - name: gate\n" + // 2
			"    action: |\n" + // 3
			"      echo one\n" + // 4
			"      echo two\n" + // 5
			"      cd x && dva app ls\n" // 6
		fs := find(t, doc)
		if len(fs) != 1 {
			t.Fatalf("findings = %d, want 1", len(fs))
		}
		if fs[0].line != 6 {
			t.Errorf("line = %d, want 6", fs[0].line)
		}
	})
}

// TestCountsAreReported guards the summary line. A rule that silently matches nothing
// reads exactly like a rule that passed.
func TestCountsAreReported(t *testing.T) {
	s, err := checkBytes([]byte("steps:\n  - name: s\n    action: |\n      dva config validate\n      jq -r .x tmp/a.json\n"), reserved)
	if err != nil {
		t.Fatal(err)
	}
	if s.dvaCalls != 1 {
		t.Errorf("dvaCalls = %d, want 1", s.dvaCalls)
	}
	if s.reportFields != 1 {
		t.Errorf("reportFields = %d, want 1", s.reportFields)
	}
	if len(s.shells) != 1 {
		t.Errorf("shells = %d, want 1", len(s.shells))
	}
}

// TestGateCountIsReported guards the when-gate rules' premise the same way
// TestReservedSetIsLive guards phantom-command: if `when:` scalars are never collected,
// both rules match nothing and the corpus reads as clean.
func TestGateCountIsReported(t *testing.T) {
	s, err := checkBytes([]byte("steps:\n  - name: a\n    when: \"{{c.k}} == 'true'\"\n    action: echo hi\n  - name: b\n    action: echo hi\n"), reserved)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.gates) != 1 {
		t.Errorf("gates = %d, want 1", len(s.gates))
	}
	if len(s.findings) != 0 {
		t.Errorf("findings = %v, want none", rules(s.findings))
	}
}

// TestEveryDocumentIsScanned pins the multi-document decode. yaml.Unmarshal stops after
// the first document, so a defect behind a `---` used to be invisible while the file
// still reported as inspected.
func TestEveryDocumentIsScanned(t *testing.T) {
	doc := "steps:\n" + // 1
		"  - name: first\n" + // 2
		"    action: echo hi\n" + // 3
		"---\n" + // 4
		"steps:\n" + // 5
		"  - name: second\n" + // 6
		"    action: dva app ls\n" // 7
	got := find(t, doc)
	if len(got) != 1 || got[0].rule != "phantom-command" {
		t.Fatalf("rules fired = %v, want one phantom-command", rules(got))
	}
	if got[0].line != 7 {
		t.Errorf("line = %d, want 7", got[0].line)
	}
}

// TestReservedSetIsLive guards the phantom-command rule's premise. The rule can only
// fire against a populated built-in set, so an empty or missing one disables it
// silently — every command would look reserved-adjacent and nothing would be reported.
func TestReservedSetIsLive(t *testing.T) {
	live := config.ReservedCommands()
	if len(live) < 10 {
		t.Fatalf("config.ReservedCommands() returned %d commands; the phantom-command rule needs the real set", len(live))
	}
	for _, want := range []string{"config", "up", "run", "validate"} {
		if !live[want] {
			t.Errorf("built-in %q missing from config.ReservedCommands()", want)
		}
	}
	if live["app"] {
		t.Error("`app` is back in the built-in set; docs/43 removed it")
	}
}
