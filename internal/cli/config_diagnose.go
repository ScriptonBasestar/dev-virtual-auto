package cli

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

//go:embed diagnose_prompt_template.txt
var diagnosePromptTemplateText string

var diagnoseMaxRetries int
var diagnoseInteractive bool
var diagnosePrint bool
var diagnoseVerbose bool
var diagnoseModel string

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "Scan app environments, run dva up, and fix configuration issues via AI",
	Long: `Runs AI-assisted diagnosis of dva.yml against actual app environments.

Unlike 'improve' which generates config from project structure,
'diagnose' runs the stack, collects errors, and fixes configuration issues.

Steps:
  1. Scan: Load dva.yml and detect app environment requirements
  2. Execute: Run dva up and collect errors/logs
  3. Diagnose: AI classifies problems (auto-fix / user-decision / out-of-scope)
  4. Fix: Apply auto-fixes to dva.yml
  5. Loop: Re-execute until success or max retries

Use --interactive (-i) to let AI ask questions for USER_DECISION items.
Use --print to output the diagnosis prompt without executing.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !dvaConfigExists() {
			return fmt.Errorf("dva.yml not found — run 'dva init' or 'dva config improve' first")
		}
		if diagnosePrint {
			return printDiagnosePrompt()
		}
		if diagnoseInteractive {
			return runDiagnoseInteractive()
		}
		return runDiagnoseBatch()
	},
}

// runDiagnoseBatch runs the diagnose feedback loop in batch mode.
func runDiagnoseBatch() error {
	prog := newProgress(diagnoseVerbose)

	prog.Start("Checking claude CLI...")
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		prog.Stop()
		return fmt.Errorf("claude CLI not found in PATH.\n  Install: https://docs.anthropic.com/en/docs/claude-code\n  Or use 'dva config improve --diagnose --print' to output the prompt manually")
	}

	for attempt := 1; attempt <= diagnoseMaxRetries; attempt++ {
		fmt.Printf("\n🔍 Diagnose attempt %d/%d\n", attempt, diagnoseMaxRetries)

		prog.Update("Collecting diagnostic data...")
		prompt, err := buildDiagnosePrompt()
		if err != nil {
			prog.Stop()
			return err
		}

		prog.StopWithMessage("🤖 Running AI diagnosis...")
		fmt.Println()

		claudeArgs := []string{"-p", "--allowedTools", "Edit,Write,Bash,Read"}
		if diagnoseModel != "" {
			claudeArgs = append(claudeArgs, "--model", diagnoseModel)
		}
		if diagnoseVerbose {
			claudeArgs = append(claudeArgs, "--verbose")
		}

		claudeCmd := exec.Command(claudePath, claudeArgs...)
		claudeCmd.Stdin = strings.NewReader(prompt)
		claudeCmd.Stdout = os.Stdout
		claudeCmd.Stderr = os.Stderr
		claudeCmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

		if err := claudeCmd.Run(); err != nil {
			return fmt.Errorf("claude CLI failed: %w", err)
		}

		// Validate after fix
		fmt.Println()
		fmt.Println("🔍 Validating dva.yml after fix...")
		validateOutput, validateErr := captureValidateOutput()
		if validateOutput != "" {
			fmt.Print(validateOutput)
		}
		if validateErr != nil {
			fmt.Printf("⚠️  Validation failed — will retry (%d/%d)\n", attempt, diagnoseMaxRetries)
			continue
		}

		// Try execution
		fmt.Println("🚀 Testing dva up...")
		upOutput, upErr := captureDvaUp()
		if upErr == nil {
			fmt.Println("✅ dva up succeeded — diagnosis complete")
			return nil
		}

		fmt.Printf("⚠️  dva up still failing:\n%s\n", upOutput)
		if attempt == diagnoseMaxRetries {
			fmt.Println("\n⚠️  Max retries reached — review remaining issues manually")
			return nil
		}
	}
	return nil
}

// runDiagnoseInteractive opens Claude Code in interactive mode for diagnosis.
func runDiagnoseInteractive() error {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found in PATH")
	}

	prompt, err := buildDiagnosePrompt()
	if err != nil {
		return err
	}

	// Write prompt as system context
	promptFile := filepath.Join(os.TempDir(), "dva-diagnose-prompt.md")
	if err := os.WriteFile(promptFile, []byte(prompt), 0600); err != nil {
		return fmt.Errorf("failed to write prompt file: %w", err)
	}
	defer func() { _ = os.Remove(promptFile) }()

	fmt.Println("🤖 Opening Claude Code interactive diagnosis session...")
	fmt.Println("   AI will scan your apps, identify issues, and ask about ambiguous fixes.")
	fmt.Println()

	initialPrompt := `위 진단 데이터를 분석하세요.

