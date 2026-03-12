package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the syntax and schema of 'dva.yml'",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()

		if err := c.Validate(); err != nil {
			return err
		}

		fmt.Println("✅ dva.yml is valid")
		return nil
	},
}
