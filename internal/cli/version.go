package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/output"
)

var versionCmd = &cobra.Command{
	Use:     "version",
	Aliases: []string{"-v", "--version"},
	Short:   "Show DVA version",
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonOutput {
			return output.PrintJSON(map[string]string{
				"version":    config.Version,
				"commit":     config.Commit,
				"build_date": config.BuildDate,
			})
		}
		fmt.Printf("dva version %s\n", config.Version)
		fmt.Printf("commit: %s\n", config.Commit)
		fmt.Printf("build date: %s\n", config.BuildDate)
		return nil
	},
}
