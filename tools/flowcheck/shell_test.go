package main

import (
	"strings"
	"testing"
)

// TestBareWordArg covers the rule that catches a quoting defect am reports only at run
// time. Every "fires" case below was measured blocked against am cb8b4ce, and every
// "silent" case was measured to run; the table is the record of that session, so a case
// should be removed only after re-measuring it, not because it looks redundant.
func TestBareWordArg(t *testing.T) {
	const fires = "bare-word-arg"

	tests := []struct {
		name string
		body string
		want []string
	}{{
		name: "unquoted filename in a test",
		body: "[ -f dva.yml ] && echo yes",
		want: []string{fires},
	}, {
		name: "quoting it is the fix",
		body: "[ -f 'dva.yml' ] && echo yes",
		want: nil,
	}, {
		// The two spellings of the same test block identically, so both are checked.
		name: "test and double bracket block the same way",
		body: "test -f dva.yml\n[[ -f dva.yml ]]",
		want: []string{fires, fires},
	}, {
		name: "printf takes its first argument as a command",
		body: "printf hello",
		want: []string{fires},
	}, {
		// A variable is opaque to the analyzer, so it cannot object to one.
		name: "expansions are not words the analyzer can read",
		body: "[ -f \"$CONFIG\" ] && [ -n $CONFIG ] && printf '%s' \"$PWD\"",
		want: nil,
	}, {
		name: "flags operators and numbers are not command names",
		body: "[ -n \"$A\" ] && [ \"$A\" = \"$B\" ] && [ \"$A\" != \"$B\" ] && [ 1 -eq 1 ]",
		want: nil,
	}, {
		// The reason `printf true || printf false` is the required gate-producer form:
		// `true` and `false` are allowlisted commands. `printf yes` is not, which is the
		// trap this exemption records.
		name: "true and false are allowlisted commands",
		body: "printf true || printf false",
		want: nil,
	}, {
		name: "printf yes is not",
		body: "printf yes",
		want: []string{fires},
	}, {
		// The analyzer reads an argument as a command only for the four trigger
		// commands. Firing on the rest would flag most of the corpus.
		name: "other commands take bare arguments",
		body: "echo dva.yml\nls dva.yml\ngrep name dva.yml\ncp a b\nmkdir -p tmp/x",
		want: nil,
	}, {
		// `$(...)` holds shell the analyzer does read, so the scanner recurses into it.
		// Measured: this exact line is blocked on `dva.yml`.
		name: "command substitution is scanned",
		body: "DVA_FILE=$([ -f dva.yml ] && echo dva.yml || echo dva.yaml)",
		want: []string{fires},
	}, {
		// And the mirror image: an awk program is a quoted argument, not shell. Its
		// `printf` is awk's, and descending into it reports a defect that cannot exist.
		name: "an awk program is not shell",
		body: "awk '{ if (r ~ /x/) { printf \"%s \", r } }' \"$RAW\" > \"$OUT\"",
		want: nil,
	}, {
		// Both closers used to run past the end of the test and read the next token as
		// an argument: `];` from an if-header and `}` from a brace group.
		name: "a test closing an if header ends there",
		body: "if [ -z \"$files\" ]; then echo none; fi",
		want: nil,
	}, {
		name: "a test closing a brace group ends there",
		body: "{ [ -f \"$a\" ] || [ -f \"$b\" ]; } && markers=1",
		want: nil,
	}, {
		// A redirection ends the argument list rather than being read as one.
		name: "redirections are not arguments",
		body: "[ -f \"$f\" ] 2>/dev/null && echo ok",
		want: nil,
	}, {
		// blankComments only blanks whole-line comments, so a trailing one reaches here.
		name: "a trailing comment is not code",
		body: "echo ok # [ -f dva.yml ]",
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

// shellDoc wraps a shell body in the smallest flow that carries it, so a case reads as
// the shell it is about. The step body starts on line 4.
func shellDoc(body string) string {
	var doc strings.Builder
	doc.WriteString("steps:\n  - name: s\n    action: |\n")
	for l := range strings.SplitSeq(body, "\n") {
		doc.WriteString("      ")
		doc.WriteString(l)
		doc.WriteString("\n")
	}
	return doc.String()
}

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
