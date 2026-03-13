package cli

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestNeedsMigration(t *testing.T) {
	tests := []struct {
		current  string
		target   string
		expected bool
	}{
		{"8.1.0", "9.2.0", true},
		{"9.2.0", "9.2.0", false},
		{"unknown", "9.2.0", true},
		{"", "9.2.0", true},
	}

	for _, tt := range tests {
		actual := needsMigration(tt.current, tt.target)
		if actual != tt.expected {
			t.Errorf("needsMigration(%q, %q) = %v, expected %v", tt.current, tt.target, actual, tt.expected)
		}
	}
}

func TestBuildMigrationPrompt(t *testing.T) {
	c := &config.Config{
		Version: "8.1.0",
		Interaction: map[string]*config.InteractionCommand{
			"rails": {
				Command:           "bundle exec rails",
				ComposeRunOptions: []string{"service-ports", "rm"},
			},
			"npm": {
				Command:           "npm",
				ComposeRunOptions: []string{"rm"},
			},
		},
		Provision: map[string][]config.ProvisionItem{
			"default": {
				{Raw: "echo 'hello'"},
			},
		},
	}

	currentVersion := "8.1.0"
	targetVersion := "9.2.0"

	// Create a mock config Version
	oldVersion := config.Version
	config.Version = targetVersion
	defer func() { config.Version = oldVersion }()

	prompt := buildMigrationPrompt(c, currentVersion, targetVersion)

	if !strings.Contains(prompt, "Current Version**: 8.1.0") {
		t.Errorf("Expected prompt to contain current version, got:\n%s", prompt)
	}

	if !strings.Contains(prompt, "compose_run_options (Deprecated)") {
		t.Errorf("Expected prompt to detect deprecated compose_run_options, got:\n%s", prompt)
	}

	if !strings.Contains(prompt, "interaction.rails") {
		t.Errorf("Expected prompt to mention interaction.rails, got:\n%s", prompt)
	}

	if !strings.Contains(prompt, "interaction.npm") {
		t.Errorf("Expected prompt to mention interaction.npm, got:\n%s", prompt)
	}

	if !strings.Contains(prompt, "Provision Legacy Format") {
		t.Errorf("Expected prompt to detect legacy provision formatting, got:\n%s", prompt)
	}

	if !strings.Contains(prompt, "env_file Support") {
		t.Errorf("Expected prompt to mention new env_file pattern, got:\n%s", prompt)
	}
}
