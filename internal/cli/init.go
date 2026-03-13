package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "embed"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

//go:embed prompt_template.txt
var promptTemplateText string

var initTemplate string
var initPrompt bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new 'dva.yml' configuration in the current directory",
	Long:  "Scaffold a new dva.yml in the current directory. Auto-detects docker-compose.yml and Dockerfile.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if initPrompt {
			generateAndPrintPrompt()
			return nil
		}

		target := "dva.yml"
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("dva.yml already exists in current directory")
		}

		tmpl := initTemplate
		if tmpl == "" {
			tmpl = detectTemplate()
		}

		content := generateConfig(tmpl)
		if err := os.WriteFile(target, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write dva.yml: %w", err)
		}

		fmt.Printf("✅ Created dva.yml (template: %s)\n", tmpl)
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  dva ls        — list available commands")
		fmt.Println("  dva validate  — validate the config")
		fmt.Println("  dva up        — start services")
		return nil
	},
}

func init() {
	initCmd.Flags().StringVarP(&initTemplate, "template", "t", "", "Template to use (minimal, rails, node, python, go)")
	initCmd.Flags().BoolVarP(&initPrompt, "prompt", "p", false, "Output an LLM prompt to help generate dva.yml instead of creating one directly")
}

// detectTemplate inspects the current directory to auto-detect project type.
func detectTemplate() string {
	// Check for language-specific files
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
		if _, err := os.Stat(ind.file); err == nil {
			return ind.template
		}
	}

	return "minimal"
}

