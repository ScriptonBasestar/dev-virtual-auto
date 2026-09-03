package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// This file holds the plan-route guidance guard: the code that decides what to TELL a user
// whose flags suppressed the default plan. It was split out of plan_lifecycle.go when
// TASK-283 grew it past that file's size limit, and the split follows the seam the defect
// exposed. plan_lifecycle.go decides what a plan invocation DOES; this decides what DVA says
// when it will not do it. The one direction of dependency runs from here into there — the
// guard asks parsePlanFlags whether a rewritten invocation is acceptable rather than
// re-deriving that rule, which is the property TASK-283 exists to establish.

// rejectSuppressedDefaultPlan refuses silent whole-stack fallthrough when exactly
// one plan exists (DefaultPlan) but leading args prevented detectPlanRoute from
// selecting it. Name the plan explicitly instead (e.g. dva up p1 --dev).
//
// Non-flag tokens are left to rejectUnknownPlanArg / rejectUpPositionalArg so
// unknown plan names keep their existing messages.
func rejectSuppressedDefaultPlan(c *config.Config, command string, args []string) error {
	// Classify what the terminator SEPARATES, not the terminator itself. `dva restart -- s1`
	// names an entry exactly as `dva restart s1` does, so judging args[0]=="--" refused a
	// perfectly explicit invocation. The message still echoes the untouched args, because the
	// suggestion has to be a command the user can paste back. TASK-210.
	head := dropLeadingTerminator(args)
	if c == nil || !c.HasPlans() || len(head) == 0 {
		return nil
	}
	// A terminator occupied the plan-name slot, so the dash test below cannot apply: the user
	// wrote no flag, and "flags suppress the default plan" would be a false account of an
	// invocation that contains none. `dva restart -- --no-wat` said exactly that where a
	// default plan resolved, while the same command in a plan-less config already answered
	// `unknown stack entry "--no-wat"`. The command's own name check is the one that can tell
	// the user what is wrong with the token; this guard steps aside so it is reached. TASK-210.
	if len(head) != len(args) {
		return nil
	}
	def := c.DefaultPlan()
	if def == "" {
		return nil
	}
	if _, exists := c.Plans[head[0]]; exists {
		return nil
	}
	if !isFlagToken(head[0]) {
		return nil
	}
	// The suggestion has to be a command that works, not merely one that names the plan.
	// Echoing the args untouched proposed `dva up p1 --tag app` for `dva up --tag app`, and
	// the plan path answers that with `unsupported plan flag: --tag` — so following the
	// advice was what broke the invocation. The four selectors are path-conditional: they
	// filter and resolve on the whole-stack path, which is the path `dva up --tag app`
	// would have taken had no plan been declared, and declaring a plan takes them away.
	// TASK-273.
	//
	// Only those selectors are stripped. `--force`, `--no-wait`, `--var K=V`, `--purge` and
	// `-v` reach the plan path intact and the original suggestion was already correct for
	// them, so it is left alone; an unknown flag keeps it too, because there the second
	// error names the flag and is the answer the user needs.
	res := stripStackPathOnlyFlags(args)

	// Two ways the args reaching here are not what the sentence below assumes. TASK-283 filed
	// both as suggestions printed with the confidence of one that runs.
	//
	// 1. A command whose whole-stack path does not read these selectors. `dva logs` forwards
	//    its argv to docker compose and calls parseDvaFlags on none of it, so `--tag` is
	//    docker's token on BOTH paths and "it works only on the whole-stack path" is simply
	//    false there. Stripping it made that false claim and then dropped a flag the user
	//    typed. The repair is to strip nothing and say only what this guard knows: the default
	//    plan is suppressed, so name it. Whether docker then accepts `--tag` is docker's to
	//    report and is unchanged by naming the plan, which is exactly why DVA must not guess
	//    at it — the flag sets that passthrough commands forward are not ours and not pinned.
	//    `status` is listed for the same reason though cobra refuses the selector before this
	//    guard runs; "unreachable today" is not a property worth depending on.
	if !slices.Contains(selectorAwarePlanCommands, command) {
		res = strippedStackPathFlags{remaining: args}
	}
	// 2. A selector parseDvaFlags will reject outright — `--tag` with nothing to take,
	//    `--tag -T`, `--tag=`. It runs a few lines below the callers that reach here with
	//    selectors intact and it alone knows which rule was broken; `--tag requires a value,
	//    got the flag -T` is the answer, and no wording available here improves on it. This is
	//    the treatment an unknown flag already gets, for the same reason. The invocation still
	//    fails, so nothing falls through to the whole stack.
	if res.malformed != "" {
		return nil
	}

	if len(res.removed) == 0 {
		return fmt.Errorf(
			"flags suppress the default plan %q; name it explicitly: %s",
			def, planSuggestion(command, def, args),
		)
	}

	verb, pronoun := "works", "it"
	if len(res.removed) > 1 {
		verb, pronoun = "work", "them"
	}

	// A selector that ate a flag-shaped token as its value. parseDvaFlags accepts this —
	// `dva up --tag --no-wait` runs with the tag named "--no-wait" — so stepping aside would be
	// the silent whole-stack fallthrough this guard exists to refuse. But dropping the selector
	// cannot be done honestly either: `dva up p1 --no-wait` runs, and means something the user
	// did not write, because their --no-wait was a value and this one is a flag. Neither choice
	// is the guard's to make, so it states what happened and offers no command. TASK-283.
	//
	// "Flag-shaped" is not the test; "would become a flag" is. `dva up --tag -5` declares a tag
	// literally named "-5" — the whole-stack path accepts it and matches nothing — and the plan
	// path answers `unsupported plan flag: -5`, so dropping the pair restores nothing and
	// `dva up p1` is the same honest suggestion the ordinary `--tag app` case gets. Only a token
	// the plan path would honour makes the rewrite change what was asked for. The first draft
	// tested isFlagToken alone and refused to advise on eight well-formed invocations.
	if res.swallowedFlag != "" && planPathHonoursFlag(command, res.swallowedValue) {
		return fmt.Errorf(
			"flags suppress the default plan %q; %s took %q as its value rather than leaving it a flag, "+
				"so dropping %s to name the plan would turn %q back into one and change what you asked for. "+
				"Write the value inline (%s=%s) if that is what you meant, or remove %s",
			def, res.swallowedFlag, res.swallowedValue, res.swallowedFlag, res.swallowedValue,
			res.swallowedFlag, res.swallowedValue, res.swallowedFlag,
		)
	}

	// Whatever survives the strip is what parsePlanFlags will read, so ask it. Hand-checking
	// for a leftover word got this wrong in both directions in one measurement: `dva up --tag app
	// web` needed rejecting and `dva up --tag app --var K=V` did not, because K=V is --var's value
	// and not a stray at all. A private copy of that rule would be a second parser to keep in step
	// with the real one, and the guard exists because the first copy drifted.
	//
	// Only on this branch. When nothing was stripped the guard is echoing what the user typed
	// and asserting only where the plan name goes; an unknown flag there keeps its suggestion on
	// purpose, because the plan path's own `unsupported plan flag` is the answer and moving the
	// user to that path is what produces it. Here the guard REWROTE the invocation, so the result
	// is its own claim to make good on. TASK-283.
	//
	// Measured, not assumed, on the question this replaced: parsePlanFlags answers `unexpected
	// argument in plan mode` for every bare word, on all four of these commands. `dva restart p1
	// web` does not name an entry the way `dva restart web` does, so the exception that was drafted
	// for restart would have printed a command that fails.
	if _, err := parsePlanFlags(command, res.remaining); err != nil {
		return fmt.Errorf(
			"flags suppress the default plan %q; %s %s only on the whole-stack path and the plan path "+
				"rejects %s, but what is left over (%s) is not accepted on the plan path either (%v), "+
				"so no corrected command can be offered",
			def, strings.Join(res.removed, ", "), verb, pronoun,
			strings.Join(res.remaining, " "), err,
		)
	}

	return fmt.Errorf(
		"flags suppress the default plan %q; %s %s only on the whole-stack path and the plan path rejects %s, so name the plan without %s: %s",
		def, strings.Join(res.removed, ", "), verb, pronoun, pronoun,
		planSuggestion(command, def, res.remaining),
	)
}

