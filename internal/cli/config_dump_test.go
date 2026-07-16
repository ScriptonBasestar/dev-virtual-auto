package cli

import (
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestConfigSchemaViewUsesYAMLFieldNames(t *testing.T) {
	c := &config.Config{
		Version: "0.1.44",
		Provision: config.ProvisionConfig{
			DefaultProfile: "setup",
			Profiles: map[string][]config.ProvisionItem{
				"setup": {{Step: "Install deps", Run: "npm install"}},
			},
		},
	}

	view, err := configSchemaView(c)
	if err != nil {
		t.Fatalf("configSchemaView error: %v", err)
	}
	root, ok := view.(map[string]any)
	if !ok {
		t.Fatalf("view type = %T, want map[string]any", view)
	}
	if _, exists := root["Provision"]; exists {
		t.Fatal("view contains Go field name Provision")
	}
	provision, ok := root["provision"].(map[string]any)
	if !ok {
		t.Fatalf("provision type = %T, want map[string]any", root["provision"])
	}
	if got := provision["default_profile"]; got != "setup" {
		t.Fatalf("default_profile = %v, want setup", got)
	}
	if _, exists := provision["setup"]; !exists {
		t.Fatal("schema view missing setup profile")
	}
}
