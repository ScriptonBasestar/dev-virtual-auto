package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestDetectTemplateIn(t *testing.T) {
	// Setup a temporary directory
	tempDir, err := os.MkdirTemp("", "dva-init-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(originalWd)

	// Test default (minimal)
	if tmpl := detectTemplateIn("."); tmpl != "minimal" {
		t.Errorf("Expected minimal, got %s", tmpl)
	}

	// Test node detection
	os.WriteFile("package.json", []byte("{}"), 0644)
	if tmpl := detectTemplateIn("."); tmpl != "node" {
		t.Errorf("Expected node, got %s", tmpl)
	}
	os.Remove("package.json")

	// Test go detection
	os.WriteFile("go.mod", []byte("module foo"), 0644)
	if tmpl := detectTemplateIn("."); tmpl != "go" {
		t.Errorf("Expected go, got %s", tmpl)
	}
	os.Remove("go.mod")
}

func TestGenerateConfig(t *testing.T) {
	// Setup a temporary directory
	tempDir, err := os.MkdirTemp("", "dva-init-config-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(originalWd)

	// Create a dummy compose file so it gets detected
	os.WriteFile("docker-compose.yml", []byte(""), 0644)

	config := generateConfigIn(".", "node")
	if !strings.Contains(config, `npm run dev`) {
		t.Errorf("Expected config to contain 'npm run dev', got:\n%s", config)
	}
	if !strings.Contains(config, `docker-compose.yml`) {
		t.Errorf("Expected config to contain 'docker-compose.yml', got:\n%s", config)
	}
}

func TestGenerateConfig_Rails(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	got := generateConfigIn(".", "rails")
	if !strings.Contains(got, "RAILS_ENV") {
		t.Error("rails config should contain RAILS_ENV")
	}
	if !strings.Contains(got, "bundle exec rspec") {
		t.Error("rails config should contain rspec command")
	}
}

func TestGenerateConfig_Go(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	got := generateConfigIn(".", "go")
	if !strings.Contains(got, "go test ./...") {
		t.Error("go config should contain go test")
	}
}

func TestGenerateConfig_Python(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	got := generateConfigIn(".", "python")
	if !strings.Contains(got, "PYTHONDONTWRITEBYTECODE") {
		t.Error("python config should contain PYTHONDONTWRITEBYTECODE")
	}
	if !strings.Contains(got, "pytest") {
		t.Error("python config should contain pytest")
	}
}

func TestGenerateConfig_Minimal(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	got := generateConfigIn(".", "minimal")
	if !strings.Contains(got, "/bin/bash") {
		t.Error("minimal config should contain shell command")
	}
	if !strings.Contains(got, "default_runner: compose") || !strings.Contains(got, "runners:") {
		t.Errorf("minimal config should use compose runner schema, got:\n%s", got)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte(got), 0644); err != nil {
		t.Fatalf("write generated config: %v", err)
	}
	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("Load generated config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate generated config: %v", err)
	}
	// Should NOT contain language-specific env
	if strings.Contains(got, "RAILS_ENV") || strings.Contains(got, "NODE_ENV") {
		t.Error("minimal config should not contain language-specific env")
	}
}

func TestGenerateConfig_NoComposeFiles(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// No compose files on disk → should fallback to docker-compose.yml
	got := generateConfigIn(".", "minimal")
	if !strings.Contains(got, "docker-compose.yml") {
		t.Error("should fallback to docker-compose.yml when no compose files exist")
	}
}

// TestScaffoldedVersionFloorDoesNotTrackBinary guards the ratchet: `version:` is the
// minimum DVA a config requires, so a config scaffolded by a newer binary must still
// load on an older one. The two constants are equal today, so moving Version is the
// only way to tell a real floor apart from a producer stamp.
func TestScaffoldedVersionFloorDoesNotTrackBinary(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	original := config.Version
	defer func() { config.Version = original }()
	config.Version = "9.9.9"

	got := generateConfigIn(".", "minimal")
	if strings.Contains(got, "9.9.9") {
		t.Error("scaffolded version: followed the running binary; it must state what the config requires")
	}
	if !strings.Contains(got, "version: \""+config.MinScaffoldVersion+"\"") {
		t.Errorf("scaffolded config must declare MinScaffoldVersion (%s); got:\n%s",
			config.MinScaffoldVersion, got)
	}
}

func TestDetectComposeFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dva-compose-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(originalWd)

	// No compose files initially
	files := detectComposeFiles()
	if len(files) != 0 {
		t.Errorf("Expected 0 files, got %d", len(files))
	}

	// Create some compose files
	os.WriteFile("docker-compose.yml", []byte(""), 0644)
	os.WriteFile("docker-compose.override.yml", []byte(""), 0644)
	os.Mkdir("db", 0755) // should be ignored

	files = detectComposeFiles()
	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}

	if files[0] != "docker-compose.yml" {
		t.Errorf("Expected primary file first, got %s", files[0])
	}
	if files[1] != "docker-compose.override.yml" {
		t.Errorf("Expected override file second, got %s", files[1])
	}
}

