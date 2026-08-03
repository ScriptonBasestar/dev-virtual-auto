package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
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

		ic := c.Interaction[cmdName]
		if ic == nil || !ic.HasHooks() {
			return original(cmd, args)
		}

		e := loadEnv(c)

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
			if err := runHookSteps(he, c, "before", cmdName, ic.Before); err != nil {
				return err
			}
		}

		// Phase 2: built-in or replace
		if len(ic.Replace) > 0 {
			if err := runHookSteps(he, c, "replace", cmdName, ic.Replace); err != nil {
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
			if err := runHookSteps(he, c, "after", cmdName, ic.After); err != nil {
				return err
			}
		}

		return nil
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
			composeArgs := append([]string{"exec"}, strings.Fields(step.ComposeExec)...)
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
			composeArgs := append([]string{"run"}, strings.Fields(step.ComposeRun)...)
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

		// Note display
		if step.Note != "" {
			fmt.Fprintln(os.Stderr)
			for line := range strings.SplitSeq(step.Note, "\n") {
				fmt.Fprintf(os.Stderr, "  %s\n", line)
			}
			fmt.Fprintln(os.Stderr)
		}
	}
	return nil
}
