package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

var configMigrateWrite bool

var configMigrateCmd = &cobra.Command{
	Use:   "migrate [path]",
	Short: "Convert legacy declarations into the stack/plan shape",
	Long: `Convert the declarations DVA has moved on from.

This opt-in compatibility command is tracked by TASK-197 and will be removed in
a future release once the corpus-based removal predicate in
docs/42-migration-and-compatibility.md is satisfied.

It converts:

  compose entries   ->  stack.<name>.runners.compose
  applications:     ->  stack.<name>.runners.native
  stack.*.order     ->  plans.<plan>.entries[].order

The compose rewrite repairs a shape DVA refuses to load at all. The other two
convert shapes that still load but that nothing reads on the plan path: an
order left on a stack declaration is overwritten by the plan entry's own value,
so a config that declares it and runs through a plan is not ordered by it.

Conversion is per declaration. Anything that cannot be converted mechanically
is left exactly where it is and listed under "left for you" with what it did
and where that behaviour now lives — 'modes' is always in that list, because
one mode spreads across plans, environments and the entries' own runners and
which plan it becomes is a naming decision rather than a derivation.

Prints the result by default and changes nothing; pass --write to apply. Only
the migrated declarations are rewritten — every other line keeps its original
bytes, comments and blank lines included. The migrated config is loaded and
validated in memory first, so a file is never written in a state DVA cannot
read.

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

		out, report, err := config.Migrate(src)
		if err != nil {
			return err
		}

		// Changes and rewrites are the same event — every conversion that reports
		// itself also edits the file — so the byte comparison decides which of the
		// three endings this is, and blocked-only configs take the no-rewrite path
		// with their guidance still printed.
		if bytes.Equal(out, src) {
			fmt.Printf("%s: nothing to convert.\n", path)
			printMigrationReport(os.Stdout, report)
			if hint := renameHint(path); hint != "" {
				fmt.Println(hint)
			}
			// Conversion covers the shapes with a mechanical target. 'dva validate'
			// knows about deprecations that have none, so "nothing to convert" is
			// not "nothing to do".
			fmt.Println("Run 'dva validate' for the deprecations this command does not convert.")
			return nil
		}

		// Migration is a file rewrite, so proving the result readable before
		// touching the original is the difference between a safe tool and one
		// that can leave a project unloadable.
		if err := config.VerifyMigrated(out); err != nil {
			return fmt.Errorf("migration produced a config DVA cannot load, so nothing was written: %w", err)
		}

		if !configMigrateWrite {
			// The config goes to stdout so the preview can be redirected or diffed;
			// everything about the run goes to stderr so that redirect stays clean.
			fmt.Print(string(out))
			fmt.Fprintln(os.Stderr)
			printMigrationReport(os.Stderr, report)
			if hint := renameHint(path); hint != "" {
				fmt.Fprintln(os.Stderr, hint)
			}
			fmt.Fprintf(os.Stderr, "%s: not written (--write to apply)\n", path)
			return nil
		}

		if err := os.WriteFile(path, out, 0o644); err != nil {
			return err
		}
		fmt.Printf("%s: migrated\n", path)
		printMigrationReport(os.Stdout, report)
		if hint := renameHint(path); hint != "" {
			fmt.Println(hint)
		}
		// A config that could not load was never schema-checked past the first
		// error, so migration routinely uncovers unrelated problems that were
		// masked rather than absent. Say so instead of implying it is now clean.
		fmt.Println("Run 'dva validate' — issues the load failure was hiding may surface now.")
		return nil
	},
}

// printMigrationReport writes what moved and what did not.
//
// Blocked is printed on every path, including the one where nothing converted. A
// config that needs hands is the whole result for that config, and swallowing the
// list because there was no rewrite to announce would leave the operator with a
// bare "nothing to convert" over a file full of 'modes'.
//
// Lines already starting with a space are continuations of the entry above and keep
// their nesting instead of getting a bullet of their own.
func printMigrationReport(w io.Writer, report config.MigrationReport) {
	for _, section := range []struct {
		title string
		lines []string
	}{
		{"Converted:", report.Changes},
		{"Left for you:", report.Blocked},
	} {
		if len(section.lines) == 0 {
			continue
		}
		_, _ = fmt.Fprintf(w, "\n%s\n", section.title)
		for _, line := range section.lines {
			if strings.HasPrefix(line, " ") {
				_, _ = fmt.Fprintf(w, "  %s\n", line)
				continue
			}
			_, _ = fmt.Fprintf(w, "  - %s\n", line)
		}
		_, _ = fmt.Fprintln(w)
	}
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
	path, ok := config.ConfigFileInDir(target)
	if !ok {
		return "", fmt.Errorf("no %s (or %s) in %s", config.FileName, config.FileNameAlt, target)
	}
	return path, nil
}

// renameHint returns the line telling the operator that path uses the legacy file name,
// or "" when it is already canonical. Migration is the moment a legacy project is being
// brought up to date, so it is also the natural moment to mention the file name — the
// converter does not rename the file itself, because the project's own scripts, docs and
// CI may refer to it.
func renameHint(path string) string {
	if filepath.Base(path) != config.FileNameAlt {
		return ""
	}
	return fmt.Sprintf("%s: legacy file name — rename to %s (canonical); DVA still loads %s but warns on every run",
		path, config.FileName, config.FileNameAlt)
}

func init() {
	configMigrateCmd.Flags().BoolVar(&configMigrateWrite, "write", false,
		"Apply the migration in place (default: print the result only)")
	configCmd.AddCommand(configMigrateCmd)
}