func TestInitCommandMentionsImproveFlow(t *testing.T) {
	if !strings.Contains(initCmd.Long, "am run dva-improve") {
		t.Fatalf("init long help should mention am run dva-improve, got: %s", initCmd.Long)
	}
	if !strings.Contains(initCmd.Long, "am run dva-discover") {
		t.Fatalf("init long help should mention am run dva-discover, got: %s", initCmd.Long)
	}
	if !strings.Contains(initCmd.Long, "am run dva-improve -p mode=rewrite") {
		t.Fatalf("init long help should document explicit rewrite syntax, got: %s", initCmd.Long)
	}
}

func TestFilterEnv(t *testing.T) {
	env := []string{"PATH=/usr/bin", "HOME=/root", "PATH=/extra"}
	got := filterEnv(env, "PATH")
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d: %v", len(got), got)
	}
	if got[0] != "HOME=/root" {
		t.Errorf("expected HOME=/root, got %s", got[0])
	}
}

func TestFilterEnv_NoMatch(t *testing.T) {
	env := []string{"HOME=/root", "USER=test"}
	got := filterEnv(env, "PATH")
	if len(got) != 2 {
		t.Errorf("expected 2 entries, got %d", len(got))
	}
}

func TestFilterEnv_Empty(t *testing.T) {
	got := filterEnv(nil, "PATH")
	if len(got) != 0 {
		t.Errorf("expected 0 entries, got %d", len(got))
	}
}

func TestExtractMakefileTargets(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	content := "build: ## Build the project\n\tgo build ./...\n\ntest: ## Run tests\n\tgo test ./...\n\n.PHONY: build test\n"
	os.WriteFile("Makefile", []byte(content), 0644)
	got := extractMakefileTargets()
	if !strings.Contains(got, "build") {
		t.Error("should contain 'build' target")
	}
	if !strings.Contains(got, "Build the project") {
		t.Error("should contain target description")
	}
}

func TestExtractMakefileTargets_NoMakefile(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	got := extractMakefileTargets()
	if got != "" {
		t.Errorf("expected empty for no Makefile, got %q", got)
	}
}

func TestExtractComposeServices(t *testing.T) {
	tmpDir := t.TempDir()
	compose := filepath.Join(tmpDir, "docker-compose.yml")
	content := "version: '3.8'\nservices:\n  postgres:\n    image: postgres:15\n  redis:\n    image: redis:7\nvolumes:\n  data:\n"
	os.WriteFile(compose, []byte(content), 0644)

	services := extractComposeServices(compose)
	if len(services) < 2 {
		t.Fatalf("expected at least 2 services, got %d: %v", len(services), services)
	}
	found := map[string]bool{}
	for _, s := range services {
		found[s] = true
	}
	if !found["postgres"] {
		t.Error("should find 'postgres' service")
	}
	if !found["redis"] {
		t.Error("should find 'redis' service")
	}
}

