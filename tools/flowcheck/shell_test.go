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

// TestCommentApostrophe covers the defect underneath TestCommentSubstitution: am does not
// merely read a comment, it lets one change quote state. `#` does not end a string and a
// string is not confined to a line, so an odd number of apostrophes in prose inverts the
// parity of everything after it and the next `'...'` argument opens where the analyzer
// believes one closes. Measured against am cb8b4ce with an otherwise identical step: one
// apostrophe in a comment blocked the multi-line `awk '...'` below it on `command "BEGIN"
// not in allowlist`, and a second apostrophe in the same comment made the step run.
//
// That is why the rule reports every apostrophe rather than an odd count. A field can be
// correct only by parity, and parity is decided by prose in a comment somebody else wrote
// -- adding a word arms a block in code nobody touched.
func TestCommentApostrophe(t *testing.T) {
	const fires = "comment-apostrophe"

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
		name: "rewriting the phrase is the fix",
		body: "# emit the keys of each block\necho ok",
		want: nil,
	}, {
		name: "a trailing comment is a comment too",
		body: "echo ok # the loop's exit status",
		want: []string{fires},
	}, {
		// Quoting in code is what an apostrophe is for. Only comment text is a defect.
		name: "quotes in code are not comments",
		body: "cd 'x' && awk '{ print }' 'y'",
		want: nil,
	}, {
		// The corpus reads .env files this way. The `#` is inside a quoted grep pattern,
		// so no comment starts and the apostrophes around it are ordinary quoting.
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

// TestLocalFunction covers a step calling a function it defines itself. am's allowlist
// knows commands, so the call is blocked with `command "<name>" not in allowlist` while
// `am validate` reports the flow valid. Measured against am cb8b4ce in three shipped
// fields, where it stayed invisible for as long as it did because a comment-substitution
// block in front of it failed first.
func TestLocalFunction(t *testing.T) {
	const fires = "local-function"

	tests := []struct {
		name string
		body string
		want []string
	}{{
		name: "defined here and called here",
		body: "keys() {\n  awk '{ print }' \"$1\"\n}\nkeys 'compose.yaml'",
		want: []string{fires},
	}, {
		// The definition is not what blocks, so a field that only defines one is not
		// reported. Nothing runs the body until something calls it.
		name: "a definition alone is not a call",
		body: "keys() {\n  awk '{ print }' \"$1\"\n}\necho ok",
		want: nil,
	}, {
		// Each call site is its own finding: inlining one leaves the step blocked.
		name: "two call sites are two findings",
		body: "keys() {\n  echo \"$1\"\n}\nkeys a\nkeys b",
		want: []string{fires, fires},
	}, {
		name: "an ordinary command is not a local function",
		body: "awk '{ print }' 'compose.yaml'\ngrep name 'compose.yaml'",
		want: nil,
	}, {
		// The name in argument position is data. am blocks on command position only,
		// and the rule follows it.
		name: "the name as an argument is not a call",
		body: "keys() {\n  echo \"$1\"\n}\necho keys",
		want: nil,
	}, {
		// The shape the corpus actually used, and the reason the tokenizer descends into
		// a `$(...)` inside a double-quoted string: with the string treated as opaque,
		// three shipped fields called a local function invisibly.
		name: "a call inside a double-quoted substitution",
		body: "keys() {\n  echo \"$1\"\n}\necho \"  services: $(keys \"$f\")\"",
		want: []string{fires},
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

// TestSubstitutionInDoubleQuotes locks the two halves of reading a `$(...)` inside a
// double-quoted string. The substitution is shell and its commands are checked, but its
// tokens are not arguments of whatever command the string is an argument to -- the
// distinction the token depth carries. Getting the second half wrong reported `jq` and
// `date` as unquoted arguments of `[` and `printf` in four shipped fields that run.
func TestSubstitutionInDoubleQuotes(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{{
		name: "a substitution is not an argument of the enclosing test",
		body: "if [ \"$(jq -r 'has(\"x\")' \"$REPORT\")\" != \"true\" ]; then exit 1; fi",
		want: nil,
	}, {
		name: "nor of the enclosing printf",
		body: "printf '%s.bak' \"$(date +%Y%m%d-%H%M%S)\"",
		want: nil,
	}, {
		// The other half: a trigger command *inside* the substitution is still checked,
		// because am reads that text as shell.
		name: "but a bare word inside one is still a bare word",
		body: "echo \"config: $([ -f dva.yml ] && echo found)\"",
		want: []string{"bare-word-arg"},
	}, {
		// An argument after the substitution is still an argument. Skipping the nested
		// tokens must not end the argument list.
		name: "the argument list continues past a substitution",
		body: "printf '%s' \"$(date +%s)\" yes",
		want: []string{"bare-word-arg"},
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
