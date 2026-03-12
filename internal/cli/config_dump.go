package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration utilities",
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
			data, err := yaml.Marshal(c)
			if err != nil {
				return err
			}
			fmt.Print(string(data))
		default:
			data, err := json.MarshalIndent(c, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
		}
		return nil
	},
}

func init() {
	configDumpCmd.Flags().StringVarP(&configDumpFormat, "format", "f", "json", "Output format (json, yaml)")
	configCmd.AddCommand(configDumpCmd)
	rootCmd.AddCommand(configCmd)
}
