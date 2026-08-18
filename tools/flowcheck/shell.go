package main

import (
	"regexp"
	"sort"
	"strings"
)

var (
	// A jq default whose fallback is a boolean literal is a decision that cannot fail
	// closed. `//` substitutes for `false` as well as `null`, so `.x // true` reads an
	// explicit `false` back as `true` and the stop path becomes unreachable.
	reBoolDefault = regexp.MustCompile(`//\s*(true|false)\b`)

	// A dva invocation, as opposed to the word "dva" in an error message: the command
	// must sit in command position — start of line, or after && || | ; ( $( !.
	reDvaCall = regexp.MustCompile(`(?:^|[\n&|;()!])[ \t]*dva[ \t]+([a-z][a-z0-9:_-]*)`)

	reJq        = regexp.MustCompile(`\bjq\b`)
	reTmpPath   = regexp.MustCompile(`\btmp/`)
	reJSONGuard = regexp.MustCompile(`jq\s+-e\s+-s\b`)
)

// checkShell runs the rules that read shell text. reserved is the live built-in command
// set, imported from internal/config so the list is never kept in two places.
func checkShell(f shellField, reserved map[string]bool, s *scan) {
	// Read before blanking: comments are invisible to every rule below, and am extracts
	// substitutions from them anyway.
	for _, m := range commentSubstitutions(f.node.Value) {
		s.add("comment-substitution", lineOf(f, f.node.Value, m.offset),
			"%s: %s sits inside a shell comment. am drops the comment's plain words but still "+
				"extracts the substitution and blocks the step on the first command it does not "+
				"allow — /bin/sh would never run any of it. Whether a given span is extracted "+
				"depends on the apostrophe parity of the whole field, because am's quote "+
				"tracking crosses lines and `#` does not end a quote: writing \"don't\" in an "+
				"earlier comment hides this span, and deleting that word arms it. A blocked "+
				"step fails a batch run and, interactively, prompts and defaults to continue — "+
				"after which every reader of this step's keys gets the literal `{{step.key}}` "+
				"text and the run still ends Done. Say it without the backticks.",
			f.name, m.text)
	}

	for _, off := range commentApostrophes(f.node.Value) {
		s.add("comment-apostrophe", lineOf(f, f.node.Value, off),
			"%s: an apostrophe in comment prose flips the quote parity of everything after "+
				"it. am carries quote state across a `#` and across lines, so an odd count "+
				"makes the next `'...'` argument open where the analyzer thinks one closes, "+
				"exposing its contents as shell: a multi-line `awk '...'` program three lines "+
				"below was blocked on `command \"BEGIN\" not in allowlist` by the word "+
				"\"block's\" alone. Parity is not a property anyone can maintain -- one word "+
				"arms a block in code nobody touched -- so no comment may carry one at all. "+
				"Rewrite the phrase without it.", f.name)
	}

	text := blankComments(f.node.Value)

	if m := reBoolDefault.FindStringSubmatchIndex(text); m != nil {
		s.add("dead-gate", lineOf(f, text, m[0]),
			"%s: jq default `// %s` cannot fail closed — `//` substitutes for `false` as well "+
				"as `null`, so an explicit `false` reads back as `%s`. Use `has(\"key\")` to "+
				"separate absent from present-and-false.", f.name, text[m[2]:m[3]], text[m[2]:m[3]])
	}

	for _, m := range reDvaCall.FindAllStringSubmatchIndex(text, -1) {
		cmd := text[m[2]:m[3]]
		s.dvaCalls++
		// A namespaced `alias:cmd` is a subproject command, resolved at runtime and not
		// part of the built-in set.
		if strings.Contains(cmd, ":") || reserved[cmd] {
			continue
		}
		s.add("phantom-command", lineOf(f, text, m[0]),
			"%s: `dva %s` is not a built-in command. Its error text renders into reports as "+
				"though it were a finding.", f.name, cmd)
	}

	for _, b := range bareWordArgs(text) {
		s.add("bare-word-arg", lineOf(f, text, b.offset),
			"%s: `%s` is unquoted where am's shell policy analyzer reads an argument as a "+
				"command name, so the step is blocked at run time with `command \"%s\" not in "+
				"allowlist` while `am validate` reports the flow valid. The run names the word "+
				"but not the line or the `context:` key, so a multi-key step has to be bisected "+
				"one key at a time. Quote it: `'%s'`.", f.name, b.word, b.word, b.word)
	}

	for _, c := range localFunctionCalls(text) {
		s.add("local-function", lineOf(f, text, c.offset),
			"%s: `%s` is a function this field defines, and am's allowlist knows commands, not "+
				"functions. The call is blocked at run time with `command \"%s\" not in "+
				"allowlist` while `am validate` reports the flow valid, and a blocked step does "+
				"not stop the run: it prompts, defaults to continue, and produces nothing while "+
				"every reader of its keys gets the literal `{{step.key}}` text. Inline the body "+
				"at each call site.", f.name, c.word, c.word)
	}

	if reJq.MatchString(text) && reTmpPath.MatchString(text) {
		s.reportFields++
		if !reJSONGuard.MatchString(text) {
			s.add("unguarded-report", lineOf(f, text, reJq.FindStringIndex(text)[0]),
				"%s: reads a tmp/ JSON artifact with jq but never checks it holds exactly one "+
					"object. `jq -e .` accepts a *stream*: for `[1][2]{...}` it exits 0, and a "+
					"later `jq -r` prints a plausible value from the trailing object while the "+
					"array errors go to stderr. Guard with "+
					"`jq -e -s 'length == 1 and (.[0] | type) == \"object\"'`.", f.name)
		}
	}
}

