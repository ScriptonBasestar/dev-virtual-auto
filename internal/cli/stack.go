package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

var stackCmd = &cobra.Command{
	Use:   "stack [command]",
	Short: "Manage infrastructure lifecycle (compose, helm, kubectl, ...)",
	Long: `Manage stack entries defined in the 'stack' section of dva.yml.

Each stack entry represents an infrastructure component managed by a driver
(compose, helm, kubectl, kustomize, script, process, etc.).

Use subcommands to control individual or all stack entries.`,
	Example: `  dva stack up                    # Start all stack entries
  dva stack up compose            # Start a specific stack entry
  dva stack stop                  # Stop all (preserves state)
  dva stack down                  # Remove all stack resources
  dva stack status                # Show stack entry statuses
  dva stack log compose           # View logs for a stack entry`,
}

var stackUpCmd = &cobra.Command{
	Use:   "up [NAME...] [OPTIONS]",
	Short: "Start stack entries (all if no name given)",
	Long: `Start stack entries defined in the 'stack' section of dva.yml.
Starts every entry unless NAME arguments or tag filters narrow the selection.
With no NAME this issues a bare 'docker compose up' (no --profile), so only
profile-less services start; it does NOT consult plans or default_plan. Keep the
default minimal by giving core data services no Docker Compose profile and
gating heavier tiers behind Compose-native 'profiles:'; use 'dva up <plan>' to
start an explicit service subset.

	DVA-specific flags:
	  --force                   Compose only: pass --force-recreate (other plugins ignore)
	  --no-wait                 Start and return without waiting for readiness
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

			mode, envName, includeTags, excludeTags, names := parseDvaFlags(args)
			mode, isDefault := applyDefaultMode(c, mode)

			force := false
			noWait := false
			var filteredNames []string
			for _, a := range names {
				switch a {
				case "--force":
					force = true
				case "--no-wait":
					noWait = true
				default:
					filteredNames = append(filteredNames, a)
				}
			}
			if err := rejectUnknownFlags("up", filteredNames, "--force", "--no-wait"); err != nil {
				return err
			}
			if err := validateStackNames(c, "up", filteredNames); err != nil {
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
		}

		orch := lifecycle.NewOrchestrator(c, e)
		if err := orch.Up(context.Background(), lifecycle.UpOptions{
			DryRun:      dryRun,
			Force:       force,
			Wait:        !noWait,
			Names:       filteredNames,
			IncludeTags: includeTags,
			ExcludeTags: excludeTags,
			Mode:        mode,
			Env:         envName,
		}); err != nil {
			return err
		}

		// Print status summary
		fmt.Fprintln(os.Stderr)
		status, statusErr := orch.Status(context.Background())
		if statusErr == nil {
			lifecycle.PrintStatus(status, c.FileDir())
		}

		return nil
	},
}

var stackStopCmd = &cobra.Command{
	Use:   "stop [NAME...] [OPTIONS]",
	Short: "Stop stack entries without removing resources",
	Long: `Stop stack entries without removing their resources.
Stops every entry unless NAME arguments or tag filters narrow the selection.

DVA-specific flags:
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

		mode, envName, includeTags, excludeTags, names := parseDvaFlags(args)
		mode, _ = applyDefaultMode(c, mode)

		// stop takes no flags of its own, so anything left with a leading dash is a typo.
		if err := rejectUnknownFlags("stop", names); err != nil {
			return err
		}
		if err := validateStackNames(c, "stop", names); err != nil {
			return err
		}

		if err := applyEnv(e, c, envName); err != nil {
			return err
		}

		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Mode != nil && len(rm.Mode.Environment) > 0 {
			e.MergeVars(rm.Mode.Environment)
		}

		orch := lifecycle.NewOrchestrator(c, e)
		return orch.Stop(context.Background(), lifecycle.StopOptions{
			DryRun:      dryRun,
			Names:       names,
			IncludeTags: includeTags,
			ExcludeTags: excludeTags,
			Mode:        mode,
			Env:         envName,
		})
	},
}

