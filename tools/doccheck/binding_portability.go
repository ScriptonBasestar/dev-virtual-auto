package main

import (
	"fmt"
	"regexp"
	"strings"
)

const legacyCheckoutPath = "/Users/archmagece/mywork/scripton/dev-virtual-auto"

var (
	verifyCriterionRe       = regexp.MustCompile(`^[ \t]*- \[[ xX~]\].*?\bverify:\s*`)
	corpusCountAssignmentRe = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)=\$\([^)]*~/mydevbox`)
	commandVDvaExit2Re      = regexp.MustCompile(`(?s)command\s+-v\s+dva(?:\s*>\S+)?\s*\|\|\s*(?:\{[^}]*|[^;\n]*)\bexit\s+2`)
	dvaInvocationRe         = regexp.MustCompile(`(?m)(?:^|[;&|]\s*)dva(?:\s|$)`)
	absoluteToolRe          = regexp.MustCompile(`/[A-Za-z0-9._/-]+`)
)

// checkBindingPortability checks the first inline binding span after verify: on
// task criterion lines. Fenced examples are not bindings, later inline spans on
// the same line are annotations, and table rows are not criterion lines.
func checkBindingPortability(from, body string) (escapedPipes, absCheckout, externalCorpus int, msgs []string) {
	for _, binding := range extractVerifyBindings(body) {
		span := binding.Span
		if hasEscapedShellPipe(span) {
			escapedPipes++
			msgs = append(msgs, fmt.Sprintf("%s:%d: verify binding has escaped shell pipe \\|", from, binding.Line))
		}
		if strings.Contains(span, legacyCheckoutPath) {
			absCheckout++
			msgs = append(msgs, fmt.Sprintf("%s:%d: verify binding names checkout path %q", from, binding.Line, legacyCheckoutPath))
		}
		if strings.Contains(span, "~/mydevbox") && !hasPortableExternalCorpusGuard(span) {
			externalCorpus++
			msgs = append(msgs, fmt.Sprintf("%s:%d: verify binding depends on external corpus ~/mydevbox", from, binding.Line))
		}
	}
	return escapedPipes, absCheckout, externalCorpus, msgs
}

// hasPortableExternalCorpusGuard accepts the two portable forms already used
// by bindings: a non-empty corpus check that exits 2 plus either absolute tool
// paths (060) or an explicit `command -v dva` guard (066). A corpus guard alone
// is insufficient when an unguarded dva invocation can be swallowed by a pipe.
func hasPortableExternalCorpusGuard(span string) bool {
	match := corpusCountAssignmentRe.FindStringSubmatch(span)
	if match == nil || !hasNonemptyCorpusExit2(span, match[1]) {
		return false
	}
	if !dvaInvocationRe.MatchString(span) {
		return absoluteToolRe.MatchString(span)
	}
	return commandVDvaExit2Re.MatchString(span)
}

func hasNonemptyCorpusExit2(span, variable string) bool {
	guard := regexp.MustCompile(`(?s)\[\s*"\$` + regexp.QuoteMeta(variable) + `"\s+-gt\s+0\s*\]\s*\|\|\s*(?:\{[^}]*|[^;\n]*)\bexit\s+2`)
	return guard.MatchString(span)
}

// hasEscapedShellPipe finds \| that a shell receives as text outside quotes.
// A command substitution starts a nested shell even inside double quotes, so its
// body is scanned independently; quoted BRE alternation remains excluded.
func hasEscapedShellPipe(s string) bool {
	type quote byte
	const (
		unquoted quote = iota
		singleQuoted
		doubleQuoted
	)
	state := unquoted
	var restore []quote
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			if i+1 < len(s) && s[i+1] == '|' && state != singleQuoted && state != doubleQuoted {
				return true
			}
			if state != singleQuoted && i+1 < len(s) {
				i++
			}
		case '\'':
			switch state {
			case unquoted:
				state = singleQuoted
			case singleQuoted:
				state = unquoted
			}
		case '"':
			switch state {
			case unquoted:
				state = doubleQuoted
			case doubleQuoted:
				state = unquoted
			}
		case '$':
			if i+1 < len(s) && s[i+1] == '(' && state != singleQuoted {
				restore = append(restore, state)
				state = unquoted
				i++
			}
		case ')':
			if state == unquoted && len(restore) > 0 {
				state = restore[len(restore)-1]
				restore = restore[:len(restore)-1]
			}
		}
	}
	return false
}