// bareWordTriggers are the commands whose unquoted arguments am's shell policy analyzer
// reads as command names, blocking the step at run time with `command "<word>" not in
// allowlist` while `am validate` reports the flow valid.
//
// This is not "any bare word anywhere". Measured against am cb8b4ce: `echo hello`, `ls
// hello`, `grep hello file`, `cp a b` and `mkdir -p tmp/x` all run, while `printf hello`,
// `test -f dva.yml`, `[ -f dva.yml ]` and `[[ -f dva.yml ]]` are each blocked on their
// first unquoted argument.
//
// `eval` and `exec` block the same way and are deliberately absent. There the first
// argument really is a command name, so the allowlist is doing its intended job rather
// than misreading data -- and to check the rest of an `eval` line correctly this rule
// would need am's allowlist, which it does not have. The four below are the ones that
// surprise people, because the word is plainly a filename.
var bareWordTriggers = map[string]bool{
	"printf": true,
	"test":   true,
	"[":      true,
	"[[":     true,
}

// bareWordArgs returns the unquoted arguments in text that those commands will have read
// as command names, with the byte offset of each.
//
// The exemptions are the whole design. Each is a word the analyzer was measured to
// accept:
//
//	quoted        'dva.yml'  "dva.yml"   -- quoting is the fix, so a quoted word is done
//	expanded      $CONFIG    "$CONFIG"   -- the analyzer does not resolve expansions
//	flags         -f -n -d -eq -p        -- and the `--` separator
//	test operators = != and friends
//	numbers       1 2                    -- `[ 1 -eq 1 ]` runs
//	true / false                         -- allowlisted *commands*, which is the trap
//	                                        below: `printf true` runs for a reason that
//	                                        has nothing to do with printf
//
// `true`/`false` are exempt because they run, not because they are safe to imitate. The
// required gate-producer form `printf true || printf false` is legal only by that
// coincidence, and the next flag written as `printf yes` blocks -- which reads as a gate
// defect rather than a quoting one. The rule cannot warn about the coincidence without
// firing on every correct gate in the corpus, so this comment carries it instead.
func bareWordArgs(text string) []bareWord {
	var out []bareWord
	toks := shellTokens(text, 0, 0)
	for i := range toks {
		if !toks[i].cmdPos || toks[i].quoted || !bareWordTriggers[toks[i].word] {
			continue
		}
		for j := i + 1; j < len(toks); j++ {
			// A substitution inside an argument is its own command line, and its tokens
			// are appended after the argument that carried it. They are not arguments of
			// this command, and the argument list continues past them.
			if toks[j].depth != toks[i].depth {
				continue
			}
			if toks[j].op || toks[j].word == "]" || toks[j].word == "]]" {
				break
			}
			if !bareWordExempt(toks[j]) {
				out = append(out, toks[j])
			}
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].offset < out[b].offset })
	return out
}

var reBareWordNumber = regexp.MustCompile(`^[0-9]+$`)

// bareWordExempt reports whether w is a word the analyzer was measured to accept.
func bareWordExempt(w bareWord) bool {
	if w.word == "" || w.quoted || strings.HasPrefix(w.word, "-") {
		return true
	}
	switch w.word {
	case "true", "false", "=", "!=", "!", "]", "]]":
		return true
	}
	return reBareWordNumber.MatchString(w.word)
}

// bareWord is one token of shell source: its text, its byte offset in the enclosing
// field, and the three facts the rule above needs about it.
type bareWord struct {
	word   string
	offset int
	// quoted means some part of the token was quoted, escaped, or an expansion, so the
	// analyzer sees a value this scanner cannot predict. Staying quiet is then the only
	// sound choice.
	quoted bool
	// op means the token is an unquoted control operator, which ends the command.
	op bool
	// cmdPos means the token stands where a command name goes.
	cmdPos bool
	// depth is how many substitutions deep the token sits. Tokens of a `$(...)` are
	// appended after the word that carried it, so depth is what tells an argument of
	// this command from a token of a command line nested inside one.
	depth int
}

