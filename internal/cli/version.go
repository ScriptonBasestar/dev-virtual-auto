package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

var versionCmd = &cobra.Command{
	Use:     "version",
	Aliases: []string{"-v", "--version"},
	Short:   "Show DVA version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.Version)
	},
}
