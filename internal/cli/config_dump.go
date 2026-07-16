package cli

import (
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ScriptonBasestar/dva/internal/config"
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
			data, err := configSchemaView(c)
			if err != nil {
				return err
			}
			return output.PrintJSON(data)
		}
	},
}

func configSchemaView(c *config.Config) (any, error) {
	data, err := yaml.Marshal(c)
	if err != nil {
		return nil, err
	}

	var view any
	if err := yaml.Unmarshal(data, &view); err != nil {
		return nil, err
	}
	return view, nil
}

func init() {
	configShowCmd.Flags().StringVarP(&configShowFormat, "format", "f", "json", "Output format (json, yaml)")
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}
