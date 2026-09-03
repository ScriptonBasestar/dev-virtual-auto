package cli

import "testing"

// TestLifecycleAndRunCommandsHaveExample pins the floor TASK-269 established: before it,
// only composeCmd (compose.go:28) set cobra's Example field, so lifecycle commands and
// run rendered no Examples: section — the invocation examples existed only as hand-formatted
// prose inside Long, which cobra, shell completion, and structured help readers cannot tell
// apart from description text. This guards against a future command regressing to no Example.
//
// Referencing the package-level command vars directly, rather than walking rootCmd like
// TestAllCommandsHaveLongHelp (long_help_test.go) does, keeps this test independent of
// whether any other test in the package has already called rootCmd.ExecuteC() and attached
// cobra's own help/completion commands.
func TestLifecycleAndRunCommandsHaveExample(t *testing.T) {
	for _, tc := range []struct {
		name    string
		example string
	}{
		{"up", upCmd.Example},
		{"down", downCmd.Example},
		{"stop", stopCmd.Example},
		{"restart", restartCmd.Example},
		{"build", buildCmd.Example},
		{"logs", logsCmd.Example},
		{"run", runCmd.Example},
	} {
		if tc.example == "" {
			t.Errorf("%s has no Example help", tc.name)
		}
	}
}
