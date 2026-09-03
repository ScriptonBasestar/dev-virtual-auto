package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

var composeCmd = &cobra.Command{
	Use:   "compose [ENTRY] [ARGS...]",
	Short: "Execute raw Docker Compose commands",
	Long: `Execute raw Docker Compose commands against a stack entry.

This is a low-level debugging escape hatch. Use 'dva up <plan>' for normal,
validated lifecycle execution.

If only one compose entry exists, the entry name can be omitted.
If multiple compose entries exist, the first argument must be the entry name.`,
	Example: `  dva compose ps                    # Single compose entry
  dva compose main-db ps            # Multiple entries: specify name
  dva compose main-db logs -f api   # Passthrough with entry name`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		e, envReport := loadEnv(c)
		if err := envReport.Err(); err != nil {
			return err
		}

		// Same leak as `dva stack log` (TASK-092), and worse-positioned: these args are
		// appended before the compose subcommand, so `dva --debug --json compose logs`
		// produced `docker compose -f … --debug --json logs`, offering both to `docker
		// compose` itself rather than to `logs`.
		var err error
		if args, err = consumeRootPersistentFlags(args); err != nil {
			return err
		}

		composeEntries := c.ComposeEntries()
		if len(composeEntries) == 0 {
			return fmt.Errorf("no compose entries in stack")
		}

		if len(composeEntries) == 1 {
			// Single entry: name can be omitted, pass all args through
			return execComposePassthroughForEntry(e, c, composeEntries[0], args)
		}

		// Multiple entries: first arg must be entry name
		if len(args) > 0 {
			if entry := c.FindStackEntry(args[0]); entry != nil && entry.ComposeConfig() != nil {
				return execComposePassthroughForEntry(e, c, entry, args[1:])
			}
		}

		// No valid entry name provided — show available entries
		var names []string
		for _, entry := range composeEntries {
			names = append(names, entry.Name)
		}
		return fmt.Errorf("multiple compose entries: %s\nSpecify one: dva compose <name> [args...]",
			strings.Join(names, ", "))
	},
}

var upCmd = &cobra.Command{
	Use:   "up [PLAN] [OPTIONS]",
	Short: "Start services (a named plan, or the whole stack)",
	Long: `Start a named plan when plans are configured.
Without a plan name it uses default_plan, or the only plan when exactly one
is declared. With several plans and no default_plan it refuses and asks you
to name one. With no plans configured it starts every declared stack entry.

A leading -- is a separator, never an argument: "dva up --" does exactly what a
bare "dva up" does in that config, and "dva up -- X" is read exactly as
"dva up X" -- including when that is a refusal. So a wrapper written as
'dva up -- "$@"' is safe when "$@" is empty. Only the LEADING one is consumed:
"dva up -- --" is still an error, and a -- written after something else is left
alone.

Plan usage:
  dva up <plan>           Start the selected plan
  --force                 Compose only: pass --force-recreate (other plugins ignore)
  --no-wait               Return without waiting for readiness
  --var KEY=VAL           Override a plan variable
  --dry-run               Print the variable resolution and the actions, without executing

Stack flags:
  --force                   Compose only: pass --force-recreate (other plugins ignore)
  --no-wait                 Start services and return immediately without waiting
  --mode, -M MODE           Use a named mode from dva.yml modes section
  --env, -E ENV             Use a named environment from dva.yml environments section
  --tag, -T TAG[,TAG]       Include only lifecycle entries matching any of the given tags
  --exclude-tag TAG[,TAG]   Exclude lifecycle entries matching any of the given tags

Plan-path flags (only when a plan is being run, e.g. 'dva up <plan>'):
  --var KEY=VAL             Override a plan variable. Ignored off the plan path.`,
	Example: `  dva up                            # Start default_plan, or the only plan, or the whole stack
  dva up local-dev                  # Start a named plan
  dva up local-dev --force          # Force-recreate a named plan
  dva up local-dev --var PORT=8080  # Override a plan variable
  dva up --dry-run                  # Print what would run without executing`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		el := rootEnvLoad(c)
		if planName, extraArgs, ok := detectPlanRoute(c, args); ok {
			return runPlanUp(c, el, planName, extraArgs)
		}
		// Root-owned route from here: no plan claimed the invocation, so the root's own
		// env-input report governs and this fails closed before any hook or backend child.
		if err := el.report.Err(); err != nil {
			return err
		}
		e := el.env
		// A LEADING `--` is a separator: `dva up --` ≡ `dva up`, and `dva up -- X` ≡ `dva up X`.
		// TASK-216 widened TASK-207's restart-local identity to up/down/stop; the argument, and
		// what keeping it restart-local cost, is written at dropFlagTerminator (selectors.go).
		//
		// "Leading" and not "the first one anywhere", which is load-bearing in both directions.
		// `dva up --debug --` keeps its terminator and is still refused — that one is not at
		// args[0]. And the identity is false at X = `--`: `dva up -- --` is `unknown flag "--"`
		// while `dva up --` starts the stack, because exactly one is consumed and `up` takes no
		// arguments. That is not an exception to the rule; it is what a separator means, and
		// `restart` behaves the same way (`dva restart -- -- s1` refuses).
		//
		// It has to go HERE — above requirePlanSelection — rather than beside the flag guard
		// that used to refuse the token. detectPlanRoute returns ok=false for "no plans" before
		// it reaches its own dropLeadingTerminator, and for "several plans, none named" as well,
		// so the terminator survives into both, and requirePlanSelection is the first guard to
		// read it: it counts one surviving token as "the user named something" and lets the
		// several-plans config through, where a bare `dva up` is refused. Dropping any later
		// leaves that row diverging.
		//
		// What the guards see is unchanged: `dva up -- X` reaches rejectSuppressedDefaultPlan,
		// rejectUpPositionalArg and rejectUnknownFlags with exactly the args `dva up X` reaches
		// them with. What that does NOT mean is "only the empty case moved" — an earlier draft
		// of this comment said so and the review refuted it. `dva up -- -` went rc=1
		// (`unknown flag "--"`) to rc=0 starting the whole stack, and so did `dva up -- --debug`,
		// because `dva up -` and `dva up --debug` were ALREADY rc=0: rejectUnknownFlags is
		// reached only after parseDvaFlags has consumed the token. So the identity is applied
		// faithfully and it inherits whatever `dva up X` does, including where that is wrong.
		// `dva up -` accepted a bare dash when this was written, so the identity inherited that
		// too. TASK-218 has since settled it the other way — isFlagToken reads a lone `-` as a
		// name and rejectUpPositionalArg reports it — and both spellings now refuse together.
		// The identity held across that change without this line being touched, which is what
		// it is for. down/stop reach `-` by another route: teardownCommon still refuses it as an
		// unknown FLAG, wording TASK-218 deliberately left alone.
		args = dropLeadingTerminator(args)
		if err := requirePlanSelection(c, "up", args); err != nil {
			return err
		}
		if err := rejectSuppressedDefaultPlan(c, "up", args); err != nil {
			return err
		}
		if err := rejectUpPositionalArg(c, args); err != nil {
			return err
		}

		mode, envName, includeTags, excludeTags, args, err := parseDvaFlags(args)
		if err != nil {
			return err
		}
		mode, isDefault := applyDefaultMode(c, mode)

		force := false
		noWait := false
		var leftover []string
		for i := 0; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "--force":
				force = true
			case a == "--no-wait":
				noWait = true

			// --var belongs to the plan path (runPlanUp consumes it before this loop is
			// reached) and upCmd.Long documents it as "Ignored off the plan path". So it is a
			// known flag here, not an unknown one, and the guard below must not reject it —
			// TestUpWithoutPlansGuardOnlyInspectsPlanNameSlot pins that. Its VALUE has to be
			// consumed with it or `--var FOO=bare` would leave FOO=bare behind, which carries
			// no leading dash and would then be rejected as a stray positional argument.
			//
			// Ignoring it silently is the same shape as the defect this task is about, so it
			// says so on stderr. The exit code and the documented semantics are unchanged.
			case a == "--var":
				if i+1 < len(args) {
					i++
				}
				fmt.Fprintln(os.Stderr, "[warn] --var applies only when running a plan ('dva up <plan>'); ignored here")
			case strings.HasPrefix(a, "--var="):
				fmt.Fprintln(os.Stderr, "[warn] --var applies only when running a plan ('dva up <plan>'); ignored here")

			default:
				leftover = append(leftover, a)
			}
		}
		// Before TASK-113 this switch had no default and nothing followed it, so every
		// unrecognised token was discarded: `dva up --force=true` and `dva up --forse` both
		// ran as if no flag had been given and exited 0.
		//
		// Two guards, because `dva up` accepts no positional names either. rejectUpPositionalArg
		// already ran above, but only on args[0] — with a flag in front of it, as in
		// `dva up --force nosuchthing`, it returns nil at its leading-dash check and the name
		// reached here to be dropped. Measured: exit 0, the whole stack started, the argument
		// silently gone. (The example read `--dev` until that flag was removed with
		// `applications:` — it is an accepted flag that has to lead, and --force is the
		// nearest surviving one.)
		if err := rejectUnknownFlags("up", "", leftover, withSelectors([]string{"--force", "--no-wait", "--var"}, stackSelectorFlags), nil); err != nil {
			return err
		}
		if err := rejectUpPositionalArg(c, leftover); err != nil {
			return err
		}

		if err := applyEnv(e, c, envName); err != nil {
			return err
		}

		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Mode != nil {
			if isDefault {
				fmt.Fprintf(os.Stderr, "[mode: %s (default)] %s\n", mode, rm.Mode.Description)
			} else {
				fmt.Fprintf(os.Stderr, "[mode: %s] %s\n", mode, rm.Mode.Description)
			}
			if len(rm.Mode.Environment) > 0 {
				e.MergeVars(rm.Mode.Environment)
			}
			if rm.Mode.Provision != "" {
				suggestProvision(c, rm.Mode.Provision)
			}
		}

		// Phase 1: Stack up (infrastructure)
		orch := lifecycle.NewOrchestrator(c, e)
		upErr := orch.Up(context.Background(), lifecycle.UpOptions{
			DryRun:      dryRun,
			Force:       force,
			Wait:        !noWait,
			IncludeTags: includeTags,
			ExcludeTags: excludeTags,
			Mode:        mode,
			Env:         envName,
		})

		// Print status summary and endpoints regardless of up errors,
		// so users can see connection info for services that did start.
		fmt.Fprintln(os.Stderr)
		status, statusErr := orch.Status(context.Background())
		if statusErr == nil {
			lifecycle.PrintStatus(status, c.FileDir())
		}
		if len(c.Endpoints) > 0 {
			allHC := checkEndpointHealth(c.Endpoints)
			var epTags []string
			if rm.Mode != nil {
				epTags = rm.Mode.EndpointTags
			}
			printEndpointTable(c.Endpoints, epTags, allHC)
		}

		// Returned here rather than at the call site so the status and endpoint tables
		// above still print first. A user whose stack half-failed wants the connection
		// details of what did come up.
		//
		// This was errors.Join(upErr, appErr) until `applications:` was removed. The app
		// half was the reason the join existed: appErr had been swallowed into
		// "[warn] app start: %v", so TASK-117's readiness fix inside StartApps could not
		// reach the exit code and `dva up && next-step` chained on a failed start. Only
		// upErr remains, and orch.Up already returns it unswallowed.
		return upErr
	},
}

