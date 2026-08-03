package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	Short: "Start a named plan (or all declared entries)",
	Long: `Start a named plan when plans are configured.
Without plans, use the legacy stack and applications lifecycle.

	Plan usage:
	  dva up <plan>           Start the selected plan
	  --force                 Compose only: pass --force-recreate (other plugins ignore)
	  --no-wait               Return without waiting for readiness
	  --var KEY=VAL           Override a plan variable
	  --dry-run               Print the variable resolution and the actions, without executing

	Legacy flags:
	  --force                   Compose only: pass --force-recreate (other plugins ignore)
	  --no-wait                 Start services and return immediately without waiting
	  --dev                     Start applications in dev mode (hot-reload)
	  --docker                  Force docker strategy for applications
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
		devMode := false
		docker := false
		var leftover []string
		for i := 0; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "--force":
				force = true
			case a == "--no-wait":
				noWait = true
			case a == "--dev":
				devMode = true
			case a == "--docker":
				docker = true

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
		// `dva up --dev nosuchthing`, it returns nil at its leading-dash check and the name
		// reached here to be dropped. Measured: exit 0, the whole stack started, the argument
		// silently gone.
		if err := rejectUnknownFlags("up", "", leftover, withSelectors([]string{"--force", "--no-wait", "--dev", "--docker", "--var"}, stackSelectorFlags)); err != nil {
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

		// Phase 2: App up (applications) — only if applications are defined
		var appErr error
		if upErr == nil && len(c.Applications) > 0 {
			strategy := ""
			if docker {
				strategy = "docker"
			}
			am := lifecycle.NewAppManager(c, e)
			appErr = am.StartApps(context.Background(), lifecycle.AppStartOptions{
				Strategy: strategy,
				DevMode:  devMode,
				DryRun:   dryRun,
				Wait:     !noWait,
				Mode:     mode,
			})
		}

		// Print status summary and endpoints regardless of up errors,
		// so users can see connection info for services that did start.
		fmt.Fprintln(os.Stderr)
		status, statusErr := orch.Status(context.Background())
		if statusErr == nil {
			lifecycle.PrintStatus(status, c.FileDir())
		}
		if len(c.Applications) > 0 {
			am := lifecycle.NewAppManager(c, e)
			statuses := am.AppStatuses()
			printAppStatuses(statuses)
		}
		if len(c.Endpoints) > 0 {
			allHC := checkEndpointHealth(c.Endpoints)
			var epTags []string
			if rm.Mode != nil {
				epTags = rm.Mode.EndpointTags
			}
			printEndpointTable(c.Endpoints, epTags, allHC)
		}

		// Both, joined, and only after the tables have printed.
		//
		// appErr used to be swallowed into "[warn] app start: %v" here, which is why
		// TASK-117's fix to StartApps was not enough on its own: the app manager could
		// report every readiness failure it wanted and `dva up` still returned nil. A
		// wrapper script or a `dva up && next-step` chain saw success.
		//
		// It is returned here rather than at the call site so the status and endpoint
		// tables above still print — the same reason upErr waits until this line. A user
		// whose app failed to bind wants the connection details of what did come up.
		return errors.Join(upErr, appErr)
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
		// the suggestion produced advice that cannot work: `dva down --bogus` answered "Use
		// 'dva stack down --bogus'", which fails the same way for the same reason. Only the
		// name-shaped case gets the selective-teardown hint. TASK-172.
		if strings.HasPrefix(remaining[0], "-") {
			return nil, nil, "", nil, nil, fmt.Errorf("unknown flag %q for \"dva %s\"\n       → 'dva %s' takes no service names or flags of its own; it %ss everything declared",
				remaining[0], verb, verb, verb)
		}
		return nil, nil, "", nil, nil, fmt.Errorf("'dva %s' %ss all services. Use 'dva stack %s %s' or 'dva app %s %s' for selective %s",
			verb, verb, verb, remaining[0], verb, remaining[0], verb)
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
	Short: "Tear down a named plan (or all declared entries)",
	Long: `Stop and remove a named plan.
Without plans, use the legacy applications and stack lifecycle.

Plan usage:
  dva down <plan>         Tear down the selected plan
  --var KEY=VAL           Override a plan variable
  --volumes, -v           Also remove volumes
  --dry-run               Print the variable resolution and the actions, without executing

Legacy flags:
  --mode, -M MODE           Use a named mode from dva.yml modes section
  --env, -E ENV             Use a named environment from dva.yml environments section
  --tag, -T TAG[,TAG]       Include only lifecycle entries matching any of the given tags
  --exclude-tag TAG[,TAG]   Exclude lifecycle entries matching any of the given tags

Plan-path flags (only when a plan is being run, e.g. 'dva down <plan>'):
  --var KEY=VAL             Override a plan variable. Ignored off the plan path.`,
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

		if len(c.Applications) > 0 {
			am := lifecycle.NewAppManager(c, e)
			// The same split as `stop`, one step more destructive: orch.Down previews while
			// DownApps deleted the pid and log files for real (TASK-166).
			if dryRun {
				am.DownAppsDryRun()
			} else {
				am.DownApps()
			}
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
	Short: "Stop applications and stack without removing resources",
	Long: `Stop a named plan without removing its resources.
