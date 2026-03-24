package config

import (
	"os"
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
	dva_os := env.Vars["DVA_OS"]
	if dva_os == "" {
		t.Error("DVA_OS should be set")
	}

	// DVA_CURRENT_USER should be set
	uid := env.Vars["DVA_CURRENT_USER"]
	if uid == "" {
		t.Error("DVA_CURRENT_USER should be set")
	}

	// DVA_CURRENT_UID should be set
	uidNum := env.Vars["DVA_CURRENT_UID"]
	if uidNum == "" {
		t.Error("DVA_CURRENT_UID should be set")
	}
}

func TestEnvSlice(t *testing.T) {
	env := NewEnvironment(map[string]string{
		"MY_VAR": "my_value",
	}, "/tmp", "/tmp")

	slice := env.EnvSlice()
	found := false
	for _, s := range slice {
		if s == "MY_VAR=my_value" {
			found = true
			break
		}
	}
	if !found {
		t.Error("MY_VAR=my_value not found in EnvSlice output")
	}
}

// Ensure tests don't leak env vars
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