1. 문제를 AUTO_FIX / USER_DECISION / OUT_OF_SCOPE로 분류
2. AUTO_FIX 항목은 즉시 dva.yml을 수정
3. USER_DECISION 항목은 나에게 질문하여 확인 후 수정
4. OUT_OF_SCOPE 항목은 리포트만 출력
5. 수정 후 dva config validate를 실행하여 검증
6. 검증 통과 시 dva up --no-wait로 테스트
7. 실패하면 새로운 오류를 분석하여 반복

지금 시작하세요.`

	claudeArgs := []string{
		"--append-system-prompt-file", promptFile,
		initialPrompt,
	}
	if diagnoseModel != "" {
		claudeArgs = append(claudeArgs, "--model", diagnoseModel)
	}
	if diagnoseVerbose {
		claudeArgs = append(claudeArgs, "--verbose")
	}

	claudeCmd := exec.Command(claudePath, claudeArgs...)
	claudeCmd.Stdin = os.Stdin
	claudeCmd.Stdout = os.Stdout
	claudeCmd.Stderr = os.Stderr
	claudeCmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	if err := claudeCmd.Run(); err != nil {
		return fmt.Errorf("claude CLI failed: %w", err)
	}

	return nil
}

// printDiagnosePrompt outputs the diagnosis prompt to stdout without executing dva up.
func printDiagnosePrompt() error {
	prompt, err := buildDiagnosePrompt(true)
	if err != nil {
		return err
	}
	fmt.Println(prompt)
	return nil
}

// buildDiagnosePrompt collects all diagnostic data and builds the AI prompt.
// If dryRun is true, skips dva up/status execution (for --print mode).
func buildDiagnosePrompt(dryRun ...bool) (string, error) {
	skipExec := len(dryRun) > 0 && dryRun[0]
	c, err := config.Load(".", config.SkipVersionCheck())
	if err != nil {
		return "", fmt.Errorf("could not load dva.yml: %w", err)
	}

	// 1. Read current dva.yml
	rawConfig, err := os.ReadFile(c.FilePath())
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", c.FilePath(), err)
	}

	// 2. Run validation
	validateOutput, _ := captureValidateOutput()
	if validateOutput == "" {
		validateOutput = "PASS"
	}

	// 3. Collect environment info
	envInfo := collectEnvInfo(c)

	// 4. Scan app environment requirements
	appScanResult := scanAppEnvironments(c)

	// 5. Try dva up and collect output (skip in dry-run/print mode)
	upOutput := "(skipped — print mode)"
	statusOutput := "(skipped — print mode)"
	if !skipExec {
		upOutput, _ = captureDvaUp()
		if upOutput == "" {
			upOutput = "(not executed or no output)"
		}

		// 6. Collect status
		statusOutput, _ = captureDvaStatus()
		if statusOutput == "" {
			statusOutput = "(not available)"
		}
	}

	return fmt.Sprintf(diagnosePromptTemplateText,
		c.FilePath(),
		string(rawConfig),
		validateOutput,
		envInfo,
		appScanResult,
		upOutput,
		statusOutput,
		libraryReferenceText,
	), nil
}

// collectEnvInfo gathers environment context for diagnosis.
func collectEnvInfo(c *config.Config) string {
	var sb strings.Builder

	// env_file info
	fmt.Fprintf(&sb, "Config dir: %s\n", c.FileDir())

	// Check .env files
	for _, f := range []string{".env", ".env.local", ".env.dev", ".env.example"} {
		path := filepath.Join(c.FileDir(), f)
		if info, err := os.Stat(path); err == nil {
			fmt.Fprintf(&sb, "Found: %s (%d bytes)\n", f, info.Size())
		}
	}

	// List declared environment variables in dva.yml
	if len(c.Environment) > 0 {
		fmt.Fprintf(&sb, "\ndva.yml environment keys: %s\n", joinMapKeys(c.Environment))
	}

	return sb.String()
}

// scanAppEnvironments inspects each declared application to detect environment requirements.
func scanAppEnvironments(c *config.Config) string {
	if len(c.Applications) == 0 {
		return "No applications declared in dva.yml"
	}

	// Sort app names for deterministic output
	appNames := make([]string, 0, len(c.Applications))
	for name := range c.Applications {
		appNames = append(appNames, name)
	}
	sort.Strings(appNames)

	var sb strings.Builder
	for _, name := range appNames {
		app := c.Applications[name]
		fmt.Fprintf(&sb, "## App: %s\n", name)
		if app.Description != "" {
			fmt.Fprintf(&sb, "  Description: %s\n", app.Description)
		}
		if app.Port != 0 {
			fmt.Fprintf(&sb, "  Declared port: %d\n", app.Port)
		}
		if app.Run.Native != "" {
			fmt.Fprintf(&sb, "  Run command: %s\n", app.Run.Native)
		}
		if app.Dir != "" {
			fmt.Fprintf(&sb, "  Working dir: %s\n", app.Dir)
		}
		if len(app.Environment) > 0 {
			fmt.Fprintf(&sb, "  Environment: %v\n", app.Environment)
		} else {
			fmt.Fprintf(&sb, "  Environment: (none declared)\n")
		}
		if len(app.DependsOn) > 0 {
			fmt.Fprintf(&sb, "  Depends on: %s\n", strings.Join(app.DependsOn, ", "))
		}

		// Scan app source for env var usage if Dir is set
		appDir := app.Dir
		if appDir == "" {
			appDir = c.FileDir()
		} else if !filepath.IsAbs(appDir) {
			appDir = filepath.Join(c.FileDir(), appDir)
		}
		if envVars := detectAppEnvVarUsage(appDir); len(envVars) > 0 {
			fmt.Fprintf(&sb, "  Detected env vars in source: %s\n", strings.Join(envVars, ", "))
		}

		// Check if health check exists
		if app.Health != nil {
			fmt.Fprintf(&sb, "  Health check: configured\n")
		} else {
			fmt.Fprintf(&sb, "  Health check: not configured\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// envVarPatterns defines regex patterns per file extension to detect env var usage.
// Each pattern must have a capturing group for the variable name.
var envVarPatterns = map[string][]*regexp.Regexp{
	".go": {
		regexp.MustCompile(`os\.Getenv\("([A-Z][A-Z0-9_]+)"`),
		regexp.MustCompile(`viper\.(?:Get|Bind|Set)\w*\("([A-Z][A-Z0-9_.]+)"`),
	},
	".rs": {
		regexp.MustCompile(`env::var\("([A-Z][A-Z0-9_]+)"`),
	},
	".py": {
		regexp.MustCompile(`os\.(?:environ|getenv)\W*\(?[^)]*"([A-Z][A-Z0-9_]+)"`),
	},
	".ts": {
		regexp.MustCompile(`process\.env\.([A-Z][A-Z0-9_]+)`),
	},
	".js": {
		regexp.MustCompile(`process\.env\.([A-Z][A-Z0-9_]+)`),
	},
	".toml": {
		regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]+)`),
	},
	".yaml": {
		regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]+)`),
	},
	".yml": {
		regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]+)`),
	},
}

