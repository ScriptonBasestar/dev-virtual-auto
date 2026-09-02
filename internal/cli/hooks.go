package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

// forceSubprocess, when true, makes execComposePassthrough delegate to
// execComposeSubprocess so the Go process survives for after-hooks.
var forceSubprocess bool

// wrapWithHooks wraps a hookable command's RunE to execute before/replace/after
// hook steps defined in the interaction section of dva.yml.
func wrapWithHooks(cmdName string, cmd *cobra.Command) {
	original := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return original(cmd, args)
		}

		var foundDryRun bool
		args, foundDryRun = consumeDryRunFlag(args)
		if foundDryRun {
			dryRun = true
		}

		// Recursion guard: skip hooks if already inside a hook execution
		if depth, _ := strconv.Atoi(os.Getenv(config.EnvHookDepthKey)); depth > 0 {
			return original(cmd, args)
		}

		c, err := loadConfig()
		if err != nil {
			// No config → just run the original command
			return original(cmd, args)
		}

		rootEnv := loadEnv(c)
		hookConfig, err := hookOwnerConfig(c, cmdName, args)
		if err != nil {
			return err
		}
		ic := hookConfig.Interaction[cmdName]
		if ic == nil || !ic.HasHooks() {
			return original(cmd, args)
		}

		e := rootEnv
		if hookConfig != c {
			e = newOwnedConfigEnvironment(hookConfig)
		}

		// The wrapped command obtains its environment through the package cache. Point that
		// cache at the already-loaded owner environment for this invocation so an imported
		// plan does not read its env_file once for hooks and again for the built-in. This also
		// preserves direct-project semantics when a before-hook edits the env_file: the
		// built-in continues with the snapshot on which its hooks were based.
		previousEnv := env
		env = e
		defer func() { env = previousEnv }()

		// Set hook depth to prevent recursion in subprocesses.
		//
		// This still guards subprocesses that inherit this process's environment wholesale
		// (exec.ExecSubprocessOutput sets no Cmd.Env), but it is no longer what carries the
		// guard to hook steps: Environment.EnvSlice filters this key out of the os.Environ()
		// passthrough, because the same EnvSlice also builds the environment for the
		// ExecReplace'd target command, where the guard has no business being. The defer
		// below cannot fire on that path either — syscall.Exec replaces the process image.
		_ = os.Setenv(config.EnvHookDepthKey, "1")
		defer func() { _ = os.Unsetenv(config.EnvHookDepthKey) }()

		// he is the one environment that re-adds the guard, so a hook step that shells back
		// into dva is still suppressed. It is a copy: loadEnv caches a package global and e
		// is also the environment the built-in path below hands to ExecReplace.
		he := e.WithHookDepth()

		// Phase 1: before hooks (fail-fast)
		if len(ic.Before) > 0 {
			if err := runHookSteps(he, hookConfig, "before", cmdName, ic.Before); err != nil {
				return err
			}
		}

		// Phase 2: built-in or replace
		if len(ic.Replace) > 0 {
			if err := runHookSteps(he, hookConfig, "replace", cmdName, ic.Replace); err != nil {
				return err
			}
		} else {
			// Force subprocess mode if after-hooks need to run,
			// because execComposePassthrough uses syscall.Exec (process replacement)
			if len(ic.After) > 0 {
				forceSubprocess = true
				defer func() { forceSubprocess = false }()
			}
			if err := original(cmd, args); err != nil {
				return err
			}
		}

		// Phase 3: after hooks
		if len(ic.After) > 0 {
			if err := runHookSteps(he, hookConfig, "after", cmdName, ic.After); err != nil {
				return err
			}
		}

		return nil
	}
}

// hookOwnerConfig selects the same project owner the built-in plan path will use.
// Parent hooks remain the owner for whole-stack and root-plan invocations; a
// standalone imported plan is wrapped only by its child's lifecycle hooks.
func hookOwnerConfig(root *config.Config, cmdName string, args []string) (*config.Config, error) {
	routeArgs, err := hookPlanRoutingArgs(cmdName, args)
	if err != nil {
		return nil, err
	}

	planName, _, ok := detectPlanRoute(root, routeArgs)
	if !ok {
		return root, nil
	}
	resolved, err := lifecycle.ResolvePlan(root, planName, nil)
	if err != nil {
		return nil, err
	}
	return resolved.OwnerConfig(root), nil
}

