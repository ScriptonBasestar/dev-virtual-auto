package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestPrintComposeNameWarnings_Missing(t *testing.T) {
	warnings := []config.ComposeNameWarning{
		{File: "compose.yml", DvaName: "myproject", ComposeName: ""},
	}

	var buf bytes.Buffer
	printComposeNameWarnings(&buf, warnings)
	output := buf.String()

	if !strings.Contains(output, "missing top-level") {
		t.Errorf("output should mention 'missing top-level', got: %s", output)
	}
	if !strings.Contains(output, "myproject") {
		t.Errorf("output should contain project name 'myproject', got: %s", output)
	}
}

func TestPrintComposeNameWarnings_Mismatch(t *testing.T) {
	warnings := []config.ComposeNameWarning{
		{File: "compose.yml", DvaName: "myproject", ComposeName: "old-name"},
	}

	var buf bytes.Buffer
	printComposeNameWarnings(&buf, warnings)
	output := buf.String()

	if !strings.Contains(output, "differs from") {
		t.Errorf("output should mention 'differs from', got: %s", output)
	}
	if !strings.Contains(output, "old-name") {
		t.Errorf("output should contain old name, got: %s", output)
	}
}

func TestPrintComposeNameWarnings_Empty(t *testing.T) {
	var buf bytes.Buffer
	printComposeNameWarnings(&buf, nil)
	if buf.Len() > 0 {
		t.Errorf("expected no output for empty warnings, got: %s", buf.String())
	}
}

func TestDetectConfigDriftWarnings_ComposeFilesMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte("services:\n  app:\n    image: nginx\n"), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "docker-compose.override.yml"), []byte("services:\n  app:\n    environment:\n      FOO: bar\n"), 0644); err != nil {
		t.Fatalf("write override: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte("version: \"0.1.0\"\nstack:\n  compose:\n    default_runner: compose\n    order: 10\n    runners:\n      compose:\n        files:\n          - docker-compose.yml\n"), 0644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}

	c, err := config.Load(".")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	warnings := detectConfigDriftWarnings(c)
	if len(warnings) == 0 {
		t.Fatal("expected drift warning for compose.files mismatch")
	}
	if !strings.Contains(warnings[0], "compose.files") {
		t.Fatalf("unexpected warning: %s", warnings[0])
	}
}

func TestDetectConfigDriftWarnings_ModernComposeOverlaysMatch(t *testing.T) {
	tmpDir := t.TempDir()
	for _, name := range []string{"compose.yaml", "compose.tools.yaml", "compose.monitor.yaml"} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("services: {}\n"), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte(`version: "0.1.44"
stack:
  compose:
    default_runner: compose
    runners:
      compose:
        files: [compose.yaml, compose.tools.yaml, compose.monitor.yaml]
`), 0644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}

	c, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if warnings := detectConfigDriftWarnings(c); len(warnings) != 0 {
		t.Fatalf("expected no compose drift warning, got %v", warnings)
	}
}

