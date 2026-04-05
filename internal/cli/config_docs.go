package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

var configDocsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Regenerate CLAUDE.md and AGENTS.md guide docs",
	Long: `Regenerate project agent guidelines (CLAUDE.md / AGENTS.md) 
with the latest DVA command reference. Does not modify dva.yml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !dvaConfigExists() {
			return fmt.Errorf("dva.yml not found.\n  Run 'dva init' first to scaffold a configuration")
		}

		guidePath, err := generateAIDocs()
		if err != nil {
			return err
		}

		fmt.Printf("Generated %s\n", guidePath)
		return nil
	},
}

// dvaConfigExists checks whether dva.yml or dva.yaml exists in the current directory only.
func dvaConfigExists() bool {
	for _, name := range []string{config.FileName, config.FileNameAlt} {
		if _, err := os.Stat(name); err == nil {
			return true
		}
	}
	return false
}

func init() {
	configCmd.AddCommand(configDocsCmd)
}