Without plans, use the legacy applications and stack lifecycle.

Plan usage:
  dva stop <plan>         Stop the selected plan without removing resources
  --var KEY=VAL           Override a plan variable
  --dry-run               Print the variable resolution and the actions, without executing

Legacy flags:
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

		if len(c.Applications) > 0 {
			am := lifecycle.NewAppManager(c, e)
			// parseDvaFlags above already consumed --dry-run into the global, and orch.Stop
			// below honours it — so without this branch the stack half previewed while the
			// app half sent a real SIGTERM, in one command (TASK-166).
			if dryRun {
				am.HaltAppsDryRun()
			} else {
				am.HaltApps()
			}
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
Without plans, use the legacy applications and stack lifecycle.

The first argument is read as a plan name when it names a plan, and as a stack
entry name otherwise; the two cannot be combined.

Plan usage:
  dva restart <plan>      Restart the selected plan
  --var KEY=VAL           Override a plan variable
  --no-wait               Return without waiting for readiness
  --dry-run               Print the variable resolution and the actions, without executing

Legacy usage:
  dva restart             Restart every stack entry
  dva restart <service>   Restart only the named entries

Legacy flags:
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
	Use:                "build [OPTIONS] [SERVICE...]",
	Short:              "Build or rebuild services (mode-aware: docker or native)",
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

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all containers, networks, and isolated volumes",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		volumes, _ := cmd.Flags().GetBool("volumes")
		images, _ := cmd.Flags().GetBool("images")
		force, _ := cmd.Flags().GetBool("force")

		// Confirmation prompt for destructive operations.
		//
		// --dry-run is exempt because consent is consent to the deletion, and a dry run
		// deletes nothing. Without the exemption the one command whose purpose is to say
		// what would be destroyed refused to say it until you agreed to the destruction,
		// and the documented default answer (N) returned before the preview ran. Worse
		// non-interactively: Scanln gets EOF from a pipe, `answer` stays empty, and
		// `dva clean --volumes --dry-run` in any script printed "Aborted." and nothing
		// else — rc 0, so nothing downstream noticed either. The only way to reach the
		// preview from a script was --force, which on a real run means destroy without
		// asking; the flag that made the preview reachable was the flag that made the
		// real thing unstoppable. TASK-170, following TASK-166's work on the eight halt
		// sites under this command.
		//
		// Exempted here rather than through a shared helper: this is the only prompt in
		// the codebase today (`dva down --volumes` has none), so a helper would abstract
		// over one caller.
		if !force && !dryRun && (volumes || images) {
			msg := "This will remove all containers, networks"
			if volumes {
				msg += ", and VOLUMES (data loss!)"
			}
			if images {
				msg += ", and locally built images"
			}
			fmt.Fprintf(os.Stderr, "%s.\nContinue? [y/N] ", msg)
			var answer string
			_, _ = fmt.Scanln(&answer)
			answer = strings.ToLower(strings.TrimSpace(answer))
			if answer != "y" && answer != "yes" {
				// stderr, with the prompt it answers. On stdout it was half of an
				// interaction whose other half was on another stream, so `2>/dev/null`
				// showed "Aborted." with nothing saying what had been aborted.
				fmt.Fprintln(os.Stderr, "Aborted.")
				return nil
			}
		}

		// When removing volumes, also clear provision markers
		if volumes {
			// The sixth site in this command where --dry-run was accepted and ignored, and
			// the only one that is not a halt. Guarding the app half alone would have left
			// `dva clean --volumes --dry-run` still deleting these for real while every
			// other line it printed said "would" — a half-honest preview is harder to
			// distrust than a plainly dishonest one. TASK-166.
			if dryRun {
				for _, m := range provisionMarkers(c.FileDir()) {
					fmt.Fprintf(os.Stderr, "[dry-run] would delete provision marker %s\n", m)
				}
			} else {
				clearProvisionMarkers(c.FileDir())
			}
		}

		// App down first (apps depend on infra)
		if len(c.Applications) > 0 {
			am := lifecycle.NewAppManager(c, e)
			// `clean` passes dryRun to orch.Down below, so it made the same half-preview
			// promise the other three paths did (TASK-166).
			if dryRun {
				am.DownAppsDryRun()
			} else {
				am.DownApps()
			}
		}

		// Orchestrator down for all lifecycle entries (with volume/image cleanup if requested)
		orch := lifecycle.NewOrchestrator(c, e)
		if err := orch.Down(context.Background(), lifecycle.DownOptions{
			DryRun:       dryRun,
			Volumes:      volumes,
			RemoveImages: images,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "[warn] lifecycle down: %v\n", err)
		}

		return nil
	},
}

var logsCmd = &cobra.Command{
	Use:                "logs [OPTIONS] [SERVICE...]",
	Short:              "View output from containers",
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
	// The first bad flag wins: reporting one is what the user has to fix first, and the scan
	// continues only so `mode`/`env` are still populated for callers that log before checking.
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
	for i := 0; i < end; i++ {
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

func init() {
	cleanCmd.Flags().BoolP("volumes", "v", false, "Also remove volumes (WARNING: data loss)")
	cleanCmd.Flags().BoolP("images", "i", false, "Also remove images built by docker compose")
	cleanCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
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
