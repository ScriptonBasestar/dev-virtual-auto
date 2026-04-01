package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "embed"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

//go:embed library_reference.txt
var libraryReferenceText string

var initTemplate string
var initRecursive bool
var initDevcontainer bool
var initAll bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a new 'dva.yml' configuration in the current directory",
	Long: `Scaffold a new dva.yml in the current directory. Auto-detects docker-compose.yml and Dockerfile.

Use --recursive to also scaffold dva.yml in detected sub-projects.
After scaffolding, run 'dva config improve' to let an AI agent optimize the configuration.`,
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
		fmt.Println("  dva config improve   — optimize config via AI agent")
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
	initAliasCmd := &cobra.Command{
		Use:    "init",
		Short:  initCmd.Short,
		Long:   initCmd.Long,
		RunE:   initCmd.RunE,
		Hidden: false, // visible alias so existing scripts just work
	}
	initAliasCmd.Flags().StringVarP(&initTemplate, "template", "t", "", "Template to use (minimal, rails, node, python, go)")
	initAliasCmd.Flags().BoolVar(&initRecursive, "recursive", false, "Also scaffold dva.yml in detected sub-projects")
	initAliasCmd.Flags().BoolVar(&initDevcontainer, "devcontainer", false, "Include devcontainer configuration (.devcontainer/devcontainer.json)")
	initAliasCmd.Flags().BoolVar(&initAll, "all", false, "Include all optional features (devcontainer, etc.)")
	rootCmd.AddCommand(initAliasCmd)
}

