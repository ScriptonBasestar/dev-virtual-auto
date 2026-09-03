package agentdeny

import "testing"

func TestMatchDenyPatternBasics(t *testing.T) {
	cases := []struct {
		pattern string
		argv    string
		want    bool
	}{
		{"Bash(dva config env show *)", "dva config env show", true},        // bare form, no trailing args
		{"Bash(dva config env show *)", "dva config env show --json", true}, // trailing flag
		{"Bash(dva config env show *)", "dva config env show .env", true},   // trailing argument
		{"Bash(dva config env show *)", "dva config env showx", false},      // no space boundary: must not over-match
		{"Bash(dva config env show *)", "dva config env unshow", false},
		{"Bash(ls *)", "ls", true},
		{"Bash(ls *)", "ls -la", true},
		{"Bash(ls *)", "lsof", false},                           // Claude Code's own documented reason for requiring the space
		{"dva config env show *", "dva config env show", false}, // missing Bash(...) wrapper: names no tool, matches nothing
	}
	for _, tc := range cases {
		if got := matchDenyPattern(tc.pattern, tc.argv); got != tc.want {
			t.Errorf("matchDenyPattern(%q, %q) = %v, want %v", tc.pattern, tc.argv, got, tc.want)
		}
	}
}

// anyPatternMatches reports whether any of a gated command's deny patterns matches argv
// — i.e. whether at least one deny rule DVA generates would block this literal command.
func anyPatternMatches(patterns []string, argv string) bool {
	for _, pattern := range patterns {
		if matchDenyPattern(pattern, argv) {
			return true
		}
	}
	return false
}

// TestGatedCommandPatternsCoverArgvVariants is the criterion-3 binding: the deny pattern
// Patterns() produces for each gated command must match the argv shapes TASK-286 named
// (the bare command and trailing arguments/flags) via its wrapped-and-spaced form, and
// must not match a neighboring, non-gated command that merely shares a text prefix.
func TestGatedCommandPatternsCoverArgvVariants(t *testing.T) {
	seal, ok := ByID("config-env-seal")
	if !ok {
		t.Fatal("config-env-seal not found in GatedCommands")
	}
	show, ok := ByID("config-env-show")
	if !ok {
		t.Fatal("config-env-show not found in GatedCommands")
	}

	t.Run("seal", func(t *testing.T) {
		patterns := seal.Patterns()
		if len(patterns) != 1 {
			t.Fatalf("expected exactly 1 wrapped-and-spaced pattern for %q, got %d: %v", seal.Argv, len(patterns), patterns)
		}
		if patterns[0] != "Bash(dva config env seal *)" {
			t.Fatalf("expected the Bash(...)-wrapped, space-before-* form, got %q", patterns[0])
		}
		mustMatch := []string{
			"dva config env seal",
			"dva config env seal .env",
			"dva config env seal --yes",
		}
		for _, argv := range mustMatch {
			if !anyPatternMatches(patterns, argv) {
				t.Errorf("seal patterns %v must match argv variant %q", patterns, argv)
			}
		}
		mustNotMatch := []string{
			"dva config env edit",
			"dva config env unseal",
			"dva config env show",
			"dva config env sealed", // no space boundary after "seal" — must not over-match a hypothetical neighbor
		}
		for _, argv := range mustNotMatch {
			if anyPatternMatches(patterns, argv) {
				t.Errorf("seal patterns %v must NOT match unrelated command %q", patterns, argv)
			}
		}
	})

	t.Run("show", func(t *testing.T) {
		patterns := show.Patterns()
		if len(patterns) != 1 {
			t.Fatalf("expected exactly 1 wrapped-and-spaced pattern for %q, got %d: %v", show.Argv, len(patterns), patterns)
		}
		if patterns[0] != "Bash(dva config env show *)" {
			t.Fatalf("expected the Bash(...)-wrapped, space-before-* form, got %q", patterns[0])
		}
		mustMatch := []string{
			"dva config env show",
			"dva config env show --json",
		}
		for _, argv := range mustMatch {
			if !anyPatternMatches(patterns, argv) {
				t.Errorf("show patterns %v must match argv variant %q", patterns, argv)
			}
		}
		mustNotMatch := []string{
			"dva config env edit",
			"dva config env unseal",
			"dva config env seal",
			"dva config env showall", // no space boundary after "show" — must not over-match a hypothetical neighbor
		}
		for _, argv := range mustNotMatch {
			if anyPatternMatches(patterns, argv) {
				t.Errorf("show patterns %v must NOT match unrelated command %q", patterns, argv)
			}
		}
	})
}

// TestVerifierDoesNotModelClaudeCodeWrapperStripping documents a limitation of this
// package's own hand-rolled matchDenyPattern verifier, NOT a claim about Claude Code's
// real behavior. Per Claude Code's own permission docs, the real runtime strips a
// leading environment-variable assignment and the timeout/nice/command/xargs wrapper
// commands before matching, so "FOO=bar dva config env show" IS blocked by the real
// runtime even though matchDenyPattern — a literal-prefix comparator only — cannot prove
// that. An earlier version of this test asserted the opposite (that this was an
// uncovered gap), which was a factually wrong claim in a card whose purpose is honest
// accounting; see TestKnownUncoveredInvocations below for the argv shapes that are
// genuinely uncovered per Claude Code's own documentation.
func TestVerifierDoesNotModelClaudeCodeWrapperStripping(t *testing.T) {
	show, _ := ByID("config-env-show")
	patterns := show.Patterns()
	notModeledByThisVerifier := []string{
		"FOO=bar dva config env show",
		"timeout 5 dva config env show",
	}
	for _, argv := range notModeledByThisVerifier {
		if anyPatternMatches(patterns, argv) {
			t.Errorf("patterns %v unexpectedly matched %q under this package's simplified verifier; Claude Code's real matcher would still block it either way", patterns, argv)
		}
	}
}

// TestKnownUncoveredInvocations asserts the argv shapes that are genuinely NOT covered
// by a dva-prefixed deny pattern, per Claude Code's own documentation: a path-qualified
// invocation, and the environment-runner wrappers Claude Code's stripped-wrapper list
// explicitly excludes. See docs/agent-deny-rules.md "Honest limits".
func TestKnownUncoveredInvocations(t *testing.T) {
	show, _ := ByID("config-env-show")
	patterns := show.Patterns()
	uncovered := []string{
		"./bin/dva config env show", // this repo's own `make build` produces bin/dva
		"mise exec -- dva config env show",
		"devbox run dva config env show",
		"direnv exec . dva config env show",
		"docker exec mycontainer dva config env show",
		"bash -c 'dva config env show'",
	}
	for _, argv := range uncovered {
		if anyPatternMatches(patterns, argv) {
			t.Errorf("patterns %v unexpectedly matched %q; if Claude Code now covers this, update docs/agent-deny-rules.md's Honest limits section too", patterns, argv)
		}
	}
}
