package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestParseDvaFlags_ModeOnly(t *testing.T) {
	mode, env, _, excludeTags, filtered := parseDvaFlags([]string{"--mode", "native", "postgres"})
	if mode != "native" {
		t.Errorf("mode = %q, want %q", mode, "native")
	}
	if env != "" {
		t.Errorf("env = %q, want empty", env)
	}
	if len(excludeTags) != 0 {
		t.Errorf("excludeTags = %v, want empty", excludeTags)
	}
	if len(filtered) != 1 || filtered[0] != "postgres" {
		t.Errorf("filtered = %v, want [postgres]", filtered)
	}
}

func TestParseDvaFlags_EnvOnly(t *testing.T) {
	mode, env, _, _, filtered := parseDvaFlags([]string{"--env", "stg", "-d"})
	if mode != "" {
		t.Errorf("mode = %q, want empty", mode)
	}
	if env != "stg" {
		t.Errorf("env = %q, want %q", env, "stg")
	}
	if len(filtered) != 1 || filtered[0] != "-d" {
		t.Errorf("filtered = %v, want [-d]", filtered)
	}
}

func TestParseDvaFlags_BothFlags(t *testing.T) {
	mode, env, _, _, filtered := parseDvaFlags([]string{"-M", "docker", "-E", "prd", "--wait"})
	if mode != "docker" {
		t.Errorf("mode = %q, want %q", mode, "docker")
	}
	if env != "prd" {
		t.Errorf("env = %q, want %q", env, "prd")
	}
	if len(filtered) != 1 || filtered[0] != "--wait" {
		t.Errorf("filtered = %v, want [--wait]", filtered)
	}
}

func TestParseDvaFlags_EqualsSyntax(t *testing.T) {
	mode, env, _, _, filtered := parseDvaFlags([]string{"--mode=hybrid", "--env=stg"})
	if mode != "hybrid" {
		t.Errorf("mode = %q, want %q", mode, "hybrid")
	}
	if env != "stg" {
		t.Errorf("env = %q, want %q", env, "stg")
	}
	if len(filtered) != 0 {
		t.Errorf("filtered = %v, want empty", filtered)
	}
}

func TestParseDvaFlags_ShortEqualsSyntax(t *testing.T) {
	mode, env, _, _, _ := parseDvaFlags([]string{"-M=native", "-E=prd"})
	if mode != "native" {
		t.Errorf("mode = %q, want %q", mode, "native")
	}
	if env != "prd" {
		t.Errorf("env = %q, want %q", env, "prd")
	}
}

func TestParseDvaFlags_Empty(t *testing.T) {
	mode, env, _, excludeTags, filtered := parseDvaFlags(nil)
	if mode != "" || env != "" {
		t.Errorf("got mode=%q env=%q, want both empty", mode, env)
	}
	if excludeTags != nil {
		t.Errorf("excludeTags = %v, want nil", excludeTags)
	}
	if filtered != nil {
		t.Errorf("filtered = %v, want nil", filtered)
	}
}

func TestParseDvaFlags_DryRun(t *testing.T) {
	oldDryRun := dryRun
	dryRun = false
	t.Cleanup(func() { dryRun = oldDryRun })

	_, _, _, _, filtered := parseDvaFlags([]string{"--mode", "infra", "--dry-run"})
	if !dryRun {
		t.Fatal("--dry-run must enable dryRun for commands with flag parsing disabled")
	}
	if len(filtered) != 0 {
		t.Fatalf("filtered = %v, want empty", filtered)
	}
}

func TestConsumeDryRunFlag(t *testing.T) {
	filtered, found := consumeDryRunFlag([]string{"local-dev", "--dry-run", "--force"})
	if !found {
		t.Fatal("expected --dry-run to be detected")
	}
	if got := strings.Join(filtered, " "); got != "local-dev --force" {
		t.Fatalf("filtered = %q, want %q", got, "local-dev --force")
	}
}

func TestParseDvaFlags_MissingValue(t *testing.T) {
	// --mode at end with no value — should not panic
	mode, _, _, _, filtered := parseDvaFlags([]string{"--mode"})
	if mode != "" {
		t.Errorf("mode = %q, want empty (no value provided)", mode)
	}
	if len(filtered) != 0 {
		t.Errorf("filtered = %v, want empty", filtered)
	}
}

