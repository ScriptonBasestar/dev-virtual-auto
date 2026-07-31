package config

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestInterpolateSimple(t *testing.T) {
	env := NewEnvironment(map[string]string{
		"FOO": "bar",
		"BAZ": "qux",
	}, "/tmp", "/tmp")

	tests := []struct {
		input string
		want  string
	}{
		{"hello $FOO", "hello bar"},
		{"${FOO} and ${BAZ}", "bar and qux"},
		{"no vars here", "no vars here"},
		{"$UNDEFINED stays", "$UNDEFINED stays"},
		{"prefix_${FOO}_suffix", "prefix_bar_suffix"},
	}

	for _, tt := range tests {
		got := env.Interpolate(tt.input)
		if got != tt.want {
			t.Errorf("Interpolate(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestInterpolateOSEnvFallback(t *testing.T) {
	t.Setenv("TEST_DVA_VAR", "from_os")

	env := NewEnvironment(nil, "/tmp", "/tmp")
	got := env.Interpolate("value=$TEST_DVA_VAR")
	if got != "value=from_os" {
		t.Errorf("got %q, want %q", got, "value=from_os")
	}
}

func TestMergeVarsOSEnvPriority(t *testing.T) {
	t.Setenv("EXISTING", "os_value")

	env := NewEnvironment(map[string]string{
		"EXISTING": "config_value",
		"NEW_VAR":  "from_config",
	}, "/tmp", "/tmp")

	// OS env should take priority
	if env.Vars["EXISTING"] != "os_value" {
		t.Errorf("EXISTING = %q, want os_value (OS env should take priority)", env.Vars["EXISTING"])
	}
	if env.Vars["NEW_VAR"] != "from_config" {
		t.Errorf("NEW_VAR = %q, want from_config", env.Vars["NEW_VAR"])
	}
}

func TestSpecialVars(t *testing.T) {
	env := NewEnvironment(nil, "/tmp", "/tmp")

	// DVA_OS should be set
	dva_os := env.Vars[EnvRuntimeOS]
	if dva_os == "" {
		t.Error("DVA_OS should be set")
	}

	// DVA_CURRENT_USER should be set
	uid := env.Vars[EnvRuntimeCurrentUser]
	if uid == "" {
		t.Error("DVA_CURRENT_USER should be set")
	}

	// DVA_CURRENT_UID should be set
	uidNum := env.Vars[EnvRuntimeCurrentUID]
	if uidNum == "" {
		t.Error("DVA_CURRENT_UID should be set")
	}
}

func TestEnvSlice(t *testing.T) {
	env := NewEnvironment(map[string]string{
		"MY_VAR": "my_value",
	}, "/tmp", "/tmp")

	slice := env.EnvSlice()
	found := slices.Contains(slice, "MY_VAR=my_value")
	if !found {
		t.Error("MY_VAR=my_value not found in EnvSlice output")
	}
}

// TASK-100 regression tests.
//
// Before the fix, EnvSlice passed DVA_HOOK_DEPTH straight through from os.Environ(), so the
// guard cli.wrapWithHooks sets ended up in the environment of the ExecReplace'd docker/kubectl
// and of everything it spawns; a nested dva there reads it and silently skips its own hooks.
//
// Both directions are asserted, because the obvious fix — drop the key in EnvSlice — breaks the
// case the guard exists for. The same EnvSlice builds the environment for hook steps, which are
// the one consumer that genuinely needs it to cross a process boundary. A one-directional test
// would have gone green on a change that turns a latent leak into an unbounded hook recursion.

// hookDepthEntries returns the DVA_HOOK_DEPTH assignments in a KEY=VALUE slice.
func hookDepthEntries(slice []string) []string {
	var found []string
	for _, kv := range slice {
		if k, _, _ := strings.Cut(kv, "="); k == EnvHookDepthKey {
			found = append(found, kv)
		}
	}
	return found
}

func TestEnvSliceDropsHookDepth(t *testing.T) {
	t.Setenv(EnvHookDepthKey, "1")

	// Control: without this the assertion below passes vacuously, because a slice that never
	// contained the key is indistinguishable from one it was correctly filtered out of.
	if os.Getenv(EnvHookDepthKey) != "1" {
		t.Fatal("control failed: the guard is not in this process's environment, so there is nothing to filter")
	}

	slice := (&Environment{Vars: map[string]string{}}).EnvSlice()
	t.Logf("EnvSlice returned %d entries", len(slice))
	if len(slice) == 0 {
		t.Fatal("EnvSlice returned nothing — the filter assertion would be vacuous")
	}

	if got := hookDepthEntries(slice); len(got) != 0 {
		t.Errorf("%s reached the child environment as %v — it is this process's state", EnvHookDepthKey, got)
	}
}

func TestWithHookDepthCarriesGuardToHookChildren(t *testing.T) {
	// Deliberately not set in the OS environment: the guard must come from Vars, so that it
	// survives the EnvSlice filter above rather than depending on ambient inheritance.
	t.Setenv(EnvHookDepthKey, "")
	os.Unsetenv(EnvHookDepthKey)

	base := NewEnvironment(map[string]string{"MY_VAR": "my_value"}, "/tmp", "/tmp")
	hookEnv := base.WithHookDepth()

	got := hookDepthEntries(hookEnv.EnvSlice())
	if len(got) != 1 || got[0] != EnvHookDepthKey+"=1" {
		t.Errorf("hook step child got %v, want exactly [%s=1] — a hook that shells back into dva would recurse", got, EnvHookDepthKey)
	}

	// The copy must not lose what it was derived from.
	if !slices.Contains(hookEnv.EnvSlice(), "MY_VAR=my_value") {
		t.Error("WithHookDepth dropped the base environment's vars")
	}
	if hookEnv.WorkDir() != base.WorkDir() || hookEnv.CfgDir() != base.CfgDir() {
		t.Errorf("WithHookDepth lost the unexported dirs: %q/%q want %q/%q",
			hookEnv.WorkDir(), hookEnv.CfgDir(), base.WorkDir(), base.CfgDir())
	}
}

func TestWithHookDepthDoesNotMutateSource(t *testing.T) {
	t.Setenv(EnvHookDepthKey, "")
	os.Unsetenv(EnvHookDepthKey)

	// cli.loadEnv caches one *Environment in a package global and hands the same pointer to
	// the hook executor and to the built-in path that reaches ExecReplace. If WithHookDepth
	// mutated in place, the guard would go right back into the target command's environment.
	base := NewEnvironment(map[string]string{}, "/tmp", "/tmp")
	_ = base.WithHookDepth()

	if _, ok := base.Vars[EnvHookDepthKey]; ok {
		t.Error("WithHookDepth mutated its receiver — the shared Environment now leaks the guard")
	}
	if got := hookDepthEntries(base.EnvSlice()); len(got) != 0 {
		t.Errorf("source environment carries %v after WithHookDepth", got)
	}
}

// Ensure tests don't leak env vars
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
