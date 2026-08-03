package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/runner"
)

var (
	publishPorts []string
	projectName  string
)

var runCmd = &cobra.Command{
	Use:                "run [OPTIONS] CMD [ARGS...]",
	Short:              "Execute a predefined script from 'dva.yml' (prefix 'run' can be omitted)",
	DisableFlagParsing: false,
	Args:               cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		cmdName := args[0]
		cmdArgs := args[1:]

		// Support namespace:command syntax (e.g., "engine:test")
		resolvedProject := projectName
		if resolvedProject == "" && !config.LiteralKeyWins(c, cmdName) {
			if parts := strings.SplitN(cmdName, ":", 2); len(parts) == 2 {
				resolvedProject = parts[0]
				cmdName = parts[1]
			}
		}

		// Route to sub-project if --project is specified or namespace syntax used
		if resolvedProject != "" {
			return runSubprojectCommand(c, e, resolvedProject, cmdName, cmdArgs)
		}

		tree := runner.NewInteractionTree(c.Interaction)
		resolved := tree.Find(cmdName, cmdArgs...)
		if resolved == nil {
			return fmt.Errorf("command `%s` not recognized! Run 'dva ls' to see available commands", cmdName)
		}

		// Merge interaction-level environment
		e.MergeVars(resolved.Environment)

		if dryRun {
			return runner.Explain(resolved, jsonOutput)
		}

		r := runner.NewRunner(resolved, runner.RunOptions{
			Publish: publishPorts,
			Explain: dryRun,
			Config:  c,
		})

		return r.Execute(e)
	},
}

// runSubprojectCommand loads and runs a command from a sub-project's dva.yml.
func runSubprojectCommand(parentCfg *config.Config, parentEnv *config.Environment, project, cmdName string, cmdArgs []string) error {
	sub, ok := parentCfg.Subprojects[project]
	if !ok {
		available := make([]string, 0, len(parentCfg.Subprojects))
		for k := range parentCfg.Subprojects {
			available = append(available, k)
		}
		return fmt.Errorf("subproject `%s` not found. Available: %s", project, strings.Join(available, ", "))
	}

	subs, err := config.LoadSubprojects(parentCfg.FileDir(), map[string]config.SubprojectConfig{project: sub})
	if err != nil {
		return fmt.Errorf("loading subproject `%s`: %w", project, err)
	}

	subCfg := subs[project]
	subEnv := config.NewEnvironment(subCfg.Environment, parentEnv.WorkDir(), subCfg.FileDir())

	tree := runner.NewInteractionTree(subCfg.FilterInteractions(sub.ExcludeTags))
	resolved := tree.Find(cmdName, cmdArgs...)
	if resolved == nil {
		return fmt.Errorf("command `%s` not found in subproject `%s`. Run 'dva ls --project %s'", cmdName, project, project)
	}

	subEnv.MergeVars(resolved.Environment)

	if dryRun {
		fmt.Printf("[subproject: %s]\n", project)
		return runner.Explain(resolved, jsonOutput)
	}

	r := runner.NewRunner(resolved, runner.RunOptions{
		Publish: publishPorts,
		Explain: dryRun,
		Config:  subCfg,
	})

	if err := r.Execute(subEnv); err != nil {
		return fmt.Errorf("[%s]: %w", project, err)
	}
	return nil
}

func init() {
	runCmd.Flags().StringArrayVarP(&publishPorts, "publish", "p", nil, "Publish container port(s) to host")
	runCmd.Flags().BoolVarP(&dryRun, "explain", "e", false, "Alias for --dry-run")
	runCmd.Flags().StringVar(&projectName, "project", "", "Target a specific sub-project")
}
