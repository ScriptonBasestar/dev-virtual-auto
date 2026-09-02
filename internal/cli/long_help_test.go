package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestAllCommandsHaveLongHelp walks the rootCmd tree recursively and asserts every
// command carries a non-empty Long. TASK-268: 21 commands shipped Short-only, the worst
// being `dva run --help`, which explained none of the concepts (interaction, prefix
// omission, subcommand/default_args resolution, --project) that a first-time user or an
// LLM needs and that today live only in USAGE.md. This guards against a future command
// regressing to Short-only.
//
// The walk never calls rootCmd.Execute()/ExecuteC(), so cobra's own `help` and
// `completion` commands — attached by InitDefaultHelpCmd/InitDefaultCompletionCmd only
// at Execute time — are absent here on their own. They are still excluded by name:
// rootCmd is a package-level var shared by every test in this binary, and another test
// (root_command_registration_test.go's runValidateCommandForTest) does call
// rootCmd.ExecuteC(), which permanently attaches both to the shared rootCmd for the rest
// of the test process. Skipping them by name keeps this test's outcome independent of
// which other tests already ran and in what order.
func TestAllCommandsHaveLongHelp(t *testing.T) {
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, child := range cmd.Commands() {
			if child.Name() == "help" || child.Name() == "completion" {
				continue
			}
			if child.Long == "" {
				t.Errorf("%s has no Long help", child.CommandPath())
			}
			walk(child)
		}
	}
	// rootCmd itself is not reached by the walk (which starts at its children), so
	// assert it directly — otherwise "every command" would silently exclude the one
	// command every user sees first.
	if rootCmd.Long == "" {
		t.Error("rootCmd has no Long help")
	}
	walk(rootCmd)
}
