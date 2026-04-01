package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Unit tests: merge helpers ---

func TestMergeStringMap(t *testing.T) {
	t.Run("nil dst", func(t *testing.T) {
		got := mergeStringMap(nil, map[string]string{"a": "1"})
		if got["a"] != "1" {
			t.Errorf("got %v, want map with a=1", got)
		}
	})
	t.Run("nil src", func(t *testing.T) {
		dst := map[string]string{"a": "1"}
		got := mergeStringMap(dst, nil)
		if got["a"] != "1" {
			t.Errorf("got %v, want original dst", got)
		}
	})
	t.Run("overlap and new keys", func(t *testing.T) {
		dst := map[string]string{"a": "1", "b": "2"}
		src := map[string]string{"b": "X", "c": "3"}
		got := mergeStringMap(dst, src)
		if got["a"] != "1" || got["b"] != "X" || got["c"] != "3" {
			t.Errorf("got %v, want a=1 b=X c=3", got)
		}
	})
}

func TestMergeLifecycleEntryPartialOverride(t *testing.T) {
	base := &LifecycleEntry{
		Name:   "compose",
		Plugin: "compose",
		Order:  10,
		Tags:   []string{"core"},
		Exports: map[string]string{
			"DB_HOST": "postgres",
		},
		Compose: &ComposePluginConfig{
			Files:       []string{"docker-compose.yml"},
			ProjectName: "myapp",
		},
	}
	other := &LifecycleEntry{
		Plugin: "compose", // same plugin — allowed
		Compose: &ComposePluginConfig{
			Files: []string{"docker-compose.yml", "docker-compose.dev.yml"},
		},
		Exports: map[string]string{
			"REDIS_HOST": "redis",
		},
	}

	merged, err := mergeLifecycleEntry(base, other)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Order preserved from base (other has zero-value)
	if merged.Order != 10 {
		t.Errorf("order = %d, want 10 (preserved)", merged.Order)
	}
	// Tags preserved from base (other has nil)
	if len(merged.Tags) != 1 || merged.Tags[0] != "core" {
		t.Errorf("tags = %v, want [core] (preserved)", merged.Tags)
	}
	// Files replaced by other
	if len(merged.Compose.Files) != 2 {
		t.Errorf("compose.files = %v, want 2 files (replaced)", merged.Compose.Files)
	}
	// ProjectName preserved from base (other has zero-value)
	if merged.Compose.ProjectName != "myapp" {
		t.Errorf("compose.project_name = %q, want myapp (preserved)", merged.Compose.ProjectName)
	}
	// Exports merged (both keys present)
	if merged.Exports["DB_HOST"] != "postgres" {
		t.Errorf("exports.DB_HOST = %q, want postgres", merged.Exports["DB_HOST"])
	}
	if merged.Exports["REDIS_HOST"] != "redis" {
		t.Errorf("exports.REDIS_HOST = %q, want redis", merged.Exports["REDIS_HOST"])
	}
}

func TestMergeLifecycleEntryRestrictedPlugin(t *testing.T) {
	base := &LifecycleEntry{
		Name:   "web",
		Plugin: "compose",
	}
	other := &LifecycleEntry{
		Plugin: "helm",
	}

	_, err := mergeLifecycleEntry(base, other)
	if err == nil {
		t.Fatal("expected error for restricted plugin change, got nil")
	}
	if !strings.Contains(err.Error(), "restricted field") {
		t.Errorf("error = %q, want mention of restricted field", err.Error())
	}
}

func TestMergeLifecycleEntryNilBase(t *testing.T) {
	other := &LifecycleEntry{Plugin: "compose", Order: 5}
	got, err := mergeLifecycleEntry(nil, other)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != other {
		t.Error("nil base should return other directly")
	}
}

func TestMergeComposeConfig(t *testing.T) {
	base := &ComposePluginConfig{
		Files:       []string{"docker-compose.yml"},
		ProjectName: "myapp",
		Services: map[string]ServiceTagConfig{
			"web": {Tags: []string{"app"}},
		},
	}
	other := &ComposePluginConfig{
		Files: []string{"docker-compose.yml", "override.yml"},
		Services: map[string]ServiceTagConfig{
			"db": {Tags: []string{"infra"}},
		},
	}

	mergeComposeConfig(base, other)

	// Files: list replace
	if len(base.Files) != 2 || base.Files[1] != "override.yml" {
		t.Errorf("files = %v, want replaced list", base.Files)
	}
	// ProjectName: preserved (other is zero-value)
	if base.ProjectName != "myapp" {
		t.Errorf("project_name = %q, want myapp", base.ProjectName)
	}
	// Services: map merge (both keys)
	if _, ok := base.Services["web"]; !ok {
		t.Error("services.web should be preserved")
	}
	if _, ok := base.Services["db"]; !ok {
		t.Error("services.db should be added")
	}
}

