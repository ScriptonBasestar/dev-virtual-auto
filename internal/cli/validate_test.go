package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestPrintComposeNameWarnings_Missing(t *testing.T) {
	warnings := []config.ComposeNameWarning{
		{File: "compose.yml", DvaName: "myproject", ComposeName: ""},
	}

	// Capture stderr
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	printComposeNameWarnings(warnings)

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
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

	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	printComposeNameWarnings(warnings)

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "differs from") {
		t.Errorf("output should mention 'differs from', got: %s", output)
	}
	if !strings.Contains(output, "old-name") {
		t.Errorf("output should contain old name, got: %s", output)
	}
}

func TestPrintComposeNameWarnings_Empty(t *testing.T) {
	// No warnings should produce no output
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	printComposeNameWarnings(nil)

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
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

	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	printConfigDriftWarnings(detectConfigDriftWarnings(c))

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
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
	// Exact matches: DVA reserved commands and meta targets
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