// teardownCommon resolves mode, applies env, and returns the parsed flags
// for both down and stop commands. verb is "down" or "stop" for error messages.
func teardownCommon(args []string, verb string) (*config.Config, *config.Environment, string, []string, []string, error) {
	c := mustLoadConfig()
	e, envReport := loadEnv(c)

	// Teardown fails closed exactly like startup. Tearing down with the wrong
	// environment resolves the wrong resource identity, and cleaning up someone
	// else's resources is worse than refusing to clean up at all (TASK-247 §4).
	// This precedes every hook, marker removal and backend child below.
	if err := envReport.Err(); err != nil {
		return nil, nil, "", nil, nil, err
	}

	mode, envName, includeTags, excludeTags, remaining, err := parseDvaFlags(args)
	if err != nil {
		return nil, nil, "", nil, nil, err
	}
	mode, isDefault := applyDefaultMode(c, mode)

	if len(remaining) > 0 {
		// A leftover that starts with "-" is a flag, not a service name, and quoting it into
		// the suggestion produced advice that cannot work: `dva down --bogus` used to answer
		// "Use 'dva stack down --bogus'", which failed the same way for the same reason. Only
		// the name-shaped case gets the selective-teardown hint. TASK-172.
		//
		// The hint's destination changed with `dva stack`: selective teardown is naming a plan,
		// which detectPlanRoute would already have taken if the argument were one. Reaching
		// here means it is not, so the message says what to name rather than quoting the word
		// back — a plan name is not derivable from a service name.
		if strings.HasPrefix(remaining[0], "-") {
			return nil, nil, "", nil, nil, fmt.Errorf("unknown flag %q for \"dva %s\"\n       → 'dva %s' takes no service names or flags of its own; it %ss everything declared",
				remaining[0], verb, verb, verb)
		}
		return nil, nil, "", nil, nil, fmt.Errorf("'dva %s' %ss all declared entries. Name a plan instead — 'dva %s <plan>' %ss just that plan's entries, and 'dva ls' lists the plans this config declares",
			verb, verb, verb, verb)
	}

	if err := applyEnv(e, c, envName); err != nil {
		return nil, nil, "", nil, nil, err
	}

	rm, err := resolveMode(c, mode)
	if err != nil {
		return nil, nil, "", nil, nil, err
	}
	if rm.Mode != nil {
		if isDefault {
			fmt.Fprintf(os.Stderr, "[mode: %s (default)]\n", mode)
		} else {
			fmt.Fprintf(os.Stderr, "[mode: %s]\n", mode)
		}
		if len(rm.Mode.Environment) > 0 {
			e.MergeVars(rm.Mode.Environment)
		}
	}

	return c, e, mode, includeTags, excludeTags, nil
}

