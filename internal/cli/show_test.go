package cli

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// captureStdout captures stdout during fn execution.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	out, _ := io.ReadAll(r)
	return string(out)
}

func TestShowText_MinimalConfig(t *testing.T) {
	c := &config.Config{}
	output := captureStdout(t, func() {
		showText(c)
	})
	if !strings.Contains(output, "DVA v") {
		t.Errorf("showText should start with DVA version header, got: %s", output)
	}
}

func TestShowText_WithCompose(t *testing.T) {
	c := &config.Config{
		Lifecycle: []config.LifecycleEntry{
			{
				Name: "compose", Plugin: "compose", Order: 10,
				Compose: &config.ComposePluginConfig{
					ProjectName: "myproject",
					Files:       []string{"compose.yml", "compose.dev.yml"},
				},
			},
		},
	}
	output := captureStdout(t, func() {
		showText(c)
	})
	if !strings.Contains(output, "Compose:") {
		t.Error("showText should display Compose section")
	}
	if !strings.Contains(output, "myproject") {
		t.Error("showText should display project name")
	}
}

func TestShowText_WithModes(t *testing.T) {
	c := &config.Config{
		Modes: map[string]config.ModeConfig{
			"docker": {Description: "Full Docker mode"},
			"native": {Description: "Native services"},
		},
	}
	output := captureStdout(t, func() {
		showText(c)
	})
	if !strings.Contains(output, "Modes (--mode/-M):") {
		t.Error("showText should display Modes section")
	}
	if !strings.Contains(output, "docker") || !strings.Contains(output, "native") {
		t.Error("showText should list all modes")
	}
}

func TestShowText_WithEnvironments(t *testing.T) {
	c := &config.Config{
		Environments: map[string]config.EnvironmentProfile{
			"dev": {Description: "Development"},
			"stg": {Description: "Staging"},
		},
	}
	output := captureStdout(t, func() {
		showText(c)
	})
	if !strings.Contains(output, "Environments (--env/-E):") {
		t.Error("showText should display Environments section")
	}
}

func TestShowText_WithInteraction(t *testing.T) {
	c := &config.Config{
		Interaction: map[string]*config.InteractionCommand{
			"test": {Description: "Run tests"},
			"lint": {
				Description: "Run linter",
				Subcommands: map[string]*config.InteractionCommand{
					"fix": {Description: "Auto-fix"},
				},
			},
		},
	}
	output := captureStdout(t, func() {
		showText(c)
	})
	if !strings.Contains(output, "Interaction Commands: 2 defined") {
		t.Error("showText should display interaction command count")
	}
	if !strings.Contains(output, "+1 sub") {
		t.Error("showText should display subcommand count")
	}
}

func TestShowText_WithProvision(t *testing.T) {
	c := &config.Config{
		Provision: config.ProvisionConfig{
			DefaultProfile: "setup",
			Profiles: map[string][]config.ProvisionItem{
				"setup": {{Step: "Install deps"}},
				"reset": {{Step: "Reset DB"}},
			},
		},
	}
	output := captureStdout(t, func() {
		showText(c)
	})
	if !strings.Contains(output, "Provision Profiles:") {
		t.Error("showText should display provision profiles")
	}
	if !strings.Contains(output, "(default)") {
		t.Error("showText should mark default profile")
	}
}

func TestShowText_WithHealthChecks(t *testing.T) {
	c := &config.Config{
		HealthChecks: map[string]config.HealthCheckConfig{
			"db":  {Type: "tcp"},
			"api": {Type: "http"},
		},
	}
	output := captureStdout(t, func() {
		showText(c)
	})
	if !strings.Contains(output, "Health Checks:") {
		t.Error("showText should display health checks")
	}
}

func TestShowText_WithEnvironmentVars(t *testing.T) {
	c := &config.Config{
		Environment: map[string]string{
			"DB_HOST": "localhost",
			"APP_ENV": "dev",
		},
	}
	output := captureStdout(t, func() {
		showText(c)
	})
	if !strings.Contains(output, "Environment Variables: 2 defined") {
		t.Error("showText should display environment variable count")
	}
}

