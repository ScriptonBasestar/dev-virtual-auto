package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFindConfigWalksUp(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	subDir := filepath.Join(projectDir, "src", "deep")
	os.MkdirAll(subDir, 0755)

	// Write dva.yml in project root
	dvaYml := filepath.Join(projectDir, FileName)
	os.WriteFile(dvaYml, []byte("version: '0.1.0'\n"), 0644)

	// Find from deep subdir
	found, err := findConfig(subDir)
	if err != nil {
		t.Fatalf("findConfig(%s) error: %v", subDir, err)
	}
	if found != dvaYml {
		t.Errorf("findConfig(%s) = %s, want %s", subDir, found, dvaYml)
	}
}

func TestFindConfigNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := findConfig(tmpDir)
	if err == nil {
		t.Error("findConfig should fail when no dva.yml exists")
	}
}

func TestFindConfigDVAFILE(t *testing.T) {
	tmpDir := t.TempDir()
	customFile := filepath.Join(tmpDir, "custom.yml")
	os.WriteFile(customFile, []byte("version: '0.1.0'\n"), 0644)

	t.Setenv(EnvFileKey, customFile)

	found, err := findConfig(tmpDir)
	if err != nil {
		t.Fatalf("findConfig with DVA_FILE error: %v", err)
	}
	if found != customFile {
		t.Errorf("got %s, want %s", found, customFile)
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, FileName)

	content := `version: "0.1.0"
stack:
  compose:
    default_runner: compose
    order: 10
    runners:
      compose:
        files:
          - docker-compose.yml
        project_name: myapp

environment:
  RAILS_ENV: development
  NODE_ENV: development

interaction:
  shell:
    description: "Open shell"
    service: app
    command: /bin/bash
  test:
    description: "Run tests"
    service: app
    command: bundle exec rspec
`
	os.WriteFile(dvaYml, []byte(content), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Version != "0.1.0" {
		t.Errorf("version = %s, want 0.1.0", cfg.Version)
	}
	if cfg.ComposeProjectName() != "myapp" {
		t.Errorf("project_name = %s, want myapp", cfg.ComposeProjectName())
	}
	files := cfg.AllComposeFiles()
	if len(files) != 1 || files[0] != "docker-compose.yml" {
		t.Errorf("compose.files = %v, want [docker-compose.yml]", files)
	}
	if len(cfg.Interaction) != 2 {
		t.Errorf("interaction count = %d, want 2", len(cfg.Interaction))
	}
	if cfg.Interaction["shell"].Command != "/bin/bash" {
		t.Errorf("shell command = %s, want /bin/bash", cfg.Interaction["shell"].Command)
	}
	if len(cfg.Environment) != 2 {
		t.Errorf("environment count = %d, want 2", len(cfg.Environment))
	}
}

func TestLoadConfigWithModules(t *testing.T) {
	tmpDir := t.TempDir()
	dvaDir := filepath.Join(tmpDir, DotDirName)
	os.MkdirAll(dvaDir, 0755)

	// Main config with module reference
	os.WriteFile(filepath.Join(tmpDir, FileName), []byte(`
modules:
  - extra

interaction:
  shell:
    description: "Open shell"
    service: app
    command: /bin/bash
`), 0644)

	// Module file
	os.WriteFile(filepath.Join(dvaDir, "extra.yml"), []byte(`
interaction:
  test:
    description: "Run tests"
    service: app
    command: bundle exec rspec
`), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	// Should have both shell and test commands
	if len(cfg.Interaction) != 2 {
		t.Errorf("interaction count = %d, want 2 (merged module)", len(cfg.Interaction))
	}
	if _, ok := cfg.Interaction["test"]; !ok {
		t.Error("module command 'test' not found after merge")
	}
}

func TestLoadConfigWithOverride(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, FileName), []byte(`
stack:
  compose:
    default_runner: compose
    order: 10
    runners:
      compose:
        project_name: original
interaction:
  shell:
    service: app
    command: /bin/bash
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "dva.override.yml"), []byte(`
stack:
  compose-override:
    default_runner: compose
    order: 20
    runners:
      compose:
        project_name: overridden
`), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	// "compose" (order 10) is primary over "compose-override" (order 20)
	if cfg.ComposeProjectName() != "original" {
		t.Errorf("project_name = %s, want original (lower order is primary)", cfg.ComposeProjectName())
	}
	if len(cfg.Stack) != 2 {
		t.Errorf("stack entries = %d, want 2 (merged)", len(cfg.Stack))
	}
}

func TestVersionCompatibility(t *testing.T) {
	tests := []struct {
		required   string
		compatible bool
	}{
		{"0.1.0", true},
		{"0.0.9", true},
		{"1.0.0", false},
	}

	for _, tt := range tests {
		if got := isVersionCompatible(tt.required); got != tt.compatible {
			t.Errorf("isVersionCompatible(%s) = %v, want %v", tt.required, got, tt.compatible)
		}
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  [3]int
	}{
		{"1.2.3", [3]int{1, 2, 3}},
		{"0.1", [3]int{0, 1, 0}},
		{"v1.0.0", [3]int{1, 0, 0}},
		{"", [3]int{0, 0, 0}},
		{"invalid", [3]int{0, 0, 0}},
	}
	for _, tt := range tests {
		got := parseVersion(tt.input)
		if got != tt.want {
			t.Errorf("parseVersion(%s) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestProvisionConfigParsing(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, FileName)

	content := `provision:
  default_profile: setup
  setup:
    - step: Install deps
      run: npm install
  reset:
    - step: Reset DB
      run: db reset
`
	os.WriteFile(dvaYml, []byte(content), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Provision.DefaultProfile != "setup" {
		t.Errorf("default_profile = %q, want %q", cfg.Provision.DefaultProfile, "setup")
	}
	if len(cfg.Provision.Profiles) != 2 {
		t.Errorf("profiles count = %d, want 2", len(cfg.Provision.Profiles))
	}
	if _, ok := cfg.Provision.Profiles["setup"]; !ok {
		t.Error("profile 'setup' not found")
	}
	if _, ok := cfg.Provision.Profiles["reset"]; !ok {
		t.Error("profile 'reset' not found")
	}
}

func TestProvisionConfigWithoutDefaultProfile(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, FileName)

	content := `provision:
  setup:
    - step: Install deps
      run: npm install
`
	os.WriteFile(dvaYml, []byte(content), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Provision.DefaultProfile != "" {
		t.Errorf("default_profile = %q, want empty", cfg.Provision.DefaultProfile)
	}
	if len(cfg.Provision.Profiles) != 1 {
		t.Errorf("profiles count = %d, want 1", len(cfg.Provision.Profiles))
	}
}

func TestProvisionConfigMarshalYAMLPreservesProfiles(t *testing.T) {
	input := ProvisionConfig{
		DefaultProfile: "setup",
		Profiles: map[string][]ProvisionItem{
			"setup": {{Step: "Install deps", Run: "npm install"}},
		},
	}

	data, err := yaml.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var output map[string]any
	if err := yaml.Unmarshal(data, &output); err != nil {
		t.Fatalf("Unmarshal marshaled data: %v", err)
	}
	if got := output["default_profile"]; got != "setup" {
		t.Fatalf("default_profile = %v, want setup", got)
	}
	if _, ok := output["setup"]; !ok {
		t.Fatalf("marshaled provision missing setup profile: %s", data)
	}
}

func TestProvisionConfigMergeOverride(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, FileName), []byte(`provision:
  setup:
    - step: Install
      run: npm install
  reset:
    - step: Reset
      run: db reset
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "dva.override.yml"), []byte(`provision:
  default_profile: reset
`), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Provision.DefaultProfile != "reset" {
		t.Errorf("default_profile = %q, want %q after override", cfg.Provision.DefaultProfile, "reset")
	}
	// Original profiles should still be present
	if len(cfg.Provision.Profiles) != 2 {
		t.Errorf("profiles count = %d, want 2 (originals preserved)", len(cfg.Provision.Profiles))
	}
}

func TestServiceRelatedFieldParsing(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, FileName)

	content := `stack:
  compose:
    default_runner: compose
    order: 10
    runners:
      compose:
        files: [docker-compose.yml]
        services:
          api:
            tags: [web]
            related: [worker, scheduler]
            hint: "Worker is needed for async processing"
          worker:
            tags: [background]
`
	os.WriteFile(dvaYml, []byte(content), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	services := cfg.ComposeServices()
	api := services["api"]
	if len(api.Related) != 2 {
		t.Fatalf("expected 2 related, got %d", len(api.Related))
	}
	if api.Related[0] != "worker" || api.Related[1] != "scheduler" {
		t.Errorf("related = %v, want [worker scheduler]", api.Related)
	}
	if api.Hint != "Worker is needed for async processing" {
		t.Errorf("hint = %q", api.Hint)
	}
}

func TestDoctorChecksParsing(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, FileName)

	content := `checks:
  - name: Docker accessible
    type: docker_socket
    fix_hint: Start Docker
  - name: .env exists
    type: file_exists
    path: .env
    fix_hint: cp .env.example .env
  - name: Migrations applied
    type: command
    command: make migrate-status
    fix_hint: dva provision setup
`
	os.WriteFile(dvaYml, []byte(content), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(cfg.DoctorChecks) != 3 {
		t.Fatalf("expected 3 checks, got %d", len(cfg.DoctorChecks))
	}
	if cfg.DoctorChecks[0].Type != "docker_socket" {
		t.Errorf("check[0].type = %q, want docker_socket", cfg.DoctorChecks[0].Type)
	}
	if cfg.DoctorChecks[1].Path != ".env" {
		t.Errorf("check[1].path = %q, want .env", cfg.DoctorChecks[1].Path)
	}
	if cfg.DoctorChecks[2].Command != "make migrate-status" {
		t.Errorf("check[2].command = %q", cfg.DoctorChecks[2].Command)
	}
}

func TestModeProvisionField(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, FileName)

	content := `modes:
  full-stack:
    description: "Everything"
    provision: setup
provision:
  setup:
    - step: Install deps
      run: npm install
`
	os.WriteFile(dvaYml, []byte(content), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	m, ok := cfg.Modes["full-stack"]
	if !ok {
		t.Fatal("mode full-stack not found")
	}
	if m.Provision != "setup" {
		t.Errorf("provision = %q, want setup", m.Provision)
	}
}

func TestEndpointsParsing(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, FileName)

	content := `endpoints:
  api:
    url: http://localhost:8080
    label: "API Server"
    tags: [app]
    paths:
      /health: "Health check"
      /api/v1: "REST API"
  git-ssh:
    url: ssh://git@localhost:2222
    label: "Git SSH"
    tags: [app, scm]
`
	os.WriteFile(dvaYml, []byte(content), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(cfg.Endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(cfg.Endpoints))
	}

	api := cfg.Endpoints["api"]
	if api.URL != "http://localhost:8080" {
		t.Errorf("api.URL = %q", api.URL)
	}
	if api.Label != "API Server" {
		t.Errorf("api.Label = %q", api.Label)
	}
	if len(api.Tags) != 1 || api.Tags[0] != "app" {
		t.Errorf("api.Tags = %v", api.Tags)
	}
	if len(api.Paths) != 2 {
		t.Errorf("api.Paths count = %d, want 2", len(api.Paths))
	}
	if api.Paths["/health"] != "Health check" {
		t.Errorf("api.Paths[/health] = %q", api.Paths["/health"])
	}

	ssh := cfg.Endpoints["git-ssh"]
	if ssh.URL != "ssh://git@localhost:2222" {
		t.Errorf("ssh.URL = %q", ssh.URL)
	}
	if len(ssh.Tags) != 2 {
		t.Errorf("ssh.Tags = %v", ssh.Tags)
	}
}