// planPathHonoursFlag reports whether parsePlanFlags reads a token as a flag it acts on, as
// opposed to refusing it. It is asked about one token in isolation, which is exactly the
// question: whether that token, standing alone in a rewritten invocation, would do something.
//
// The verb is not decoration. TASK-279 made parsePlanFlags verb-aware, so --no-wait is honoured
// on up and restart and rejected on stop and down, and the guard's answer moves with it: on up,
// `dva up --tag --no-wait` gets the refusal below because dropping --tag would arm a flag the
// user wrote as a value; on stop, the same shape gets an ordinary `dva stop p1` suggestion,
// because restoring --no-wait there restores nothing — that verb rejects it outright, so the
// token is only ever a tag value and dropping it with its flag is faithful. Asking the parser
// per verb is what makes both answers correct without either being written down here.
func planPathHonoursFlag(verb, token string) bool {
	_, err := parsePlanFlags(verb, []string{token})
	return err == nil
}

// selectorAwarePlanCommands are the plan-aware commands whose whole-stack path actually reads
// the four path-conditional selectors, so that "they work only on the whole-stack path" is a
// true sentence about them.
//
// Measured against the built binary rather than read off the call sites, because reading
// misses which parser runs first. `build` parses the selectors — parseDvaFlags is the first
// statement of its RunE — but it does so BEFORE calling this guard, so the guard is handed
// args with the selectors already gone and never takes the branch this list gates. It is
// absent from the list because the list is about what the guard can truthfully say, and on
// build the question does not arise. `logs` and `status` are absent because they read none of
// the four on either path. TASK-283.
var selectorAwarePlanCommands = []string{"up", "down", "stop", "restart"}