var downCmd = &cobra.Command{
	Use:   "down [PLAN] [OPTIONS]",
	Short: "Stop and remove services (a named plan, or the whole stack)",
	Long: `Stop and remove a named plan.
Without a plan name it uses default_plan, or the only plan when exactly one
is declared. With several plans and no default_plan it refuses and asks you
to name one. With no plans configured it tears down every declared stack entry.

A leading -- is a separator, never an argument: "dva down --" does exactly what a
bare "dva down" does in that config, and "dva down -- X" is read exactly as
"dva down X" -- including when that is a refusal. So a wrapper written as
'dva down -- "$@"' is safe when "$@" is empty. Only the LEADING one is consumed:
"dva down -- --" is still an error, and a -- written after something else is left
alone.

Plan usage:
  dva down <plan>         Tear down the selected plan
  --var KEY=VAL           Override a plan variable
  --volumes, -v           Also remove volumes
  --purge                 Also remove volumes, locally built images and provision markers.
                          Asks for confirmation first; --force answers it.
  --dry-run               Print the variable resolution and the actions, without executing

Stack flags:
  --mode, -M MODE           Use a named mode from dva.yml modes section
  --env, -E ENV             Use a named environment from dva.yml environments section
  --tag, -T TAG[,TAG]       Include only lifecycle entries matching any of the given tags
  --exclude-tag TAG[,TAG]   Exclude lifecycle entries matching any of the given tags

Plan-path flags (only when a plan is being run, e.g. 'dva down <plan>'):
  --var KEY=VAL             Override a plan variable. Ignored off the plan path.
  --volumes, -v             Also remove volumes. Rejected off the plan path.
  --purge                   Also remove volumes, locally built images and provision
                            markers. Rejected off the plan path.`,
	Example: `  dva down                            # Tear down default_plan, or the only plan, or the whole stack
  dva down local-dev                  # Tear down a named plan
  dva down local-dev -v               # Also remove volumes
  dva down local-dev --purge --force  # Remove volumes, images and provision markers; skip the prompt`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		el := rootEnvLoad(c)
		if planName, extraArgs, ok := detectPlanRoute(c, args); ok {
			return runPlanDown(c, el, planName, extraArgs)
		}
		// `dva down --` ≡ `dva down`. TASK-216; the ruling and its cost are at
		// dropFlagTerminator (selectors.go), the placement argument at `up` above.
		//
		// down refuses a surviving terminator through a different door than up — teardownCommon's
		// own dash test, not rejectUnknownFlags — and the fix is deliberately not applied at
		// either door. Consuming the separator once, before the guards, is what stops the table
		// looking arbitrary; a third classifier that knows about `--` is what TASK-216 asked not
		// to add. A token after the terminator still reaches teardownCommon and is still refused
		// there, flag-shaped or name-shaped alike.
		args = dropLeadingTerminator(args)
		if err := requirePlanSelection(c, "down", args); err != nil {
			return err
		}
		if err := rejectSuppressedDefaultPlan(c, "down", args); err != nil {
			return err
		}

		c, e, mode, includeTags, excludeTags, err := teardownCommon(args, "down")
		if err != nil {
			return err
		}
		_, envName, _, _, _, err := parseDvaFlags(args)
		if err != nil {
			return err
		}

		orch := lifecycle.NewOrchestrator(c, e)
		return orch.Down(context.Background(), lifecycle.DownOptions{
			DryRun:      dryRun,
			IncludeTags: includeTags,
			ExcludeTags: excludeTags,
			Mode:        mode,
			Env:         envName,
		})
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop [PLAN] [OPTIONS]",
	Short: "Stop services without removing them (a named plan, or the whole stack)",
	Long: `Stop a named plan without removing its resources.
Without a plan name it uses default_plan, or the only plan when exactly one
is declared. With several plans and no default_plan it refuses and asks you
to name one. With no plans configured it stops every declared stack entry.

A leading -- is a separator, never an argument: "dva stop --" does exactly what a
bare "dva stop" does in that config, and "dva stop -- X" is read exactly as
"dva stop X" -- including when that is a refusal. So a wrapper written as
'dva stop -- "$@"' is safe when "$@" is empty. Only the LEADING one is consumed:
"dva stop -- --" is still an error, and a -- written after something else is left
alone.

Plan usage:
  dva stop <plan>         Stop the selected plan without removing resources
  --var KEY=VAL           Override a plan variable
  --dry-run               Print the variable resolution and the actions, without executing

Stack flags:
  --mode, -M MODE           Use a named mode from dva.yml modes section
  --env, -E ENV             Use a named environment from dva.yml environments section
  --tag, -T TAG[,TAG]       Include only lifecycle entries matching any of the given tags
  --exclude-tag TAG[,TAG]   Exclude lifecycle entries matching any of the given tags

Plan-path flags (only when a plan is being run, e.g. 'dva stop <plan>'):
  --var KEY=VAL             Override a plan variable. Ignored off the plan path.`,
	Example: `  dva stop                      # Stop default_plan, or the only plan, or the whole stack
  dva stop local-dev            # Stop a named plan without removing resources
  dva stop local-dev --dry-run  # Print what would stop, without executing`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		el := rootEnvLoad(c)
		if planName, extraArgs, ok := detectPlanRoute(c, args); ok {
			return runPlanStop(c, el, planName, extraArgs)
		}
		// `dva stop --` ≡ `dva stop`. TASK-216; same ruling, same placement argument, and the
		// same teardownCommon door as `down` above — stop and down share that helper, but not
		// this line, because each RunE reads the raw args through its own plan guards first.
		args = dropLeadingTerminator(args)
		if err := requirePlanSelection(c, "stop", args); err != nil {
			return err
		}
		if err := rejectSuppressedDefaultPlan(c, "stop", args); err != nil {
			return err
		}

		c, e, mode, includeTags, excludeTags, err := teardownCommon(args, "stop")
		if err != nil {
			return err
		}
		_, envName, _, _, _, err := parseDvaFlags(args)
		if err != nil {
			return err
		}

		orch := lifecycle.NewOrchestrator(c, e)
		return orch.Stop(context.Background(), lifecycle.StopOptions{
			DryRun:      dryRun,
			IncludeTags: includeTags,
			ExcludeTags: excludeTags,
			Mode:        mode,
			Env:         envName,
		})
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart [PLAN | ENTRY...] [OPTIONS]",
	Short: "Restart services (stop + start)",
	Long: `Restart a named plan (stop followed by start).
Without a plan name it uses default_plan, or the only plan when exactly one
is declared. With several plans and no default_plan it refuses and asks you
to name one. With no plans configured it restarts every declared stack entry.

The first argument is read as a plan name when it names a plan, and as a stack
entry name otherwise; the two cannot be combined.

Plan usage:
  dva restart <plan>      Restart the selected plan
  --var KEY=VAL           Override a plan variable (plan only)
  --no-wait               Return without waiting for readiness (plan only)
  --dry-run               Print the variable resolution and the actions, without executing

--var and --no-wait apply only when a plan is being restarted. On the stack path
they are rejected rather than accepted and ignored: it always waits, and there
are no plan variables to override.

Stack usage:
  dva restart <entry>     Restart only the named stack entries (works in any config)
  dva restart -- <name>   Read what follows as names, never as flags

A name matching no declared stack entry is an error. That rule is restart's own,
not a parity with its siblings: restart is the only lifecycle verb taking stack
entry names at all, up reads a positional as a plan name and rejects a real entry
with "plan not found", and down and stop refuse positionals outright. After --
every argument is a name whatever it spells, so a flag written there is reported
as an unknown name rather than silently dropped.

A bare "dva restart --" means "no names given" and does whatever a bare
"dva restart" does in that config, with no exceptions: it restarts every declared
entry where a bare restart does, it runs the default plan where a bare restart
runs it, and it refuses to guess where several plans are configured. Wrapper
scripts should know that "dva restart -- $@" with an empty "$@" restarts
everything in a plan-less config; it was a no-op before TASK-207. A config with a
resolvable default plan — an explicit default_plan, or a lone plan, which counts
as one — was the last exception, and TASK-210 removed it: the terminator is a
separator, so what follows it is classified, never the separator itself.

Stack flags:
  --mode, -M MODE           Use a named mode from dva.yml modes section
  --env, -E ENV             Use a named environment from dva.yml environments section
  --tag, -T TAG[,TAG]       Include only lifecycle entries matching any of the given tags
  --exclude-tag TAG[,TAG]   Exclude lifecycle entries matching any of the given tags`,
	Example: `  dva restart              # Restart default_plan, or the only plan, or the whole stack
  dva restart local-dev    # Restart a named plan
  dva restart api web      # Restart only these stack entries (no plan route)
  dva restart -- --oddname # Everything after -- is read as a name, never a flag`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		el := rootEnvLoad(c)
		if planName, extraArgs, ok := detectPlanRoute(c, args); ok {
			return runPlanRestart(c, el, planName, extraArgs)
		}
		// Root-owned route from here: no plan claimed the invocation, so the root's own
		// env-input report governs and this fails closed before any hook or backend child.
		if err := el.report.Err(); err != nil {
			return err
		}
		e := el.env
		if err := requirePlanSelection(c, "restart", args); err != nil {
			return err
		}
		if err := rejectSuppressedDefaultPlan(c, "restart", args); err != nil {
			return err
		}

		mode, envName, includeTags, excludeTags, names, err := parseDvaFlags(args)
		if err != nil {
			return err
		}
		// Everything parseDvaFlags did not recognise arrives here as a service name, and
		// DisableFlagParsing means cobra never vets it. Measured: `dva restart --no-wat`
		// matched no entry, restarted nothing and exited 0, reporting only `[warn] no
		// lifecycle entries matched filters` — the same shape TASK-113 closed for `up`.
		// The other three lifecycle verbs already reject a leftover flag; restart is the
		// only one taking flags AND positional names, so it needs up's allowlist form
		// (:168) rather than teardownCommon's wholesale refusal (:261). TASK-198.
		//
		// The advertised list is stackSelectorFlags alone. --var and --no-wait appear in
		// this command's help under "Plan usage" and runPlanRestart consumes them there.
		// Here they are unknown, and the reason is the path rather than the config: this
		// line is reached whenever no plan was selected, and then Wait is hardcoded true
		// below and there is no plan whose variables --var could override. Naming them in
		// "accepted here" would advertise a flag this path then ignores, which is what
		// rejectUnknownFlags' contract forbids. They are rejected with the rest instead of
		// silently swallowed; whether they should warn and continue, as `dva up` does for
		// --var, is a separate ruling this card does not make.
		//
		// Do NOT restate that as "the config declares no plans at all" — measured false.
		// requirePlanSelection returns nil as soon as planRoutingArgs leaves anything
		// behind, and planRoutingArgs strips only --debug and --json, so a leading FLAG
		// counts as something left behind. With two plans and no default_plan,
		// `dva restart --no-wait` and `dva restart s1 --zzznonsense` both land here with
		// plans configured. The second is the worse half of the defect this guard closes:
		// before it, that invocation restarted s1 and exited 0 having silently discarded
		// the argument, rather than merely doing nothing.
		//
		// The `--` terminator is exempt from the FLAG check, and it is the one place restart
		// must not copy up. parseDvaFlags keeps the terminator deliberately so each caller can
		// rule on it, and up rejects a stray one because it takes no positional names at all.
		// restart does take them, so `dva restart -- s1` is the ordinary way to say s1 is a
		// name and not a flag — measured working before this guard, and rc=1 `unknown flag
		// "--"` with an unconditional check, which the card's "no change to which flags restart
		// accepts" forbids. Only what precedes the terminator is flag-checked.
		guarded := names
		if i := slices.Index(names, "--"); i >= 0 {
			guarded = names[:i]
		}
		if err := rejectUnknownFlags("restart", "a stack entry name", guarded, stackSelectorFlags, []string{"--no-wait", "--var"}); err != nil {
			return err
		}

		// TASK-207 — the unmatchable-name ruling TASK-198 deferred to here, for `--`, `-`, a
		// bad flag after the terminator, and a typo'd entry name alike. 198 left all four
		// exiting 0 on purpose: deciding them inside a flag-rejection card would have settled
		// four cases by accident. They are one class, because after `--` every token is a name
		// whatever it spells, so a single name-shaped check answers all four.
		//
		// Consuming the terminator has to come first, and it is what makes rejecting the rest
		// safe. 198's comment argued the token had to stay in names, reasoning that an empty
		// Names means "every entry" to lifecycle, so dropping it would escalate `dva restart --`
		// from a no-op into a stop+start of the whole stack. The premise is right and the
		// conclusion does not survive the next line: with unknown names rejected, keeping it
		// makes `dva restart --` exit 1 instead — and `dva restart -- "$@"` with an empty "$@"
		// is the exact idiom `--` exists for, whose meaning is "no names given", i.e. exactly
		// what a bare `dva restart` does. Dropping it is what preserves that; the escalation
		// 198 measured is the correct behaviour once the token means "separator" here.
		//
		// Emptying the name list re-opens a gate that has already run. requirePlanSelection is
		// called above against the RAW args, where ["--"] counts as "a token was given", so a
		// config with several plans and no default_plan lets it through — and dropping the token
		// then leaves zero names, which lifecycle reads as "every entry". Measured, not reasoned
		// about: `dva restart --` stopped and started the whole stack in a config whose bare
		// `dva restart` is refused as too ambiguous to act on. That is 198's escalation arriving
		// from the other side, so the gate is re-applied. Found by review; the test above
		// asserted the divergence as correct.
		//
		// It is re-applied on the RAW args, not on the empty name list, and the difference is
		// the whole guard. An empty `names` is not the terminator's signature: `dva restart
		// --tag web` empties it too, and so does every other stack selector. A first draft
		// gated on `len(names) == 0` alone and refused all of them — measured, `restart --tag
		// web` went from rc=0 bouncing s1 on master to rc=1 "multiple plans configured", while
		// `up --tag web` and `stop --tag web` kept working, making restart the one lifecycle
		// verb whose tag filter needs a plan name. `restart --mode dev` stopped reporting the
		// unknown mode and reported the plan gate instead. The raw-args gate above has already
		// ruled on flag-only invocations — any surviving token means "do not ask for a plan" —
		// and that ruling stands. Only an invocation whose every token was the terminator means
		// "no names given", so that is what this asks: strip --debug/--json exactly as the gate
		// itself does, drop the terminator, and require nothing else to be left.
		//
		// The identity this restores is with the STACK route. The PLAN route was decided one
		// card later: a config with a default_plan refused `dva restart --` earlier still, from
		// rejectSuppressedDefaultPlan, because args[0] starts with "-". TASK-210 consumed the
		// leading terminator in detectPlanRoute and in that helper instead, which is why this
		// gate now sees an empty list here rather than being unreachable in that shape.
		//
		// Checked against SortedStack, the DECLARED entries, not the post-filter selection —
		// a name that exists but is excluded by --tag selects nothing legitimately and keeps
		// its warning. See rejectUnknownEntryNames.
		names = dropFlagTerminator(names)
		if len(dropFlagTerminator(planRoutingArgs(args))) == 0 {
			if err := requirePlanSelection(c, "restart", nil); err != nil {
				return err
			}
		}
		declared := make([]string, 0, len(c.Stack))
		for _, entry := range c.SortedStack() {
			declared = append(declared, entry.Name)
		}
		if err := rejectUnknownEntryNames("restart", "stack entry", names, declared); err != nil {
			return err
		}
		mode, isDefault := applyDefaultMode(c, mode)

		if err := applyEnv(e, c, envName); err != nil {
			return err
		}

		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Mode != nil {
			if isDefault {
				fmt.Fprintf(os.Stderr, "[mode: %s (default)]\n", mode)
			} else {
				fmt.Fprintf(os.Stderr, "[mode: %s]\n", mode)
			}
			if len(rm.Mode.Environment) > 0 {
				e.MergeVars(rm.Mode.Environment)
			}
		}

		orch := lifecycle.NewOrchestrator(c, e)
		return orch.Restart(context.Background(), lifecycle.UpOptions{
			DryRun:      dryRun,
			Force:       true,
			Wait:        true,
			Names:       names,
			IncludeTags: includeTags,
			ExcludeTags: excludeTags,
			Mode:        mode,
			Env:         envName,
		})
	},
}

