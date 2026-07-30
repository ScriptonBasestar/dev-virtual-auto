package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

var configMigrateWrite bool

var configMigrateCmd = &cobra.Command{
	Use:   "migrate [path]",
	Short: "Rewrite legacy compose declarations into the runners shape",
	Long: `Rewrite the compose declarations DVA no longer accepts:

  stack:                         stack:
    compose:                       compose:
      files: [compose.yml]   ->      default_runner: compose
      project_name: app              runners:
                                       compose:
                                         files: [compose.yml]
                                         project_name: app

Covers all three legacy forms: a stack entry named 'compose' carrying compose
keys directly, an explicit 'plugin: compose', and a nested 'compose:' sub-key.

Prints the result by default and changes nothing; pass --write to apply. Only
the migrated entries are rewritten — every other line keeps its original bytes,
comments and blank lines included. The migrated config is loaded and validated
in memory first, so a file is never written in a state DVA cannot read.

  dva config migrate                # preview the current project
  dva config migrate --write        # apply
  dva config migrate ../other-repo  # preview another project`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := "."
		if len(args) == 1 {
			target = args[0]
		}
		path, err := resolveConfigPath(target)
		if err != nil {
			return err
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		out, migrated, err := config.MigrateLegacyCompose(src)
		if err != nil {
			return err
		}
		if len(migrated) == 0 {
			// This command only repairs the one compose shape DVA refuses to load
			// (see the Long help above); it says nothing about the deprecated-but-
			// loadable shapes 'dva validate' warns about, so a bare "nothing to
			// migrate" reads as "nothing to do" when validate may disagree. Name
			// what was checked and where the rest of the migration guidance lives.
			//
			// 'modes', 'stack.*.order' and 'applications' below are a hint, not a
			// rule sourced from one place: internal/config/validate_warnings.go's
			// warnLegacyModes/warnLegacyStackOrder/warnLegacyApplications have no
			// exported list of the section names they cover, and adding one is out
			// of scope for this fix (TASK-069). If validate's set of deprecated
			// sections changes, update this string by hand.
			fmt.Printf("%s: no legacy compose declarations found (this command only converts the\n", path)
			fmt.Println("compose shape DVA cannot load). Run 'dva validate' for deprecation warnings —")
			fmt.Println("'modes', 'stack.*.order' and 'applications' are migrated by hand.")
			return nil
		}

		// Migration is a file rewrite, so proving the result readable before
		// touching the original is the difference between a safe tool and one
		// that can leave a project unloadable.
		if err := config.VerifyMigrated(out); err != nil {
			return fmt.Errorf("migration produced a config DVA cannot load, so nothing was written: %w", err)
		}

		if !configMigrateWrite {
			fmt.Print(string(out))
			fmt.Fprintf(os.Stderr, "\n%s: would migrate %s (--write to apply)\n",
				path, strings.Join(migrated, ", "))
			return nil
		}

		if err := os.WriteFile(path, out, 0o644); err != nil {
			return err
		}
		fmt.Printf("%s: migrated %s\n", path, strings.Join(migrated, ", "))
		// A config that could not load was never schema-checked past the first
		// error, so migration routinely uncovers unrelated problems that were
		// masked rather than absent. Say so instead of implying it is now clean.
		fmt.Println("Run 'dva validate' — issues the load failure was hiding may surface now.")
		return nil
	},
}

// resolveConfigPath accepts either a directory or the config file itself.
func resolveConfigPath(target string) (string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return target, nil
	}
	path := filepath.Join(target, config.FileName)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("no %s in %s", config.FileName, target)
	}
	return path, nil
}

func init() {
	configMigrateCmd.Flags().BoolVar(&configMigrateWrite, "write", false,
		"Apply the migration in place (default: print the result only)")
	configCmd.AddCommand(configMigrateCmd)
}