// shellReserved are the words after which a command name may follow, so that `if [ -f x
// ]` and `a && [ -f x ]` are both recognised as putting `[` in command position.
var shellReserved = map[string]bool{
	"if": true, "then": true, "else": true, "elif": true,
	"do": true, "while": true, "until": true, "!": true,
}

// shellTokens splits text into shell tokens, adding base to every reported offset.
//
// It is deliberately not a shell parser. It knows only enough to answer "is this word
// quoted, and is it where a command name goes" -- which is exactly the question am's
// analyzer asks. Two asymmetries in it are load-bearing and easy to get backwards:
//
//   - `'...'` is opaque. An awk or sed program is a quoted argument, and the analyzer
//     does not descend into it. Scanning inside one reports the `printf` in an awk body
//     as a shell command, which it is not.
//
//   - `$(...)` is not opaque. Its contents are shell that the analyzer does read, so
//     this function recurses into them. That is how `X=$([ -f dva.yml ] && ...)` is
//     caught, and it is the whole reason the two cases cannot share a code path.
//
//   - `"..."` is data, but a `$(...)` inside one is not: am reads it, so the interior of
//     every double-quoted string is searched for substitutions and only those are
//     scanned. Measured: `echo "  services: $(yaml_block_keys "$f" services)"` blocked
//     the step on `yaml_block_keys`, which is why treating the whole string as opaque --
//     as this function used to -- hid three shipped defects from the rules below.
func shellTokens(text string, base int, depth int) []bareWord {
	var out []bareWord
	cmdPos := true
	i := 0
	for i < len(text) {
		c := text[i]
		switch {
		case c == ' ' || c == '\t':
			i++
			continue
		case c == '#' && (i == 0 || text[i-1] == ' ' || text[i-1] == '\t' || text[i-1] == '\n'):
			for i < len(text) && text[i] != '\n' {
				i++
			}
			continue
		case c == '\n' || strings.ContainsRune(";&|()", rune(c)):
			start := i
			for i < len(text) && (text[i] == c || (c != '\n' && text[i] == c)) {
				i++
			}
			out = append(out, bareWord{word: text[start:i], offset: base + start, op: true, depth: depth})
			cmdPos = true
			continue
		case c == '<' || c == '>':
			// A redirection and its target say nothing about command position, but they
			// do end the argument list this rule walks.
			start := i
			for i < len(text) && (text[i] == '<' || text[i] == '>' || text[i] == '&') {
				i++
			}
			out = append(out, bareWord{word: text[start:i], offset: base + start, op: true, depth: depth})
			continue
		}

		start := i
		quoted := false
		var nested []bareWord
		for i < len(text) {
			c := text[i]
			if c == ' ' || c == '\t' || c == '\n' || strings.ContainsRune(";&|()<>", rune(c)) {
				break
			}
			switch {
			case c == '\\':
				quoted = true
				i += 2
			case c == '\'':
				quoted = true
				i = skipQuoted(text, i+1, '\'')
			case c == '"':
				quoted = true
				end := skipQuoted(text, i+1, '"')
				nested = append(nested, dquoteTokens(text[i+1:max(i+1, end-1)], base+i+1, depth+1)...)
				i = end
			case c == '`':
				quoted = true
				i = skipQuoted(text, i+1, '`')
			case c == '$' && i+1 < len(text) && text[i+1] == '(':
				quoted = true
				end := skipBalancedParen(text, i+1)
				nested = append(nested, shellTokens(text[i+2:max(i+2, end-1)], base+i+2, depth+1)...)
				i = end
			case c == '$':
				quoted = true
				i++
			default:
				i++
			}
		}
		word := text[start:i]
		// A `{`, `}` or `]` glued to the end of a word -- `];` is handled above, but
		// `markers"}` is not -- would otherwise read as part of it. Only standalone
		// braces are control, and those arrive here as whole words.
		if word == "{" || word == "}" {
			out = append(out, bareWord{word: word, offset: base + start, op: true, depth: depth})
			cmdPos = true
			out = append(out, nested...)
			continue
		}
		if word != "" {
			out = append(out, bareWord{word: word, offset: base + start, quoted: quoted, cmdPos: cmdPos, depth: depth})
			cmdPos = shellReserved[word]
		}
		out = append(out, nested...)
	}
	return out
}

// skipQuoted returns the index just past the closing quote starting the scan at i, or the
// end of text when the quote is unterminated.
func skipQuoted(text string, i int, quote byte) int {
	for i < len(text) {
		if text[i] == '\\' && quote == '"' {
			i += 2
			continue
		}
		if text[i] == quote {
			return i + 1
		}
		i++
	}
	return len(text)
}

