package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the syntax and schema of 'dva.yml'",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()

		if err := c.Validate(); err != nil {
			return err
		}

		// Check compose file project name alignment
		printComposeNameWarnings(c.ValidateComposeProjectNames())

		fmt.Println("✅ dva.yml is valid")
		return nil
	},
}

// printComposeNameWarnings prints warnings about compose file name mismatches to stderr.
func printComposeNameWarnings(warnings []config.ComposeNameWarning) {
	for _, w := range warnings {
		if w.ComposeName == "" {
			fmt.Fprintf(os.Stderr, "[warn] %s: missing top-level 'name: %s'\n", w.File, w.DvaName)
			fmt.Fprintf(os.Stderr, "       Running 'docker compose up' directly will use the directory name as project,\n")
			fmt.Fprintf(os.Stderr, "       causing port conflicts with dva. Fix: add 'name: %s' to %s\n", w.DvaName, w.File)
		} else {
			fmt.Fprintf(os.Stderr, "[warn] %s: name '%s' differs from dva.yml project_name '%s'\n", w.File, w.ComposeName, w.DvaName)
			fmt.Fprintf(os.Stderr, "       Fix: change 'name: %s' to 'name: %s' in %s\n", w.ComposeName, w.DvaName, w.File)
		}
	}
}