func TestMergeInteractionCommandPartialOverride(t *testing.T) {
	base := &InteractionCommand{
		Description: "Open shell",
		Service:     "app",
		Command:     "/bin/bash",
		Environment: map[string]string{"TERM": "xterm"},
	}
	other := &InteractionCommand{
		Command: "/bin/zsh",
		Environment: map[string]string{
			"SHELL": "/bin/zsh",
		},
	}

	merged, err := mergeInteractionCommand(base, other)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if merged.Description != "Open shell" {
		t.Errorf("description = %q, want preserved", merged.Description)
	}
	if merged.Service != "app" {
		t.Errorf("service = %q, want preserved", merged.Service)
	}
	if merged.Command != "/bin/zsh" {
		t.Errorf("command = %q, want /bin/zsh (replaced)", merged.Command)
	}
	if merged.Environment["TERM"] != "xterm" {
		t.Errorf("env.TERM = %q, want xterm (preserved)", merged.Environment["TERM"])
	}
	if merged.Environment["SHELL"] != "/bin/zsh" {
		t.Errorf("env.SHELL = %q, want /bin/zsh (added)", merged.Environment["SHELL"])
	}
}

func TestMergeInteractionCommandRestrictedRunner(t *testing.T) {
	base := &InteractionCommand{Runner: "docker"}
	other := &InteractionCommand{Runner: "kubectl"}

	_, err := mergeInteractionCommand(base, other)
	if err == nil {
		t.Fatal("expected error for restricted runner change, got nil")
	}
	if !strings.Contains(err.Error(), "restricted field") {
		t.Errorf("error = %q, want mention of restricted field", err.Error())
	}
}

func TestMergeModeConfigEnvironmentMerge(t *testing.T) {
	base := ModeConfig{
		Description:     "dev mode",
		ComposeProfiles: []string{"dev"},
		Environment: map[string]string{
			"DEBUG": "true",
		},
	}
	other := ModeConfig{
		ComposeProfiles: []string{"dev", "tools"},
		Environment: map[string]string{
			"LOG_LEVEL": "debug",
		},
	}

	merged := mergeModeConfig(base, other)

	if merged.Description != "dev mode" {
		t.Errorf("description = %q, want preserved", merged.Description)
	}
	// List replace
	if len(merged.ComposeProfiles) != 2 || merged.ComposeProfiles[1] != "tools" {
		t.Errorf("compose_profiles = %v, want replaced", merged.ComposeProfiles)
	}
	// Map merge
	if merged.Environment["DEBUG"] != "true" {
		t.Errorf("env.DEBUG = %q, want preserved", merged.Environment["DEBUG"])
	}
	if merged.Environment["LOG_LEVEL"] != "debug" {
		t.Errorf("env.LOG_LEVEL = %q, want added", merged.Environment["LOG_LEVEL"])
	}
}

func TestMergeApplicationConfigPartialOverride(t *testing.T) {
	base := &ApplicationConfig{
		Description: "web app",
		Run: AppExecPaths{
			Native: "npm start",
		},
		Tags:        []string{"frontend"},
		Environment: map[string]string{"PORT": "3000"},
	}
	other := &ApplicationConfig{
		Environment: map[string]string{"NODE_ENV": "development"},
	}

	merged := mergeApplicationConfig(base, other)

	if merged.Description != "web app" {
		t.Errorf("description = %q, want preserved", merged.Description)
	}
	if merged.Run.Native != "npm start" {
		t.Errorf("run.native = %q, want preserved", merged.Run.Native)
	}
	if len(merged.Tags) != 1 || merged.Tags[0] != "frontend" {
		t.Errorf("tags = %v, want preserved", merged.Tags)
	}
	if merged.Environment["PORT"] != "3000" {
		t.Errorf("env.PORT = %q, want preserved", merged.Environment["PORT"])
	}
	if merged.Environment["NODE_ENV"] != "development" {
		t.Errorf("env.NODE_ENV = %q, want added", merged.Environment["NODE_ENV"])
	}
}

func TestMergeApplicationConfigPortOverride(t *testing.T) {
	base := &ApplicationConfig{
		Description: "api",
		Port:        8080,
	}
	other := &ApplicationConfig{
		Port: 11400,
	}

	merged := mergeApplicationConfig(base, other)

	if merged.Port != 11400 {
		t.Errorf("port = %d, want overridden to 11400", merged.Port)
	}
	if merged.Description != "api" {
		t.Errorf("description = %q, want preserved", merged.Description)
	}
}

func TestMergeApplicationConfigPortPreserved(t *testing.T) {
	base := &ApplicationConfig{
		Port: 8080,
	}
	other := &ApplicationConfig{
		Description: "updated",
	}

	merged := mergeApplicationConfig(base, other)

	if merged.Port != 8080 {
		t.Errorf("port = %d, want preserved 8080", merged.Port)
	}
}