func TestDetectConfigDriftWarnings_IgnoresConfiguredSubdirectoryComposeFiles(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmpDir, "compose"), 0755); err != nil {
		t.Fatalf("mkdir compose: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "compose.yaml"), []byte("services: {}\n"), 0644); err != nil {
		t.Fatalf("write root compose: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "compose", "minor-guardian-e2e.yaml"), []byte("services: {}\n"), 0644); err != nil {
		t.Fatalf("write isolated compose: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte(`version: "0.1.44"
stack:
  minor-guardian-e2e-infra:
    default_runner: compose
    runners:
      compose:
        files: [compose/minor-guardian-e2e.yaml]
        project_name: isolated-e2e
  compose:
    default_runner: compose
    runners:
      compose:
        files: [compose.yaml]
        project_name: app
plans:
  local-infra:
    entries:
      - name: compose
`), 0644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}

	c, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if warnings := detectConfigDriftWarnings(c); len(warnings) != 0 {
		t.Fatalf("expected no compose drift warning for configured subdirectory file, got %v", warnings)
	}
}

func TestDetectConfigDriftWarnings_MissingConfiguredRootComposeFile(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte(`version: "0.1.44"
stack:
  compose:
    default_runner: compose
    runners:
      compose:
        files: [compose.yaml]
`), 0644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}

	c, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	warnings := detectConfigDriftWarnings(c)
	if len(warnings) != 2 || !strings.Contains(warnings[0], "compose.files is compose.yaml but detected root compose files are (none)") || !strings.Contains(warnings[1], `compose file "compose.yaml" is configured by dva.yml but does not exist`) {
		t.Fatalf("expected missing root compose drift warning, got %v", warnings)
	}
}

func TestDetectConfigDriftWarnings_MissingConfiguredSubdirectoryAndAbsoluteComposeFiles(t *testing.T) {
	tmpDir := t.TempDir()
	missingAbsolute := filepath.Join(t.TempDir(), "missing-absolute.yaml")
	if err := os.Mkdir(filepath.Join(tmpDir, "compose"), 0755); err != nil {
		t.Fatalf("mkdir compose: %v", err)
	}
	dvaConfig := fmt.Sprintf(`version: "0.1.44"
stack:
  subdirectory:
    default_runner: compose
    runners:
      compose:
        files: [compose/missing.yaml, compose/missing.yaml]
  absolute:
    default_runner: compose
    runners:
      compose:
        files: [%q]
`, missingAbsolute)
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte(dvaConfig), 0644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}

	c, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	warnings := detectConfigDriftWarnings(c)
	if len(warnings) != 2 {
		t.Fatalf("expected one warning per missing resolved path, got %v", warnings)
	}
	if !strings.Contains(strings.Join(warnings, "\n"), `compose file "compose/missing.yaml" is configured by dva.yml but does not exist`) {
		t.Fatalf("missing subdirectory compose warning not found: %v", warnings)
	}
	if !strings.Contains(strings.Join(warnings, "\n"), missingAbsolute) {
		t.Fatalf("missing absolute compose warning not found: %v", warnings)
	}
}

func TestDetectConfigDriftWarnings_ResolvesPathSourcedComposeFilesPerEntry(t *testing.T) {
	tmpDir := t.TempDir()
	for _, source := range []string{"source-a", "source-b"} {
		if err := os.Mkdir(filepath.Join(tmpDir, source), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", source, err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "source-a", "compose.yaml"), []byte("services: {}\n"), 0644); err != nil {
		t.Fatalf("write source-a compose: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte(`version: "0.1.44"
stack:
  source-a:
    default_runner: compose
    source:
      path: source-a
    runners:
      compose:
        files: [compose.yaml]
  source-b:
    default_runner: compose
    source:
      path: source-b
    runners:
      compose:
        files: [compose.yaml]
`), 0644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}

	c, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	warnings := detectConfigDriftWarnings(c)
	if len(warnings) != 1 {
		t.Fatalf("expected missing compose only for source-b, got %v", warnings)
	}
	if !strings.Contains(warnings[0], `compose file "compose.yaml (stack entry \"source-b\")"`) {
		t.Fatalf("missing source-b warning not found: %v", warnings)
	}
}

func TestDetectConfigDriftWarnings_DefersUnavailableGitSource(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte(`version: "0.1.44"
stack:
  remote:
    default_runner: compose
    source:
      git: https://example.com/shared-infra.git
      ref: v1
    runners:
      compose:
        files: [compose.yaml]
`), 0644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}

	c, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if warnings := detectConfigDriftWarnings(c); len(warnings) != 0 {
		t.Fatalf("expected unavailable git source to defer to source readiness, got %v", warnings)
	}
}

func TestDetectConfigDriftWarnings_SourceStackFixtureHasNoFalseComposeWarnings(t *testing.T) {
	fixture := filepath.Join("..", "integration", "testdata", "fixtures", "source-stack")
	c, err := config.Load(fixture)
	if err != nil {
		t.Fatalf("load source-stack fixture: %v", err)
	}
	if warnings := detectConfigDriftWarnings(c); len(warnings) != 0 {
		t.Fatalf("expected no root or missing compose warnings for source-stack fixture, got %v", warnings)
	}
}

func TestDetectConfigDriftWarnings_ResolvesComposeFilesWithEntryVars(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "compose.dev.yml"), []byte("services:\n  api:\n    image: nginx\n"), 0644); err != nil {
		t.Fatalf("write entry-resolved compose: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte(`version: "0.1.44"
stack:
  compose:
    default_runner: compose
    vars:
      STAGE: dev
    runners:
      compose:
        files: ["compose.${STAGE}.yml"]
interaction:
  api-shell:
    service: api
    command: sh
`), 0644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}

	c, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if warnings := detectConfigDriftWarnings(c); len(warnings) != 0 {
		t.Fatalf("expected entry vars to resolve compose path and service discovery, got %v", warnings)
	}
	if !configuredComposeServices(c)["api"] {
		t.Fatal("service discovery did not use entry-resolved compose file")
	}
}

func TestDetectConfigDriftWarnings_DefersPlanOrSiteDrivenComposePath(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "compose.yaml"), []byte("services: {}\n"), 0644); err != nil {
		t.Fatalf("write root compose: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte(`version: "0.1.44"
stack:
  compose:
    default_runner: compose
    runners:
      compose:
        files: ["compose.${STAGE}.yml"]
plans:
  local:
    site: local
    entries:
      - name: compose
        vars:
          STAGE: dev
sites:
  local:
    vars:
      STAGE: dev
`), 0644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}

	c, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if warnings := detectConfigDriftWarnings(c); len(warnings) != 0 {
		t.Fatalf("expected unresolved plan/site path to defer static drift checks, got %v", warnings)
	}
}

func TestMissingConfiguredComposeFiles_DeduplicatesEntryResolvedPath(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte(`version: "0.1.44"
stack:
  compose:
    default_runner: compose
    vars:
      STAGE: dev
    runners:
      compose:
        files: ["compose.${STAGE}.yml", compose.dev.yml]
`), 0644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}

	c, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	missing := missingConfiguredComposeFiles(c)
	if len(missing) != 1 || missing[0] != "compose.${STAGE}.yml" {
		t.Fatalf("expected one missing resolved path, got %v", missing)
	}
}

func TestDetectConfigDriftWarnings_MissingService(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte("services:\n  web:\n    image: nginx\n"), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte("version: \"0.1.0\"\nstack:\n  compose:\n    default_runner: compose\n    order: 10\n    runners:\n      compose:\n        files:\n          - docker-compose.yml\ninteraction:\n  test:\n    service: app\n    command: go test ./...\n"), 0644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}

	c, err := config.Load(".")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	warnings := detectConfigDriftWarnings(c)
	if len(warnings) == 0 {
		t.Fatal("expected service drift warning")
	}

	found := false
	for _, warning := range warnings {
		if strings.Contains(warning, `references compose service "app"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected missing-service warning, got: %v", warnings)
	}
}

func TestPrintConfigDriftWarnings(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte("services:\n  web:\n    image: nginx\n"), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte("version: \"0.1.0\"\nstack:\n  compose:\n    default_runner: compose\n    order: 10\n    runners:\n      compose:\n        files:\n          - docker-compose.yml\ninteraction:\n  test:\n    service: app\n    command: go test ./...\n"), 0644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}

	c, err := config.Load(".")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	var buf bytes.Buffer
	printConfigDriftWarnings(&buf, detectConfigDriftWarnings(c))
	output := buf.String()
	if !strings.Contains(output, "[warn] config drift:") {
		t.Fatalf("expected config drift warning prefix, got: %s", output)
	}
}

func TestDetectConfigSuggestionWarnings_FromMakefileAndPackageJSON(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	makefile := "build: ## Build project\nlint: ## Run lint\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte(makefile), 0644); err != nil {
		t.Fatalf("write Makefile: %v", err)
	}
	packageJSON := `{"scripts":{"dev":"vite","test":"vitest","pretest":"echo pre"}}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte("version: \"0.1.0\"\ninteraction:\n  build:\n    runner: local\n    command: make build\n"), 0644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}

	c, err := config.Load(".")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	warnings := detectConfigSuggestionWarnings(c)
	if len(warnings) == 0 {
		t.Fatal("expected suggestion warnings")
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, `Makefile defines "lint"`) {
		t.Fatalf("expected Makefile suggestion, got: %s", joined)
	}
	if !strings.Contains(joined, `package.json defines "dev"`) || !strings.Contains(joined, `package.json defines "test"`) {
		t.Fatalf("expected package.json suggestions, got: %s", joined)
	}
	if strings.Contains(joined, "pretest") {
		t.Fatalf("pre/post scripts should be ignored, got: %s", joined)
	}
}

func TestShouldIgnoreMakefileTarget(t *testing.T) {
	// Exact matches: DVA reserved commands and meta targets — plus `ps` and `clean`, which
	// are ignored on their own merit and not because a built-in shares the name. `clean`
	// stayed on this list when the built-in was removed; see the grouping in validate.go.
	exactIgnored := []string{
		"help", "all", "default",
		"stop", "up", "down", "restart",
		"ps", "run", config.LogsDirName, "build", "clean",
		"infra-up", "infra-down", "infra-start", "infra-stop",
		"deps", "install", "prepare", "setup", "install-hooks",
		"docs", "docs-build", "docs-serve",
	}
	for _, name := range exactIgnored {
		if !shouldIgnoreMakefileTarget(name) {
			t.Errorf("expected %q to be ignored (exact match)", name)
		}
	}

	// Suffix patterns: compose lifecycle
	suffixIgnored := []string{
		"dev-full-up", "dev-full-down", "dev-full-logs", "dev-full-ps",
		"dev-full-build",
		"e2e-up", "e2e-down", "e2e-stop", "e2e-restart",
		"app-logs", "backend-ps", "frontend-build",
	}
	for _, name := range suffixIgnored {
		if !shouldIgnoreMakefileTarget(name) {
			t.Errorf("expected %q to be ignored (suffix pattern)", name)
		}
	}

	// Should NOT be ignored: legitimate development workflow targets
	kept := []string{
		"build-ce", "build-ee", "build-mirror", "build-all",
		"test-ce", "test-all", "test-cloud",
		"e2e-smoke", "e2e-full", "e2e-rust",
		"clippy", "clippy-all", "fmt-check", "check-all",
		"lint", "dev", "test",
	}
	for _, name := range kept {
		if shouldIgnoreMakefileTarget(name) {
			t.Errorf("expected %q to be kept, but was ignored", name)
		}
	}
}

func TestIsDVAWrapperRecipe(t *testing.T) {
	wrappers := [][]string{
		{"dva up"},
		{"dva down"},
		{"dva up", "dva status"},
	}
	for _, recipe := range wrappers {
		if !isDVAWrapperRecipe(recipe) {
			t.Errorf("expected %v to be DVA wrapper", recipe)
		}
	}

	notWrappers := [][]string{
		{},
		{"go build ./..."},
		{"dva up", "echo done"},
		{"cargo test"},
		{"docker compose up -d"},
	}
	for _, recipe := range notWrappers {
		if isDVAWrapperRecipe(recipe) {
			t.Errorf("expected %v NOT to be DVA wrapper", recipe)
		}
	}
}

func TestMatchesSuggestionIgnore(t *testing.T) {
	patterns := []string{"*-release", "clippy*", "test-e2e-*"}

	matches := []string{"build-ce-release", "clippy", "clippy-all", "test-e2e-ci", "test-e2e-dev"}
	for _, name := range matches {
		if !matchesSuggestionIgnore(name, patterns) {
			t.Errorf("expected %q to match suggestion_ignore patterns", name)
		}
	}

	noMatches := []string{"build-ce", "test-ce", "e2e-smoke", "clipboard", "lint"}
	for _, name := range noMatches {
		if matchesSuggestionIgnore(name, patterns) {
			t.Errorf("expected %q NOT to match suggestion_ignore patterns", name)
		}
	}

	// Empty patterns never match
	if matchesSuggestionIgnore("anything", nil) {
		t.Error("empty patterns should never match")
	}
}

func TestDetectConfigSuggestionWarnings_SubcommandCoverage(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	makefile := "build-ce: ## Build CE edition\nbuild-ee: ## Build EE edition\norphan: ## No coverage\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte(makefile), 0644); err != nil {
		t.Fatalf("write Makefile: %v", err)
	}
	// cargo:build has subcommands ce and ee — should suppress build-ce and build-ee warnings
	dvaYml := "version: \"0.1.0\"\ninteraction:\n  cargo:build:\n    runner: local\n    command: cargo build\n    subcommands:\n      ce:\n        command: cargo build --features ce\n      ee:\n        command: cargo build --features ee\n"
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte(dvaYml), 0644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}

	c, err := config.Load(".")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	warnings := detectConfigSuggestionWarnings(c)
	joined := strings.Join(warnings, "\n")
	if strings.Contains(joined, `"build-ce"`) {
		t.Errorf("build-ce should be suppressed by subcommand coverage, got: %s", joined)
	}
	if strings.Contains(joined, `"build-ee"`) {
		t.Errorf("build-ee should be suppressed by subcommand coverage, got: %s", joined)
	}
	if !strings.Contains(joined, `"orphan"`) {
		t.Errorf("orphan should still warn (not covered), got: %s", joined)
	}
}

func TestDetectConfigSuggestionWarnings_SuggestionIgnore(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	makefile := "build-ce-release: ## Release build\nclippy: ## Lint\norphan: ## No coverage\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte(makefile), 0644); err != nil {
		t.Fatalf("write Makefile: %v", err)
	}
	dvaYml := "version: \"0.1.0\"\nsuggestion_ignore:\n  - \"*-release\"\n  - \"clippy*\"\n"
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte(dvaYml), 0644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}

	c, err := config.Load(".")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	warnings := detectConfigSuggestionWarnings(c)
	joined := strings.Join(warnings, "\n")
	if strings.Contains(joined, `"build-ce-release"`) {
		t.Errorf("build-ce-release should be suppressed by suggestion_ignore, got: %s", joined)
	}
	if strings.Contains(joined, `"clippy"`) {
		t.Errorf("clippy should be suppressed by suggestion_ignore, got: %s", joined)
	}
	if !strings.Contains(joined, `"orphan"`) {
		t.Errorf("orphan should still warn (not ignored), got: %s", joined)
	}
}

func TestShouldIgnorePackageScript(t *testing.T) {
	for _, name := range []string{"pretest", "postinstall", "prepare"} {
		if !shouldIgnorePackageScript(name) {
			t.Fatalf("expected %q to be ignored", name)
		}
	}
	for _, name := range []string{"test", "dev", "build"} {
		if shouldIgnorePackageScript(name) {
			t.Fatalf("expected %q to be kept", name)
		}
	}
}

func TestFixComposeNameWarnings_AddsMissingName(t *testing.T) {
	c := loadTestConfig(t, "version: \"0.1.22\"\nstack:\n  compose:\n    default_runner: compose\n    order: 10\n    runners:\n      compose:\n        project_name: myproject\n        files: [compose.yml]\n")

	// Create compose file without name
	composePath := filepath.Join(c.FileDir(), "compose.yml")
	os.WriteFile(composePath, []byte("services:\n  web:\n    image: nginx\n"), 0644)

	warnings := []config.ComposeNameWarning{
		{File: composePath, DvaName: "myproject", ComposeName: ""},
	}

	fixComposeNameWarnings(c, warnings)

	// Verify compose file was updated
	data, _ := os.ReadFile(composePath)
	if !strings.Contains(string(data), "name:") {
		t.Error("compose file should now contain 'name:' field")
	}
}

func TestFixComposeNameWarnings_Empty(t *testing.T) {
	c := &config.Config{}
	// Should not panic with empty warnings
	fixComposeNameWarnings(c, nil)
}

// --- TASK-115: a command: that splits to no words ---

// Each of these passes JSON-schema validation — to the schema they are strings — and each
// one made every compose runner index into a nil slice. `dva validate` reporting them is
// the difference between finding out at edit time and finding out from a stack trace.
func TestDetectUnrunnableComposeCommands_Reports(t *testing.T) {
	for _, command := range []string{"   ", "\t", `''`, `""`} {
		t.Run(strings.ReplaceAll(command, "\t", "<tab>"), func(t *testing.T) {
			c := loadTestConfig(t, "version: \"0.1.22\"\nstack:\n  db:\n    default_runner: compose\n    runners:\n      compose:\n        command: "+strconv.Quote(command)+"\n        files: [compose.yml]\n")

			problems := detectUnrunnableComposeCommands(c)
			if len(problems) != 1 {
				t.Fatalf("problems = %v, want exactly 1 for command %q", problems, command)
			}
			// The author has to be able to find the line. A message that says only "invalid
			// command" sends them looking through the whole file.
			for _, want := range []string{"stack.db", "command"} {
				if !strings.Contains(problems[0], want) {
					t.Errorf("problem does not mention %q:\n%s", want, problems[0])
				}
			}
		})
	}
}

func TestDetectUnrunnableComposeCommands_AcceptsRealCommands(t *testing.T) {
	for _, command := range []string{"", "docker compose", "podman-compose", "docker --context remote compose"} {
		t.Run(command, func(t *testing.T) {
			c := loadTestConfig(t, "version: \"0.1.22\"\nstack:\n  db:\n    default_runner: compose\n    runners:\n      compose:\n        command: "+strconv.Quote(command)+"\n        files: [compose.yml]\n")

			if problems := detectUnrunnableComposeCommands(c); len(problems) != 0 {
				t.Errorf("command %q was rejected: %v", command, problems)
			}
		})
	}
}
