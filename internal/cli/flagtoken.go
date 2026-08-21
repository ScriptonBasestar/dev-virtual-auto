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
// Six other places wrote the same test as a bare strings.HasPrefix and so answered the
// opposite. On the plan-selection path that inverted the verdict rather than the wording,
// because those guards return early for a flag: `dva up -` in a config with two plans and
// no default started every entry in the stack while plain `dva up` refused with "multiple
// plans configured". Measured at c51dd95 across six fixtures. TASK-218.
//
// "--" is a flag by this rule, as it already was by rejectUnknownFlags' — len 2 clears the
// length test. The terminator is dvaFlagEnd's business, not this predicate's; a caller that
// means "and not the terminator either" says so with dropLeadingTerminator.
//
// isFlag (root.go) answers "-" the other way, and the two are not merged today — but not
// because isFlag's answer is free. It has two call sites and they differ. Execute:190 gates
// the interaction lookup, and there answering "flag" only hides an interaction named "-"
// (nothing validates the charset — `dva validate` accepts one; `dva -` prints root help
// while `dva run -` reaches it). Execute:210 partitions *every* argument flags-first before
// rewriting os.Args, and there the same answer moves "-" ahead of the command name: with an
// interaction named "-" declared, `dva greet -` becomes `run - greet` and runs "-", passing
// the name the user actually typed to it as an argument, at rc=0. Measured 2026-08-21:
//
//	dva greet -        RAN_DASH_with=[] greet     ← asked for greet, ran "-"
//	dva run greet -    RAN_GREET_with=[] -        ← the explicit form disagrees
//
// A first draft of this comment claimed that slot merely withheld a shorthand; the two rows
// above are what refuted it. So both predicates can turn a wrong answer into an action, and
// root.go's is an open defect (TASK-223), not a settled counterweight. root_test.go pins
// isFlag("-") == true and TestDashPredicatesDisagreeOnPurpose pins the pair — so whoever
// fixes root.go fails both deliberately, with that measurement in hand, instead of merging
// the two on the strength of their similar names.
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
// unknown-flag rejection to name". That held for 7 of its 12 call sites. The other 5 have no
// rejection behind them, and on `dva build` the leftover *is* the external argv, so
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
// There is exactly one caller — parseDvaFlags' takeValue closure, which turns ok=false
// into that error (TASK-211). Before TASK-211 there were four, the four value-taking
// cases, each ignoring ok=false; this comment justified their silence by claiming the
// helper also served callers for which taking the next token is optional. There were
// none then and there are none now, so a future caller that wants the silence has to
// earn the exception rather than inherit it from a sentence that was never true.
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