var buildCmd = &cobra.Command{
	Use:   "build [PLAN] [ENTRY] [OPTIONS] [SERVICE...]",
	Short: "Build a plan's entries (or compose services)",
	Long: `Build what a named plan runs, one of its entries, or compose services.

Plan usage:
  dva build <plan>           Build every entry of the plan that has something to build
  dva build <plan> <entry>   Build one entry of the plan

Compose entries run 'docker compose build'; native entries run their
'runners.native.build' command in the entry's directory, with the entry's
variables. Entries with neither are skipped, not reported as failures.

Everything after the plan and entry names is passed to whatever does the
building — '--no-cache', '--pull' and service names reach docker compose
unchanged. A native build command is run as written and takes no extra
arguments. With more than one entry to build there is nothing to pass them to,
so name the entry first.

Without plans, or when the first argument is not a plan name, this stays a
mode-aware compose passthrough: 'dva build api' still means the 'api' service.`,
	Example: `  dva build                # Build default_plan, or the only plan, or every declared entry
  dva build local-dev      # Build every entry of a named plan that has something to build
  dva build local-dev api  # Build one entry of a plan
  dva build api --no-cache # Compose passthrough: build the api service`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		el := rootEnvLoad(c)

		// remaining is docker's argv from here on — `dva build --no-cache` has to reach
		// docker, so nothing downstream can tell a malformed DVA flag from a valid docker
		// one. parseDvaFlags is the last code that can, and now does. TASK-172.
		mode, _, _, _, remaining, err := parseDvaFlags(args)
		if err != nil {
			return err
		}
		// Consume build's own leading separator before plan detection. The shared helper below
		// deliberately consumes only the plan-name slot; a second terminator remains backend
		// argv, so `build -- -- --` still reaches compose as `build -- --`.
		remaining = dropLeadingTerminator(remaining)

		// Plan routing reads what parseDvaFlags left, so --dry-run and --mode are already
		// claimed and the plan-name slot holds a name or a tool's flag. logsCmd calls
		// consumeRootPersistentFlags at this point instead; here parseDvaFlags has done that
		// job and more, and calling both would walk the same argv twice.
		if planName, extraArgs, ok := detectPlanRoute(c, remaining); ok {
			return runPlanBuild(c, el, planName, extraArgs)
		}
		// Root-owned route from here: no plan claimed the invocation, so the root's own
		// env-input report governs and this fails closed before any hook or backend child.
		if err := el.report.Err(); err != nil {
			return err
		}
		e := el.env
		if err := requirePlanSelection(c, "build", remaining); err != nil {
			return err
		}
		if err := rejectSuppressedDefaultPlan(c, "build", remaining); err != nil {
			return err
		}
		// No rejectUnknownPlanArg, as in logs: `dva build api` naming a compose service
		// predates plans and is still what most of that argument's uses mean.

		mode, _ = applyDefaultMode(c, mode)

		// Check mode build strategy
		if mode != "" {
			if m, ok := c.Modes[mode]; ok && m.Build != "" {
				switch m.Build {
				case "docker":
					return execComposePassthrough(e, c, append([]string{"build"}, remaining...))
				case "native":
					// One executor, not two. This branch and wrapWithHooks fire on the same
					// len(ic.Replace) > 0 condition and the wrapper always wins, so the only way
					// in is DVA_HOOK_DEPTH>0 — `dva build` invoked from inside another hook step,
					// where the wrapper defers to the original RunE. The second copy that used to
					// live here rendered the same steps differently (stdout instead of stderr,
					// four-space instead of two, note before the commands instead of after) and,
					// because it never consulted dryRun, `dva build --dry-run --mode <native>`
					// executed for real once nested. Delegating settles all of it, and brings the
					// compose keys — which the copy did not implement — to the nested path.
					// TASK-093.
					if ic, ok := c.Interaction["build"]; ok && len(ic.Replace) > 0 {
						return runHookSteps(e, c, "replace", "build", ic.Replace)
					}
					return fmt.Errorf("mode %q build=native but no interaction.build.replace defined", mode)
				default:
					// Custom build command
					fmt.Printf("  $ %s\n", m.Build)
					return runShellCommand(e, m.Build)
				}
			}
		}

		return execComposePassthrough(e, c, append([]string{"build"}, remaining...))
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs [PLAN] [ENTRY] [OPTIONS] [SERVICE...]",
	Short: "View output from a plan's entries (or from compose services)",
	Long: `Show logs for a named plan, one of its entries, or compose services.

Plan usage:
  dva logs <plan>           Logs for the plan's only log-producing entry
  dva logs <plan> <entry>   Logs for one entry of the plan

Everything after the plan and entry names is passed to whatever owns the logs —
'-f', '--tail 50' and service names reach docker compose unchanged. Entries that
run as a process or a script are read from their log file instead, which cannot
follow, so those take no extra arguments.

Without plans, or when the first argument is not a plan name, this stays a
compose passthrough: 'dva logs api' still means the 'api' service.`,
	Example: `  dva logs                # Logs for default_plan, or the only plan, or the whole stack
  dva logs local-dev      # Logs for a named plan's log-producing entry
  dva logs local-dev api  # Logs for one entry of a plan
  dva logs api -f         # Compose passthrough: follow the api service's logs`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		el := rootEnvLoad(c)
		// TASK-092, third site: `dva --debug logs` sent --debug on to docker as a flag of
		// `compose logs`.
		args, err := consumeRootPersistentFlags(args)
		if err != nil {
			return err
		}
		if planName, extraArgs, ok := detectPlanRoute(c, args); ok {
			return runPlanLogs(c, el, planName, extraArgs)
		}
		// Observation, not execution: nothing reaches stdout and no compose child or
		// log file is read. The diagnostic target is the literal word `stack` whatever
		// trailing argv follows, because DVA does not guess a service name out of raw
		// passthrough argv (TASK-247 §5).
		if el.report.Incomplete() {
			return fmt.Errorf("logs not queried for stack: environment inputs are incomplete")
		}
		e := el.env
		if err := requirePlanSelection(c, "logs", args); err != nil {
			return err
		}
		if err := rejectSuppressedDefaultPlan(c, "logs", args); err != nil {
			return err
		}
		// No rejectUnknownPlanArg here, unlike up/down/stop/restart. Their positional slot
		// means a plan and nothing else, so an unmatched name is a typo. This one has a
		// second legitimate occupant — `dva logs api` naming a compose service predates
		// plans and still works — and rejecting it would break that to catch a misspelling.
		return execComposePassthrough(e, c, append([]string{config.LogsDirName}, args...))
	},
}