// hookPlanRoutingArgs applies the same command-specific normalization as the wrapped
// built-in before asking which project owns its hooks. Keeping this in lockstep matters
// for flags that may precede the plan name: routing to a child plan after running parent
// hooks is a cross-project side effect, not merely a different diagnostic.
func hookPlanRoutingArgs(cmdName string, args []string) ([]string, error) {
	switch cmdName {
	case "build":
		_, _, _, _, filtered, err := parseDvaFlags(args)
		if err != nil {
			return nil, err
		}
		return dropLeadingTerminator(filtered), nil
	case config.LogsDirName:
		return consumeRootPersistentFlags(args)
	default:
		// wrapWithHooks has already applied consumeDryRunFlag. The four lifecycle
		// built-ins call detectPlanRoute on precisely this slice.
		return args, nil
	}
}

// runHookSteps executes a list of provision items as hook steps.
func runHookSteps(e *config.Environment, c *config.Config, phase, cmdName string, steps []config.ProvisionItem) error {
	// Hooks are a second executor, so they need their own copy of the notice runStepLoop
	// prints — `dva up` never reaches runStepLoop, and a before-hook marked `parallel:`
	// otherwise runs at half the expected speed with nothing said. Once per list, matching
	// the other executor. TASK-140.
	if config.StepsIgnoreParallel(steps) {
		fmt.Fprintf(os.Stderr, "  ⚠ %s\n", config.IgnoredParallelMessage)
	}
	for i, step := range steps {
		label := step.Step
		if label == "" {
			label = fmt.Sprintf("step %d", i+1)
		}
		fmt.Fprintf(os.Stderr, "[hook:%s:%s] [%d/%d] %s\n", phase, cmdName, i+1, len(steps), label)

		// The line above has already announced the step by name. Without this branch the
		// shell loop below simply iterates zero commands, so the announcement stands as the
		// whole output and reads exactly like a step that succeeded.
		if step.IsInert() {
			fmt.Fprintf(os.Stderr, "  ⚠ %s\n", config.InertStepMessage)
			continue
		}

		// Compose-aware commands
		if len(step.ComposeUp) > 0 {
			composeArgs := append([]string{"up", "-d"}, step.ComposeUp...)
			if dryRun {
				cmd, args, err := buildComposeArgs(e, c, composeArgs)
				if err != nil {
					return fmt.Errorf("hook %s:%s step '%s': %w", phase, cmdName, label, err)
				}
				fmt.Fprintf(os.Stderr, "  [dry-run] $ %s %s\n", cmd, strings.Join(args, " "))
			} else {
				if err := runProvisionCompose(e, c, label, composeArgs); err != nil {
					return fmt.Errorf("hook %s:%s step '%s' failed: %w", phase, cmdName, label, err)
				}
			}
			continue
		}

		if step.ComposeExec != "" {
			composeArgs := append([]string{"exec"}, dvaexec.SplitCommand(step.ComposeExec)...)
			if dryRun {
				cmd, args, err := buildComposeArgs(e, c, composeArgs)
				if err != nil {
					return fmt.Errorf("hook %s:%s step '%s': %w", phase, cmdName, label, err)
				}
				fmt.Fprintf(os.Stderr, "  [dry-run] $ %s %s\n", cmd, strings.Join(args, " "))
			} else {
				if err := runProvisionCompose(e, c, label, composeArgs); err != nil {
					return fmt.Errorf("hook %s:%s step '%s' failed: %w", phase, cmdName, label, err)
				}
			}
			continue
		}

		if step.ComposeRun != "" {
			composeArgs := append([]string{"run"}, dvaexec.SplitCommand(step.ComposeRun)...)
			if dryRun {
				cmd, args, err := buildComposeArgs(e, c, composeArgs)
				if err != nil {
					return fmt.Errorf("hook %s:%s step '%s': %w", phase, cmdName, label, err)
				}
				fmt.Fprintf(os.Stderr, "  [dry-run] $ %s %s\n", cmd, strings.Join(args, " "))
			} else {
				if err := runProvisionCompose(e, c, label, composeArgs); err != nil {
					return fmt.Errorf("hook %s:%s step '%s' failed: %w", phase, cmdName, label, err)
				}
			}
			continue
		}

		// Shell commands
		cmds := step.RunCommands()
		for _, cmdStr := range cmds {
			if dryRun {
				fmt.Fprintf(os.Stderr, "  [dry-run] $ %s\n", cmdStr)
			} else {
				fmt.Fprintf(os.Stderr, "  $ %s\n", cmdStr)
				if err := runShellCommand(e, cmdStr); err != nil {
					return fmt.Errorf("hook %s:%s step '%s' failed: %w", phase, cmdName, label, err)
				}
			}
		}

		// Note display — writeNote for indent/blank-line parity with provision (TASK-141).
		// stderr deliberately: hooks are a progress channel on the same stream as the
		// `$ cmd` lines above; provision notes go to stdout as result-adjacent output.
		// One indent (writeNote's four spaces) rather than a second two-space dialect.
		writeNote(os.Stderr, step.Note)
	}
	return nil
}