func TestMergeEnvironmentProfile(t *testing.T) {
	base := EnvironmentProfile{
		Description: "staging",
		Stack:       []string{"compose", "kubectl"},
		Environment: map[string]string{"ENV": "stg"},
	}
	other := EnvironmentProfile{
		Stack:       []string{"compose"},
		Environment: map[string]string{"REGION": "us-east-1"},
	}

	merged := mergeEnvironmentProfile(base, other)

	if merged.Description != "staging" {
		t.Errorf("description = %q, want preserved", merged.Description)
	}
	// List replace
	if len(merged.Stack) != 1 || merged.Stack[0] != "compose" {
		t.Errorf("stack = %v, want replaced to [compose]", merged.Stack)
	}
	// Map merge
	if merged.Environment["ENV"] != "stg" {
		t.Errorf("env.ENV = %q, want preserved", merged.Environment["ENV"])
	}
	if merged.Environment["REGION"] != "us-east-1" {
		t.Errorf("env.REGION = %q, want added", merged.Environment["REGION"])
	}
}

// --- Integration tests: YAML file → Load() ---

func TestLoadDeepMergeOverride(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "dva.yml"), []byte(`
stack:
  compose:
    plugin: compose
    order: 10
    files:
      - docker-compose.yml
    project_name: myapp
    tags: [core]
    exports:
      DB_HOST: postgres

interaction:
  shell:
    description: "Open shell"
    service: app
    command: /bin/bash
    environment:
      TERM: xterm

modes:
  dev:
    description: "Development mode"
    compose_profiles: [dev]
    environment:
      DEBUG: "true"
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "dva.override.yml"), []byte(`
stack:
  compose:
    files:
      - docker-compose.yml
      - docker-compose.dev.yml
    exports:
      REDIS_HOST: redis

interaction:
  shell:
    command: /bin/zsh
    environment:
      SHELL: /bin/zsh

modes:
  dev:
    compose_profiles: [dev, tools]
    environment:
      LOG_LEVEL: debug
`), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	// Stack: compose entry deep merged
	entry := cfg.Stack["compose"]
	if entry == nil {
		t.Fatal("stack.compose not found")
	}
	if entry.Order != 10 {
		t.Errorf("stack.compose.order = %d, want 10 (preserved)", entry.Order)
	}
	if entry.Compose.ProjectName != "myapp" {
		t.Errorf("stack.compose.project_name = %q, want myapp (preserved)", entry.Compose.ProjectName)
	}
	if len(entry.Compose.Files) != 2 {
		t.Errorf("stack.compose.files = %v, want 2 files", entry.Compose.Files)
	}
	if entry.Exports["DB_HOST"] != "postgres" {
		t.Errorf("exports.DB_HOST = %q, want postgres", entry.Exports["DB_HOST"])
	}
	if entry.Exports["REDIS_HOST"] != "redis" {
		t.Errorf("exports.REDIS_HOST = %q, want redis", entry.Exports["REDIS_HOST"])
	}
	if len(entry.Tags) != 1 || entry.Tags[0] != "core" {
		t.Errorf("tags = %v, want [core] (preserved)", entry.Tags)
	}

	// Interaction: shell deep merged
	shell := cfg.Interaction["shell"]
	if shell == nil {
		t.Fatal("interaction.shell not found")
	}
	if shell.Description != "Open shell" {
		t.Errorf("shell.description = %q, want preserved", shell.Description)
	}
	if shell.Service != "app" {
		t.Errorf("shell.service = %q, want preserved", shell.Service)
	}
	if shell.Command != "/bin/zsh" {
		t.Errorf("shell.command = %q, want /bin/zsh (replaced)", shell.Command)
	}
	if shell.Environment["TERM"] != "xterm" {
		t.Errorf("shell.env.TERM = %q, want preserved", shell.Environment["TERM"])
	}
	if shell.Environment["SHELL"] != "/bin/zsh" {
		t.Errorf("shell.env.SHELL = %q, want added", shell.Environment["SHELL"])
	}

	// Modes: dev deep merged
	dev := cfg.Modes["dev"]
	if dev.Description != "Development mode" {
		t.Errorf("modes.dev.description = %q, want preserved", dev.Description)
	}
	if len(dev.ComposeProfiles) != 2 || dev.ComposeProfiles[1] != "tools" {
		t.Errorf("modes.dev.compose_profiles = %v, want [dev tools]", dev.ComposeProfiles)
	}
	if dev.Environment["DEBUG"] != "true" {
		t.Errorf("modes.dev.env.DEBUG = %q, want preserved", dev.Environment["DEBUG"])
	}
	if dev.Environment["LOG_LEVEL"] != "debug" {
		t.Errorf("modes.dev.env.LOG_LEVEL = %q, want added", dev.Environment["LOG_LEVEL"])
	}
}

func TestLoadRestrictedFieldError(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "dva.yml"), []byte(`
stack:
  web:
    plugin: compose
    order: 10
    files: [docker-compose.yml]
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "dva.override.yml"), []byte(`
stack:
  web:
    plugin: helm
`), 0644)

	_, err := Load(tmpDir)
	if err == nil {
		t.Fatal("expected error for restricted plugin change, got nil")
	}
	if !strings.Contains(err.Error(), "restricted field") {
		t.Errorf("error = %q, want mention of restricted field", err.Error())
	}
}