// parseDvaFlags extracts --mode/-M, --env/-E, --tags/-T, and --exclude-tags from args.
// It also consumes root persistent --dry-run/--debug/--json because callers set
// DisableFlagParsing and cobra therefore never parses them for them. --debug/--json
// are also pre-parsed in PersistentPreRun for logger.Init; stripping them here
// prevents them from being treated as entry/service names.
func parseDvaFlags(args []string) (mode, env string, includeTags, excludeTags []string, filtered []string, err error) {
	// --dry-run is handled in the switch below, not by a consumeDryRunFlag pre-pass. The
	// pre-pass was a second walk that did the same thing, and after TASK-145 it would have
	// been a walk with its own idea of where the `--` terminator is.
	end := dvaFlagEnd(args)

	// A malformed boolean value (`--debug=notabool`) is rejected here rather than passed down
	// in filtered. It used to fall through "for the caller's own rejectUnknownFlags to name",
	// which most call sites do not have; `dva build` instead appended it to docker's argv. No
	// caller can take over this job, because a passthrough command must forward the flags it
	// does not recognise — this is the last code that knows `--debug` is DVA's. TASK-172.
	//
	// The first bad flag wins: reporting one is what the user has to fix first. That is what
	// the `if err == nil` guards below are for, and it is pinned by
	// TestParseDvaFlags_FirstBadFlagIsReported — a review deleted both guards, making the
	// last flag win, and nothing failed until that test existed. The loop then runs to the
	// end rather than returning early because a closure has no return to take — not because
	// anything reads the values it keeps filling in. Every caller checks err before touching
	// any other return value, so those values are never observed.
	//
	// Both sentences carried stale call-site counts until TASK-213. Neither was ever reread,
	// which is TASK-208's subject exactly. They are stated as properties now; the counts are one
	// command away and belong in a commit message rather than in a comment that outlives it:
	//   grep -n 'parseDvaFlags(args)' internal/cli/*.go | grep -v _test  # callers: 6 today
	//   grep -n 'rejectUnknownFlags(' internal/cli/*.go | grep -v _test | grep -v 'func '  # call sites: 2, upCmd + restartCmd
	//
	// The second command carried `# 2` for one commit while returning 3, because it also
	// matched the func declaration in selectors.go. A command written to retire a stale
	// count is worth running once before it ships.
	takeBool := func(name, value string, hasValue bool, target *bool) {
		if v, ok := flagBoolValue(value, hasValue); ok {
			*target = v
			return
		}
		if err == nil {
			err = fmt.Errorf("invalid boolean value %q for %s", value, name)
		}
	}

	// A value-taking flag with nothing to take is an error, reported here for the same
	// reason takeBool reports a malformed boolean: only this code knows a value was
	// required. flagValue stays a bool — what it cannot supply is the flag's name, and it
	// has no err to set. Note that its own doc comment used to justify the silence by
	// saying the helper is also used where taking the next token is optional; it is not.
	// Before this change flagValue had exactly four callers and they were the four cases
	// below, so the helper's neutrality was prospective rather than a current constraint.
	// It now has exactly one — this closure — which is the point of the change: the four
	// cases no longer touch it. Past tense on purpose. TASK-208 exists because five
	// comments went on quoting a call-site count a refactor had already changed, and the
	// first draft of this one said "grep says flagValue has exactly four callers" in the
	// present tense, in the very commit that made it one. TASK-211.
	//
	// Until then every case ignored ok=false, and a recognised flag is never appended to
	// filtered, so the token vanished: `dva restart --mode` ran the whole stack and
	// reported success — the widest possible result for someone who typed a narrowing
	// flag. Same for --env, --tag and --exclude-tag.
	//
	// "Nothing to take" includes a flag sitting just before the terminator, because
	// dvaFlagEnd puts end at the first `--`: in `--mode --` the flag is last as far as
	// flagValue is concerned even though a token follows. Both spellings have to error or
	// the fix closes one door and leaves its twin open — that pair is what TASK-207's
	// review was looking at when it filed this.
	// An empty value is refused here too, and for the same harm rather than for symmetry.
	// `--mode=` reaches flagValue's hasValue branch, which reports ok=true because a value
	// was supplied — so before TASK-213 mode was set to "", which is precisely what no
	// --mode at all leaves behind, applyDefaultMode then filled in the default, and the
	// narrowing flag produced the widest possible run at rc=0. `--tag=` is the mirror:
	// includeTags becomes [""], which matches nothing, so the run is empty and still
	// reported as success. Both spellings of emptiness are covered — `--mode=` through the
	// hasValue branch and `--mode ""` through the next-token branch — because a fix aimed
	// at the `=` spelling alone leaves its twin open, the same way TASK-211 had to take
	// `--mode --` along with `--mode`.
	//
	// Distinct wording on purpose: the user did supply a value, so "requires a value" would
	// describe a different mistake than the one they made. It also keeps the two branches
	// separable by test.
	//
	// Returning ok=false without consuming n means the empty next token in `--mode ""` is
	// The rejection paths return the token count they consumed, and the cases below advance
	// by it whether or not the value was accepted. That is not symmetry either: a rejected
	// `--mode ""` used to leave i where it was, so the loop re-read the empty token and
	// appended it to filtered — the value of a recognised flag reaching a passthrough
	// command's argv. It was unreachable, because err is set and every non-test caller of
	// parseDvaFlags (upCmd, teardownCommon, downCmd, stopCmd, restartCmd, buildCmd) checks it
	// on the line immediately after the call. "Unreachable" was the first draft of this
	// comment and a review was right to call it a latent invariant violation rather than a
	// property: it holds because of what six callers happen to do next, and buildCmd is one
	// reorder away from forwarding that "" to docker. flagValue returns consumed=0 on the
	// nothing-to-take branch, so advancing unconditionally is a no-op there and the fix costs
	// one line per case.
	takeValue := func(name, value string, hasValue bool, i int) (string, int, bool) {
		if !hasValue && i+1 < end && isRecognizedDVAFlagToken(args[i+1]) {
			if err == nil {
				err = fmt.Errorf("%s requires a value, got the flag %s", name, args[i+1])
			}
			return "", 1, false
		}
		v, n, ok := flagValue(args, i, end, value, hasValue)
		if !ok {
			if err == nil {
				err = fmt.Errorf("%s requires a value", name)
			}
			return v, n, ok
		}
		if v == "" {
			if err == nil {
				err = fmt.Errorf("%s requires a non-empty value", name)
			}
			return "", n, false
		}
		if strings.TrimSpace(v) == "" {
			if err == nil {
				err = fmt.Errorf("%s requires a non-blank value, got %q", name, v)
			}
			return "", n, false
		}
		return v, n, ok
	}

	// takeList is takeValue plus the rule the comma-separated flags need, and it exists
	// because the check above sits on the wrong side of the split to see the whole defect.
	// `--exclude-tag=,` is one character, so it passes as non-empty, and strings.Split turns
	// it into ["", ""] — two tags nothing carries. For --exclude-tag, matching nothing means
	// excluding nothing, so that spelling bounced the entire stack at rc=0: the same harm
	// TASK-211 and the check above were filed against, reached one character past the
	// spelling they refuse. `--tag=,` is the mirror and fails the other way, running nothing
	// and still exiting 0. Both were measured on the unfixed build, and the first draft of
	// TASK-213 examined only the second and called the family harmless.
	//
	// TrimSpace rather than == "" because a blank element is the same non-value written
	// differently — for the ~25 runes unicode.IsSpace calls space, which is the boundary
	// this draws and not the boundary "invisible" would draw. `--tag=<U+200B>` is a
	// zero-width space, is not IsSpace, and passes as a one-element list; it then matches
	// no declared tag, which lands it in TASK-214 with every other tag no entry carries.
	// Widening this check to "unprintable" would be a second, different rule about what a
	// tag may be made of, and it belongs with the code that validates tag names.
	//
	// Nothing is trimmed off the values that survive: an element with surrounding spaces is
	// a tag that will not match, which is an unknown-tag complaint rather than this one.
	// Rewriting the user's input on the way through would hide that from them.
	takeList := func(name, value string, hasValue bool, i int) ([]string, int, bool) {
		v, n, ok := takeValue(name, value, hasValue, i)
		if !ok {
			return nil, n, false
		}
		parts := strings.Split(v, ",")
		for _, p := range parts {
			if strings.TrimSpace(p) == "" {
				if err == nil {
					err = fmt.Errorf("%s requires non-empty tags, got %q", name, v)
				}
				return nil, n, false
			}
		}
		return parts, n, true
	}

	for i := 0; i < end; i++ {
		a := args[i]
		name, value, hasValue := splitFlagToken(a)
		switch name {
		case "--mode", "-M":
			v, n, ok := takeValue(name, value, hasValue, i)
			i += n
			if ok {
				mode = v
			}
		case "--env", "-E":
			v, n, ok := takeValue(name, value, hasValue, i)
			i += n
			if ok {
				env = v
			}
		case "--tag", "--tags", "-T":
			v, n, ok := takeList(name, value, hasValue, i)
			i += n
			if ok {
				includeTags = append(includeTags, v...)
			}
		case "--exclude-tag", "--exclude-tags":
			v, n, ok := takeList(name, value, hasValue, i)
			i += n
			if ok {
				excludeTags = append(excludeTags, v...)
			}
		// Callers set DisableFlagParsing, so cobra never parses the root
		// persistent --dry-run. Without this it falls through to filtered and
		// is read as an entry/service name, leaving dryRun false: `dva up
		// --dry-run` then executes for real.
		case "--dry-run":
			takeBool(name, value, hasValue, &dryRun)
		case "--debug":
			takeBool(name, value, hasValue, &debug)
		case "--json":
			takeBool(name, value, hasValue, &jsonOutput)
		default:
			filtered = append(filtered, a)
		}
	}
	// The terminator itself is kept, unlike in consumeRootPersistentFlags. This output is
	// still inside DVA, and the callers that reject unknown flags have always rejected a
	// stray `--` — dropping it here would newly accept it.
	filtered = append(filtered, args[end:]...)
	return
}