func TestEndpointsMergeOverride(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, FileName), []byte(`endpoints:
  api:
    url: http://localhost:8080
    label: "API"
  db:
    url: localhost:5432
    label: "DB"
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "dva.override.yml"), []byte(`endpoints:
  api:
    url: http://localhost:9090
    label: "API Override"
  admin:
    url: http://localhost:3000
    label: "Admin"
`), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(cfg.Endpoints) != 3 {
		t.Fatalf("expected 3 endpoints after merge, got %d", len(cfg.Endpoints))
	}
	if cfg.Endpoints["api"].URL != "http://localhost:9090" {
		t.Errorf("api.URL = %q, want override", cfg.Endpoints["api"].URL)
	}
	if cfg.Endpoints["db"].URL != "localhost:5432" {
		t.Errorf("db should be preserved from base")
	}
	if cfg.Endpoints["admin"].URL != "http://localhost:3000" {
		t.Errorf("admin should be added from override")
	}
}

func TestModeEndpointTags(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, FileName)

	content := `modes:
  dev:
    description: "Dev mode"
    endpoint_tags: [app, monitoring]
`
	os.WriteFile(dvaYml, []byte(content), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	m := cfg.Modes["dev"]
	if len(m.EndpointTags) != 2 {
		t.Fatalf("expected 2 endpoint_tags, got %d", len(m.EndpointTags))
	}
	if m.EndpointTags[0] != "app" || m.EndpointTags[1] != "monitoring" {
		t.Errorf("endpoint_tags = %v", m.EndpointTags)
	}
}

func TestEmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, FileName)

	// Empty file
	os.WriteFile(dvaYml, []byte(""), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Empty config should load without error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("Expected cfg to not be nil")
	}
}

func TestResolveEndpoints_SourceToURL(t *testing.T) {
	cfg := &Config{
		Endpoints: map[string]EndpointConfig{
			"app": {Source: "app:11700", Label: "App HTTP"},
			"db":  {Source: "postgres:15432", Label: "PostgreSQL"},
		},
	}
	cfg.ResolveEndpoints()

	if cfg.Endpoints["app"].URL != "http://localhost:11700" {
		t.Errorf("app.URL = %q, want http://localhost:11700", cfg.Endpoints["app"].URL)
	}
	if cfg.Endpoints["db"].URL != "localhost:15432" {
		t.Errorf("db.URL = %q, want localhost:15432", cfg.Endpoints["db"].URL)
	}
}

func TestResolveEndpoints_URLAlreadySet(t *testing.T) {
	cfg := &Config{
		Endpoints: map[string]EndpointConfig{
			"api": {Source: "app:8080", URL: "https://custom.dev:8080", Label: "API"},
		},
	}
	cfg.ResolveEndpoints()

	if cfg.Endpoints["api"].URL != "https://custom.dev:8080" {
		t.Errorf("should not override existing URL, got %q", cfg.Endpoints["api"].URL)
	}
}

func TestResolveEndpoints_NoSource(t *testing.T) {
	cfg := &Config{
		Endpoints: map[string]EndpointConfig{
			"api": {URL: "http://localhost:8080", Label: "API"},
		},
	}
	cfg.ResolveEndpoints()

	if cfg.Endpoints["api"].URL != "http://localhost:8080" {
		t.Errorf("should keep existing URL, got %q", cfg.Endpoints["api"].URL)
	}
}

func TestResolveEndpoints_NilEndpoints(t *testing.T) {
	cfg := &Config{}
	cfg.ResolveEndpoints() // should not panic
}

func TestResolveEndpoints_NonHTTPServices(t *testing.T) {
	tests := []struct {
		source  string
		wantURL string
	}{
		{"redis:16379", "localhost:16379"},
		{"mysql:13306", "localhost:13306"},
		{"mongo:27017", "localhost:27017"},
		{"kafka:9092", "localhost:9092"},
		{"rabbitmq:5672", "localhost:5672"},
		{"ssh:2222", "localhost:2222"},
		// Common aliases
		{"db:15432", "localhost:15432"},
		{"database:13306", "localhost:13306"},
		{"cache:16379", "localhost:16379"},
		{"mq:5672", "localhost:5672"},
		{"queue:9092", "localhost:9092"},
		{"broker:9092", "localhost:9092"},
		// HTTP services
		{"gitea:3000", "http://localhost:3000"},
		{"nginx:8080", "http://localhost:8080"},
		{"api:3000", "http://localhost:3000"},
	}
	for _, tt := range tests {
		cfg := &Config{
			Endpoints: map[string]EndpointConfig{
				"svc": {Source: tt.source, Label: "test"},
			},
		}
		cfg.ResolveEndpoints()
		if cfg.Endpoints["svc"].URL != tt.wantURL {
			t.Errorf("source=%q → URL=%q, want %q", tt.source, cfg.Endpoints["svc"].URL, tt.wantURL)
		}
	}
}

func TestResolveEndpoints_InvalidSource(t *testing.T) {
	cfg := &Config{
		Endpoints: map[string]EndpointConfig{
			"bad1": {Source: "nocolon", Label: "bad"},
			"bad2": {Source: "svc:", Label: "bad"},
		},
	}
	cfg.ResolveEndpoints()

	if cfg.Endpoints["bad1"].URL != "" {
		t.Errorf("invalid source should not resolve, got %q", cfg.Endpoints["bad1"].URL)
	}
	if cfg.Endpoints["bad2"].URL != "" {
		t.Errorf("empty port should not resolve, got %q", cfg.Endpoints["bad2"].URL)
	}
}

func TestResolveEndpoints_IntegrationWithLoad(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, FileName)

	content := `endpoints:
  app-http:
    source: "app:11700"
    label: "App HTTP"
    tags: [app]
    paths:
      /health: "Health check"
  app-ssh:
    url: "ssh://git@localhost:2222"
    label: "Git SSH"
    tags: [app]
  db:
    source: "postgres:15432"
    label: "PostgreSQL"
    tags: [infra]
`
	os.WriteFile(dvaYml, []byte(content), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	// source-resolved HTTP endpoint
	app := cfg.Endpoints["app-http"]
	if app.URL != "http://localhost:11700" {
		t.Errorf("app-http.URL = %q, want http://localhost:11700", app.URL)
	}
	if app.Source != "app:11700" {
		t.Errorf("source should be preserved, got %q", app.Source)
	}

	// explicit URL not touched
	ssh := cfg.Endpoints["app-ssh"]
	if ssh.URL != "ssh://git@localhost:2222" {
		t.Errorf("app-ssh.URL = %q, should not be modified", ssh.URL)
	}

	// non-HTTP service
	db := cfg.Endpoints["db"]
	if db.URL != "localhost:15432" {
		t.Errorf("db.URL = %q, want localhost:15432", db.URL)
	}
}

func TestDefaultMode_ValidReference(t *testing.T) {
	cfg := &Config{
		DefaultMode: "dev",
		Modes: map[string]ModeConfig{
			"dev": {Description: "dev mode"},
		},
	}
	// Valid reference — no error expected from semantic check
	if _, ok := cfg.Modes[cfg.DefaultMode]; !ok {
		t.Errorf("expected default_mode '%s' to exist in modes", cfg.DefaultMode)
	}
}

func TestDefaultMode_InvalidReference(t *testing.T) {
	cfg := &Config{
		DefaultMode: "nonexistent",
		Modes: map[string]ModeConfig{
			"dev": {Description: "dev mode"},
		},
	}
	if _, ok := cfg.Modes[cfg.DefaultMode]; ok {
		t.Errorf("expected default_mode '%s' NOT to exist in modes", cfg.DefaultMode)
	}
}

func TestDefaultMode_Empty(t *testing.T) {
	cfg := &Config{
		DefaultMode: "",
		Modes: map[string]ModeConfig{
			"dev": {Description: "dev mode"},
		},
	}
	// Empty default_mode should not cause issues
	if cfg.DefaultMode != "" {
		t.Errorf("expected empty default_mode")
	}
}

func TestResolveApp_DirectLookup(t *testing.T) {
	cfg := &Config{
		Applications: map[string]*ApplicationConfig{
			"api": {
				Run:  AppExecPaths{Native: "cargo run -p api"},
				Dir:  "services",
				Port: 8080,
				Tags: []string{"backend"},
			},
		},
	}

	name, app, err := cfg.ResolveApp("api")
	if err != nil {
		t.Fatalf("ResolveApp() error: %v", err)
	}
	if name != "api" {
		t.Errorf("name = %q, want %q", name, "api")
	}
	if app.Run.Native != "cargo run -p api" {
		t.Errorf("run = %q, want %q", app.Run.Native, "cargo run -p api")
	}
}

func TestResolveApp_VariantLookup(t *testing.T) {
	cfg := &Config{
		Applications: map[string]*ApplicationConfig{
			"proxynd": {
				Dir:   "nd-stack-rs",
				Port:  11400,
				Tags:  []string{"app", "ce"},
				Run:   AppExecPaths{Native: "cargo run -p proxynd"},
				Build: AppExecPaths{Native: "cargo build -p proxynd"},
				Variants: map[string]*AppVariant{
					"json": {
						Port:  11401,
						Run:   AppExecPaths{Native: "cargo run -p proxynd-json"},
						Build: AppExecPaths{Native: "cargo build -p proxynd-json"},
					},
				},
			},
		},
	}

	name, app, err := cfg.ResolveApp("proxynd.json")
	if err != nil {
		t.Fatalf("ResolveApp() error: %v", err)
	}
	if name != "proxynd.json" {
		t.Errorf("name = %q, want %q", name, "proxynd.json")
	}
	// Variant overrides run/build
	if app.Run.Native != "cargo run -p proxynd-json" {
		t.Errorf("run = %q, want %q", app.Run.Native, "cargo run -p proxynd-json")
	}
	if app.Build.Native != "cargo build -p proxynd-json" {
		t.Errorf("build = %q, want %q", app.Build.Native, "cargo build -p proxynd-json")
	}
	// Variant overrides port
	if app.Port != 11401 {
		t.Errorf("port = %d, want %d", app.Port, 11401)
	}
	// Inherits from parent
	if app.Dir != "nd-stack-rs" {
		t.Errorf("dir = %q, want %q", app.Dir, "nd-stack-rs")
	}
	if len(app.Tags) != 2 || app.Tags[0] != "app" {
		t.Errorf("tags = %v, want [app ce]", app.Tags)
	}
}

func TestResolveApp_VariantInheritsDev(t *testing.T) {
	cfg := &Config{
		Applications: map[string]*ApplicationConfig{
			"api": {
				Dev: AppExecPaths{Native: "cargo watch -x 'run -p api'"},
				Variants: map[string]*AppVariant{
					"worker": {
						Run: AppExecPaths{Native: "cargo run -p api-worker"},
					},
				},
			},
		},
	}

	_, app, err := cfg.ResolveApp("api.worker")
	if err != nil {
		t.Fatalf("ResolveApp() error: %v", err)
	}
	// Variant doesn't override dev → inherits from parent
	if app.Dev.Native != "cargo watch -x 'run -p api'" {
		t.Errorf("dev = %q, want parent dev", app.Dev.Native)
	}
	// Variant overrides run
	if app.Run.Native != "cargo run -p api-worker" {
		t.Errorf("run = %q, want %q", app.Run.Native, "cargo run -p api-worker")
	}
}

func TestResolveApp_NotFound(t *testing.T) {
	cfg := &Config{
		Applications: map[string]*ApplicationConfig{
			"api": {},
		},
	}

	_, _, err := cfg.ResolveApp("web")
	if err == nil {
		t.Error("expected error for unknown app")
	}
}

func TestResolveApp_VariantNotFound(t *testing.T) {
	cfg := &Config{
		Applications: map[string]*ApplicationConfig{
			"api": {
				Variants: map[string]*AppVariant{
					"worker": {},
				},
			},
		},
	}

	_, _, err := cfg.ResolveApp("api.missing")
	if err == nil {
		t.Error("expected error for unknown variant")
	}
}

func TestResolveApp_NoVariants(t *testing.T) {
	cfg := &Config{
		Applications: map[string]*ApplicationConfig{
			"api": {},
		},
	}

	_, _, err := cfg.ResolveApp("api.worker")
	if err == nil {
		t.Error("expected error when app has no variants")
	}
}

func TestListAppNames(t *testing.T) {
	cfg := &Config{
		Applications: map[string]*ApplicationConfig{
			"api": {
				Variants: map[string]*AppVariant{
					"worker": {},
				},
			},
			"web": {},
		},
	}

	names := cfg.ListAppNames()
	if len(names) != 3 {
		t.Errorf("ListAppNames() returned %d names, want 3: %v", len(names), names)
	}

	hasAPI := false
	hasAPIWorker := false
	hasWeb := false
	for _, n := range names {
		switch n {
		case "api":
			hasAPI = true
		case "api.worker":
			hasAPIWorker = true
		case "web":
			hasWeb = true
		}
	}
	if !hasAPI || !hasAPIWorker || !hasWeb {
		t.Errorf("missing names: api=%v api.worker=%v web=%v", hasAPI, hasAPIWorker, hasWeb)
	}
}

func TestResolveApp_VariantWithHyphen(t *testing.T) {
	cfg := &Config{
		Applications: map[string]*ApplicationConfig{
			"proxynd": {
				Dir: "nd-stack-rs",
				Variants: map[string]*AppVariant{
					"inline-html": {
						Run: AppExecPaths{Native: "cargo run -p proxynd-inline-html"},
					},
				},
			},
		},
	}

	name, app, err := cfg.ResolveApp("proxynd.inline-html")
	if err != nil {
		t.Fatalf("ResolveApp() error: %v", err)
	}
	if name != "proxynd.inline-html" {
		t.Errorf("name = %q, want %q", name, "proxynd.inline-html")
	}
	if app.Run.Native != "cargo run -p proxynd-inline-html" {
		t.Errorf("run = %q, want variant run", app.Run.Native)
	}
	if app.Dir != "nd-stack-rs" {
		t.Errorf("dir = %q, want inherited dir", app.Dir)
	}
}

func TestResolveApp_VariantEnvironmentMerge(t *testing.T) {
	cfg := &Config{
		Applications: map[string]*ApplicationConfig{
			"api": {
				Environment: map[string]string{
					"LOG_LEVEL": "debug",
					"PORT":      "8080",
				},
				Variants: map[string]*AppVariant{
					"worker": {
						Environment: map[string]string{
							"PORT":   "8081",
							"WORKER": "true",
						},
					},
				},
			},
		},
	}

	_, app, err := cfg.ResolveApp("api.worker")
	if err != nil {
		t.Fatalf("ResolveApp() error: %v", err)
	}
	if app.Environment["LOG_LEVEL"] != "debug" {
		t.Errorf("LOG_LEVEL = %q, want %q", app.Environment["LOG_LEVEL"], "debug")
	}
	if app.Environment["PORT"] != "8081" {
		t.Errorf("PORT = %q, want %q (overridden by variant)", app.Environment["PORT"], "8081")
	}
	if app.Environment["WORKER"] != "true" {
		t.Errorf("WORKER = %q, want %q", app.Environment["WORKER"], "true")
	}
}
