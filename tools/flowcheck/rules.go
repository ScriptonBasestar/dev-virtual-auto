package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// finding is one rule violation, anchored to the YAML node that produced it.
type finding struct {
	rule string
	line int
	msg  string
}

// shellField is a scalar the flow runtime hands to /bin/sh: a step `action`, or any
// value under a step `context`. Prompt bodies are deliberately excluded. They are
// prose *about* dva, and scanning them is what made an earlier hand audit report
// `dva repo` and `dva commands` as real commands and miss the one that mattered.
type shellField struct {
	node *yaml.Node
	name string
}

type scan struct {
	shells   []shellField
	gates    []shellField
	findings []finding
	dvaCalls int
	// reportFields counts fields that read a tmp/ JSON artifact with jq, not individual
	// jq invocations. One field may hold several; the rule's unit is the field, because
	// that is what carries (or lacks) the single-object guard.
	reportFields int
	// producers indexes every `context:` value by the `<step id>.<key>` name a `when:`
	// reference uses, so a gate can be checked against the shell that feeds it. A gate is
	// only as sound as its producer, and the two sit hundreds of lines apart.
	producers map[string]*yaml.Node
	// gateRefsSeen keeps one producer from being reported once per gate that reads it.
	// `validate_pass1.is_valid` feeds three gates; the defect is in the producer, once.
	gateRefsSeen map[string]bool
}

func (s *scan) add(rule string, line int, format string, args ...any) {
	s.findings = append(s.findings, finding{rule: rule, line: line, msg: fmt.Sprintf(format, args...)})
}

// walk descends the document, collecting shell fields and checking the rules that are
// about structure rather than shell text. parentKey is the mapping key that owns n.
func walk(n *yaml.Node, parentKey string, s *scan) {
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range n.Content {
			walk(c, parentKey, s)
		}
	case yaml.MappingNode:
		stepID := scalarField(n, "id")
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Value == "context" && stepID != "" && v.Kind == yaml.MappingNode {
				for j := 0; j+1 < len(v.Content); j += 2 {
					if v.Content[j+1].Kind == yaml.ScalarNode {
						if s.producers == nil {
							s.producers = map[string]*yaml.Node{}
						}
						s.producers[stepID+"."+v.Content[j].Value] = v.Content[j+1]
					}
				}
			}
			switch k.Value {
			case "exit_if_empty":
				s.add("exit-if-empty", k.Line,
					"`exit_if_empty` ends the pipeline *successfully*, so a missing prerequisite "+
						"produces a clean run that did nothing — indistinguishable from a run with "+
						"nothing to do. Exit non-zero from the step instead.")
			case "action":
				if v.Kind == yaml.ScalarNode {
					s.shells = append(s.shells, shellField{node: v, name: "action"})
				}
			case "parameters":
				checkParameters(v, s)
			case "when":
				if v.Kind == yaml.ScalarNode {
					s.gates = append(s.gates, shellField{node: v, name: "when"})
				}
			}
			if parentKey == "context" && v.Kind == yaml.ScalarNode {
				s.shells = append(s.shells, shellField{node: v, name: "context." + k.Value})
			}
			walk(v, k.Value, s)
		}
	}
}

// checkParameters enforces flow.schema.json's Parameter contract, where `default` is a
// string and `enum` holds strings. YAML resolves a bare `true` to a boolean, so
// `enum: [true, false]` type-fails validation while looking perfectly reasonable.
//
// Both spellings are accepted: this corpus writes `parameters:` as a sequence of maps
// carrying a `name:` field, while the schema also allows a mapping keyed by name.
func checkParameters(params *yaml.Node, s *scan) {
	switch params.Kind {
	case yaml.SequenceNode:
		for _, spec := range params.Content {
			checkParameterSpec(paramName(spec), spec, s)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(params.Content); i += 2 {
			checkParameterSpec(params.Content[i].Value, params.Content[i+1], s)
		}
	}
}

// scalarField reads a scalar field off a mapping, or "" when it is absent or not a
// scalar. Used to recover the step `id` that owns a `context:` block.
func scalarField(n *yaml.Node, key string) string {
	if n.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key && n.Content[i+1].Kind == yaml.ScalarNode {
			return n.Content[i+1].Value
		}
	}
	return ""
}

// paramName reads the `name:` field of a sequence-style parameter, for the message only.
func paramName(spec *yaml.Node) string {
	if spec.Kind != yaml.MappingNode {
		return "?"
	}
	for i := 0; i+1 < len(spec.Content); i += 2 {
		if spec.Content[i].Value == "name" {
			return spec.Content[i+1].Value
		}
	}
	return "?"
}

