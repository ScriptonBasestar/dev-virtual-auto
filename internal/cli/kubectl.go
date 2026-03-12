package cli

import (
	"github.com/spf13/cobra"

	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

var ktlCmd = &cobra.Command{
	Use:                "ktl [ARGS...]",
	Short:              "Execute kubectl commands within the configured namespace",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		var kubectlArgs []string

		if ns := c.Kubectl.Namespace; ns != "" {
			kubectlArgs = append(kubectlArgs, "--namespace", e.Interpolate(ns))
		}
		kubectlArgs = append(kubectlArgs, args...)

		return dvaexec.ExecReplace(e, "kubectl", kubectlArgs, false)
	},
}
