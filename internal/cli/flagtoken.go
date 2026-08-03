package cli

import (
	"strconv"
	"strings"
)

// This file is the one place DVA decides what an argv token means.
//
// Four consumers used to decide it independently — applyRootPersistentFlagsFromArgs and
// consumeRootPersistentFlags (root.go), parseDvaFlags and consumeDryRunFlag (compose.go) —
// and all four compared tokens by exact string. So `--debug=true` matched nothing, was
// neither applied nor stripped, and walked into the docker argv (TASK-145):
//
//	dva stack log infra --debug=true --tail=5   → compose … logs --debug=true --tail=5
//	dva --debug=true stack log infra --tail=5   → compose … logs --debug=true infra --tail=5
//	dva compose logs --json=true                → compose … logs --json=true
//
// Debug was not enabled either, so the user got neither the flag's effect nor a diagnosis,
// only an unexplained error from docker.
//
// The consumers own different subsets of flags — that is why the shared piece is at the
// token level and not the argv level. A single "scan the whole slice" helper would have to
// know every subset, and the subsets are genuinely different: consumeRootPersistentFlags
// must leave --dry-run alone because compose owns it on the passthrough path, while
// parseDvaFlags must consume it.

// splitFlagToken splits an argv token into the flag it names and any inline value.
//
//	--debug        → ("--debug", "",     false)
//	--debug=true   → ("--debug", "true", true)
//	-M=dev         → ("-M",      "dev",  true)
//	--tag=a,b      → ("--tag",   "a,b",  true)
//	infra          → ("infra",   "",     false)
//	KEY=value      → ("KEY=value", "",   false)
//
// The leading dash is required before an `=` is treated as a separator, so a positional
// `KEY=value` — which `dva run` forwards to interaction commands — stays one token.
//
// The value is returned unsplit: `--tag=a,b` carries one value and the comma is the tag
// flag's business, not the token grammar's.
func splitFlagToken(a string) (name, value string, hasValue bool) {
	if !strings.HasPrefix(a, "-") {
		return a, "", false
	}
	if i := strings.IndexByte(a, '='); i > 0 {
		return a[:i], a[i+1:], true
	}
	return a, "", false
}

// dvaFlagEnd returns the index where DVA's own flags stop: the position of the first `--`,
// or len(args) when there is none. Tokens at and after it are the other program's, whatever
// they spell — before TASK-145 nothing looked for the terminator at all, so
// `dva stack log infra -- --debug --tail=5` reached docker as `logs -- --tail=5`, with the
// literal the `--` was there to protect eaten anyway.
func dvaFlagEnd(args []string) int {
	for i, a := range args {
		if a == "--" {
			return i
		}
	}
	return len(args)
}

// flagBoolValue reads a boolean flag's value. A bare `--debug` is true; `--debug=X` takes X.
//
// ok is false when X is not a boolean. Callers must not guess in that case: what they do
// with it differs by position, and the difference is deliberate. consumeRootPersistentFlags
// is the boundary with the external program, so it rejects; parseDvaFlags leaves the token
// in place for its caller's own unknown-flag rejection to name.
//
// The accepted spellings are strconv.ParseBool's, which are pflag's, so a DisableFlagParsing
// command answers `--debug=1` the same way its normally-parsed siblings do.
func flagBoolValue(value string, hasValue bool) (v, ok bool) {
	if !hasValue {
		return true, true
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, false
	}
	return parsed, true
}

// flagValue returns a value-taking flag's value and how many extra tokens it consumed.
//
// `--mode=dev` consumes none, `--mode dev` consumes one. ok is false when a bare flag ends
// the run of DVA flags with nothing to take — `dva up --mode` — which every caller has
// always treated as "no value given" rather than as an error.
func flagValue(args []string, i, end int, value string, hasValue bool) (v string, consumed int, ok bool) {
	if hasValue {
		return value, 0, true
	}
	if i+1 < end {
		return args[i+1], 1, true
	}
	return "", 0, false
}