// scaffoldDvaYml creates a dva.yml in the given directory if one doesn't exist.
// Returns true if a file was created.
func scaffoldDvaYml(dir, tmpl string) (bool, error) {
	target := filepath.Join(dir, "dva.yml")
	if _, err := os.Stat(target); err == nil {
		fmt.Printf("⏭  dva.yml already exists in %s (skipped)\n", dir)
		return false, nil
	}

	if tmpl == "" {
		tmpl = detectTemplateIn(dir)
	}

	content := generateConfigIn(dir, tmpl)

	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		return false, fmt.Errorf("failed to write %s: %w", target, err)
	}

	fmt.Printf("✅ Created %s (template: %s)\n", target, tmpl)

	// Ensure .gitignore exists and ignores the dot directory
	if updated, err := ensureGitignore(dir); err == nil && updated {
		fmt.Printf("📎 Updated .gitignore to ignore %s/\n", config.DotDirName)
	}

	return true, nil
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

		for _, indicator := range []string{".git", "dva.yml", "dva.yaml"} {
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

	_, _ = fmt.Fprintf(&b, "version: \"%s\"\n\n", config.Version)

	// Detect compose files relative to dir
	composeFiles := detectComposeFilesIn(dir)
	b.WriteString("stack:\n")
	b.WriteString("  compose:\n")
	b.WriteString("    order: 10\n")
	b.WriteString("    files:\n")
	if len(composeFiles) > 0 {
		for _, f := range composeFiles {
			_, _ = fmt.Fprintf(&b, "      - %s\n", f)
		}
	} else {
		b.WriteString("      - docker-compose.yml\n")
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
  console:
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
  run:
    description: "Run the Go application"
    service: app
    command: go run .
  test:
    description: "Run tests"
    service: app
    command: go test ./...
  build:
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

	return found
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func formatComposeWarnings(warnings []config.ComposeNameWarning) string {
	if len(warnings) == 0 {
		return ""
	}

	lines := make([]string, 0, len(warnings))
	for _, w := range warnings {
		if w.ComposeName == "" {
			lines = append(lines, fmt.Sprintf("%s: missing top-level name; expected %q", w.File, w.DvaName))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: compose name %q differs from dva project_name %q", w.File, w.ComposeName, w.DvaName))
	}
	return strings.Join(lines, "\n")
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
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				for _, t := range strings.Fields(trimmed[idx+1:]) {
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

	for i := 0; i < len(lines); i++ {
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

// detectSubprojects recursively searches for sub-projects by finding .git directories
// (up to maxDepth 3) and extracts rich metadata for each one:
// - docker-compose services, Dockerfile, build files, scripts, dva.yml status
func detectSubprojects(prog *progress) string {
	type subProject struct {
		path            string
		composeServices []string
		composeFiles    []string
		buildFiles      []string
		hasDockerfile   bool
		hasDvaYml       bool
		language        string
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

	var subs []subProject

	// scanDir reads directory entries and finds sub-projects
	var scanDir func(dir string, depth int)
	scanDir = func(dir string, depth int) {
		if depth > 3 {
			return
		}
		if prog != nil {
			prog.Update(fmt.Sprintf("Scanning sub-projects: %s/", dir))
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			// Skip hidden dirs and known non-project dirs
			if strings.HasPrefix(name, ".") || skipDirs[name] {
				continue
			}

			childPath := filepath.Join(dir, name)

			// Determine if childPath is a sub-project root
			isSubProject := false

			// 1. Check for .git (strongest indicator)
			if _, err := os.Stat(filepath.Join(childPath, ".git")); err == nil {
				isSubProject = true
			}

			// 2. Check for dva.yml or dva.yaml
			if !isSubProject {
				for _, df := range []string{"dva.yml", "dva.yaml"} {
					if _, err := os.Stat(filepath.Join(childPath, df)); err == nil {
						isSubProject = true
						break
					}
				}
			}

			// 3. Check for compose files
			if !isSubProject {
				childEntries, _ := os.ReadDir(childPath)
				for _, ce := range childEntries {
					cn := ce.Name()
					if !ce.IsDir() && (strings.HasPrefix(cn, "docker-compose.") || strings.HasPrefix(cn, "compose.")) && (strings.HasSuffix(cn, ".yml") || strings.HasSuffix(cn, ".yaml")) {
						isSubProject = true
						break
					}
				}
			}

			// 4. Check for strong build files (often used in monorepos without nested .git or compose)
			if !isSubProject {
				for _, bf := range []string{"package.json", "go.mod", "pyproject.toml", "Cargo.toml", "build.gradle"} {
					if _, err := os.Stat(filepath.Join(childPath, bf)); err == nil {
						isSubProject = true
						break
					}
				}
			}

			if isSubProject {
				if prog != nil {
					prog.Update(fmt.Sprintf("Found sub-project: %s/", childPath))
				}
				sp := subProject{path: childPath}

				// Detect compose files and extract service names
				for _, cf := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
					cfPath := filepath.Join(childPath, cf)
					if _, err := os.Stat(cfPath); err == nil {
						sp.composeFiles = append(sp.composeFiles, cf)
						if services := extractComposeServices(cfPath); len(services) > 0 {
							sp.composeServices = append(sp.composeServices, services...)
						}
					}
				}
				// Also detect additional compose files (tools, monitor, etc.)
				if subEntries, err := os.ReadDir(childPath); err == nil {
					for _, se := range subEntries {
						n := se.Name()
						if !se.IsDir() && strings.HasPrefix(n, "docker-compose.") &&
							(strings.HasSuffix(n, ".yml") || strings.HasSuffix(n, ".yaml")) &&
							!contains(sp.composeFiles, n) {
							sp.composeFiles = append(sp.composeFiles, n)
						}
					}
				}

				// Detect build files and language
				buildIndicators := []struct {
					file string
					lang string
				}{
					{"go.mod", "Go"}, {"package.json", "Node.js"}, {"pyproject.toml", "Python"},
					{"requirements.txt", "Python"}, {"Gemfile", "Ruby"}, {"Cargo.toml", "Rust"},
					{"pom.xml", "Java"}, {"build.gradle", "Java"}, {"Makefile", ""},
				}
				for _, bi := range buildIndicators {
					if _, err := os.Stat(filepath.Join(childPath, bi.file)); err == nil {
						sp.buildFiles = append(sp.buildFiles, bi.file)
						if sp.language == "" && bi.lang != "" {
							sp.language = bi.lang
						}
					}
				}

				// Detect Dockerfile
				for _, df := range []string{"Dockerfile", "Dockerfile.dev", "Dockerfile.prod"} {
					if _, err := os.Stat(filepath.Join(childPath, df)); err == nil {
						sp.hasDockerfile = true
						break
					}
				}

				// Detect dva.yml / dva.yaml
				for _, df := range []string{"dva.yml", "dva.yaml"} {
					if _, err := os.Stat(filepath.Join(childPath, df)); err == nil {
						sp.hasDvaYml = true
						break
					}
				}

				subs = append(subs, sp)
				// Don't recurse into sub-project (it's self-contained)
				continue
			}

			// Not a sub-project; recurse deeper to find nested sub-projects
			scanDir(childPath, depth+1)
		}
	}

	scanDir(".", 0)

	if len(subs) == 0 {
		return "None"
	}

	// Format output: structured multi-line per sub-project
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "%d sub-projects detected:\n", len(subs))
	for _, sp := range subs {
		_, _ = fmt.Fprintf(&b, "  - %s/", sp.path)
		if sp.language != "" {
			_, _ = fmt.Fprintf(&b, " [%s]", sp.language)
		}
		if sp.hasDvaYml {
			b.WriteString(" (has dva.yml)")
		}
		b.WriteString("\n")

		if len(sp.composeFiles) > 0 {
			_, _ = fmt.Fprintf(&b, "    compose: %s\n", strings.Join(sp.composeFiles, ", "))
		}
		if len(sp.composeServices) > 0 {
			_, _ = fmt.Fprintf(&b, "    services: %s\n", strings.Join(sp.composeServices, ", "))
		}
		if sp.hasDockerfile {
			b.WriteString("    dockerfile: yes\n")
		}
		if len(sp.buildFiles) > 0 {
			_, _ = fmt.Fprintf(&b, "    build: %s\n", strings.Join(sp.buildFiles, ", "))
		}
	}
	return b.String()
}

// extractComposeServices reads a docker-compose file and returns service names.
// Uses simple YAML line parsing to avoid heavy dependencies.
func extractComposeServices(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	inServices := false
	var services []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Top-level key detection
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			if strings.HasPrefix(trimmed, "services:") {
				inServices = true
				continue
			}
			if inServices {
				// Hit another top-level key, stop
				inServices = false
			}
			continue
		}

		if !inServices {
			continue
		}

		// Detect service names: exactly 2-space indent (or 1 tab) + name + ":"
		if (strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ")) ||
			(strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, "\t\t")) {
			name := strings.TrimSpace(trimmed)
			name = strings.TrimSuffix(name, ":")
			if name != "" && !strings.Contains(name, " ") {
				services = append(services, name)
			}
		}
	}

	return services
}
