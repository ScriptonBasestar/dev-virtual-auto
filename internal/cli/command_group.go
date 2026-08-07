package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// setGroupParentBehavior makes a subcommand-only parent reject unknown children with
// cobra's own SuggestionsFor list (TASK-148).
//
// Without this, cobra leaves leftover args on a non-runnable parent, shows help, and
// exits 0 — so `dva config migrat` / `dva ssh statu` never reach the suggestion path that
// top-level `dva stauts` uses. Making the parent runnable with no leftover args restores
// the unknown-command error, and SuggestionsFor is the same function the top level uses
// (not a second levenshtein implementation).
//
// Call after all AddCommand registrations so SuggestionsFor sees the full child set.
func setGroupParentBehavior(cmd *cobra.Command) {
	if cmd.SuggestionsMinimumDistance <= 0 {
		// Match findSuggestions' default of 2 (SuggestionsFor alone leaves 0 as "match nothing").
		cmd.SuggestionsMinimumDistance = 2
	}
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if len(args) == 0 {
			return c.Help()
		}
		return unknownSubcommandError(c, args[0])
	}
	cmd.SilenceUsage = true
}

// unknownSubcommandError formats the error the same way cobra does for a top-level miss,
// including the "Did you mean this?" block built from c.SuggestionsFor.
func unknownSubcommandError(c *cobra.Command, name string) error {
	msg := fmt.Sprintf("unknown command %q for %q", name, c.CommandPath())
	// SuggestionsFor is cobra's algorithm (prefix + levenshtein vs SuggestionsMinimumDistance).
	suggestions := c.SuggestionsFor(name)
	if len(suggestions) == 0 {
		return fmt.Errorf("%s", msg)
	}
	var b strings.Builder
	b.WriteString(msg)
	b.WriteString("\n\nDid you mean this?\n")
	for _, s := range suggestions {
		b.WriteByte('\t')
		b.WriteString(s)
		b.WriteByte('\n')
	}
	return fmt.Errorf("%s", strings.TrimRight(b.String(), "\n"))
}