func checkParameterSpec(name string, spec *yaml.Node, s *scan) {
	if spec.Kind != yaml.MappingNode {
		return
	}
	for j := 0; j+1 < len(spec.Content); j += 2 {
		fk, fv := spec.Content[j], spec.Content[j+1]
		switch fk.Value {
		case "default":
			if fv.Kind == yaml.ScalarNode && fv.Tag != "!!str" {
				s.add("param-type", fv.Line,
					"parameter %q: default %s is %s, but flow.schema.json requires a string — quote it",
					name, fv.Value, yamlType(fv.Tag))
			}
		case "enum":
			for _, item := range fv.Content {
				if item.Tag != "!!str" {
					s.add("param-type", item.Line,
						"parameter %q: enum value %s is %s, but flow.schema.json requires strings — quote it",
						name, item.Value, yamlType(item.Tag))
				}
			}
		}
	}
}

func yamlType(tag string) string { return strings.TrimPrefix(tag, "!!") }

var (
	// A jq default whose fallback is a boolean literal is a decision that cannot fail
	// closed. `//` substitutes for `false` as well as `null`, so `.x // true` reads an
	// explicit `false` back as `true` and the stop path becomes unreachable.
	reBoolDefault = regexp.MustCompile(`//\s*(true|false)\b`)

	// A dva invocation, as opposed to the word "dva" in an error message: the command
	// must sit in command position — start of line, or after && || | ; ( $( !.
	reDvaCall = regexp.MustCompile(`(?:^|[\n&|;()!])[ \t]*dva[ \t]+([a-z][a-z0-9:_-]*)`)

	// A filter inside a `when:` reference defeats the gate: measured against am cb8b4ce,
	// `{{ref | trim}} == 'true'` and `{{ref | trim}} == 'false'` both ran the step.
	reGateFilter = regexp.MustCompile(`\{\{[^}]*\|[^}]*\}\}`)

	// An unquoted `true`/`false` operand parses as a boolean while the template renders to
	// a string, and a string never equals a boolean. `!= true` therefore ran the step for
	// every value and `== true` skipped it for every value. `== 'true'` is correct, so the
	// quote is what the rule looks for.
	reGateBareBool = regexp.MustCompile(`(==|!=)[ \t]*(true|false)\b`)

	reTemplate = regexp.MustCompile(`\{\{`)

	// The `<step>.<key>` a gate reads, so the producing shell can be found and checked.
	reGateRef = regexp.MustCompile(`\{\{[ \t]*([A-Za-z_][A-Za-z0-9_-]*)\.([A-Za-z_][A-Za-z0-9_-]*)[ \t]*\}\}`)

	// A flag emitted with `echo` carries a trailing newline. am does not trim before
	// comparing, so the value renders as "true\n" and never equals 'true' -- the same
	// silent fail-open as an unquoted operand, reached from the other end.
	reEchoFlag = regexp.MustCompile(`\becho\b[ \t]+["']?(true|false)["']?`)

	reJq        = regexp.MustCompile(`\bjq\b`)
	reTmpPath   = regexp.MustCompile(`\btmp/`)
	reJSONGuard = regexp.MustCompile(`jq\s+-e\s+-s\b`)
)

// blankComments replaces whole-line shell comments with blank lines, preserving line
// numbering. Comments do not execute, and the fix for a defect routinely explains that
// defect in a comment directly above the code — a scanner that reads them reports the
// explanation as the bug.
func blankComments(text string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimLeft(l, " \t"), "#") {
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}

// lineOf maps a byte offset inside a scalar back to a file line. A block scalar's
// node.Line is the `key: |` line, so its content starts on the next one; every other
// style sits on the line node.Line already names. Most `context:` entries in this corpus
// are single-line quoted scalars, so treating them as block scalars misreports every
// finding on them by one line.
func lineOf(f shellField, text string, offset int) int {
	start := f.node.Line
	if f.node.Style == yaml.LiteralStyle || f.node.Style == yaml.FoldedStyle {
		start++
	}
	return start + strings.Count(text[:offset], "\n")
}

