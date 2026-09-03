package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

var initTemplate string
var initRecursive bool
var initDevcontainer bool
var initAll bool

// initAliasCmd is the top-level `dva init` backward-compatibility alias for
// `dva config init`. It is a package-level var (rather than local to init())
// so tests can assert its registration and flags directly.
var initAliasCmd *cobra.Command

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a new 'dva.yml' configuration in the current directory",
	Long: `Scaffold a new dva.yml in the current directory. Auto-detects docker-compose.yml and Dockerfile.

Use --recursive to also scaffold dva.yml in detected sub-projects.
After scaffolding, run 'am run dva-discover' to inspect the project, then
'am run dva-improve' to let an AI agent optimize the existing configuration.
Use 'am run dva-improve -p mode=rewrite' only when a full rewrite is intentional.`,
	Example: `  dva init                  # Scaffold dva.yml in the current directory
  dva init --template node  # Scaffold using the node template
  dva init --recursive      # Also scaffold dva.yml in detected sub-projects
  dva init --devcontainer   # Also write .devcontainer/devcontainer.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		created, err := scaffoldDvaYml(".", initTemplate)
		if err != nil {
			return err
		}

		if created {
			withDevcontainer := initDevcontainer || initAll
			if withDevcontainer {
				composeFiles := detectComposeFiles()
				dcService := "app"
				if len(composeFiles) > 0 {
					if services := extractComposeServices(composeFiles[0]); len(services) > 0 {
						dcService = services[0]
					}
				}
				dc := map[string]any{
					"enabled":         true,
					"name":            "Development Environment",
					"service":         dcService,
					"workspaceFolder": "/workspace",
				}
				if err := writeDevcontainerFiles(dc, composeFiles, "."); err != nil {
					fmt.Fprintf(os.Stderr, "⚠️  Could not create .devcontainer/: %v\n", err)
				} else {
					fmt.Println("📦 Created .devcontainer/devcontainer.json")
				}
			}
		}

		if initRecursive {
			scaffoldSubprojects()
		}

		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  am run dva-discover   — inspect the project and configuration options")
		fmt.Println("  am run dva-improve    — optimize the existing config via AI agent")
		fmt.Println("  am run dva-improve -p mode=rewrite — explicitly rewrite the config")
		fmt.Println("  dva config validate  — validate the config")
		fmt.Println("  dva ls               — list available commands")
		return nil
	},
}

func init() {
	initCmd.Flags().StringVarP(&initTemplate, "template", "t", "", "Template to use (minimal, rails, node, python, go)")
	initCmd.Flags().BoolVar(&initRecursive, "recursive", false, "Also scaffold dva.yml in detected sub-projects")
	initCmd.Flags().BoolVar(&initDevcontainer, "devcontainer", false, "Include devcontainer configuration (.devcontainer/devcontainer.json)")
	initCmd.Flags().BoolVar(&initAll, "all", false, "Include all optional features (devcontainer, etc.)")

	// Register under config group
	configCmd.AddCommand(initCmd)

	// Keep a top-level alias for backward compatibility: dva init → dva config init
	initAliasCmd = &cobra.Command{
		Use:     "init",
		Short:   initCmd.Short,
		Long:    initCmd.Long,
		Example: initCmd.Example,
		RunE:    initCmd.RunE,
		Hidden:  false, // visible alias so existing scripts just work
	}
	initAliasCmd.Flags().StringVarP(&initTemplate, "template", "t", "", "Template to use (minimal, rails, node, python, go)")
	initAliasCmd.Flags().BoolVar(&initRecursive, "recursive", false, "Also scaffold dva.yml in detected sub-projects")
	initAliasCmd.Flags().BoolVar(&initDevcontainer, "devcontainer", false, "Include devcontainer configuration (.devcontainer/devcontainer.json)")
	initAliasCmd.Flags().BoolVar(&initAll, "all", false, "Include all optional features (devcontainer, etc.)")
	rootCmd.AddCommand(initAliasCmd)
}

// scaffoldSubprojects detects sub-projects and scaffolds dva.yml in each.
func scaffoldSubprojects() {
	var subs []subInfo
	scanForSubprojects(".", 0, 3, &subs)

	if len(subs) == 0 {
		fmt.Println("No sub-projects detected.")
		return
	}

	fmt.Printf("\n📂 Found %d sub-project(s):\n", len(subs))
	for _, sp := range subs {
		tmpl := languageToTemplate(sp.language)
		if _, err := scaffoldDvaYml(sp.path, tmpl); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  %s: %v\n", sp.path, err)
		}
	}
}

type subInfo struct {
	path     string
	language string
}

// scanForSubprojects recursively finds sub-project directories.
func scanForSubprojects(dir string, depth, maxDepth int, result *[]subInfo) {
	if depth > maxDepth {
		return
	}
	skipDirs := map[string]bool{
		"node_modules": true, "vendor": true, ".venv": true, "venv": true,
		"dist": true, "target": true, "__pycache__": true, ".mypy_cache": true,
		"collected_static": true, ".pytest_cache": true, "tmp": true,
		// Config/infra directories — not sub-projects
		"compose": true, "docker": true, "infra": true, "infrastructure": true,
		"scripts": true, "docs": true, "monitoring": true, "k8s": true,
		"build": true, "data": true, "specs": true, "reports": true,
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || skipDirs[e.Name()] {
			continue
		}
		childPath := filepath.Join(dir, e.Name())

		// Detect sub-project indicators
		isSubProject := false
		lang := ""

		for _, indicator := range []string{".git", config.FileName, config.FileNameAlt} {
			if _, err := os.Stat(filepath.Join(childPath, indicator)); err == nil {
				isSubProject = true
				break
			}
		}

		// Check for build files
		buildIndicators := []struct {
			file string
			lang string
		}{
			{"go.mod", "go"}, {"package.json", "node"}, {"pyproject.toml", "python"},
			{"requirements.txt", "python"}, {"Gemfile", "rails"}, {"Cargo.toml", ""},
		}
		for _, bi := range buildIndicators {
			if _, err := os.Stat(filepath.Join(childPath, bi.file)); err == nil {
				isSubProject = true
				if lang == "" && bi.lang != "" {
					lang = bi.lang
				}
				break
			}
		}

		if isSubProject {
			*result = append(*result, subInfo{childPath, lang})
			continue
		}

		scanForSubprojects(childPath, depth+1, maxDepth, result)
	}
}

// languageToTemplate maps detected language to init template name.
func languageToTemplate(lang string) string {
	switch lang {
	case "go":
		return "go"
	case "node":
		return "node"
	case "python":
		return "python"
	case "rails":
		return "rails"
	default:
		return ""
	}
}

// filterEnv returns a copy of env with entries matching key removed.
func filterEnv(env []string, key string) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// detectTemplateIn inspects the given directory to auto-detect project type.
func detectTemplateIn(dir string) string {
	indicators := []struct {
		file     string
		template string
	}{
		{"Gemfile", "rails"},
		{"package.json", "node"},
		{"requirements.txt", "python"},
		{"Pipfile", "python"},
		{"pyproject.toml", "python"},
		{"go.mod", "go"},
	}

	for _, ind := range indicators {
		if _, err := os.Stat(filepath.Join(dir, ind.file)); err == nil {
			return ind.template
		}
	}

	return "minimal"
}

// generateConfigIn produces the dva.yml content for the given template,
// detecting compose files relative to dir.
func generateConfigIn(dir, tmpl string) string {
	var b strings.Builder

	// MinScaffoldVersion, not config.Version: `version:` is what this config requires
	// of the DVA reading it, so it must describe the emitted schema, not the binary
	// that happened to run `dva init`. See config.MinScaffoldVersion.
	_, _ = fmt.Fprintf(&b, "version: \"%s\"\n\n", config.MinScaffoldVersion)

	// Detect compose files relative to dir
	composeFiles := detectComposeFilesIn(dir)
	b.WriteString("stack:\n")
	b.WriteString("  compose:\n")
	b.WriteString("    default_runner: compose\n")
	b.WriteString("    runners:\n")
	b.WriteString("      compose:\n")
	b.WriteString("        files:\n")
	if len(composeFiles) > 0 {
		for _, f := range composeFiles {
			_, _ = fmt.Fprintf(&b, "          - %s\n", f)
		}
	} else {
		b.WriteString("          - docker-compose.yml\n")
	}
	b.WriteString("\n")

	switch tmpl {
	case "rails":
		b.WriteString(`environment:
  RAILS_ENV: development