// dquoteTokens returns the tokens of every `$(...)` inside a double-quoted string. The
// surrounding text is data and is not scanned.
func dquoteTokens(text string, base int, depth int) []bareWord {
	var out []bareWord
	for i := 0; i < len(text); {
		switch {
		case text[i] == '\\':
			i += 2
		case text[i] == '$' && i+1 < len(text) && text[i+1] == '(':
			end := skipBalancedParen(text, i+1)
			out = append(out, shellTokens(text[i+2:max(i+2, end-1)], base+i+2, depth)...)
			i = end
		default:
			i++
		}
	}
	return out
}

// skipBalancedParen returns the index just past the `)` matching the `(` at i.
func skipBalancedParen(text string, i int) int {
	depth := 0
	for i < len(text) {
		switch text[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
		i++
	}
	return len(text)
}

// reCommentSubstitution finds what am reads as code inside a comment: a backtick pair, or
// `$(`. Measured against am cb8b4ce -- a comment carrying `backend` blocked the step on
// "backend not in allowlist", and one carrying $(nosuchcmd) blocked it on "nosuchcmd",
// while the same words without the wrapper ran fine. POSIX sh executes neither.
var reCommentSubstitution = regexp.MustCompile("`[^`]*`|\\$\\([^)]*\\)?")

// commentSubstitutions returns, for every comment in text, the offset and source text of
// each substitution span. Whole-line comments and trailing ones are both reported: am
// does not care where the `#` sits, and neither does the block.
//
// A `#` inside single or double quotes is not a comment, so the scan reuses the same
// quote-skipping the bare-word tokenizer does rather than matching `#` in raw text.
func commentSubstitutions(text string) []commentSpan {
	var out []commentSpan
	for _, r := range shellCommentRanges(text) {
		for _, m := range reCommentSubstitution.FindAllStringIndex(text[r[0]:r[1]], -1) {
			out = append(out, commentSpan{offset: r[0] + m[0], text: text[r[0]+m[0] : r[0]+m[1]]})
		}
	}
	return out
}

// shellCommentRanges returns the half-open byte range of every comment in text.
//
// A `#` inside single or double quotes is not a comment, so the scan reuses the same
// quote-skipping the bare-word tokenizer does rather than matching `#` in raw text. Note
// that this is /bin/sh semantics, not am semantics: am is the one that lets a comment
// change quote state, which is what `comment-apostrophe` below is about.
func shellCommentRanges(text string) [][2]int {
	var out [][2]int
	for i := 0; i < len(text); {
		switch text[i] {
		case '\'', '"':
			i = skipQuoted(text, i+1, text[i])
		case '\\':
			i += 2
		case '#':
			end := strings.IndexByte(text[i:], '\n')
			if end < 0 {
				end = len(text)
			} else {
				end += i
			}
			out = append(out, [2]int{i, end})
			i = end
		default:
			i++
		}
	}
	return out
}

// commentApostrophes returns the offset of every apostrophe sitting inside a comment.
//
// am carries quote state across a comment: `#` does not end a string, and a string is not
// confined to a line. An odd number of apostrophes in prose therefore inverts the quote
// parity of everything after it, and the next legitimate `\'...\'` argument opens where the
// analyzer believes one closes -- exposing its contents as shell. Measured against am
// cb8b4ce: a step whose comment said "block\'s" blocked its own multi-line `awk \'...\'`
// program on `command "BEGIN" not in allowlist`, and adding a second apostrophe to the
// same comment made the step run.
func commentApostrophes(text string) []int {
	var out []int
	for _, r := range shellCommentRanges(text) {
		for i := r[0]; i < r[1]; i++ {
			if text[i] == '\'' {
				out = append(out, i)
			}
		}
	}
	return out
}

// reShellFuncDef matches a POSIX function definition at the start of a line, which is the
// only shape the corpus uses and the only one worth guessing at.
var reShellFuncDef = regexp.MustCompile(`(?m)^[ \t]*([A-Za-z_][A-Za-z0-9_]*)[ \t]*\(\)[ \t]*\{`)

// localFunctionCalls returns each place text calls a function text itself defines. The
// definition is harmless -- am blocks on the call, so only the call is reported.
func localFunctionCalls(text string) []bareWord {
	defined := map[string]bool{}
	atDef := map[int]bool{}
	for _, m := range reShellFuncDef.FindAllStringSubmatchIndex(text, -1) {
		defined[text[m[2]:m[3]]] = true
		atDef[m[2]] = true
	}
	if len(defined) == 0 {
		return nil
	}
	var out []bareWord
	for _, t := range shellTokens(text, 0, 0) {
		if t.cmdPos && !t.quoted && defined[t.word] && !atDef[t.offset] {
			out = append(out, t)
		}
	}
	return out
}

// commentSpan is one substitution found inside a comment.
type commentSpan struct {
	offset int
	text   string
}