func TestShowJSON_MinimalConfig(t *testing.T) {
	c := &config.Config{}
	output := captureStdout(t, func() {
		showJSON(c)
	})
	if !strings.Contains(output, "dva_version") {
		t.Error("showJSON should contain dva_version field")
	}
	if !strings.Contains(output, "environment_variables_count") {
		t.Error("showJSON should contain environment_variables_count")
	}
}

func TestShowJSON_FullConfig(t *testing.T) {
	c := &config.Config{
		Lifecycle: []config.LifecycleEntry{
			{
				Name: "compose", Plugin: "compose", Order: 10,
				Compose: &config.ComposePluginConfig{
					ProjectName: "test",
					Files:       []string{"compose.yml"},
				},
			},
		},
		Modes: map[string]config.ModeConfig{
			"docker": {Description: "Docker"},
		},
		Environments: map[string]config.EnvironmentProfile{
			"dev": {Description: "Development"},
		},
		Interaction: map[string]*config.InteractionCommand{
			"test": {Description: "Run tests", Command: "make test"},
		},
		Provision: config.ProvisionConfig{
			DefaultProfile: "setup",
			Profiles: map[string][]config.ProvisionItem{
				"setup": {{Step: "Install"}},
			},
		},
		HealthChecks: map[string]config.HealthCheckConfig{
			"db": {Type: "tcp"},
		},
		Infra: map[string]config.InfraConfig{
			"shared": {},
		},
		Environment: map[string]string{
			"APP_ENV": "dev",
		},
	}
	output := captureStdout(t, func() {
		showJSON(c)
	})
	// Verify all sections are present
	for _, key := range []string{"compose", "modes", "environments", "interaction_commands", "provision", "health_checks", "infra"} {
		if !strings.Contains(output, key) {
			t.Errorf("showJSON output missing %q section", key)
		}
	}
}

func TestShowJSON_WithModes(t *testing.T) {
	c := &config.Config{
		Modes: map[string]config.ModeConfig{
			"docker": {Description: "Docker mode"},
		},
	}
	output := captureStdout(t, func() {
		showJSON(c)
	})
	if !strings.Contains(output, `"modes"`) {
		t.Error("showJSON should contain modes key")
	}
	if !strings.Contains(output, "Docker mode") {
		t.Error("showJSON should contain mode description")
	}
}

func TestSortedKeys(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]string
		want []string
	}{
		{"empty", map[string]string{}, nil},
		{"single", map[string]string{"a": "1"}, []string{"a"}},
		{"sorted", map[string]string{"c": "3", "a": "1", "b": "2"}, []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortedKeys(tt.m)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("sortedKeys()[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestMaxKeyLen(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want int
	}{
		{"empty", nil, 0},
		{"single", []string{"abc"}, 3},
		{"multiple", []string{"a", "abcd", "ab"}, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxKeyLen(tt.keys)
			if got != tt.want {
				t.Errorf("maxKeyLen() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountSubcommands(t *testing.T) {
	tests := []struct {
		name string
		ic   *config.InteractionCommand
		want int
	}{
		{
			"nil subcommands",
			&config.InteractionCommand{},
			0,
		},
		{
			"flat subcommands",
			&config.InteractionCommand{
				Subcommands: map[string]*config.InteractionCommand{
					"sub1": {},
					"sub2": {},
				},
			},
			2,
		},
		{
			"nested subcommands",
			&config.InteractionCommand{
				Subcommands: map[string]*config.InteractionCommand{
					"sub1": {
						Subcommands: map[string]*config.InteractionCommand{
							"nested1": {},
						},
					},
					"sub2": {},
				},
			},
			3, // sub1 + sub2 + nested1
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countSubcommands(tt.ic)
			if got != tt.want {
				t.Errorf("countSubcommands() = %d, want %d", got, tt.want)
			}
		})
	}
}
