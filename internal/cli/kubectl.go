package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

var ktlCmd = &cobra.Command{
	Use:   "ktl [ENTRY] [ARGS...]",
	Short: "Execute kubectl commands within the configured namespace",
	Long: `Execute kubectl commands against a stack entry with kubectl driver.

If only one kubectl entry exists, the entry name can be omitted.
If multiple kubectl entries exist, the first argument must be the entry name.`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		kubectlEntries := c.KubectlEntries()
		if len(kubectlEntries) == 0 {
			// Fallback: use primary kubectl config for backward compatibility
			var kubectlArgs []string
			if kc := c.PrimaryKubectlConfig(); kc != nil && kc.Namespace != "" {
				kubectlArgs = append(kubectlArgs, "--namespace", e.Interpolate(kc.Namespace))
			}
			kubectlArgs = append(kubectlArgs, args...)
			return dvaexec.ExecReplace(e, "kubectl", kubectlArgs, false)
		}

		// Resolve which entry to use
		var entry = kubectlEntries[0]
		var passArgs = args

		if len(kubectlEntries) > 1 {
			// Multiple entries: first arg must be entry name
			if len(args) > 0 {
				if found := c.FindStackEntry(args[0]); found != nil && found.Kubectl != nil {
					entry = found
					passArgs = args[1:]
				} else {
					var names []string
					for _, e := range kubectlEntries {
						names = append(names, e.Name)
					}
					return fmt.Errorf("multiple kubectl entries: %s\nSpecify one: dva ktl <name> [args...]",
						strings.Join(names, ", "))
				}
			}
		}

		var kubectlArgs []string
		if entry.Kubectl != nil && entry.Kubectl.Namespace != "" {
			kubectlArgs = append(kubectlArgs, "--namespace", e.Interpolate(entry.Kubectl.Namespace))
		}
		kubectlArgs = append(kubectlArgs, passArgs...)
		return dvaexec.ExecReplace(e, "kubectl", kubectlArgs, false)
	},
}
