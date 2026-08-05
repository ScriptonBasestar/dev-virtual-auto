package config

import "testing"

func nativeEnvStack(entryVars, runnerEnv map[string]string) *Config {
	return &Config{
		Stack: map[string]*LifecycleEntry{
			"api": {
				DefaultRunner: "native",
				Vars:          entryVars,
				Runners: map[string]any{
					"native": &NativeRunnerConfig{Run: "go run ./cmd/api", Env: runnerEnv},
				},
			},
		},
	}
}

func soleNativeEntry(t *testing.T, c *Config) LifecycleEntry {
	t.Helper()
	entries := c.SortedStack()
	if len(entries) != 1 {
		t.Fatalf("SortedStack: got %d entries, want 1", len(entries))
	}
	return entries[0]
}

// The plan path merges runners.native.env in the resolver; entries reached through
// SortedStack are desugared by applyRunnerConfig instead, so the same key has to arrive
// on this path too. Fixing only one leaves native.env working in half the callers.
func TestSortedStack_NativeRunnerEnvReachesVars(t *testing.T) {
	entry := soleNativeEntry(t, nativeEnvStack(nil, map[string]string{"API_TOKEN": "from-runner"}))

	if got := entry.Vars["API_TOKEN"]; got != "from-runner" {
		t.Errorf("Vars[API_TOKEN] = %q, want %q — runners.native.env did not reach the entry", got, "from-runner")
	}
}

// SortedStack hands out `entry := *e`, a shallow copy, so the Vars map it carries is the
// one still held in c.Stack. Merging the runner env in place would publish it to every
// other reader of that entry; the fix replaces the field instead. This is the assertion
// that tells those two implementations apart.
func TestSortedStack_NativeRunnerEnvDoesNotMutateSharedVars(t *testing.T) {
	c := nativeEnvStack(map[string]string{"PORT": "8080"}, map[string]string{"API_TOKEN": "from-runner"})
	shared := c.Stack["api"].Vars

	entry := soleNativeEntry(t, c)
	if entry.Vars["API_TOKEN"] == "" {
		t.Fatal("precondition: runner env did not reach the copy")
	}

	if _, leaked := shared["API_TOKEN"]; leaked {
		t.Error("runner env leaked into the shared c.Stack Vars map — merged in place instead of replacing the field")
	}
	if got := shared["PORT"]; got != "8080" {
		t.Errorf("shared Vars[PORT] = %q, want %q — the declared map was disturbed", got, "8080")
	}
}
