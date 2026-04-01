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
		// Recursion guard: skip hooks if already inside a hook execution
		if depth, _ := strconv.Atoi(os.Getenv("DVA_HOOK_DEPTH")); depth > 0 {
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

		// Set hook depth to prevent recursion in subprocesses
		_ = os.Setenv("DVA_HOOK_DEPTH", "1")
		defer func() { _ = os.Unsetenv("DVA_HOOK_DEPTH") }()

		// Phase 1: before hooks (fail-fast)
		if len(ic.Before) > 0 {
			if err := runHookSteps(e, c, "before", cmdName, ic.Before); err != nil {
				return err
			}
		}

		// Phase 2: built-in or replace
		if len(ic.Replace) > 0 {
			if err := runHookSteps(e, c, "replace", cmdName, ic.Replace); err != nil {
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
			if err := runHookSteps(e, c, "after", cmdName, ic.After); err != nil {
				return err
			}
		}

		return nil
	}
}

// runHookSteps executes a list of provision items as hook steps.
func runHookSteps(e *config.Environment, c *config.Config, phase, cmdName string, steps []config.ProvisionItem) error {
	for i, step := range steps {
		label := step.Step
		if label == "" {
			label = fmt.Sprintf("step %d", i+1)
		}
		fmt.Fprintf(os.Stderr, "[hook:%s:%s] [%d/%d] %s\n", phase, cmdName, i+1, len(steps), label)

		// Compose-aware commands
		if len(step.ComposeUp) > 0 {
			composeArgs := append([]string{"up", "-d"}, step.ComposeUp...)
			if dryRun {
				cmd, args := buildComposeArgs(e, c, composeArgs)
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
				cmd, args := buildComposeArgs(e, c, composeArgs)
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
				cmd, args := buildComposeArgs(e, c, composeArgs)
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
			for _, line := range strings.Split(step.Note, "\n") {
				fmt.Fprintf(os.Stderr, "  %s\n", line)
			}
			fmt.Fprintln(os.Stderr)
		}
	}
	return nil
}