// consumeDryRunFlag strips --dry-run and reports whether it was set. Callers that also need
// --mode/--env/--tags use parseDvaFlags instead; this one exists for the paths that must
// leave every other flag where it is.
//
// It reports "not found" for an explicit `--dry-run=false` while still consuming the token.
// Every caller uses the result as `if found { dryRun = true }`, so absent and explicitly
// false already mean the same thing to them.
func consumeDryRunFlag(args []string) ([]string, bool) {
	end := dvaFlagEnd(args)
	filtered := make([]string, 0, len(args))
	found := false
	for i := range end {
		a := args[i]
		if name, value, hasValue := splitFlagToken(a); name == "--dry-run" {
			if v, ok := flagBoolValue(value, hasValue); ok {
				found = v
				continue
			}
		}
		filtered = append(filtered, a)
	}
	// Terminator kept: this feeds back into DVA (wrapWithHooks hands its result to the
	// built-in's own RunE, which parses flags again), so a later consumer still needs it.
	filtered = append(filtered, args[end:]...)
	return filtered, found
}

// applyEnv resolves and applies environment configuration from --env flag.
func applyEnv(e *config.Environment, c *config.Config, envName string) error {
	if envName == "" {
		return nil
	}
	ec, ok := c.Environments[envName]
	if !ok {
		available := make([]string, 0, len(c.Environments))
		for k := range c.Environments {
			available = append(available, k)
		}
		if len(available) == 0 {
			return fmt.Errorf("env '%s' not found. No environments defined in dva.yml under 'environments:'", envName)
		}
		return fmt.Errorf("env '%s' not found. Available: %s", envName, strings.Join(available, ", "))
	}
	fmt.Fprintf(os.Stderr, "[env: %s] %s\n", envName, ec.Description)
	if len(ec.Stack) > 0 {
		fmt.Fprintf(os.Stderr, "[env: %s] stack: %s\n", envName, strings.Join(ec.Stack, ", "))
	}
	if len(ec.Environment) > 0 {
		e.MergeVars(ec.Environment)
	}
	return nil
}

