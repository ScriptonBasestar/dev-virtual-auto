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
		e := loadEnv(c)

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
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		e := loadEnv(c)
		if planName, extraArgs, ok := detectPlanRoute(c, args); ok {
			return runPlanUp(c, e, planName, extraArgs)
		}
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
		if err := rejectUnknownFlags("up", "", leftover, withSelectors([]string{"--force", "--no-wait", "--var"}, stackSelectorFlags)); err != nil {
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
	e := loadEnv(c)

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
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		e := loadEnv(c)
		if planName, extraArgs, ok := detectPlanRoute(c, args); ok {
			return runPlanDown(c, e, planName, extraArgs)
		}
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
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		e := loadEnv(c)
		if planName, extraArgs, ok := detectPlanRoute(c, args); ok {
			return runPlanStop(c, e, planName, extraArgs)
		}
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
	Use:   "restart [PLAN | SERVICE...] [OPTIONS]",
	Short: "Restart services (stop + start)",
	Long: `Restart a named plan (stop followed by start).
Without a plan name it uses default_plan, or the only plan when exactly one
is declared. With several plans and no default_plan it refuses and asks you
to name one. With no plans configured it restarts every declared stack entry.

The first argument is read as a plan name when it names a plan, and as a stack
entry name otherwise; the two cannot be combined.

Plan usage:
  dva restart <plan>      Restart the selected plan
  --var KEY=VAL           Override a plan variable
  --no-wait               Return without waiting for readiness
  --dry-run               Print the variable resolution and the actions, without executing

Stack usage:
  dva restart <service>   Restart only the named entries (works in any config)

Stack flags:
  --mode, -M MODE           Use a named mode from dva.yml modes section
  --env, -E ENV             Use a named environment from dva.yml environments section
  --tag, -T TAG[,TAG]       Include only lifecycle entries matching any of the given tags
  --exclude-tag TAG[,TAG]   Exclude lifecycle entries matching any of the given tags`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		e := loadEnv(c)
		if planName, extraArgs, ok := detectPlanRoute(c, args); ok {
			return runPlanRestart(c, e, planName, extraArgs)
		}
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
		// The `--` terminator is exempt, and it is the one place restart must NOT copy up.
		// parseDvaFlags keeps the terminator deliberately so each caller can rule on it, and
		// up rejects a stray one because it takes no positional names at all. restart does
		// take them, so `dva restart -- s1` is the ordinary way to say s1 is a name and not
		// a flag — measured working before this guard and rc=1 "unknown flag \"--\"" with an
		// unconditional check, which is a regression the card's "no change to which flags
		// restart accepts" forbids. Everything after the terminator is a name by
		// construction, so only what precedes it is checked.
		guarded := names
		if i := slices.Index(names, "--"); i >= 0 {
			guarded = names[:i]
			names = append(names[:i:i], names[i+1:]...)
		}
		if err := rejectUnknownFlags("restart", "a stack entry name", guarded, stackSelectorFlags); err != nil {
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
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		e := loadEnv(c)

		// remaining is docker's argv from here on — `dva build --no-cache` has to reach
		// docker, so nothing downstream can tell a malformed DVA flag from a valid docker
		// one. parseDvaFlags is the last code that can, and now does. TASK-172.
		mode, _, _, _, remaining, err := parseDvaFlags(args)
		if err != nil {
			return err
		}

		// Plan routing reads what parseDvaFlags left, so --dry-run and --mode are already
		// claimed and the plan-name slot holds a name or a tool's flag. logsCmd calls
		// consumeRootPersistentFlags at this point instead; here parseDvaFlags has done that
		// job and more, and calling both would walk the same argv twice.
		if planName, extraArgs, ok := detectPlanRoute(c, remaining); ok {
			return runPlanBuild(c, e, planName, extraArgs)
		}
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
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		e := loadEnv(c)
		// TASK-092, third site: `dva --debug logs` sent --debug on to docker as a flag of
		// `compose logs`.
		args, err := consumeRootPersistentFlags(args)
		if err != nil {
			return err
		}
		if planName, extraArgs, ok := detectPlanRoute(c, args); ok {
			return runPlanLogs(c, e, planName, extraArgs)
		}
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
	// which only 7 of the 12 call sites have; `dva build` instead appended it to docker's
	// argv. No caller can take over this job, because a passthrough command must forward the
	// flags it does not recognise — this is the last code that knows `--debug` is DVA's.
	// TASK-172.
	//
	// The first bad flag wins: reporting one is what the user has to fix first. The loop then
	// runs to the end rather than returning early because a closure has no return to take —
	// not because anything reads the values it keeps filling in. All 12 callers check err
	// before touching any other return value, so those values are never observed.
	takeBool := func(name, value string, hasValue bool, target *bool) {
		if v, ok := flagBoolValue(value, hasValue); ok {
			*target = v
			return
		}
		if err == nil {
			err = fmt.Errorf("invalid boolean value %q for %s", value, name)
		}
	}

	for i := 0; i < end; i++ {
		a := args[i]
		name, value, hasValue := splitFlagToken(a)
		switch name {
		case "--mode", "-M":
			if v, n, ok := flagValue(args, i, end, value, hasValue); ok {
				mode = v
				i += n
			}
		case "--env", "-E":
			if v, n, ok := flagValue(args, i, end, value, hasValue); ok {
				env = v
				i += n
			}
		case "--tag", "--tags", "-T":
			if v, n, ok := flagValue(args, i, end, value, hasValue); ok {
				includeTags = append(includeTags, strings.Split(v, ",")...)
				i += n
			}
		case "--exclude-tag", "--exclude-tags":
			if v, n, ok := flagValue(args, i, end, value, hasValue); ok {
				excludeTags = append(excludeTags, strings.Split(v, ",")...)
				i += n
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
	markerFile := filepath.Join(markerDir, "provisioned-"+provisionProfile)

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
