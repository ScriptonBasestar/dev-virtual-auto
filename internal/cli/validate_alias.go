package cli

import "github.com/spf13/cobra"

func init() {
	rootCmd.AddCommand(newRootValidateCommand())
}

func newRootValidateCommand() *cobra.Command {
	rootValidateCmd := &cobra.Command{
		Use:     validateCmd.Use,
		Short:   validateCmd.Short,
		Long:    validateCmd.Long,
		GroupID: "advanced",
		RunE:    validateCmd.RunE,
	}
	addValidateFlags(rootValidateCmd)
	return rootValidateCmd
}

func addValidateFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("fix", false, "Auto-fix compose file project name mismatches")
	cmd.Flags().BoolVar(&validateStrict, "strict", false, "Fail validation when config drift warnings are detected")
}
