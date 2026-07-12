package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/yaml.v3"
)

// examplesDir returns the absolute path to the project examples/ directory.
func examplesDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "examples")
}

func TestExamplesParseSuccessfully(t *testing.T) {
	dir := examplesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read examples dir: %v", err)
	}

	var count int
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yml" {
			continue
		}
		count++
		t.Run(e.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatalf("read file: %v", err)
			}
			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatalf("parse YAML: %v", err)
			}
			if cfg.Version == "" {
				t.Error("example should have a version field")
			}
		})
	}
	if count == 0 {
		t.Fatal("no example YAML files found")
	}
	t.Logf("validated %d example files", count)
}

func TestLifecycleEntryParsing(t *testing.T) {
	cases := []struct {
		name        string
		yaml        string
		wantPlugin  string
		wantCompose func(*ComposePluginConfig) bool
		wantKubectl func(*KubectlPluginConfig) bool
	}{
		{
			name: "compose runner",
			yaml: `version: "0.1.0"
stack:
  db:
    default_runner: compose
    order: 10
    runners:
      compose:
        files: [docker-compose.yml]
        project_name: myapp
`,
			wantCompose: func(c *ComposePluginConfig) bool {
				return len(c.Files) == 1 && c.Files[0] == "docker-compose.yml" && c.ProjectName == "myapp"
			},
		},
		{
			name: "kubectl flat",
			yaml: `version: "0.1.0"
stack:
  k8s:
    plugin: kubectl
    order: 20
    namespace: myapp-dev
    context: minikube
`,
			wantPlugin: "kubectl",
			wantKubectl: func(k *KubectlPluginConfig) bool {
				return k.Namespace == "myapp-dev" && k.Context == "minikube"
			},
		},
		{
			name: "nested format (backward compat)",
			yaml: `version: "0.1.0"
stack:
  compose:
    order: 10
    compose:
      files: [docker-compose.yml]
`,
			wantPlugin: "compose",
			wantCompose: func(c *ComposePluginConfig) bool {
				return len(c.Files) == 1 && c.Files[0] == "docker-compose.yml"
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cfg Config
			if err := yaml.Unmarshal([]byte(tc.yaml), &cfg); err != nil {
				t.Fatalf("parse: %v", err)
			}
			if len(cfg.Stack) == 0 {
				t.Fatal("expected stack entries")
			}
			for _, entry := range cfg.Stack {
				if entry.Plugin != tc.wantPlugin {
					t.Errorf("Plugin = %q, want %q", entry.Plugin, tc.wantPlugin)
				}
				if tc.wantCompose != nil {
					composeCfg := entry.ComposeConfig()
					if composeCfg == nil {
						t.Fatal("expected Compose config")
					}
					if !tc.wantCompose(composeCfg) {
						t.Errorf("Compose config mismatch: %+v", composeCfg)
					}
				}
				if tc.wantKubectl != nil {
					if entry.Kubectl == nil {
						t.Fatal("expected Kubectl config")
					}
					if !tc.wantKubectl(entry.Kubectl) {
						t.Errorf("Kubectl config mismatch: %+v", entry.Kubectl)
					}
				}
			}
		})
	}
}

func TestExampleModulesParseSuccessfully(t *testing.T) {
	dir := examplesDir()

	// Main module config
	t.Run("modules/main.yml", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(dir, "modules", "main.yml"))
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("parse YAML: %v", err)
		}
		if len(cfg.Modules) == 0 {
			t.Error("modules/main.yml should declare modules")
		}
	})

	// Sub-module files (.sb/dva/)
	dvaDir := filepath.Join(dir, "modules", ".sb", "dva")
	entries, err := os.ReadDir(dvaDir)
	if err != nil {
		t.Fatalf("read .dva dir: %v", err)
	}

	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".yml" {
			continue
		}
		t.Run(".sb/dva/"+e.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dvaDir, e.Name()))
			if err != nil {
				t.Fatalf("read file: %v", err)
			}
			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatalf("parse YAML: %v", err)
			}
			if len(cfg.Interaction) == 0 {
				t.Error("module should define interaction commands")
			}
		})
	}
}
