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
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		e := loadEnv(c)

		// The fourth site of TASK-092's leak: both exec paths below append args straight
		// into kubectl's argv, so `dva --debug ktl get pods` ran `kubectl get pods --debug`.
		// This must run before the args[0] entry lookup, or `dva --debug ktl <entry> …`
		// would try to resolve "--debug" as the entry name. --dry-run is deliberately left
		// in place, as on the compose passthroughs: kubectl has its own. TASK-103.
		var err error
		if args, err = consumeRootPersistentFlags(args); err != nil {
			return err
		}

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
			matched := false
			if len(args) > 0 {
				// KubectlConfig, not .Kubectl: FindStackEntry returns the raw entry, whose
				// typed field is nil for the runners.kubectl shape. TASK-102.
				if found := c.FindStackEntry(args[0]); found != nil && found.KubectlConfig() != nil {
					entry = found
					passArgs = args[1:]
					matched = true
				}
			}
			if !matched {
				var names []string
				for _, e := range kubectlEntries {
					names = append(names, e.Name)
				}
				return fmt.Errorf("multiple kubectl entries: %s\nSpecify one: dva ktl <name> [args...]",
					strings.Join(names, ", "))
			}
		}

		var kubectlArgs []string
		if kc := entry.KubectlConfig(); kc != nil && kc.Namespace != "" {
			kubectlArgs = append(kubectlArgs, "--namespace", e.Interpolate(kc.Namespace))
		}
		kubectlArgs = append(kubectlArgs, passArgs...)
		return dvaexec.ExecReplace(e, "kubectl", kubectlArgs, false)
	},
}