var stackDownCmd = &cobra.Command{
	Use:   "down [NAME...] [OPTIONS]",
	Short: "Stop and remove stack resources",
	Long: `Stop stack entries and remove their resources.
Tears down every entry unless NAME arguments or tag filters narrow the selection.

DVA-specific flags:
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

		mode, envName, includeTags, excludeTags, names := parseDvaFlags(args)
		mode, _ = applyDefaultMode(c, mode)

		volumes := false
		var filteredNames []string
		for _, a := range names {
			switch a {
			case "-v", "--volumes":
				volumes = true
			default:
				filteredNames = append(filteredNames, a)
			}
		}
		if err := rejectUnknownFlags("down", filteredNames, "-v", "--volumes"); err != nil {
			return err
		}
		if err := validateStackNames(c, "down", filteredNames); err != nil {
			return err
		}

		if err := applyEnv(e, c, envName); err != nil {
			return err
		}

		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Mode != nil && len(rm.Mode.Environment) > 0 {
			e.MergeVars(rm.Mode.Environment)
		}

		orch := lifecycle.NewOrchestrator(c, e)
		return orch.Down(context.Background(), lifecycle.DownOptions{
			DryRun:      dryRun,
			Volumes:     volumes,
			Names:       filteredNames,
			IncludeTags: includeTags,
			ExcludeTags: excludeTags,
			Mode:        mode,
			Env:         envName,
		})
	},
}

var stackStatusCmd = &cobra.Command{
	Use:   "status [NAME...]",
	Short: "Show status of stack entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		orch := lifecycle.NewOrchestrator(c, e)
		status, err := orch.Status(context.Background())
		if err != nil {
			return err
		}

		// Filter by names if specified
		if len(args) > 0 {
			nameSet := make(map[string]bool, len(args))
			for _, n := range args {
				nameSet[n] = true
			}
			var filtered []lifecycle.EntryStatus
			for _, e := range status.Entries {
				if nameSet[e.Name] {
					filtered = append(filtered, e)
				}
			}
			status.Entries = filtered
		}

			lifecycle.PrintStatus(status, c.FileDir())
			return lifecycle.StatusExitError(status)
		},
	}

	var stackLogCmd = &cobra.Command{
	Use:                "log [NAME] [OPTIONS]",
	Short:              "View logs for a stack entry",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		e := loadEnv(c)

		// DVA's own root flags must not travel into docker's argv. Every sibling
		// DisableFlagParsing command strips them via parseDvaFlags; this one used to append
		// args verbatim, so `dva --debug stack log infra` reached docker as
		// `compose ... logs --debug infra`. Not parseDvaFlags here: it would also eat
		// --dry-run, which compose owns on this path. TASK-092.
		args = consumeRootPersistentFlags(args)

		// If a name is given that matches a non-compose entry, show its log file
		if len(args) > 0 {
			if entry := c.FindStackEntry(args[0]); entry != nil {
				plugin := entry.DetectPlugin()
				switch plugin {
				case "process", "script":
					return showStackEntryLog(c, args[0])
				case "compose", "podman-compose":
					return execComposePassthroughForEntry(e, c, entry, append([]string{config.LogsDirName}, args[1:]...))
				}
			}
		}

		// Default: delegate to compose logs passthrough
		return execComposePassthrough(e, c, append([]string{config.LogsDirName}, args...))
	},
}

// showStackEntryLog reads and prints the log file for a non-compose stack entry.
func showStackEntryLog(c *config.Config, name string) error {
	logFile := filepath.Join(c.FileDir(), config.DotDirName, config.LogsDirName, name+".log")
	data, err := os.ReadFile(logFile)
	if err != nil {
		return fmt.Errorf("no log file for stack entry %q: %w", name, err)
	}

	lines := strings.Split(string(data), "\n")
	start := 0
	if len(lines) > 100 {
		start = len(lines) - 100
	}
	for _, line := range lines[start:] {
		fmt.Println(line)
	}
	return nil
}

// stackSelectorFlags are the flags parseDvaFlags consumes for every stack subcommand.
// Listed here only to build the error message below — parseDvaFlags stays the one place
// that interprets them.
var stackSelectorFlags = []string{
	"--mode", "-M", "--env", "-E", "--tag", "--tags", "-T",
	"--exclude-tag", "--exclude-tags", "--dry-run", "--debug", "--json",
}

// rejectUnknownFlags fails on a leftover argument that still looks like a flag.
//
// up/stop/down read whatever parseDvaFlags leaves behind as stack entry NAMEs, and
// DisableFlagParsing means cobra never vets it. A mistyped flag therefore became an
// entry name, matched nothing, and the command exited 0 having silently dropped it:
// `dva stack up infra --nowait` started infra with `--wait` still on, and `dva stack up
// --nowait` started nothing at all and reported success (TASK-087). No stack entry can be
// named "--nowait", so a leading dash surviving to this point is a user error, not a name.
//
// Deliberately not applied to `stack log`, which forwards its arguments verbatim to
// `docker compose logs` — measured: `dva stack log infra --tail=5 --since=1h` reaches
// docker as `logs infra --tail=5 --since=1h`. There an unrecognised flag is docker's to
// interpret, and rejecting it would delete a working feature.
func rejectUnknownFlags(sub string, args []string, own ...string) error {
	known := append(append([]string{}, own...), stackSelectorFlags...)
	for _, a := range args {
		if len(a) < 2 || !strings.HasPrefix(a, "-") {
			continue
		}
		msg := fmt.Sprintf("unknown flag %q for \"dva stack %s\"", a, sub)
		msg += "\n       → a stack entry name cannot start with \"-\", so this was read as one and matched nothing"
		msg += "\n       → accepted here: " + strings.Join(known, ", ")
		if s := similarTo(a, known); len(s) > 0 {
			msg += "\n\nDid you mean?"
			for _, k := range s {
				msg += fmt.Sprintf("\n  dva stack %s %s", sub, k)
			}
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

// validateStackNames rejects a requested name that matches no stack entry.
//
// The orchestrator's filterByNames keeps the entries whose name was requested and never
// asks the reverse question, so an unmatched name contributed nothing and was never
// reported: `dva stack up nosuchentry` printed the generic "no lifecycle entries matched
// filters" warning and exited 0 (TASK-087). Checked against c.Stack, which is the same map
// SortedStack builds the orchestrator's entries from — so a name accepted here cannot then
// fail to match there, and this cannot reject a name that would have worked.
//
// Only user-supplied names are checked. Entry lists that come from a mode or environment
// profile are config, not input, and belong to validation.
func validateStackNames(c *config.Config, sub string, names []string) error {
	var unknown []string
	for _, n := range names {
		if c.FindStackEntry(n) == nil {
			unknown = append(unknown, n)
		}
	}
	if len(unknown) == 0 {
		return nil
	}

	available := make([]string, 0, len(c.Stack))
	for name := range c.Stack {
		available = append(available, name)
	}
	sort.Strings(available)

	msg := fmt.Sprintf("no such stack entry: %s", strings.Join(unknown, ", "))
	if len(available) == 0 {
		msg += "\n       → dva.yml defines no stack entries"
	} else {
		msg += "\n       → defined in dva.yml: " + strings.Join(available, ", ")
	}
	var suggestions []string
	for _, n := range unknown {
		suggestions = append(suggestions, similarTo(n, available)...)
	}
	if len(suggestions) > 0 {
		msg += "\n\nDid you mean?"
		for _, k := range suggestions {
			msg += fmt.Sprintf("\n  dva stack %s %s", sub, k)
		}
	}
	return fmt.Errorf("%s", msg)
}

// similarTo returns the candidates within edit distance 2 of s, matching the threshold
// resolveProvisionProfile already uses for its "Did you mean?".
func similarTo(s string, candidates []string) []string {
	var out []string
	for _, c := range candidates {
		if levenshtein(s, c) <= 2 {
			out = append(out, c)
		}
	}
	return out
}

func init() {
	stackCmd.AddCommand(stackUpCmd)
	stackCmd.AddCommand(stackStopCmd)
	stackCmd.AddCommand(stackDownCmd)
	stackCmd.AddCommand(stackStatusCmd)
	stackCmd.AddCommand(stackLogCmd)
}