interaction:
  shell:
    description: "Open Bash shell in app container"
    service: app
    command: /bin/bash
  rails-console:
    description: "Open Rails console"
    service: app
    command: bundle exec rails console
  server:
    description: "Start Rails server"
    service: app
    command: bundle exec rails server -b 0.0.0.0
    compose:
      method: up
  test:
    description: "Run tests"
    service: app
    command: bundle exec rspec
  bundle:
    description: "Run Bundler commands"
    service: app
    command: bundle
  db:
    description: "Run database commands"
    service: app
    command: bundle exec rails
    subcommands:
      migrate:
        description: "Run migrations"
        command: bundle exec rails db:migrate
      seed:
        description: "Seed database"
        command: bundle exec rails db:seed
      reset:
        description: "Reset database"
        command: bundle exec rails db:reset
`)

	case "node":
		b.WriteString(`environment:
  NODE_ENV: development

interaction:
  shell:
    description: "Open shell in app container"
    service: app
    command: /bin/sh
  dev:
    description: "Start development server"
    service: app
    command: npm run dev
    compose:
      method: up
  test:
    description: "Run tests"
    service: app
    command: npm test
  lint:
    description: "Run linter"
    service: app
    command: npm run lint
  npm:
    description: "Run npm commands"
    service: app
    command: npm
