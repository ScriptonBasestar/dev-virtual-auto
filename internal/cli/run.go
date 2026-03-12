package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/runner"
)

var (
	publishPorts []string
)

var runCmd = &cobra.Command{
	Use:                "run [OPTIONS] CMD [ARGS...]",
	Short:              "Run configured command (run prefix may be omitted)",
	DisableFlagParsing: false,
	Args:               cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		cmdName := args[0]
		cmdArgs := args[1:]

		tree := runner.NewInteractionTree(c.Interaction)
		resolved := tree.Find(cmdName, cmdArgs...)
		if resolved == nil {
			return fmt.Errorf("command `%s` not recognized! Run 'dva ls' to see available commands", cmdName)
		}

		// Merge interaction-level environment
		e.MergeVars(resolved.Environment)

		if dryRun {
			runner.Explain(resolved, jsonOutput)
			return nil
		}

		r := runner.NewRunner(resolved, runner.RunOptions{
			Publish: publishPorts,
			Explain: dryRun,
		})

		if err := r.Execute(e); err != nil {
			fmt.Fprintf(os.Stderr, "\nERROR: %s\n", err)
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	runCmd.Flags().StringArrayVarP(&publishPorts, "publish", "p", nil, "Publish container port(s) to host")
	runCmd.Flags().BoolVarP(&dryRun, "explain", "e", false, "Alias for --dry-run")
}
