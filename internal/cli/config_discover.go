package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/output"
)

var configDiscoverFormat string
var configDiscoverPrint bool

var configDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Analyze the project via agent-mesh and list discovered DVA options",
	Long: `Run the agent-mesh discovery flow and emit the discovered DVA configuration options.

This is the first step of the two-stage AI workflow:
  1. dva config discover   # scan project and review options
  2. dva config improve    # generate or refine dva.yml from those options

Requires 'am' (agent-mesh) CLI in PATH.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if configDiscoverPrint {
			amArgs := buildAmArgs("dva-discover", map[string]string{
				"target": ".",
			})
			fmt.Printf("am %s\n", joinArgs(amArgs))
			return nil
		}

		amPath, err := findAmCLI()
		if err != nil {
			return err
		}

		fmt.Println("Running project discovery via agent-mesh...")
		fmt.Println()

		if err := execAm(amPath, buildAmArgs("dva-discover", map[string]string{
			"target": ".",
		})); err != nil {
			return err
		}

		reportPath := filepath.Join("tmp", "improve-guided", "00-analysis-report.json")
		data, err := os.ReadFile(reportPath)
		if err != nil {
			return fmt.Errorf("discovery flow completed but report was not found at %s: %w", reportPath, err)
		}

		var decoded any
		if err := json.Unmarshal(data, &decoded); err != nil {
			return fmt.Errorf("decode discovery report: %w", err)
		}

		fmt.Println()
		switch configDiscoverFormat {
		case "yaml":
			return output.PrintYAML(decoded)
		default:
			return output.PrintJSON(decoded)
		}
	},
}

func init() {
	configDiscoverCmd.Flags().StringVarP(&configDiscoverFormat, "format", "f", "json", "Output format (json, yaml)")
	configDiscoverCmd.Flags().BoolVar(&configDiscoverPrint, "print", false, "Output the am run command to stdout (for manual use)")
	configCmd.AddCommand(configDiscoverCmd)
}

func joinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	out := args[0]
	for _, arg := range args[1:] {
		out += " " + arg
	}
	return out
}