func TestExtractComposeServices_NoFile(t *testing.T) {
	services := extractComposeServices("/nonexistent/compose.yml")
	if services != nil {
		t.Errorf("expected nil for nonexistent file, got %v", services)
	}
}

func TestDetectInfraComposeFiles(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// No infra dirs
	files := detectInfraComposeFiles()
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}

	// Create infra dir with compose file
	os.MkdirAll("infra", 0755)
	os.WriteFile("infra/compose.yml", []byte("services:\n  pg:\n"), 0644)
	files = detectInfraComposeFiles()
	if len(files) != 1 || files[0] != "infra/compose.yml" {
		t.Errorf("expected [infra/compose.yml], got %v", files)
	}
}

func TestDetectInfraComposeFiles_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	os.MkdirAll("infra", 0755)
	os.MkdirAll("docker", 0755)
	os.WriteFile("infra/docker-compose.yml", []byte(""), 0644)
	os.WriteFile("docker/compose.yaml", []byte(""), 0644)

	files := detectInfraComposeFiles()
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}
}

// TestInitPublicSurfaceCompatibility pins TASK-250's canonical-path contract:
// all five templates, the four scaffolding flags, `config init`, and the
// top-level `init` alias all resolve through scaffoldDvaYml/generateConfigIn,
// and TASK-249's compose-only / native-only / hybrid / no-discovery discovery
// outcomes behave as decided.
func TestInitPublicSurfaceCompatibility(t *testing.T) {
	t.Run("five templates remain selectable and produce valid compose-only configs", func(t *testing.T) {
		for _, tmpl := range []string{"minimal", "rails", "node", "python", "go"} {
			t.Run(tmpl, func(t *testing.T) {
				tmpDir := t.TempDir()
				oldDir, _ := os.Getwd()
				os.Chdir(tmpDir)
				defer os.Chdir(oldDir)

				os.WriteFile("docker-compose.yml", []byte("services:\n  app:\n    image: busybox\n"), 0644)

				created, err := scaffoldDvaYml(".", tmpl)
				if err != nil || !created {
					t.Fatalf("scaffoldDvaYml(%q) = (%v, %v), want (true, nil)", tmpl, created, err)
				}
				cfg, err := config.Load(tmpDir)
				if err != nil {
					t.Fatalf("Load: %v", err)
				}
				if err := cfg.Validate(); err != nil {
					t.Fatalf("Validate: %v", err)
				}
			})
		}
	})

	t.Run("four flags are registered on both config init and the top-level alias", func(t *testing.T) {
		for _, cmd := range []*cobra.Command{initCmd, initAliasCmd} {
			for _, flag := range []string{"template", "recursive", "devcontainer", "all"} {
				if cmd.Flags().Lookup(flag) == nil {
					t.Errorf("%s: missing --%s flag", cmd.CommandPath(), flag)
				}
			}
		}
		if initCmd.Flags().ShorthandLookup("t") == nil {
			t.Error("config init: missing -t shorthand for --template")
		}
	})

	t.Run("config init is registered under config and init remains a visible top-level alias", func(t *testing.T) {
		found := false
		for _, c := range configCmd.Commands() {
			if c.Name() == "init" {
				found = true
			}
		}
		if !found {
			t.Error("expected 'init' registered under 'config'")
		}
		if initAliasCmd.Hidden {
			t.Error("top-level 'init' alias must stay visible for backward compatibility")
		}
		topFound := false
		for _, c := range rootCmd.Commands() {
			if c.Name() == "init" {
				topFound = true
			}
		}
		if !topFound {
			t.Error("expected top-level 'init' alias registered on rootCmd")
		}
	})

	t.Run("config init and the top-level alias invoke the same canonical RunE", func(t *testing.T) {
		dirA := t.TempDir()
		dirB := t.TempDir()
		os.WriteFile(filepath.Join(dirA, "docker-compose.yml"), []byte(""), 0644)
		os.WriteFile(filepath.Join(dirB, "docker-compose.yml"), []byte(""), 0644)

		origWd, _ := os.Getwd()
		defer os.Chdir(origWd)

		savedTemplate, savedRecursive, savedDevcontainer, savedAll := initTemplate, initRecursive, initDevcontainer, initAll
		defer func() {
			initTemplate, initRecursive, initDevcontainer, initAll = savedTemplate, savedRecursive, savedDevcontainer, savedAll
		}()
		initTemplate, initRecursive, initDevcontainer, initAll = "node", false, false, false

		// initAliasCmd.RunE is literally the same function value as
		// initCmd.RunE (see init.go's initAliasCmd construction) — invoking
		// both here exercises that shared function, not just two direct
		// scaffoldDvaYml calls standing in for command-level equivalence.
		if err := os.Chdir(dirA); err != nil {
			t.Fatalf("chdir dirA: %v", err)
		}
		if err := initCmd.RunE(initCmd, nil); err != nil {
			t.Fatalf("initCmd.RunE: %v", err)
		}
		if err := os.Chdir(dirB); err != nil {
			t.Fatalf("chdir dirB: %v", err)
		}
		if err := initAliasCmd.RunE(initAliasCmd, nil); err != nil {
			t.Fatalf("initAliasCmd.RunE: %v", err)
		}

		a, _ := os.ReadFile(filepath.Join(dirA, config.FileName))
		b, _ := os.ReadFile(filepath.Join(dirB, config.FileName))
		if string(a) != string(b) {
			t.Fatalf("expected identical output from the shared RunE, got:\nA:\n%s\nB:\n%s", a, b)
		}
	})

	t.Run("compose-only: Compose file present, no language manifest", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte(""), 0644)

		outcome, composeFiles, nativeLang := classifyDiscovery(tmpDir)
		if outcome != outcomeComposeOnly {
			t.Fatalf("expected outcomeComposeOnly, got %v", outcome)
		}
		if len(composeFiles) == 0 || nativeLang != "" {
			t.Fatalf("compose-only classification carried unexpected evidence: files=%v lang=%q", composeFiles, nativeLang)
		}
	})

	t.Run("native-only: language manifest present, no Compose file, no guessed runner", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/x\n"), 0644)

		outcome, composeFiles, nativeLang := classifyDiscovery(tmpDir)
		if outcome != outcomeNativeOnly {
			t.Fatalf("expected outcomeNativeOnly, got %v", outcome)
		}
		if len(composeFiles) != 0 || nativeLang != "go" {
			t.Fatalf("native-only classification mismatch: files=%v lang=%q", composeFiles, nativeLang)
		}

		created, err := scaffoldDvaYml(tmpDir, "")
		if err != nil || !created {
			t.Fatalf("scaffoldDvaYml native-only = (%v, %v), want (true, nil)", created, err)
		}
		data, _ := os.ReadFile(filepath.Join(tmpDir, config.FileName))
		content := string(data)
		if !strings.Contains(content, "native") {
			// documentation comment should at least mention how to add one
			t.Errorf("native-only output should document how to add a native runner manually, got:\n%s", content)
		}
		cfg, err := config.Load(tmpDir)
		if err != nil {
			t.Fatalf("Load native-only config: %v", err)
		}
		if len(cfg.Stack) != 0 {
			t.Errorf("native-only output must not author a stack entry without verified evidence, got: %+v", cfg.Stack)
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate native-only config: %v", err)
		}
	})

	t.Run("hybrid: both Compose file and language manifest present prefers Compose", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte(""), 0644)
		os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte("{}"), 0644)

		outcome, composeFiles, nativeLang := classifyDiscovery(tmpDir)
		if outcome != outcomeHybrid {
			t.Fatalf("expected outcomeHybrid, got %v", outcome)
		}
		if len(composeFiles) == 0 || nativeLang != "node" {
			t.Fatalf("hybrid classification mismatch: files=%v lang=%q", composeFiles, nativeLang)
		}

		created, err := scaffoldDvaYml(tmpDir, "")
		if err != nil || !created {
			t.Fatalf("scaffoldDvaYml hybrid = (%v, %v), want (true, nil)", created, err)
		}
		data, _ := os.ReadFile(filepath.Join(tmpDir, config.FileName))
		if !strings.Contains(string(data), "stack:") {
			t.Errorf("hybrid output should still generate the verified Compose stack, got:\n%s", data)
		}
	})

	t.Run("no-discovery: neither Compose file nor language manifest rejects and writes nothing", func(t *testing.T) {
		tmpDir := t.TempDir()

		outcome, _, _ := classifyDiscovery(tmpDir)
		if outcome != outcomeNoDiscovery {
			t.Fatalf("expected outcomeNoDiscovery, got %v", outcome)
		}

		// This is a plain error return, not a preview/dry-run mechanism — DVA
		// has no preview feature. Ambiguous/no evidence means scaffoldDvaYml
		// refuses outright and writes nothing; it does not stage a preview.
		created, err := scaffoldDvaYml(tmpDir, "")
		if created || err == nil {
			t.Fatalf("scaffoldDvaYml no-discovery = (%v, %v), want (false, non-nil)", created, err)
		}
		if _, statErr := os.Stat(filepath.Join(tmpDir, config.FileName)); !os.IsNotExist(statErr) {
			t.Fatalf("no-discovery must not write dva.yml, stat err = %v", statErr)
		}
	})

	t.Run("no-overwrite and idempotence: an existing dva.yml is never modified", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte(""), 0644)

		existing := "version: \"0.0.1\"\n# hand-authored\n"
		os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte(existing), 0644)

		created, err := scaffoldDvaYml(tmpDir, "node")
		if err != nil {
			t.Fatalf("scaffoldDvaYml: %v", err)
		}
		if created {
			t.Fatal("scaffoldDvaYml must not overwrite an existing dva.yml")
		}
		data, _ := os.ReadFile(filepath.Join(tmpDir, config.FileName))
		if string(data) != existing {
			t.Fatalf("existing dva.yml content changed:\nwant:\n%s\ngot:\n%s", existing, data)
		}

		// Calling again is idempotent: still a no-op, still unchanged.
		created2, err2 := scaffoldDvaYml(tmpDir, "node")
		if created2 || err2 != nil {
			t.Fatalf("second scaffoldDvaYml call = (%v, %v), want (false, nil)", created2, err2)
		}
		data2, _ := os.ReadFile(filepath.Join(tmpDir, config.FileName))
		if string(data2) != existing {
			t.Fatalf("existing dva.yml content changed on second call")
		}
	})
}

// TestInitDoesNotAuthorRejectedPlanLabels guards TASK-250 completion criterion 5:
// the Go `dva init` generator must never author the rejected plan labels
// local-infra, local-dev, or full-stack as a default, across every discovery
// outcome and template. (TASK-233's `am` preset corpus is a separate surface
// and is explicitly out of scope for this guard — see the TASK-249 Decision
// Record's "Go init generator only" ruling.)
func TestInitDoesNotAuthorRejectedPlanLabels(t *testing.T) {
	rejected := []string{"local-infra", "local-dev", "full-stack"}

	check := func(t *testing.T, label, content string) {
		t.Helper()
		for _, r := range rejected {
			if strings.Contains(content, r) {
				t.Errorf("%s: generated content must never author rejected plan label %q, got:\n%s", label, r, content)
			}
		}
	}

	for _, tmpl := range []string{"minimal", "rails", "node", "python", "go"} {
		content := generateConfigIn(".", tmpl)
		check(t, "compose-only template="+tmpl, content)
	}

	for _, lang := range []string{"", "go", "node", "python", "rails"} {
		content := generateNativeOnlyConfigIn(lang)
		check(t, "native-only lang="+lang, content)
	}
}