func TestParseDvaFlags_ExcludeTags(t *testing.T) {
	_, _, _, excludeTags, filtered := parseDvaFlags([]string{"--exclude-tags", "infra", "-d"})
	if len(excludeTags) != 1 || excludeTags[0] != "infra" {
		t.Errorf("excludeTags = %v, want [infra]", excludeTags)
	}
	if len(filtered) != 1 || filtered[0] != "-d" {
		t.Errorf("filtered = %v, want [-d]", filtered)
	}
}

func TestParseDvaFlags_ExcludeTagsCommaSeparated(t *testing.T) {
	_, _, _, excludeTags, _ := parseDvaFlags([]string{"--exclude-tags=infra,dev"})
	if len(excludeTags) != 2 {
		t.Fatalf("excludeTags = %v, want 2 items", excludeTags)
	}
	if excludeTags[0] != "infra" || excludeTags[1] != "dev" {
		t.Errorf("excludeTags = %v, want [infra dev]", excludeTags)
	}
}

func TestParseDvaFlags_IncludeTags(t *testing.T) {
	_, _, includeTags, _, filtered := parseDvaFlags([]string{"--tags", "backend", "-d"})
	if len(includeTags) != 1 || includeTags[0] != "backend" {
		t.Errorf("includeTags = %v, want [backend]", includeTags)
	}
	if len(filtered) != 1 || filtered[0] != "-d" {
		t.Errorf("filtered = %v, want [-d]", filtered)
	}
}

func TestParseDvaFlags_IncludeTagsCommaSeparated(t *testing.T) {
	_, _, includeTags, _, _ := parseDvaFlags([]string{"-T=backend,ui"})
	if len(includeTags) != 2 {
		t.Fatalf("includeTags = %v, want 2 items", includeTags)
	}
	if includeTags[0] != "backend" || includeTags[1] != "ui" {
		t.Errorf("includeTags = %v, want [backend ui]", includeTags)
	}
}