// skipDirs are directories to skip during env var scanning.
var skipDirs = map[string]bool{
	"vendor": true, "node_modules": true, "target": true,
	".git": true, ".sb": true, "dist": true, "build": true,
}

// detectAppEnvVarUsage scans source files in dir using Go regexp (no external grep dependency).
// Returns sorted unique env var names found in source code.
func detectAppEnvVarUsage(dir string) []string {
	if _, err := os.Stat(dir); err != nil {
		return nil
	}

	seen := make(map[string]bool)

	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(d.Name())
		patterns, ok := envVarPatterns[ext]
		if !ok {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			for _, re := range patterns {
				for _, match := range re.FindAllStringSubmatch(line, -1) {
					if len(match) > 1 {
						seen[match[1]] = true
					}
				}
			}
		}
		return nil
	})

	result := make([]string, 0, len(seen))
	for v := range seen {
		result = append(result, v)
	}
	sort.Strings(result)
	return result
}

// captureDvaUp runs dva up --no-wait and captures output.
func captureDvaUp() (string, error) {
	selfPath, err := os.Executable()
	if err != nil {
		selfPath = "dva"
	}
	cmd := exec.Command(selfPath, "up", "--no-wait")
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// captureDvaStatus runs dva status and captures output.
func captureDvaStatus() (string, error) {
	selfPath, err := os.Executable()
	if err != nil {
		selfPath = "dva"
	}
	cmd := exec.Command(selfPath, "status")
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// joinMapKeys returns sorted comma-separated keys of a map.
func joinMapKeys(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

func init() {
	diagnoseCmd.Flags().BoolVarP(&diagnoseInteractive, "interactive", "i", false, "Open Claude Code interactive session (ask about ambiguous fixes)")
	diagnoseCmd.Flags().BoolVar(&diagnosePrint, "print", false, "Output the diagnosis prompt to stdout (for manual use)")
	diagnoseCmd.Flags().BoolVarP(&diagnoseVerbose, "verbose", "v", false, "Show detailed progress")
	diagnoseCmd.Flags().IntVar(&diagnoseMaxRetries, "max-retries", 3, "Maximum feedback loop iterations")
	diagnoseCmd.Flags().StringVar(&diagnoseModel, "model", "sonnet", "Claude model to use")
	improveCmd.AddCommand(diagnoseCmd)
}
