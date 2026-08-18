package main

import (
	"strings"
	"testing"
)

// TestCommentSubstitution covers a defect /bin/sh does not have. am's policy analyzer
// reads a backtick span or a $(...) inside a `#` comment as a command substitution and
// blocks the step on its first word, so prose explaining the code -- the one thing a
// comment is for -- is what stops the step from running. Measured against am cb8b4ce:
// a comment naming backend in backticks blocked; the same sentence without them ran.
//
// The blocked step does not stop the run. am prompts, defaults to continue, every reader
// of the step's keys gets the literal `{{step.key}}` text, and the run reports success --
// which is how this survived in six shipped steps until the rule was written.
func TestCommentSubstitution(t *testing.T) {
	const fires = "comment-substitution"

	tests := []struct {
		name string
		body string
		want []string
	}{{
		name: "backticks in a comment",
		body: "# a network named `backend` reads as a service\necho ok",
		want: []string{fires},
	}, {
		name: "saying it without them is the fix",
		body: "# a network named backend reads as a service\necho ok",
		want: nil,
	}, {
		name: "dollar-paren in a comment",
		body: "# measured against $(git rev-parse HEAD)\necho ok",
		want: []string{fires},
	}, {
		// Every span is reported, not the first one on the line: the fix is to rewrite
		// the sentence, and a partial rewrite leaves the step blocked just the same.
		name: "two spans on one line are two findings",
		body: "# `jq -e .` accepts a stream, and `jq -r` prints from it\necho ok",
		want: []string{fires, fires},
	}, {
		name: "a trailing comment is a comment too",
		body: "echo ok # see `backend`",
		want: []string{fires},
	}, {
		// A `#` inside quotes opens no comment, so nothing here is comment text. A
		// backtick in that position is code, which the command-position rules read.
		name: "a hash inside quotes is not a comment",
		body: "grep '#tag `x`' \"$f\"",
		want: nil,
	}, {
		// The rule is about substitutions the shell will never run. One in code is
		// ordinary shell and not this rule's business.
		name: "a substitution outside a comment is not this rule",
		body: "V=$(printf true)",
		want: nil,
	}, {
		name: "an empty comment is not a span",
		body: "#\necho ok",
		want: nil,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rules(find(t, shellDoc(tt.body)))
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("rules = %v, want %v\nbody:\n%s", got, tt.want, tt.body)
			}
		})
	}

	// The message names a word; without the line the reader has to search the file for
	// it, and the same word usually appears in the code the comment describes.
	t.Run("the finding points at the comment line", func(t *testing.T) {
		fs := find(t, shellDoc("echo one\necho two\n# names `backend` here"))
		if len(fs) != 1 {
			t.Fatalf("findings = %d, want 1", len(fs))
		}
		if fs[0].line != 6 {
			t.Errorf("line = %d, want 6", fs[0].line)
		}
	})
}

// TestCommentQuote covers the defect underneath TestCommentSubstitution: am does not
// merely read a comment, it lets one change quote state. `#` does not end a string and a
// string is not confined to a line, so an odd number of quotes in prose inverts the parity
// of everything after it and the next quoted argument opens where the analyzer believes
// one closes. Measured against am cb8b4ce on otherwise identical steps: one apostrophe in
// a comment blocked the multi-line `awk '...'` below it on `command "BEGIN" not in
// allowlist`, and a second apostrophe in the same comment made the step run. The same pair
// of measurements holds for a double quote and an `awk "..."`.
//
// That is why the rule reports every quote rather than an odd count. A field can be
// correct only by parity, and parity is decided by prose in a comment somebody else wrote
// -- adding a word arms a block in code nobody touched.
func TestCommentQuote(t *testing.T) {
	const fires = "comment-quote"

	tests := []struct {
		name string
		body string
		want []string
	}{{
		name: "one apostrophe in a comment",
		body: "# emit each block's keys\necho ok",
		want: []string{fires},
	}, {
		// Even parity is not a defence: the next edit to either comment breaks it, and
		// the break lands on a line neither edit touched.
		name: "an even count is reported just the same",
		body: "# am's allowlist knows the step's commands\necho ok",
		want: []string{fires, fires},
	}, {
		// am tracks the two characters separately, so this pair cannot cancel out -- but
		// each one is still reported.
		name: "a double quote counts too",
		body: "# blocked on \"DVA\" here\necho ok",
		want: []string{fires, fires},
	}, {
		name: "rewriting the phrase is the fix",
		body: "# emit the keys of each block\necho ok",
		want: nil,
	}, {
		name: "a trailing comment is a comment too",
		body: "echo ok # the loop's exit status",
		want: []string{fires},
	}, {
		// Quoting in code is what a quote is for. Only comment text is a defect.
		name: "quotes in code are not comments",
		body: "cd 'x' && awk '{ print }' \"$f\"",
		want: nil,
	}, {
		// The corpus reads .env files this way. The `#` is inside a quoted grep pattern,
		// so no comment starts and the quotes around it are ordinary quoting.
		name: "a hash inside quotes opens no comment",
		body: "grep -v '^#' .env | sed 's/^export //'",
		want: nil,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rules(find(t, shellDoc(tt.body)))
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("rules = %v, want %v\nbody:\n%s", got, tt.want, tt.body)
			}
		})
	}
}
