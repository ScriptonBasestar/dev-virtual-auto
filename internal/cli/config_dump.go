package cli

import (
	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/output"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or manage DVA configuration settings",
}

var configShowFormat string

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the final merged configuration (JSON/YAML)",
	Long:  "Output the fully resolved dva.yml configuration after merging modules and overrides. Useful for LLM consumption and debugging.",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()

		switch configShowFormat {
		case "yaml":
			return output.PrintYAML(c)
		default:
			return output.PrintJSON(c)
		}
	},
}

func init() {
	configShowCmd.Flags().StringVarP(&configShowFormat, "format", "f", "json", "Output format (json, yaml)")
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}