// planSuggestion assembles the command line this guard tells the user to run: the plan named
// explicitly, the arguments that survive on the plan path, and the root flags this invocation
// consumed before the guard could see them.
//
// That last part is the whole reason it is a function. wrapWithHooks calls consumeDryRunFlag
// (hooks.go:29) before a hookable command's RunE runs, which REMOVES --dry-run from args and
// records it in the package-level dryRun; parseDvaFlags writes the same global on the paths
// cobra does not parse. Either way the guard is handed args with no --dry-run in them, so
// every suggestion silently dropped it: `dva up --dry-run --tag app` was answered with
// `dva up p1`, and following that advice performed the change the user had asked to preview.
//
// Only --dry-run is restored. --debug and --json are consumed by consumeRootPersistentFlags,
// which serves the passthroughs and not up/down/stop/restart, so on this path they are still
// in args and adding them here would print them twice. dryRun is written from nothing but a
// user-typed --dry-run, which is what makes reading it back exact rather than a guess.
// TASK-283.
func planSuggestion(command, plan string, rest []string) string {
	parts := make([]string, 0, len(rest)+4)
	parts = append(parts, "dva", command, plan)
	parts = append(parts, rest...)
	if dryRun {
		parts = append(parts, "--dry-run")
	}
	return strings.Join(parts, " ")
}

// strippedStackPathFlags is what stripStackPathOnlyFlags found: the arguments that survive on
// the plan path, the selector names it removed in the order they were written, and the two
// ways a selector can make any suggestion built from the rest untrustworthy.
//
// malformed and swallowedFlag are both "this pair is not a clean selector", split because the
// caller does opposite things with them: malformed means parseDvaFlags will report it better,
// swallowed means parseDvaFlags will ACCEPT it and the guard is the only code that can point
// out what the acceptance costs. TASK-283.
type strippedStackPathFlags struct {
	remaining []string
	removed   []string
	// malformed names the first selector parseDvaFlags will reject: nothing to take, an
	// empty or blank value, or another DVA flag where the value belongs.
	malformed string
	// swallowedFlag names the first selector that consumed a flag-shaped token as its value,
	// with swallowedValue the token it consumed. parseDvaFlags does the same thing and calls
	// it a legal value.
	swallowedFlag  string
	swallowedValue string
}

// stripStackPathOnlyFlags removes the selectors parsePlanFlags rejects, together with the
// values they take, and reports what it removed and what was wrong with it.
//
// The value rule is parseDvaFlags' takeValue, reproduced through the same flagValue helper so
// the two cannot drift apart silently. It is not a matter of taste: what this walk leaves
// behind becomes a command printed to the user, so a token the real parser would have eaten
// as a value is a token that must not reappear as an argument. An earlier version diverged
// deliberately — it left any flag-shaped token in place so that `dva up --tag --no-wait` would
// suggest `dva up p1 --no-wait` rather than swallow a flag the user typed — and the divergence
// produced exactly the wrong suggestions TASK-283 was filed for. `--tag -5` stranded a `-5`
// that parseDvaFlags reads as a perfectly ordinary tag name, so the suggestion carried a token
// nothing accepts; `--tag -T web` stranded `web`; and `--no-wait` did not survive as the flag
// the divergence was meant to preserve, because on the invocation as written it was never a
// flag at all. The three anomalies are reported instead of papered over, and the caller
// decides which of them it can speak to.
//
// Tokens at and after a `--` terminator belong to whatever the invocation forwards to, so
// dvaFlagEnd bounds the walk. rejectSuppressedDefaultPlan only reaches here when no
// terminator occupies the plan-name slot, but one can still appear later in args.
func stripStackPathOnlyFlags(args []string) strippedStackPathFlags {
	end := dvaFlagEnd(args)
	res := strippedStackPathFlags{remaining: make([]string, 0, len(args))}
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, value, hasValue := splitFlagToken(a)
		if i >= end || !slices.Contains(stackPathOnlySelectorFlags, name) {
			res.remaining = append(res.remaining, a)
			continue
		}
		if !slices.Contains(res.removed, name) {
			res.removed = append(res.removed, name)
		}
		if !hasValue && i+1 < end && isRecognizedDVAFlagToken(args[i+1]) {
			if res.malformed == "" {
				res.malformed = name
			}
			i++
			continue
		}
		v, consumed, ok := flagValue(args, i, end, value, hasValue)
		if !ok || strings.TrimSpace(v) == "" {
			if res.malformed == "" {
				res.malformed = name
			}
			i += consumed
			continue
		}
		if consumed > 0 && isFlagToken(v) && res.swallowedFlag == "" {
			res.swallowedFlag, res.swallowedValue = name, v
		}
		i += consumed
	}
	return res
}
