package cli

import (
	"slices"
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
// The first two were measured on `dva stack log`, which has since been removed;
// `dva logs <plan> <entry>` reaches docker by the same path.
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

// isFlagToken reports whether a token is a flag rather than a name.
//
// A lone "-" is not one. It is the rule rejectUnknownFlags has applied since TASK-172 —
// `len(a) < 2` skips it — and the one selectors.go states to the user's face when the
// token reaches a name guard: `read as a %s name: a lone "-" is too short to be a flag`.
//
// Seven other places wrote the same test as a bare strings.HasPrefix and so answered the
// opposite — 7 of the 10 bare-HasPrefix sites under internal/cli at c51dd95. The other
// three decide nothing for a lone "-": selectors.go:60 guards it with len(a) < 2,
// selectors.go:158 sits behind `case n == "-"` at :154, and splitFlagToken
// above classifies no token at all. Five of the seven adopt this helper; the two left are
// message-only and are recorded in TASK-218.
//
// On the plan-selection path that inverted the verdict rather than the wording,
// because those guards return early for a flag: `dva up -` in a config with two plans and
// no default started every entry in the stack while plain `dva up` refused with "multiple
// plans configured". Measured at c51dd95 across six fixtures. TASK-218.
//
// "--" is a flag by this rule, as it already was by rejectUnknownFlags' — len 2 clears the
// length test. The terminator is dvaFlagEnd's business, not this predicate's; a caller that
// means "and not the terminator either" says so with dropLeadingTerminator.
//
// root.go's isFlag uses this same len>1 rule. A lone dash is a supported interaction or entry
// name, so treating it as a flag would suppress shorthand lookup for `dva -`. Before TASK-223,
// the same answer also fed a flags-first rewrite that could turn `dva greet -` into
// `dva run - greet` and run a different declared interaction. TASK-223 aligned the predicates
// and removed that reordering; root_test.go compares shorthand and explicit routing, while
// TestDashPredicatesDisagreeOnPurpose preserves the historical test name and pins the agreement.
func isFlagToken(a string) bool {
	return len(a) > 1 && strings.HasPrefix(a, "-")
}

// dvaFlagEnd returns the index where DVA's own flags stop: the position of the first `--`,
// or len(args) when there is none. Tokens at and after it are the other program's, whatever
// they spell — before TASK-145 nothing looked for the terminator at all, so
// `dva stack log infra -- --debug --tail=5` reached docker as `logs -- --tail=5`, with the
// literal the `--` was there to protect eaten anyway. (Measured on the since-removed stack
// family; `dva logs` inherits the path.)
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
// ok is false when X is not a boolean. What happens next is the caller's business, and the
// four callers do three different things. Do not summarise them as one:
//
//	parseDvaFlags                     rejects — see the paragraph below
//	consumeRootPersistentFlags        rejects — the last code that knows the flag is DVA's
//	applyRootPersistentFlagsFromArgs  skips, deliberately: it runs before RunE, so it has
//	                                  no way to return an error and leaves it to the above
//	consumeDryRunFlag                 leaves the token in its output, and that is safe only
//	                                  because its caller names a flag-shaped leftover itself —
//	                                  hooks.go re-enters the built-in's RunE, which parses the
//	                                  args again. (It had a second caller, infra.go's
//	                                  resolveInfraTargets, which rejected the token directly;
//	                                  that went with `dva infra`.)
//
// parseDvaFlags used to instead leave the token in filtered "for its caller's own
// unknown-flag rejection to name". That held only on command paths that explicitly reject
// leftovers. Other paths have no rejection behind them, and on `dva build` the leftover *is*
// the external argv, so
// `--debug=notabool` was appended to docker's command line — the same leak TASK-145 closed,
// in the one spelling it did not claim. The promise could not be kept there even in
// principle: `dva build` must forward the flags it does not recognise, because `--no-cache`
// is docker's, so its caller cannot tell a malformed DVA flag from a valid docker one.
// parseDvaFlags is the last code that knows `--debug` is DVA's, so it is where the
// rejection belongs. TASK-172.
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
// the run of DVA flags with nothing to take — `dva up --mode`, and also `dva up --mode --`,
// since dvaFlagEnd puts end at the terminator.
//
// ok=false is a report about the token run, not a verdict: this helper is not told the
// flag's name and has no error to return, so it cannot say "--mode requires a value". Its
// caller decides.
//
// There are two callers and neither is silent about ok=false. parseDvaFlags' takeValue
// closure turns it into that error (TASK-211). stripStackPathOnlyFlags (plan_lifecycle.go)
// turns it into a `malformed` selector name, which makes its caller step aside so takeValue
// reports the same thing a few lines later — the second caller exists to reproduce the
// first one's value rule exactly, which is the one reason to reach for this helper rather
// than write the two-line branch again. Before TASK-211 there were four callers, the four
// value-taking cases, each ignoring ok=false; this comment justified their silence by
// claiming the helper also served callers for which taking the next token is optional.
// There were none then and there are none now, so a caller that wants the silence still has
// to earn the exception rather than inherit it from a sentence that was never true.
//
// Counts here are stated with the commit they describe because they go stale silently —
// TASK-208 is five comments that did not.
func flagValue(args []string, i, end int, value string, hasValue bool) (v string, consumed int, ok bool) {
	if hasValue {
		return value, 0, true
	}
	if i+1 < end {
		return args[i+1], 1, true
	}
	return "", 0, false
}

// isRecognizedDVAFlagToken reports whether token names a selector that
// parseDvaFlags consumes. It deliberately does not treat every leading dash as
// a flag: values such as "-weird-but-real" remain values unless DVA owns their
// name. Inline values still name their flag through splitFlagToken.
func isRecognizedDVAFlagToken(token string) bool {
	name, _, _ := splitFlagToken(token)
	return slices.Contains(stackSelectorFlags, name)
}
