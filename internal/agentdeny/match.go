package agentdeny

import "strings"

// matchDenyPattern reports whether pattern — one of the deny-pattern strings
// GatedCommand.Patterns produces, e.g. "Bash(dva config env show *)" — would block the
// literal command string argv.
//
// This models Claude Code's own documented Bash permission matching for the one shape
// Patterns() ever emits: a "Bash(...)" wrapper names the tool, and inside it a trailing
// " *" (a literal space before the wildcard) matches both the bare prefix on its own and
// the prefix followed by a space and anything after it — per Claude Code's docs, the
// space is part of the rule so "Bash(ls *)" matches bare "ls" and "ls -la" but not
// "lsof". It is deliberately narrow: it exists only so match_test.go and deploy_test.go
// can prove, inside this package's own tests, that Patterns()'s output actually blocks
// the argv variants TASK-286 was asked to cover — it does not reproduce Claude Code's
// full command-parsing pipeline (leading environment-assignment and
// timeout/nice/command/xargs wrapper stripping, shell-operator splitting), so a "false"
// here is not proof Claude Code itself would fail to block argv; see
// docs/agent-deny-rules.md "Honest limits" for what is and is not actually covered.
func matchDenyPattern(pattern, argv string) bool {
	inner, ok := strings.CutPrefix(pattern, "Bash(")
	if !ok {
		return false
	}
	inner, ok = strings.CutSuffix(inner, ")")
	if !ok {
		return false
	}
	prefix, hasTrailingWildcard := strings.CutSuffix(inner, " *")
	if !hasTrailingWildcard {
		return inner == argv
	}
	return argv == prefix || strings.HasPrefix(argv, prefix+" ")
}