// resolvedMode holds the result of resolving a --mode flag against config modes.
type resolvedMode struct {
	Mode *config.ModeConfig
}

// applyDefaultMode returns the effective mode and whether it was applied from default_mode.
// It does NOT print any output — callers decide how to log.
func applyDefaultMode(c *config.Config, mode string) (string, bool) {
	if mode != "" || c.DefaultMode == "" {
		return mode, false
	}
	if _, ok := c.Modes[c.DefaultMode]; ok {
		return c.DefaultMode, true
	}
	fmt.Fprintf(os.Stderr, "[warn] default_mode '%s' not found in modes — starting all services\n", c.DefaultMode)
	return mode, false
}

// resolveMode looks up a mode name in config modes and returns resolved settings.
func resolveMode(c *config.Config, mode string) (*resolvedMode, error) {
	if mode == "" {
		return &resolvedMode{}, nil
	}

	m, ok := c.Modes[mode]
	if !ok {
		available := make([]string, 0, len(c.Modes))
		for k := range c.Modes {
			available = append(available, k)
		}
		if len(available) == 0 {
			return nil, fmt.Errorf("mode '%s' not found. No modes defined in dva.yml under 'modes:'", mode)
		}
		return nil, fmt.Errorf("mode '%s' not found. Available: %s", mode, strings.Join(available, ", "))
	}

	return &resolvedMode{
		Mode: &m,
	}, nil
}

