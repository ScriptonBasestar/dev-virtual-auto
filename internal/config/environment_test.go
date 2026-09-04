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

// TestInterpolateDefaultSyntax pins shell-style `${VAR:-default}` and `${VAR-default}`
// semantics (TASK-303). The previous regex-based expander treated the closing brace as
// optional, so `${POSTGRES_USER:-gorisa}` with POSTGRES_USER=gorisa became
// `gorisa:-gorisa}` — the value was replaced but the operator and default were left behind.
func TestInterpolateDefaultSyntax(t *testing.T) {
	t.Setenv("DVA_TEST_UNSET_303", "")
	os.Unsetenv("DVA_TEST_UNSET_303")

	env := NewEnvironment(map[string]string{
		"SET":   "gorisa",
		"EMPTY": "",
		"HOST":  "db",
	}, "/tmp", "/tmp")

	tests := []struct {
		input string
		want  string
	}{
		// set: default is discarded, nothing of the operator survives
		{"${SET:-fallback}", "gorisa"},
		{"${SET-fallback}", "gorisa"},
		// unset: default is used
		{"${DVA_TEST_UNSET_303:-fallback}", "fallback"},
		{"${DVA_TEST_UNSET_303-fallback}", "fallback"},
		// empty: `:-` substitutes, `-` keeps the empty value (shell semantics)
		{"${EMPTY:-fallback}", "fallback"},
		{"${EMPTY-fallback}", ""},
		// empty default, colon-containing default, adjacent references
		{"${DVA_TEST_UNSET_303:-}", ""},
		{"${DVA_TEST_UNSET_303:-localhost:5432}", "localhost:5432"},
		{"${SET:-a}${SET:-b}", "gorisagorisa"},
		{"${DVA_TEST_UNSET_303:-x}${SET}", "xgorisa"},
		// default itself is interpolated, including a nested braced reference
		{"${DVA_TEST_UNSET_303:-${HOST}}", "db"},
		{"${DVA_TEST_UNSET_303:-${HOST}:5432}", "db:5432"},
		{"${DVA_TEST_UNSET_303:-$HOST/x}", "db/x"},
		// embedded in a larger value
		{"postgres://${SET:-u}@${DVA_TEST_UNSET_303:-${HOST}}:5432/app", "postgres://gorisa@db:5432/app"},
		// plain forms are unchanged
		{"${SET}", "gorisa"},
		{"$SET", "gorisa"},
		{"${DVA_TEST_UNSET_303}", "${DVA_TEST_UNSET_303}"},
		// malformed: no closing brace is left literally
		{"${SET", "${SET"},
		{"${SET:-x", "${SET:-x"},
	}

	for _, tt := range tests {
		if got := env.Interpolate(tt.input); got != tt.want {
			t.Errorf("Interpolate(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestMergeVarsDefaultSyntax checks the default operator through the batch resolver, where
// the sibling lookup rather than Environment.lookup answers the reference.
func TestMergeVarsDefaultSyntax(t *testing.T) {
	os.Unsetenv("DVA_TEST_UNSET_303")
	env := NewEnvironment(nil, "/tmp", "/tmp")
	env.MergeVars(map[string]string{
		"POSTGRES_USER": "gorisa",
		"DB_URL":        "postgres://${POSTGRES_USER:-app}@${DVA_TEST_UNSET_303:-localhost}/db",
		"REGION":        "${DVA_TEST_UNSET_303:-kr}",
	})
	if got := env.Vars["DB_URL"]; got != "postgres://gorisa@localhost/db" {
		t.Errorf("DB_URL = %q", got)
	}
	if got := env.Vars["REGION"]; got != "kr" {
		t.Errorf("REGION = %q", got)
	}
}