`)

	case "python":
		b.WriteString(`environment:
  PYTHONDONTWRITEBYTECODE: "1"
  PYTHONUNBUFFERED: "1"

interaction:
  shell:
    description: "Open shell in app container"
    service: app
    command: /bin/bash
  manage:
    description: "Run Django management commands"
    service: app
    command: python manage.py
  test:
    description: "Run tests"
    service: app
    command: python -m pytest
  pip:
    description: "Run pip commands"
    service: app
    command: pip
`)

	case "go":
		b.WriteString(`interaction:
  shell:
    description: "Open shell in app container"
    service: app
    command: /bin/sh
  dev:
    description: "Run the Go application"
    service: app
    command: go run .
  test:
    description: "Run tests"
    service: app
    command: go test ./...
  build-app:
    description: "Build the application"
    service: app
    command: go build -o /app/bin/app .
`)

	default: // minimal
		b.WriteString(`interaction:
  shell:
    description: "Open shell in app container"
    service: app
    command: /bin/bash
`)
	}

	return b.String()
}

// detectComposeFiles finds existing docker compose files in the current directory.
func detectComposeFiles() []string {
	return detectComposeFilesIn(".")
}

// detectComposeFilesIn finds existing docker compose files in the given directory.
func detectComposeFilesIn(dir string) []string {
	candidates := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	var found []string
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(dir, c)); err == nil {
			found = append(found, c)
		}
	}

	// Also check for override files
	overrides := []string{
		"docker-compose.override.yml",
		"docker-compose.override.yaml",
	}
	for _, o := range overrides {
		if _, err := os.Stat(filepath.Join(dir, o)); err == nil {
			found = append(found, o)
		}
	}

	// Check for additional docker-compose.* patterns
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasPrefix(name, "docker-compose.") &&
				(strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) &&
				!contains(found, name) {
				found = append(found, name)
			}
		}
	}

	// Sort for deterministic output: primary files first
	if len(found) > 1 {
		primary := []string{}
		rest := []string{}
		for _, f := range found {
			base := filepath.Base(f)
			if base == "docker-compose.yml" || base == "docker-compose.yaml" ||
				base == "compose.yml" || base == "compose.yaml" {
				primary = append(primary, f)
			} else {
				rest = append(rest, f)
			}
		}
		found = append(primary, rest...)
	}

	return deduplicateComposeFiles(dir, found)
}

func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}

// detectInfraComposeFiles finds compose files in common infrastructure subdirectories.
func detectInfraComposeFiles() []string {
	dirs := []string{"infra", "docker", "deploy", "compose", "infrastructure"}
	var found []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if !e.IsDir() &&
				(strings.HasPrefix(name, "compose") || strings.HasPrefix(name, "docker-compose")) &&
				(strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) {
				found = append(found, filepath.Join(dir, name))
			}
		}
	}
	return found
}

// extractMakefileTargets reads documented Makefile targets (target: ## description)
// with their recipe lines. It scans Makefile, GNUmakefile, *.mk in the current
// directory, and .make/ directory. Include directives are followed recursively.
func extractMakefileTargets() string {
	seen := map[string]bool{}
	var targets []string

	// Primary Makefile entry points
	for _, name := range []string{"Makefile", "GNUmakefile", "makefile"} {
		if _, err := os.Stat(name); err == nil {
			collectMakefileTargets(name, seen, &targets)
		}
	}

	// Standalone *.mk files in project root (not already included)
	if matches, err := filepath.Glob("*.mk"); err == nil {
		for _, m := range matches {
			collectMakefileTargets(m, seen, &targets)
		}
	}

	// .make/ directory (common fragment pattern)
	if matches, err := filepath.Glob(".make/*.mk"); err == nil {
		for _, m := range matches {
			collectMakefileTargets(m, seen, &targets)
		}
	}
	if matches, err := filepath.Glob(".make/Makefile*"); err == nil {
		for _, m := range matches {
			collectMakefileTargets(m, seen, &targets)
		}
	}

	if len(targets) == 0 {
		return ""
	}
	return strings.Join(targets, "\n")
}

// collectMakefileTargets recursively reads Makefile and included files,
// extracting target names, descriptions, and recipe lines.
func collectMakefileTargets(path string, seen map[string]bool, targets *[]string) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	if seen[absPath] {
		return
	}
	seen[absPath] = true

	data, err := os.ReadFile(path)
	if err != nil {
		// Try glob expansion for patterns like .make/*.mk
		matches, globErr := filepath.Glob(path)
		if globErr != nil || len(matches) == 0 {
			return
		}
		for _, m := range matches {
			collectMakefileTargets(m, seen, targets)
		}
		return
	}

	dir := filepath.Dir(path)
	lines := strings.Split(string(data), "\n")

	// Collect .PHONY targets for undocumented target detection
	phonyTargets := map[string]bool{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ".PHONY") {
			if _, after, ok := strings.Cut(trimmed, ":"); ok {
				for t := range strings.FieldsSeq(after) {
					if !strings.HasPrefix(t, ".") && !strings.Contains(t, "$") {
						phonyTargets[t] = true
					}
				}
			}
		}
	}

	// Build a map of target → recipe lines for all targets
	recipeMap := extractMakefileRecipes(lines)

	documentedTargets := map[string]bool{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Follow include/-include directives
		if strings.HasPrefix(trimmed, "include ") || strings.HasPrefix(trimmed, "-include ") {
			includePath := strings.TrimPrefix(trimmed, "-include ")
			includePath = strings.TrimPrefix(includePath, "include ")
			includePath = strings.TrimSpace(includePath)
			if !filepath.IsAbs(includePath) {
				includePath = filepath.Join(dir, includePath)
			}
			matches, globErr := filepath.Glob(includePath)
			if globErr == nil && len(matches) > 0 {
				for _, m := range matches {
					collectMakefileTargets(m, seen, targets)
				}
			} else {
				collectMakefileTargets(includePath, seen, targets)
			}
			continue
		}

		// Extract target: ## description lines (documented targets)
		if strings.Contains(line, "##") && !strings.HasPrefix(line, "#") &&
			!strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 && !strings.HasPrefix(parts[0], ".") {
				target := strings.TrimSpace(parts[0])
				if strings.Contains(target, "$") || strings.Contains(target, "%") {
					continue
				}
				documentedTargets[target] = true
				recipe := recipeMap[target]
				dvaTag := ""
				if isDVAWrapperRecipe(recipe) {
					dvaTag = " [DVA wrapper — skip]"
				}
				desc := ""
				if idx := strings.Index(parts[1], "##"); idx >= 0 {
					desc = strings.TrimSpace(parts[1][idx+2:])
				}
				if desc != "" {
					*targets = append(*targets, fmt.Sprintf("  make %-18s # %s%s", target, desc, dvaTag))
				} else {
					*targets = append(*targets, fmt.Sprintf("  make %s%s", target, dvaTag))
				}
				// Append recipe lines (max 5)
				for _, r := range recipe {
					*targets = append(*targets, fmt.Sprintf("    → %s", r))
				}
			}
		}
	}

	// Undocumented .PHONY targets
	var undocumented []string
	for phony := range phonyTargets {
		if !documentedTargets[phony] {
			undocumented = append(undocumented, phony)
		}
	}
	sort.Strings(undocumented)
	for _, name := range undocumented {
		recipe := recipeMap[name]
		dvaTag := ""
		if isDVAWrapperRecipe(recipe) {
			dvaTag = " [DVA wrapper — skip]"
		}
		*targets = append(*targets, fmt.Sprintf("  make %s%s", name, dvaTag))
		for _, r := range recipe {
			*targets = append(*targets, fmt.Sprintf("    → %s", r))
		}
	}
}

// isDVAWrapperRecipe returns true if all recipe lines are just `dva` command
// invocations (e.g., `dva up`, `dva down`). These targets are thin wrappers
// that DVA already handles natively and should be excluded from improve prompts.
func isDVAWrapperRecipe(recipe []string) bool {
	if len(recipe) == 0 {
		return false
	}
	for _, line := range recipe {
		if !strings.HasPrefix(line, "dva ") && line != "dva" {
			return false
		}
	}
	return true
}

// extractMakefileRecipes builds a map of target name → recipe lines (tab-indented
// commands following a target definition). At most 5 recipe lines per target.
func extractMakefileRecipes(lines []string) map[string][]string {
	const maxRecipeLines = 5
	result := map[string][]string{}

	for i := range lines {
		line := lines[i]
		// Detect target definition: starts at column 0, contains ':', not a comment/variable
		if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, " ") ||
			strings.HasPrefix(line, "#") || strings.HasPrefix(line, ".") {
			continue
		}
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		target := strings.TrimSpace(parts[0])
		if target == "" || strings.Contains(target, "$") || strings.Contains(target, "%") ||
			strings.Contains(target, "=") || strings.HasPrefix(target, "export") {
			continue
		}

		// Collect tab-indented recipe lines following this target
		var recipe []string
		for j := i + 1; j < len(lines) && len(recipe) < maxRecipeLines; j++ {
			rLine := lines[j]
			if !strings.HasPrefix(rLine, "\t") {
				break
			}
			cleaned := strings.TrimSpace(rLine)
			// Skip empty lines, pure comments, and echo-only lines
			if cleaned == "" || strings.HasPrefix(cleaned, "#") {
				continue
			}
			// Strip leading @ (silent prefix)
			cleaned = strings.TrimPrefix(cleaned, "@")
			cleaned = strings.TrimSpace(cleaned)
			if cleaned != "" {
				recipe = append(recipe, cleaned)
			}
		}
		if len(recipe) > 0 {
			result[target] = recipe
		}
	}

	return result
}
