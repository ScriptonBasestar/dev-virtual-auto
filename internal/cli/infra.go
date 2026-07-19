package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
	"github.com/spf13/cobra"
)

var infraCmd = &cobra.Command{
	Use:   "infra",
	Short: "Manage infrastructure services (deprecated — folded into stack, use 'dva up')",
	Long: `Manage infrastructure services.

DEPRECATED (TASK-051): the top-level 'infra:' section is now folded into 'stack:'
as source-backed compose entries tagged "infra". These commands delegate to the
normal stack lifecycle and will be removed in a future release.

  dva infra up            → dva up <plan> (preferred) or dva stack up -T infra
  dva infra up <service>  → dva stack up <service> (legacy direct control)
  dva compose ...         → raw Docker Compose escape hatch`,
}

func warnInfraDeprecated() {
	fmt.Fprintln(os.Stderr, "⚠  'dva infra' is deprecated (TASK-051): infra services are now stack entries (tag: infra).")
	fmt.Fprintln(os.Stderr, "   Use a named plan with 'dva up <plan>'. For legacy direct control, use 'dva stack up -T infra'.")
}

// infraServiceNames returns stack entries tagged "infra", sorted.
func infraServiceNames(c *config.Config) []string {
	var names []string
	for name, entry := range c.Stack {
		for _, tag := range entry.Tags {
			if tag == "infra" {
				names = append(names, name)
				break
			}
		}
	}
	sort.Strings(names)
	return names
}

// resolveInfraTargets returns the requested infra service names from args
// An empty result means "all infra services". Flags must be consumed by the
// caller first; silently ignoring unsupported flags would report false success.
// Unknown names produce an error listing the available services.
func resolveInfraTargets(c *config.Config, args []string) ([]string, error) {
	avail := infraServiceNames(c)
	availSet := make(map[string]bool, len(avail))
	for _, n := range avail {
		availSet[n] = true
	}

	var names []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			return nil, fmt.Errorf("flag %q is unsupported on deprecated 'dva infra'; use a named plan with 'dva up <plan>'", a)
		}
		if !availSet[a] {
			if len(avail) == 0 {
				return nil, fmt.Errorf("no infra services defined. Declare stack entries with a 'source:' (see TASK-051)")
			}
			return nil, fmt.Errorf("infra service %q not found. Available: %s", a, strings.Join(avail, ", "))
		}
		names = append(names, a)
	}
	return names, nil
}

var infraUpCmd = &cobra.Command{
	Use:                "up [SERVICE...] [OPTIONS]",
	Short:              "Start infrastructure services (SERVICE optional; omit to start all)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		warnInfraDeprecated()
		c := mustLoadConfig()
		e := loadEnv(c)

		cleanArgs, localDryRun := consumeDryRunFlag(args)
		names, err := resolveInfraTargets(c, cleanArgs)
		if err != nil {
			return err
		}

		orch := lifecycle.NewOrchestrator(c, e)
		opts := lifecycle.UpOptions{Wait: true}
		if localDryRun || dryRun {
			opts.DryRun = true
		}
		if len(names) > 0 {
			opts.Names = names
		} else {
			opts.IncludeTags = []string{"infra"}
		}
		return orch.Up(context.Background(), opts)
	},
}

var infraDownCmd = &cobra.Command{
	Use:                "down [SERVICE...] [OPTIONS]",
	Short:              "Stop infrastructure services (SERVICE optional; omit to stop all)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		warnInfraDeprecated()
		c := mustLoadConfig()
		e := loadEnv(c)

		cleanArgs, localDryRun := consumeDryRunFlag(args)
		names, err := resolveInfraTargets(c, cleanArgs)
		if err != nil {
			return err
		}

		orch := lifecycle.NewOrchestrator(c, e)
		opts := lifecycle.DownOptions{}
		if localDryRun || dryRun {
			opts.DryRun = true
		}
		if len(names) > 0 {
			opts.Names = names
		} else {
			opts.IncludeTags = []string{"infra"}
		}
		return orch.Down(context.Background(), opts)
	},
}

var infraUpdateCmd = &cobra.Command{
	Use:   "update [SERVICE]",
	Short: "Update a git-sourced infrastructure service (git pull or clone)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		warnInfraDeprecated()
		c := mustLoadConfig()
		serviceName := args[0]

		entry, ok := c.Stack[serviceName]
		if !ok {
			avail := infraServiceNames(c)
			return fmt.Errorf("infra service %q not found. Available: %s", serviceName, strings.Join(avail, ", "))
		}
		if !entry.Source.IsGit() {
			return fmt.Errorf("infra service %q has no git source to update", serviceName)
		}

		location, err := config.SourceDir(entry.Source, serviceName, c.FileDir())
		if err != nil {
			return err
		}

		if _, err := os.Stat(location); err == nil {
			// Check for uncommitted changes before updating
			statusCmd := exec.Command("git", "status", "--porcelain")
			statusCmd.Dir = location
			statusOut, _ := statusCmd.Output()
			if len(statusOut) > 0 {
				fmt.Fprintf(os.Stderr, "[warn] %s has uncommitted changes in %s:\n%s\n", serviceName, location, string(statusOut))
				return fmt.Errorf("aborted: %s has uncommitted changes in %s. Commit or discard them first", serviceName, location)
			}
			fmt.Printf("Updating %s...\n", serviceName)
			return runInDir(location, "git", "pull", "--rebase")
		}

		fmt.Printf("Cloning %s...\n", serviceName)
		if err := os.MkdirAll(filepath.Dir(location), 0755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", serviceName, err)
		}
		cloneArgs := []string{"clone", "--single-branch", "--depth", "1"}
		if entry.Source.Ref != "" {
			cloneArgs = append(cloneArgs, "--branch", entry.Source.Ref)
		}
		cloneArgs = append(cloneArgs, entry.Source.Git, location)
		return execInfraCmd("git", cloneArgs...)
	},
}

func init() {
	infraCmd.AddCommand(infraUpCmd, infraDownCmd, infraUpdateCmd)
}

func runInDir(dir, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func execInfraCmd(name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
