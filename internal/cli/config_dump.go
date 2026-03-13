package cli

import (
	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/output"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or manage DVA configuration settings",
}

var configDumpFormat string

var configDumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Dump the final merged configuration (LLM-optimized)",
	Long:  "Output the fully resolved dva.yml configuration after merging modules and overrides. Useful for LLM consumption and debugging.",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()

		switch configDumpFormat {
		case "yaml":
			return output.PrintYAML(c)
		default:
			return output.PrintJSON(c)
		}
	},
}

func init() {
	configDumpCmd.Flags().StringVarP(&configDumpFormat, "format", "f", "json", "Output format (json, yaml)")
	configCmd.AddCommand(configDumpCmd)
	rootCmd.AddCommand(configCmd)
}