func TestResolveMode_Empty(t *testing.T) {
	c := &config.Config{}
	rm, err := resolveMode(c, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rm.Mode != nil {
		t.Error("Mode should be nil for empty mode")
	}
}

func TestResolveMode_NotFound(t *testing.T) {
	c := &config.Config{
		Modes: map[string]config.ModeConfig{
			"docker": {Description: "Docker mode"},
		},
	}
	_, err := resolveMode(c, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent mode")
	}
}

func TestResolveMode_NoModesDefined(t *testing.T) {
	c := &config.Config{}
	_, err := resolveMode(c, "native")
	if err == nil {
		t.Fatal("expected error when no modes defined")
	}
}

func TestResolveMode_Found(t *testing.T) {
	c := &config.Config{
		Modes: map[string]config.ModeConfig{
			"native": {
				Description: "Native mode",
				Stack:       []string{"api", "worker"},
			},
		},
	}
	rm, err := resolveMode(c, "native")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rm.Mode == nil {
		t.Fatal("Mode should not be nil")
	}
	if rm.Mode.Description != "Native mode" {
		t.Errorf("Description = %q, want %q", rm.Mode.Description, "Native mode")
	}
}

func TestApplyEnv_Empty(t *testing.T) {
	e := config.NewEnvironment(nil, "/tmp", "/tmp")
	c := &config.Config{}
	if err := applyEnv(e, c, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyEnv_NotFound(t *testing.T) {
	e := config.NewEnvironment(nil, "/tmp", "/tmp")
	c := &config.Config{
		Environments: map[string]config.EnvironmentProfile{
			"dev": {Description: "Dev"},
		},
	}
	if err := applyEnv(e, c, "nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent env")
	}
}

func TestApplyEnv_NoEnvironmentsDefined(t *testing.T) {
	e := config.NewEnvironment(nil, "/tmp", "/tmp")
	c := &config.Config{}
	if err := applyEnv(e, c, "stg"); err == nil {
		t.Fatal("expected error when no environments defined")
	}
}

func TestApplyEnv_MergesVars(t *testing.T) {
	e := config.NewEnvironment(nil, "/tmp", "/tmp")
	c := &config.Config{
		Environments: map[string]config.EnvironmentProfile{
			"stg": {
				Description: "Staging",
				Environment: map[string]string{
					"RAILS_ENV": "staging",
					"DB_HOST":   "stg-db",
				},
			},
		},
	}
	if err := applyEnv(e, c, "stg"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Vars["RAILS_ENV"] != "staging" {
		t.Errorf("RAILS_ENV = %q, want %q", e.Vars["RAILS_ENV"], "staging")
	}
	if e.Vars["DB_HOST"] != "stg-db" {
		t.Errorf("DB_HOST = %q, want %q", e.Vars["DB_HOST"], "stg-db")
	}
}

// --- buildComposeArgs tests ---

// mustComposeArgs fails the test if the builder rejects the config. Both builders
// grew an error return in TASK-115 — a `command:` that splits to no words is a
// config error now, where it used to index into a nil slice and panic — and every
// config below is well-formed, so an error here means the builder broke.
func mustComposeArgs(t *testing.T, e *config.Environment, c *config.Config, args []string) (string, []string) {
	t.Helper()
	cmd, argv, err := buildComposeArgs(e, c, args)
	if err != nil {
		t.Fatalf("buildComposeArgs returned an error: %v", err)
	}
	return cmd, argv
}

func mustComposeArgsForEntry(t *testing.T, e *config.Environment, c *config.Config, entry *config.LifecycleEntry, args []string) (string, []string) {
	t.Helper()
	cmd, argv, err := buildComposeArgsForEntry(e, c, entry, args)
	if err != nil {
		t.Fatalf("buildComposeArgsForEntry returned an error: %v", err)
	}
	return cmd, argv
}

func loadTestConfig(t *testing.T, yamlContent string) *config.Config {
	t.Helper()
	tmpDir := t.TempDir()
	dvaFile := filepath.Join(tmpDir, config.FileName)
	if err := os.WriteFile(dvaFile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestBuildComposeArgs_Default(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.22"
stack:
  compose:
    default_runner: compose
    order: 10
    runners:
      compose:
        files: [compose.yml]
`)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	cmd, args := mustComposeArgs(t, e, c, []string{"up", "-d"})
	if cmd != "docker" {
		t.Errorf("cmd = %q, want %q", cmd, "docker")
	}
	if args[0] != "compose" {
		t.Errorf("args[0] = %q, want 'compose'", args[0])
	}
	if !strings.Contains(strings.Join(args, " "), "-f") {
		t.Error("args should contain -f flag for compose file")
	}
	if args[len(args)-2] != "up" || args[len(args)-1] != "-d" {
		t.Errorf("args should end with 'up -d', got %v", args)
	}
}

func TestBuildComposeArgs_WithProjectName(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.22"
stack:
  compose:
    default_runner: compose
    order: 10
    runners:
      compose:
        project_name: myproject
`)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	_, args := mustComposeArgs(t, e, c, []string{"ps"})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--project-name") {
		t.Error("args should contain --project-name")
	}
	if !strings.Contains(joined, "myproject") {
		t.Errorf("args should contain project name, got: %s", joined)
	}
}

func TestBuildComposeArgs_MultipleFiles(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.22"
stack:
  compose:
    default_runner: compose
    order: 10
    runners:
      compose:
        files: [compose.yml, compose.override.yml]
`)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	_, args := mustComposeArgs(t, e, c, nil)
	fCount := 0
	for _, a := range args {
		if a == "-f" {
			fCount++
		}
	}
	if fCount != 2 {
		t.Errorf("-f count = %d, want 2", fCount)
	}
}

func TestParseDvaFlags_TagShortFlag(t *testing.T) {
	_, _, includeTags, _, filtered := parseDvaFlags([]string{"-T", "backend", "postgres"})
	if len(includeTags) != 1 || includeTags[0] != "backend" {
		t.Errorf("includeTags = %v, want [backend]", includeTags)
	}
	if len(filtered) != 1 || filtered[0] != "postgres" {
		t.Errorf("filtered = %v, want [postgres]", filtered)
	}
}

func TestParseDvaFlags_TagEqualsFormat(t *testing.T) {
	_, _, includeTags, _, _ := parseDvaFlags([]string{"--tag=web"})
	if len(includeTags) != 1 || includeTags[0] != "web" {
		t.Errorf("includeTags = %v, want [web]", includeTags)
	}
}

func TestParseDvaFlags_TagsEqualsFormat(t *testing.T) {
	_, _, includeTags, _, _ := parseDvaFlags([]string{"--tags=a,b"})
	if len(includeTags) != 2 {
		t.Errorf("includeTags = %v, want 2 items", includeTags)
	}
}

func TestParseDvaFlags_ExcludeTagEquals(t *testing.T) {
	_, _, _, excludeTags, _ := parseDvaFlags([]string{"--exclude-tag=db"})
	if len(excludeTags) != 1 || excludeTags[0] != "db" {
		t.Errorf("excludeTags = %v, want [db]", excludeTags)
	}
}

func TestParseDvaFlags_ExcludeTagsEquals(t *testing.T) {
	_, _, _, excludeTags, _ := parseDvaFlags([]string{"--exclude-tags=db,cache"})
	if len(excludeTags) != 2 {
		t.Errorf("excludeTags = %v, want 2 items", excludeTags)
	}
}

func TestParseDvaFlags_MissingEnvValue(t *testing.T) {
	_, env, _, _, _ := parseDvaFlags([]string{"--env"})
	if env != "" {
		t.Errorf("env = %q, want empty (no value)", env)
	}
}

func TestParseDvaFlags_MissingTagValue(t *testing.T) {
	_, _, includeTags, _, _ := parseDvaFlags([]string{"--tag"})
	if len(includeTags) != 0 {
		t.Errorf("includeTags = %v, want empty (no value)", includeTags)
	}
}

func TestParseDvaFlags_MissingExcludeTagValue(t *testing.T) {
	_, _, _, excludeTags, _ := parseDvaFlags([]string{"--exclude-tag"})
	if len(excludeTags) != 0 {
		t.Errorf("excludeTags = %v, want empty (no value)", excludeTags)
	}
}

func TestBuildComposeArgs_CustomCommand(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.22"
stack:
  compose:
    default_runner: compose
    order: 10
    runners:
      compose:
        command: "podman compose"
        files: [compose.yml]
`)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	cmd, args := mustComposeArgs(t, e, c, []string{"ps"})
	if cmd != "podman" {
		t.Errorf("cmd = %q, want 'podman'", cmd)
	}
	if args[0] != "compose" {
		t.Errorf("args[0] = %q, want 'compose'", args[0])
	}
}

func TestBuildComposeArgs_InterpolateFiles(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.22"
stack:
  compose:
    default_runner: compose
    order: 10
    runners:
      compose:
        files: [compose.yml]
        project_name: "${APP_NAME}"
`)
	e := config.NewEnvironment(map[string]string{"APP_NAME": "myapp"}, c.FileDir(), c.FileDir())

	_, args := mustComposeArgs(t, e, c, []string{"up"})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "myapp") {
		t.Errorf("args should contain interpolated project name 'myapp', got: %s", joined)
	}
}

func TestBuildComposeArgsForEntry_ReadsComposeRunner(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.22"
stack:
  compose:
    default_runner: compose
    order: 10
    runners:
      compose:
        command: "/bin/echo compose"
        files: [compose.yml]
        project_name: "${APP_NAME}"
`)
	e := config.NewEnvironment(map[string]string{"APP_NAME": "myapp"}, c.FileDir(), c.FileDir())
	entry := c.FindStackEntry("compose")
	if entry == nil {
		t.Fatal("compose entry not found")
	}

	cmd, args := mustComposeArgsForEntry(t, e, c, entry, []string{"ps"})
	joined := strings.Join(args, " ")
	if cmd != "/bin/echo" {
		t.Errorf("cmd = %q, want /bin/echo", cmd)
	}
	if !strings.Contains(joined, "-f "+c.FileDir()+"/compose.yml") {
		t.Errorf("args should contain compose file, got: %s", joined)
	}
	if !strings.Contains(joined, "--project-name myapp") {
		t.Errorf("args should contain interpolated project name, got: %s", joined)
	}
}

// --- suggestProvision tests ---

func TestSuggestProvision_AlreadyProvisioned(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.22"
provision:
  setup:
    - step: test
      run: echo ok
`)
	// Create marker file
	markerDir := filepath.Join(c.FileDir(), config.DotDirName)
	os.MkdirAll(markerDir, 0755)
	os.WriteFile(filepath.Join(markerDir, "provisioned-setup"), []byte(""), 0644)

	// Should not print anything (already provisioned)
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	suggestProvision(c, "setup")
	w.Close()
	os.Stderr = old

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	if n > 0 {
		t.Errorf("expected no output for already-provisioned, got: %s", string(buf[:n]))
	}
}

func TestSuggestProvision_NotProvisioned(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.22"
provision:
  setup:
    - step: test
      run: echo ok
`)

	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	suggestProvision(c, "setup")
	w.Close()
	os.Stderr = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])
	if !strings.Contains(output, "hint") {
		t.Errorf("expected hint message, got: %s", output)
	}
}

func TestSuggestProvision_ProfileNotExists(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.22"
`)

	// Should not print anything (profile doesn't exist)
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	suggestProvision(c, "nonexistent")
	w.Close()
	os.Stderr = old

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	if n > 0 {
		t.Errorf("expected no output for non-existent profile, got: %s", string(buf[:n]))
	}
}