// generateConfig produces the dva.yml content for the given template.
func generateConfig(tmpl string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("version: \"%s\"\n\n", config.Version))

	// Detect compose files
	composeFiles := detectComposeFiles()
	if len(composeFiles) > 0 {
		b.WriteString("compose:\n")
		b.WriteString("  files:\n")
		for _, f := range composeFiles {
			b.WriteString(fmt.Sprintf("    - %s\n", f))
		}
		b.WriteString("\n")
	} else {
		b.WriteString("compose:\n")
		b.WriteString("  files:\n")
		b.WriteString("    - docker-compose.yml\n\n")
	}

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
	candidates := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	var found []string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			found = append(found, c)
		}
	}

	// Also check for override files
	overrides := []string{
		"docker-compose.override.yml",
		"docker-compose.override.yaml",
	}
	for _, o := range overrides {
		if _, err := os.Stat(o); err == nil {
			found = append(found, o)
		}
	}

	// Check subdirectories for common patterns
	if entries, err := os.ReadDir("."); err == nil {
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

	// Sort for deterministic output
	if len(found) > 1 {
		// Keep docker-compose.yml first, then overrides
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

// generateAndPrintPrompt inspects the current directory and outputs an LLM prompt tailored to the project.
func generateAndPrintPrompt() {
	var detectedEnv string
	composeFiles := detectComposeFiles()
	detectedCompose := "None"
	if len(composeFiles) > 0 {
		// Include compose file names + extracted service names
		var parts []string
		parts = append(parts, strings.Join(composeFiles, ", "))
		var allServices []string
		for _, cf := range composeFiles {
			if services := extractComposeServices(cf); len(services) > 0 {
				allServices = append(allServices, services...)
			}
		}
		if len(allServices) > 0 {
			parts = append(parts, fmt.Sprintf("services: %s", strings.Join(allServices, ", ")))
		}
		detectedCompose = strings.Join(parts, " → ")
	}

	// Detect infra/subdirectory compose files
	infraComposeFiles := detectInfraComposeFiles()
	detectedInfraCompose := "None"
	if len(infraComposeFiles) > 0 {
		var infraParts []string
		for _, icf := range infraComposeFiles {
			entry := icf
			if services := extractComposeServices(icf); len(services) > 0 {
				entry += fmt.Sprintf(" → services: %s", strings.Join(services, ", "))
			}
			infraParts = append(infraParts, entry)
		}
		detectedInfraCompose = strings.Join(infraParts, "\n")
	}

	buildFiles := []string{}
	for _, f := range []string{"Makefile", "package.json", "build.gradle", "pom.xml", "pyproject.toml", "Gemfile", "go.mod", "Cargo.toml"} {
		if _, err := os.Stat(f); err == nil {
			buildFiles = append(buildFiles, f)
		}
	}
	detectedBuild := "None"
	if len(buildFiles) > 0 {
		detectedBuild = strings.Join(buildFiles, ", ")
	}

	// Extract Makefile targets
	detectedMakeTargets := "None"
	if makeTargets := extractMakefileTargets(); makeTargets != "" {
		detectedMakeTargets = makeTargets
	}

	envFiles := []string{}
	for _, f := range []string{".env.example", ".env"} {
		if _, err := os.Stat(f); err == nil {
			envFiles = append(envFiles, f)
		}
	}
	if len(envFiles) > 0 {
		detectedEnv = strings.Join(envFiles, ", ")
	} else {
		detectedEnv = "None"
	}

	// Detect sub-projects: subdirectories containing their own project files
	detectedSubprojects := detectSubprojects()

	prompt := fmt.Sprintf(promptTemplateText,
		detectedCompose,      // %s 1 - root compose
		detectedInfraCompose, // %s 2 - infra compose
		detectedBuild,        // %s 3 - build files
		detectedMakeTargets,  // %s 4 - Makefile targets
		detectedEnv,          // %s 5 - env files
		detectedSubprojects,  // %s 6 - subprojects
		config.Version,       // %s 7 - dva version
	)

	fmt.Println(prompt)
}

// detectInfraComposeFiles finds compose files in common infrastructure subdirectories.
func detectInfraComposeFiles() []string {
	dirs := []string{"infra", "docker", "deploy"}
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

// extractMakefileTargets reads documented Makefile targets (target: ## description).
func extractMakefileTargets() string {
	data, err := os.ReadFile("Makefile")
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	var targets []string
	for _, line := range lines {
		if strings.Contains(line, "##") && !strings.HasPrefix(line, "#") &&
			!strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 && !strings.HasPrefix(parts[0], ".") {
				target := strings.TrimSpace(parts[0])
				desc := ""
				if idx := strings.Index(parts[1], "##"); idx >= 0 {
					desc = strings.TrimSpace(parts[1][idx+2:])
				}
				if desc != "" {
					targets = append(targets, fmt.Sprintf("  make %-18s # %s", target, desc))
				} else {
					targets = append(targets, fmt.Sprintf("  make %s", target))
				}
			}
		}
	}
	if len(targets) == 0 {
		return ""
	}
	return strings.Join(targets, "\n")
}

// detectSubprojects recursively searches for sub-projects by finding .git directories
// (up to maxDepth 3) and extracts rich metadata for each one:
// - docker-compose services, Dockerfile, build files, scripts, dva.yml status
func detectSubprojects() string {
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
	}

	var subs []subProject

	// scanDir reads directory entries and finds sub-projects
	var scanDir func(dir string, depth int)
	scanDir = func(dir string, depth int) {
		if depth > 3 {
			return
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
	b.WriteString(fmt.Sprintf("%d sub-projects detected:\n", len(subs)))
	for _, sp := range subs {
		b.WriteString(fmt.Sprintf("  - %s/", sp.path))
		if sp.language != "" {
			b.WriteString(fmt.Sprintf(" [%s]", sp.language))
		}
		if sp.hasDvaYml {
			b.WriteString(" (has dva.yml)")
		}
		b.WriteString("\n")

		if len(sp.composeFiles) > 0 {
			b.WriteString(fmt.Sprintf("    compose: %s\n", strings.Join(sp.composeFiles, ", ")))
		}
		if len(sp.composeServices) > 0 {
			b.WriteString(fmt.Sprintf("    services: %s\n", strings.Join(sp.composeServices, ", ")))
		}
		if sp.hasDockerfile {
			b.WriteString("    dockerfile: yes\n")
		}
		if len(sp.buildFiles) > 0 {
			b.WriteString(fmt.Sprintf("    build: %s\n", strings.Join(sp.buildFiles, ", ")))
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
