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
	Use:   "run [OPTIONS] CMD [ARGS...]",
	Short: "Execute a predefined script from 'dva.yml' (prefix 'run' can be omitted)",
	Long: `Run a command declared under dva.yml's interaction: section (an "interaction").

The 'run' prefix can be omitted for any interaction name that does not collide with a
reserved built-in command name: 'dva shell' reaches the same interaction as
'dva run shell' when 'shell' is not reserved. A name that does collide is only reachable
through the explicit 'dva run <name>' form — see USAGE.md's "interaction (예약어와 훅)"
section for the reserved list and the conflict rules.

A subcommand declared under interaction.<name>.subcommands inherits its parent's command
and default_args unless it declares its own command/script/script_file/steps, which
resets default_args to empty rather than carrying the parent's over — see USAGE.md's
"interaction.subcommands (default_args 상속)" section for the full inheritance table.

--project NAME (or the 'project:cmd' namespace form) routes to a subproject's own
interaction tree and runs it against that subproject's effective config — its own vars,
environment, and env_file, rooted at the subproject's directory rather than the parent's.

See USAGE.md's "run" section for worked examples.`,
	DisableFlagParsing: false,
	Args:               cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()

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

		// Route to sub-project if --project is specified or namespace syntax used.
		// Reached before any environment is loaded: the root env_file is irrelevant to a
		// child route and must not get a vote in whether that route runs (TASK-264).
		if resolvedProject != "" {
			return runSubprojectCommand(c, resolvedProject, cmdName, cmdArgs)
		}

		tree := runner.NewInteractionTree(c.Interaction)
		resolved := tree.Find(cmdName, cmdArgs...)
		if resolved == nil {
			return fmt.Errorf("command `%s` not recognized! Run 'dva ls' to see available commands", cmdName)
		}

		// Owner before environment. For a locally declared command this is the root config
		// and the loaded env is exactly what it always was; for one imported from a
		// subproject it is the child, whose vars, environment: and env_file the command is
		// documented to run against.
		rt, err := resolveInteractionRuntime(c, cmdName)
		if err != nil {
			return err
		}
		e := rt.env

		// Merge interaction-level environment
		e.MergeVars(resolved.Environment)

		if dryRun {
			return runner.Explain(resolved, jsonOutput)
		}

		// Fail before building a runner: an empty resolved node used to become
		// `sh -c ""` and exit 0 (TASK-173). --explain still runs above so diagnosis works.
		if !resolved.HasExecutionTarget() {
			return runner.ErrNothingToRun(resolved)
		}

		r := runner.NewRunner(resolved, runner.RunOptions{
			Publish: publishPorts,
			Explain: dryRun,
			Config:  rt.config,
		})

		return r.Execute(e)
	},
}

// runSubprojectCommand loads and runs a command from a sub-project's dva.yml.
func runSubprojectCommand(parentCfg *config.Config, project, cmdName string, cmdArgs []string) error {
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
	if subCfg == nil {
		return fmt.Errorf("subproject `%s` loaded no configuration", project)
	}
	// Was config.NewEnvironment(subCfg.Environment, parentEnv.WorkDir(), subCfg.FileDir()),
	// which dropped the child's `vars:` (the base slot held `environment:` instead), never
	// read the child's env_file at all, and ran from the caller's cwd. ownedRuntime applies
	// the documented child precedence — vars < environment: < env_file, OS env still
	// winning — and roots the run at the child config directory (TASK-264).
	rt := ownedRuntime(subCfg)
	subEnv := rt.env

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

	if !resolved.HasExecutionTarget() {
		return runner.ErrNothingToRun(resolved)
	}

	r := runner.NewRunner(resolved, runner.RunOptions{
		Publish: publishPorts,
		Explain: dryRun,
		Config:  rt.config,
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
