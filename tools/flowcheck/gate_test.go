package main

import "testing"

// TestSkipPropagation covers the two `when:` contract rules that describe what happens
// after a gate closes. Every expectation here was measured against am cb8b4ce with a
// probe flow, not read off am's source: the runs that produced them are recorded in
// tasks/done/187, and each one exited 0 while doing the wrong thing.
//
// The fixtures share a shape: `flag` produces a value, `gated` carries a `when:` on it,
// and the step under test reads `{{gated.k}}`.
func TestSkipPropagation(t *testing.T) {
	const preamble = `steps:
  - id: flag
    type: context
    context:
      v: |
        printf true
  - id: gated
    type: context
    depends_on: [flag]
    when: "{{flag.v}} == 'true'"
    context:
      k: |
        printf 'value'
`
	tests := []struct {
		name string
		doc  string
		want []string
	}{{
		// Measured: this step ran and printed `k=[{{gated.k}}]`.
		name: "ungated dependent reads a skippable key",
		doc: `  - id: consumer
    type: shell
    depends_on: [gated]
    action: |
      printf 'k=%s' '{{gated.k}}'
`,
		want: []string{"gate-skip-leak"},
	}, {
		// Measured: skipped with `dependency 'gated' was skipped`. Its own gate was true
		// at the time — propagation overrides it, which is what makes this safe.
		name: "gated dependent is protected",
		doc: `  - id: consumer
    type: shell
    depends_on: [gated]
    when: "{{flag.v}} == 'true'"
    action: |
      printf 'k=%s' '{{gated.k}}'
`,
		want: nil,
	}, {
		// The trap: a `when:` looks like protection but propagation travels `depends_on`,
		// not gate presence. Measured running, with the literal in hand.
		name: "own gate without a depends_on edge is not protection",
		doc: `  - id: consumer
    type: shell
    depends_on: [flag]
    when: "{{flag.v}} == 'true'"
    action: |
      printf 'k=%s' '{{gated.k}}'
`,
		want: []string{"gate-skip-leak"},
	}, {
		// Measured: `chain2` skipped through `mid`. Propagation is transitive as long as
		// every link carries a gate — the shape dva-improve.yaml itself relies on.
		name: "all-gated chain is protected",
		doc: `  - id: mid
    type: shell
    depends_on: [gated]
    when: "{{flag.v}} == 'true'"
    action: |
      printf 'mid'
  - id: consumer
    type: shell
    depends_on: [mid]
    when: "{{flag.v}} == 'true'"
    action: |
      printf 'k=%s' '{{gated.k}}'
`,
		want: nil,
	}, {
		// One ungated link is enough: `mid` runs, so everything below it runs.
		name: "ungated link breaks the chain",
		doc: `  - id: mid
    type: shell
    depends_on: [gated]
    action: |
      printf 'mid'
  - id: consumer
    type: shell
    depends_on: [mid]
    when: "{{flag.v}} == 'true'"
    action: |
      printf 'k=%s' '{{gated.k}}'
`,
		want: []string{"gate-skip-leak"},
	}, {
		name: "llm instruction reads a skippable key",
		doc: `  - id: consumer
    type: llm
    depends_on: [gated]
    agent: claude
    instruction: |
      Fix the errors below.
      {{gated.k}}
`,
		want: []string{"gate-skip-prompt"},
	}, {
		name: "gated llm instruction is protected",
		doc: `  - id: consumer
    type: llm
    depends_on: [gated]
    when: "{{flag.v}} == 'true'"
    agent: claude
    instruction: |
      Fix the errors below.
      {{gated.k}}
`,
		want: nil,
	}, {
		// A literal that reaches disk outlives the run, so it is reported as a prompt
		// leak rather than an internal one.
		name: "file body reads a skippable key",
		doc: `  - id: consumer
    type: file
    depends_on: [gated]
    file:
      op: write
      path: "tmp/out.txt"
      content: "{{gated.k}}"
`,
		want: []string{"gate-skip-prompt"},
	}, {
		name: "reference to an ungated producer is silent",
		doc: `  - id: consumer
    type: shell
    depends_on: [flag]
    action: |
      printf 'v=%s' '{{flag.v}}'
`,
		want: nil,
	}, {
		// `param.*` and steps defined in another flow file cannot be resolved here, and
		// guessing at them would report the corpus's cross-file pipelines as defects.
		name: "unresolvable reference is silent",
		doc: `  - id: consumer
    type: shell
    depends_on: [flag]
    action: |
      printf 't=%s' '{{param.target}}'
`,
		want: nil,
	}, {
		// The same key read twice in one field is one defect with one fix.
		name: "repeated read of one key reports once",
		doc: `  - id: consumer
    type: shell
    depends_on: [gated]
    action: |
      printf '%s %s' '{{gated.k}}' '{{gated.k}}'
`,
		want: []string{"gate-skip-leak"},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rules(find(t, preamble+tt.doc))
			if len(got) != len(tt.want) {
				t.Fatalf("rules = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("rules = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
