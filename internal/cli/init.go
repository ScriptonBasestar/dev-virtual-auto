package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

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

	prompt := fmt.Sprintf(`# Role & Objective
당신은 프로젝트 개발 환경을 최적화하는 'DVA(Docker Virtual Auto)' 설정 전문가입니다.
현재 시스템의 프로젝트 구조를 분석하여, 개발자가 복잡한 Docker Compose나 스크립트 명령어 대신 단순한 `+"`dva [cmd]`"+` 형식으로 작업할 수 있도록 최적의 구조로 `+"`dva.yml`"+` 구성 파일을 작성하는 것이 목표입니다.

# Phase 1: Project Exploration (탐색 및 분석)
이 프로젝트에서 다음 파일들이 감지되었습니다. 이를 바탕으로 프로젝트의 성격을 파악하세요:
1. Docker 설정 (감지됨: %s): 이 파일들을 확인하여 메인 서비스 이름(예: app, web, api)을 식별합니다. (이 서비스 이름이 interaction의 타겟이 됩니다.)
2. 빌드/실행 환경 (감지됨: %s): 이 파일들에서 명령어를 추출합니다. (test, lint, build, dev 등)
3. 환경 변수 (감지됨: %s): 환경 변수 파일 구성을 확인합니다.

# Phase 2: DVA Schema Constraints (DVA 속성 및 규칙)
분석된 내용을 바탕으로 `+"`dva.yml`"+`을 생성합니다. 정의된 스키마를 엄격하게 준수해야 하며, 지원하지 않는 속성을 지어내면(Hallucinate) 안 됩니다.

[구조 및 필수 제약사항]
1. Root Attributes: `+"`version`, `compose`, `interaction`, `provision`"+` 필드 위주로 사용
2. `+"`version`"+`: 문자열 (예: "%s")
3. `+"`compose`"+`:
   - `+"`files`"+`: 사용할 compose 파일의 배열 (예: ["docker-compose.yml"])
   - `+"`project_name`"+`: (선택) 컴포즈 프로젝트 이름
   - `+"`up_options`"+`: (선택) up 명령어의 기본 옵션 (예: ["-d", "--wait"])
4. `+"`interaction`"+`: CLI를 통해 실행될 명령어 매핑.
   각 명령어는 다음 속성을 가집니다:
   - `+"`description`"+`: 명령어에 대한 설명
   - `+"`service`"+`: docker-compose 안에서의 타겟 대상 서비스명 (가장 중요)
   - `+"`command`"+`: 컨테이너 내부에서 실행할 쉘 명령어 (예: "npm run test", "bundle exec rspec")
   - `+"`workdir`"+`: (선택) 작업 디렉토리
   - `+"`environment`"+`: (선택) 주입될 환경변수 Map
   - `+"`subcommands`"+`: (선택) 중첩 명령어 트리 구성용 객체 (예: db 하위 명령)
5. `+"`provision`"+`: 초기 셋업 스크립트.
   - 프로필 이름 (예: `+"`default`"+`) 아래 객체 형태의 배열로 구성. (예: `+"`{\"step\": \"단계 설명\", \"run\": \"실제 쉘 혹은 docker 명령어\"}`"+`)

[표준 Interaction 권장 목록]
다음 기능은 프로젝트에 맞춰 가능한 한 필수로 포함하세요:
- `+"`shell`"+` (기본 쉘 환경 접속, 시스템에 따라 bash, sh, zsh 등 선택)
- `+"`test`"+` (테스트 실행)
- `+"`lint`, `fmt`"+` (코드 검사 및 포매팅)
- `+"`start` / `dev`"+` (의존성 설치 및 백그라운드 서버 실행)

# Phase 3: Action & Output
1. 탐색한 프로젝트의 메인 서비스 이름과 주요 스크립트들을 기반으로 완벽하게 동작하는 `+"`dva.yml`"+` 파일의 전체 내용을 출력하세요.
2. 매핑 결과에 대한 간단한 설계 근거를 주석이나 본문 설명으로 제시하세요.
3. [중요] Cursor나 자율 에이전트 환경에서 작업 중이라면, 생성 완료 후 터미널에 `+"`dva validate`"+` 명령어를 실행하여 DVA 스키마 밸리데이션 검증까지 자체적으로 수행하고 에러가 나면 스스로 수정하세요.
`, detectedCompose, detectedBuild, detectedEnv, config.Version)

	fmt.Println(prompt)
}