// checkShell runs the rules that read shell text. reserved is the live built-in command
// set, imported from internal/config so the list is never kept in two places.
func checkShell(f shellField, reserved map[string]bool, s *scan) {
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
	toks := shellTokens(text, 0)
	for i := range toks {
		if !toks[i].cmdPos || toks[i].quoted || !bareWordTriggers[toks[i].word] {
			continue
		}
		for j := i + 1; j < len(toks) && !toks[j].op; j++ {
			if toks[j].word == "]" || toks[j].word == "]]" {
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
//   - `$(...)` is not opaque. Its contents are shell that the analyzer does read, so
//     this function recurses into them. That is how `X=$([ -f dva.yml ] && ...)` is
//     caught, and it is the whole reason the two cases cannot share a code path.
//
// `"..."` is treated as opaque like `'...'`. A trigger command inside a double-quoted
// string would have to arrive through a nested `$(...)`; nothing in the corpus does that,
// and guessing at it would cost more than it catches.
func shellTokens(text string, base int) []bareWord {
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
			out = append(out, bareWord{word: text[start:i], offset: base + start, op: true})
			cmdPos = true
			continue
		case c == '<' || c == '>':
			// A redirection and its target say nothing about command position, but they
			// do end the argument list this rule walks.
			start := i
			for i < len(text) && (text[i] == '<' || text[i] == '>' || text[i] == '&') {
				i++
			}
			out = append(out, bareWord{word: text[start:i], offset: base + start, op: true})
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
				i = skipQuoted(text, i+1, '"')
			case c == '`':
				quoted = true
				i = skipQuoted(text, i+1, '`')
			case c == '$' && i+1 < len(text) && text[i+1] == '(':
				quoted = true
				end := skipBalancedParen(text, i+1)
				nested = append(nested, shellTokens(text[i+2:max(i+2, end-1)], base+i+2)...)
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
			out = append(out, bareWord{word: word, offset: base + start, op: true})
			cmdPos = true
			out = append(out, nested...)
			continue
		}
		if word != "" {
			out = append(out, bareWord{word: word, offset: base + start, quoted: quoted, cmdPos: cmdPos})
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

// checkGate runs the rules for `when:` expressions. Both defects below are silent: the
// gate does not error, the step simply runs (or does not) regardless of the value, and
// the pipeline still exits 0 — so neither `am validate` nor a passing run reveals them.
//
// Only expressions carrying a template are checked. `when: "true == true"` is a literal
// comparison and the evaluator handles it correctly.
func checkGate(f shellField, s *scan) {
	text := f.node.Value
	if !reTemplate.MatchString(text) {
		return
	}

	if m := reGateFilter.FindStringIndex(text); m != nil {
		s.add("gate-filter", lineOf(f, text, m[0]),
			"when: a filter inside the reference defeats the gate — the step runs whatever "+
				"the value and whatever the operator. Drop the filter; `printf` in the "+
				"producing step emits no trailing newline to trim.")
	}

	if m := reGateBareBool.FindStringSubmatchIndex(text); m != nil {
		op, lit := text[m[2]:m[3]], text[m[4]:m[5]]
		s.add("gate-operand", lineOf(f, text, m[0]),
			"when: `%s %s` compares a rendered string against a YAML boolean, which can never "+
				"be equal — `!= %s` runs the step for every value and `== %s` skips it for "+
				"every value. Quote the operand: `%s '%s'`.", op, lit, lit, lit, op, lit)
	}

	checkGateProducers(f, text, s)
}

// checkGateProducers follows each `{{step.key}}` a gate reads back to the shell that
// produces it. Quoting the operand is not sufficient on its own: the comparison is exact,
// so a producer that appends a newline defeats a correctly written gate. The finding is
// reported at the producer, which is where the fix goes and routinely hundreds of lines
// from the gate. References the file does not define — `param.*`, or a step in another
// file — are skipped rather than guessed at.
func checkGateProducers(f shellField, text string, s *scan) {
	for _, m := range reGateRef.FindAllStringSubmatch(text, -1) {
		ref := m[1] + "." + m[2]
		p, ok := s.producers[ref]
		if !ok || s.gateRefsSeen[ref] {
			continue
		}
		body := blankComments(p.Value)
		em := reEchoFlag.FindStringSubmatchIndex(body)
		if em == nil {
			continue
		}
		if s.gateRefsSeen == nil {
			s.gateRefsSeen = map[string]bool{}
		}
		s.gateRefsSeen[ref] = true
		s.add("gate-producer-newline", lineOf(shellField{node: p, name: ref}, body, em[0]),
			"context.%s: feeds the `when:` gate at line %d but emits its flag with `echo`, "+
				"which appends a newline. am does not trim, so the value renders as %q and "+
				"never equals '%s' — the gate runs for every value under `!=` and skips for "+
				"every value under `==`. Emit with `printf %s` instead.",
			m[2], f.node.Line, body[em[2]:em[3]]+"\n", body[em[2]:em[3]], body[em[2]:em[3]])
	}
}
