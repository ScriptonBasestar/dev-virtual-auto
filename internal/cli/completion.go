package cli

import (
	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func init() {
	// Dynamic completion for 'run CMD': first arg suggests interaction command names
	runCmd.ValidArgsFunction = func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		c, err := config.Load(".")
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names := make([]string, 0, len(c.Interaction))
		for name := range c.Interaction {
			names = append(names, name)
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}
