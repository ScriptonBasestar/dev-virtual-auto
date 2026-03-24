package cli

import (
	"encoding/json"
	"testing"
)

func TestIsDevcontainerEnabled(t *testing.T) {
	tests := []struct {
		name string
		dc   map[string]any
		want bool
	}{
		{"nil map", nil, true}, // default true
		{"empty map", map[string]any{}, true},
		{"explicit true", map[string]any{"enabled": true}, true},
		{"explicit false", map[string]any{"enabled": false}, false},
		{"wrong type", map[string]any{"enabled": "yes"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDevcontainerEnabled(tt.dc); got != tt.want {
				t.Errorf("isDevcontainerEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExpandFeatures(t *testing.T) {
	features := map[string]any{
		"go":    "1",
		"ts/ts": "2",
	}
	expanded := expandFeatures(features)
	if expanded == nil {
		t.Fatal("expected expanded map, got nil")
	}
	if _, ok := expanded["ghcr.io/devcontainers/features/go:latest"]; !ok {
		t.Errorf("expected go to be expanded to ghcr.io URI")
	}
	if v, ok := expanded["ts/ts"]; !ok || v != "2" {
		t.Errorf("expected explicit URI to remain unchanged")
	}
}

func TestToDevcontainerRelative(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"docker-compose.yml", "../docker-compose.yml"},
		{"../parent/docker-compose.yml", "../parent/docker-compose.yml"},
		{"/absolute/path", "/absolute/path"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := toDevcontainerRelative(tt.input); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateJSON(t *testing.T) {
	dc := map[string]any{
		"enabled":  true,
		"name":     "Test env",
		"features": map[string]any{"go": "latest"},
	}
	data, err := generateDevcontainerJSON(dc, []string{"compose.yaml"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]any
	json.Unmarshal(data, &out)

	if _, ok := out["enabled"]; ok {
		t.Errorf("enabled key should be omitted")
	}
	if out["name"] != "Test env" {
		t.Errorf("name mismatch")
	}
	if out["dockerComposeFile"] != "../compose.yaml" {
		t.Errorf("dockerComposeFile auto generation failed, got %v", out["dockerComposeFile"])
	}
}
