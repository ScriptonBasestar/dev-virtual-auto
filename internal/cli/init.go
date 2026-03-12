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
		detectedCompose = strings.Join(composeFiles, ", ")
	}

	buildFiles := []string{}
	for _, f := range []string{"Makefile", "package.json", "build.gradle", "pom.xml", "pyproject.toml", "Gemfile", "go.mod"} {
		if _, err := os.Stat(f); err == nil {
			buildFiles = append(buildFiles, f)
		}
	}
	detectedBuild := "None"
	if len(buildFiles) > 0 {
		detectedBuild = strings.Join(buildFiles, ", ")
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

	prompt := fmt.Sprintf(promptTemplateText, detectedCompose, detectedBuild, detectedEnv, config.Version)

	fmt.Println(prompt)
}
