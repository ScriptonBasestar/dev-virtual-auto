package cli

import (
	"fmt"
	"os"
	"path/filepath"

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
		warnings := c.ValidateComposeProjectNames()
		fix, _ := cmd.Flags().GetBool("fix")

		if fix {
			fixComposeNameWarnings(c, warnings)
		} else {
			printComposeNameWarnings(warnings)
		}

		// Check devcontainer sync
		if len(c.Devcontainer) > 0 && isDevcontainerEnabled(c.Devcontainer) {
			dcPath := filepath.Join(c.FileDir(), ".devcontainer", "devcontainer.json")
			if _, err := os.Stat(dcPath); os.IsNotExist(err) {
				if fix {
					if err := writeDevcontainerFiles(c.Devcontainer, c.Compose.Files, c.FileDir()); err != nil {
						fmt.Fprintf(os.Stderr, "[error] devcontainer: %v\n", err)
					} else {
						fmt.Fprintf(os.Stderr, "[fixed] created .devcontainer/devcontainer.json\n")
					}
				} else {
					fmt.Fprintf(os.Stderr, "[warn] devcontainer section found but .devcontainer/devcontainer.json missing\n")
					fmt.Fprintf(os.Stderr, "       → run: dva add devcontainer  (or dva validate --fix)\n")
				}
			}
		}

		fmt.Println("✅ dva.yml is valid")
		return nil
	},
}

func init() {
	validateCmd.Flags().Bool("fix", false, "Auto-fix compose file project name mismatches")
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

// fixComposeNameWarnings auto-fixes compose file name mismatches.
func fixComposeNameWarnings(c *config.Config, warnings []config.ComposeNameWarning) {
	for _, w := range warnings {
		if err := c.FixComposeProjectName(w); err != nil {
			fmt.Fprintf(os.Stderr, "[error] failed to fix %s: %v\n", w.File, err)
		} else {
			fmt.Fprintf(os.Stderr, "[fixed] %s: set 'name: %s'\n", w.File, w.DvaName)
		}
	}
}