// suggestProvision checks if a provision profile has been run before
// (via marker file in .sb/dva/) and prints a suggestion if not.
func suggestProvision(c *config.Config, provisionProfile string) {
	markerDir := filepath.Join(c.FileDir(), config.DotDirName)
	markerFile := filepath.Join(markerDir, provisionMarkerName(provisionProfile))

	if _, err := os.Stat(markerFile); err == nil {
		return // already provisioned
	}

	if _, ok := c.Provision.Profiles[provisionProfile]; !ok {
		return
	}

	fmt.Fprintf(os.Stderr, "\n[hint] Provision profile '%s' has not been run yet.\n", provisionProfile)
	fmt.Fprintf(os.Stderr, "       Run: dva provision %s\n\n", provisionProfile)
}

// execComposeSubprocess runs a docker compose command as a subprocess,
// returning control to the caller after completion.
func execComposeSubprocess(e *config.Environment, c *config.Config, args []string) error {
	composeCmd, composeArgs, err := buildComposeArgs(e, c, args)
	if err != nil {
		return err
	}

	if dvaexec.Debug {
		fmt.Fprintf(os.Stderr, "[debug] compose subprocess: %s %v\n", composeCmd, composeArgs)
	}

	return dvaexec.ExecSubprocess(e, composeCmd, composeArgs, false)
}

// execComposePassthrough builds and execs a docker compose command using config.
// When forceSubprocess is true (set by hook wrapper for after-hooks), it delegates
// to execComposeSubprocess so the Go process survives for post-command hooks.
func execComposePassthrough(e *config.Environment, c *config.Config, args []string) error {
	if forceSubprocess {
		return execComposeSubprocess(e, c, args)
	}

	composeCmd, composeArgs, err := buildComposeArgs(e, c, args)
	if err != nil {
		return err
	}

	if dvaexec.Debug {
		fmt.Fprintf(os.Stderr, "[debug] compose: %s %v\n", composeCmd, composeArgs)
	}

	return dvaexec.ExecReplace(e, composeCmd, composeArgs, false)
}

// execComposePassthroughForEntry runs docker compose against a specific stack entry.
func execComposePassthroughForEntry(e *config.Environment, c *config.Config, entry *config.LifecycleEntry, args []string) error {
	composeCmd, composeArgs, err := buildComposeArgsForEntry(e, c, entry, args)
	if err != nil {
		return err
	}

	if forceSubprocess {
		if dvaexec.Debug {
			fmt.Fprintf(os.Stderr, "[debug] compose subprocess [%s]: %s %v\n", entry.Name, composeCmd, composeArgs)
		}
		return dvaexec.ExecSubprocess(e, composeCmd, composeArgs, false)
	}

	if dvaexec.Debug {
		fmt.Fprintf(os.Stderr, "[debug] compose [%s]: %s %v\n", entry.Name, composeCmd, composeArgs)
	}
	return dvaexec.ExecReplace(e, composeCmd, composeArgs, false)
}

// buildComposeArgsForEntry builds docker compose arguments from a specific lifecycle entry.
func buildComposeArgsForEntry(e *config.Environment, c *config.Config, entry *config.LifecycleEntry, args []string) (string, []string, error) {
	composeCmd, composeArgs, err := dvaexec.ComposeArgv(e, entry.ComposeConfig(), c.FileDir())
	if err != nil {
		return "", nil, fmt.Errorf("entry %q: %w", entry.Name, err)
	}
	return composeCmd, append(composeArgs, args...), nil
}

// buildComposeArgs builds docker compose arguments using config settings.
// Returns the command and args that can be used with exec or shell.
func buildComposeArgs(e *config.Environment, c *config.Config, args []string) (string, []string, error) {
	composeCmd, composeArgs, err := dvaexec.ComposeArgv(e, c.PrimaryComposeConfig(), c.FileDir())
	if err != nil {
		return "", nil, err
	}
	return composeCmd, append(composeArgs, args...), nil
}
