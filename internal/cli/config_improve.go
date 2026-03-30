package cli

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ScriptonBasestar/dva/internal/config"
)

//go:embed improve_prompt_template.txt
var improvePromptTemplateText string

//go:embed improve_guardrails_default.txt
var improveGuardrailsDefaultText string

//go:embed improve_guardrails_rewrite.txt
var improveGuardrailsRewriteText string

//go:embed improve_guided_workflow.txt
var improveGuidedWorkflowText string

var improvePrint bool
var improveDocsOnly bool
var improveVerbose bool
var improveRecursive bool
var improveRewrite bool
var improveInteractive bool

var improveCmd = &cobra.Command{
	Use:   "improve",
	Short: "Review and improve the current dva.yml via AI",
	Long: `Runs AI-assisted improvement of dva.yml.

If dva.yml does not exist, it scaffolds one first (auto-detecting project type),
then runs AI improvement on it. If dva.yml already exists, it improves in place.

Use --rewrite to rebuild dva.yml from scratch based on project analysis.
Use --recursive to also improve dva.yml in detected sub-projects.
Use --interactive to open Claude Code in interactive mode (session stays open).
Use --print to output the prompt to stdout for manual use.
Use --docs-only to only regenerate CLAUDE.md/AGENTS.md (dva.yml unchanged).

Flags --print, --docs-only, --interactive are mutually exclusive (first match wins).
--interactive cannot be combined with --recursive.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if improveInteractive && improveRecursive {
			return fmt.Errorf("--interactive and --recursive cannot be combined")
		}
		if improvePrint {
			return generateAndPrintImprovePrompt()
		}
		if improveDocsOnly {
			return runAIDocsOnly()
		}
		if improveInteractive {
			return runAIImproveInteractive()
		}
		if err := runAIImprove(); err != nil {
			return err
		}
		if improveRecursive {
			return runAIImproveRecursive()
		}
		return nil
	},
}

// runAIImprove generates the improve prompt and executes it via Claude Code CLI.
// If dva.yml does not exist, it scaffolds one first via dva init logic, then improves it.
func runAIImprove() error {
	prog := newProgress(improveVerbose)

	// If dva.yml doesn't exist, scaffold it first (auto-detect project type)
	if !dvaConfigExists() {
		fmt.Println("📋 No dva.yml found — scaffolding initial configuration...")
		if _, err := scaffoldDvaYml(".", ""); err != nil {
			return fmt.Errorf("failed to scaffold dva.yml: %w", err)
		}
		fmt.Println()
	}

	prog.Start("Checking claude CLI...")
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		prog.Stop()
		return fmt.Errorf("claude CLI not found in PATH.\n  Install: https://docs.anthropic.com/en/docs/claude-code\n  Or use 'dva config improve --print' to output the prompt manually")
	}

	prog.Update("Building improvement prompt...")
	prompt, err := buildImprovePrompt()
	if err != nil {
		prog.Stop()
		return err
	}

	prog.StopWithMessage("🤖 Running AI improvement via Claude Code...")
	fmt.Println()

	claudeArgs := []string{"-p", "--allowedTools", "Edit,Write,Bash"}
	if improveVerbose {
		claudeArgs = append(claudeArgs, "--verbose")
	}

	claudeCmd := exec.Command(claudePath, claudeArgs...)
	claudeCmd.Stdin = strings.NewReader(prompt)
	claudeCmd.Stdout = os.Stdout
	claudeCmd.Stderr = os.Stderr
	// Clear CLAUDECODE env var to allow spawning from within a Claude Code session
	claudeCmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	if err := claudeCmd.Run(); err != nil {
		return fmt.Errorf("claude CLI failed: %w", err)
	}

	fmt.Println()
	return runValidationFeedbackLoop(claudePath, improveVerbose)
}

// runAIImproveInteractive launches Claude Code in interactive mode with the improve prompt
// as system context, then chains into the guided improve pipeline. The session stays open for follow-up.
func runAIImproveInteractive() error {
	// If dva.yml doesn't exist, scaffold it first (auto-detect project type)
	if !dvaConfigExists() {
		fmt.Println("📋 No dva.yml found — scaffolding initial configuration...")
		if _, err := scaffoldDvaYml(".", ""); err != nil {
			return fmt.Errorf("failed to scaffold dva.yml: %w", err)
		}
		fmt.Println()
	}

	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found in PATH.\n  Install: https://docs.anthropic.com/en/docs/claude-code\n  Or use 'dva config improve --print' to output the prompt manually")
	}

	prompt, err := buildImprovePrompt()
	if err != nil {
		return err
	}

	// Write prompt to temp file for --append-system-prompt-file
	promptFile := filepath.Join(os.TempDir(), "dva-improve-prompt.md")
	if err := os.WriteFile(promptFile, []byte(prompt), 0600); err != nil {
		return fmt.Errorf("failed to write prompt file: %w", err)
	}
	defer os.Remove(promptFile)

	// Extract guided improve workflow to a unique temp dir
	workflowDir, err := os.MkdirTemp("", "dva-improve-guided-*")
	if err != nil {
		return fmt.Errorf("failed to create workflow temp dir: %w", err)
	}
	defer os.RemoveAll(workflowDir)
	if err := extractGuidedWorkflow(workflowDir); err != nil {
		return fmt.Errorf("failed to extract guided workflow: %w", err)
	}

	fmt.Println("🤖 Opening Claude Code interactive session...")
	fmt.Println("   Pipeline: improve dva.yml → validate → guided improve")
	fmt.Println()

	initialPrompt := fmt.Sprintf(`다음 순서로 작업을 진행하세요:

1. **dva.yml 개선**: system prompt의 improve 지침에 따라 dva.yml을 분석하고 개선하세요.
2. **검증**: dva config validate를 실행하여 검증하세요. 실패하면 수정 후 재검증.
3. **guided improve 파이프라인 실행**: %s/orchestrator.md를 읽고, 그 안의 파이프라인을 실행하세요.
   - stage 파일들은 %s/stages/ 에 있습니다.
   - 검증 체크리스트는 %s/verify/checklist.md 에 있습니다.
   - DVA library reference는 이미 system prompt에 포함되어 있습니다.

지금 시작하세요.`, workflowDir, workflowDir, workflowDir)

	claudeArgs := []string{
		"--append-system-prompt-file", promptFile,
		initialPrompt,
	}
	if improveVerbose {
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

// runAIDocsOnly generates DVA guide and updates agent configs without regenerating dva.yml.
func runAIDocsOnly() error {
	if !dvaConfigExists() {
		return fmt.Errorf("dva.yml not found.\n  Run 'dva init' first to scaffold a configuration")
	}

	guidePath, err := generateAIDocs()
	if err != nil {
		return err
	}

	fmt.Printf("✅ Generated %s\n", guidePath)
	return nil
}

// runAIImproveRecursive finds sub-projects with dva.yml and runs improve in each.
func runAIImproveRecursive() error {
	var subs []subInfo
	scanForSubprojects(".", 0, 3, &subs)

	var targets []string
	for _, sp := range subs {
		for _, name := range []string{"dva.yml", "dva.yaml"} {
			if _, err := os.Stat(filepath.Join(sp.path, name)); err == nil {
				targets = append(targets, sp.path)
				break
			}
		}
	}

	if len(targets) == 0 {
		fmt.Println("No sub-projects with dva.yml found.")
		return nil
	}

	originalWd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	for _, dir := range targets {
		fmt.Printf("\n📂 Improving %s/dva.yml...\n", dir)
		func() {
			if err := os.Chdir(dir); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  Could not enter %s: %v\n", dir, err)
				return
			}
			// Restore working directory and reset cached config on exit
			defer func() {
				os.Chdir(originalWd)
				cfg = nil
				env = nil
			}()

			// Reset cached config for sub-project context
			cfg = nil
			env = nil

			if err := runAIImprove(); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  %s: %v\n", dir, err)
			}
		}()
	}

	return nil
}

// generateAndPrintImprovePrompt outputs the unified LLM prompt to stdout.
// If dva.yml does not exist, it scaffolds one first.
func generateAndPrintImprovePrompt() error {
	if !dvaConfigExists() {
		fmt.Fprintln(os.Stderr, "📋 No dva.yml found — scaffolding initial configuration...")
		if _, err := scaffoldDvaYml(".", ""); err != nil {
			return fmt.Errorf("failed to scaffold dva.yml: %w", err)
		}
		fmt.Fprintln(os.Stderr)
	}
	prompt, err := buildImprovePrompt()
	if err != nil {
		return err
	}
	fmt.Println(prompt)
	return nil
}

// buildImprovePrompt builds the unified prompt that includes both project exploration
// and current DVA state analysis.
func buildImprovePrompt() (string, error) {
	c, err := config.Load(".")
	if err != nil {
		return "", fmt.Errorf("could not load current dva.yml: %w", err)
	}

	// --- Project exploration data (Phase 1) ---
	detectedCompose := detectComposeSummary()
	detectedInfraCompose := detectInfraComposeSummary()
	detectedBuild := detectBuildSummary()
	detectedMakeTargets := detectMakeTargetsSummary()
	detectedEnv := detectEnvSummary()
	detectedSubprojects := detectSubprojects(nil)

	// --- Current DVA state (Phase 3) ---
	rawConfig, err := os.ReadFile(c.FilePath())
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", c.FilePath(), err)
	}

	manifestJSON, err := json.MarshalIndent(buildManifest(c), "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to build manifest: %w", err)
	}

	resolvedConfigYAML, err := yaml.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("failed to render merged config: %w", err)
	}

	validationStatus := "PASS"
	if err := c.Validate(); err != nil {
		validationStatus = "FAIL: " + err.Error()
	}

	composeWarnings := formatComposeWarnings(c.ValidateComposeProjectNames())
	if composeWarnings == "" {
		composeWarnings = "None"
	}

	semanticWarnings := c.ValidateWarnings()
	semanticWarningsText := "None"
	if len(semanticWarnings) > 0 {
		semanticWarningsText = strings.Join(semanticWarnings, "\n")
	}

	driftWarnings := detectConfigDriftWarnings(c)
	driftWarningsText := "None"
	if len(driftWarnings) > 0 {
		driftWarningsText = strings.Join(driftWarnings, "\n")
	}

	suggestionWarnings := detectConfigSuggestionWarnings(c)
	suggestionWarningsText := "None"
	if len(suggestionWarnings) > 0 {
		suggestionWarningsText = strings.Join(suggestionWarnings, "\n")
	}

	// Apply mode-specific guardrails
	guardrails := improveGuardrailsDefaultText
	selfReviewPreserve := "□ 기존 `stack.compose.services` 메타데이터가 보존되었는가?"
	if improveRewrite {
		guardrails = improveGuardrailsRewriteText
		selfReviewPreserve = ""
	}
	promptTemplate := strings.Replace(improvePromptTemplateText, "{{GUARDRAILS}}", guardrails, 1)
	promptTemplate = strings.Replace(promptTemplate, "{{SELF_REVIEW_PRESERVE}}", selfReviewPreserve, 1)

	// Unified prompt: version + Phase 1 (exploration) + Phase 3 (current state) + Phase 4 version + library ref + version
	return fmt.Sprintf(promptTemplate,
		// Guardrails: CRITICAL version (1 slot)
		config.Version,
		// Phase 1: Project exploration (6 slots)
		detectedCompose,
		detectedInfraCompose,
		detectedBuild,
		detectedMakeTargets,
		detectedEnv,
		detectedSubprojects,
		// Phase 3: Current DVA state (9 slots)
		c.FilePath(),
		string(rawConfig),
		string(manifestJSON),
		string(resolvedConfigYAML),
		validationStatus,
		composeWarnings,
		semanticWarningsText,
		driftWarningsText,
		suggestionWarningsText,
		// Phase 4: version enforcement (1 slot)
		config.Version,
		// Library reference + version (2 slots)
		libraryReferenceText,
		config.Version,
	), nil
}

// --- Project exploration helpers ---

func detectComposeSummary() string {
	composeFiles := detectComposeFiles()
	if len(composeFiles) == 0 {
		return "None"
	}
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
	return strings.Join(parts, " → ")
}

func detectInfraComposeSummary() string {
	infraComposeFiles := detectInfraComposeFiles()
	if len(infraComposeFiles) == 0 {
		return "None"
	}
	var parts []string
	for _, icf := range infraComposeFiles {
		entry := icf
		if services := extractComposeServices(icf); len(services) > 0 {
			entry += fmt.Sprintf(" → services: %s", strings.Join(services, ", "))
		}
		parts = append(parts, entry)
	}
	return strings.Join(parts, "\n")
}

func detectBuildSummary() string {
	buildFiles := []string{}
	for _, f := range []string{"Makefile", "package.json", "build.gradle", "pom.xml", "pyproject.toml", "Gemfile", "go.mod", "Cargo.toml"} {
		if _, err := os.Stat(f); err == nil {
			buildFiles = append(buildFiles, f)
		}
	}
	if len(buildFiles) == 0 {
		return "None"
	}
	return strings.Join(buildFiles, ", ")
}

func detectMakeTargetsSummary() string {
	if makeTargets := extractMakefileTargets(); makeTargets != "" {
		return makeTargets
	}
	return "None"
}

func detectEnvSummary() string {
	envFiles := []string{}
	for _, f := range []string{".env.example", ".env"} {
		if _, err := os.Stat(f); err == nil {
			envFiles = append(envFiles, f)
		}
	}
	if len(envFiles) == 0 {
		return "None"
	}
	return strings.Join(envFiles, ", ")
}

func init() {
	improveCmd.Flags().BoolVar(&improvePrint, "print", false, "Output the improvement prompt to stdout (for manual use)")
	improveCmd.Flags().BoolVar(&improveDocsOnly, "docs-only", false, "Only regenerate CLAUDE.md/AGENTS.md (dva.yml unchanged)")
	improveCmd.Flags().BoolVarP(&improveVerbose, "verbose", "v", false, "Show detailed progress during AI execution")
	improveCmd.Flags().BoolVar(&improveRecursive, "recursive", false, "Also improve dva.yml in detected sub-projects")
	improveCmd.Flags().BoolVar(&improveRewrite, "rewrite", false, "Rewrite dva.yml from scratch based on project analysis (ignores existing structure)")
	improveCmd.Flags().BoolVarP(&improveInteractive, "interactive", "i", false, "Open Claude Code in interactive mode (session stays open for follow-up work)")
	configCmd.AddCommand(improveCmd)
}

const maxValidationRetries = 3

// runValidationFeedbackLoop runs dva config validate after AI finishes.
// If validation fails, it feeds the errors back to the AI for fixing, up to maxValidationRetries.
// After validation passes, it also checks for version mismatch and fixes it.
func runValidationFeedbackLoop(claudePath string, verbose bool) error {
	for attempt := 1; attempt <= maxValidationRetries; attempt++ {
		fmt.Printf("🔍 Validating dva.yml (attempt %d/%d)...\n", attempt, maxValidationRetries)

		validateOutput, validateErr := captureValidateOutput()

		if validateOutput != "" {
			fmt.Print(validateOutput)
		}

		if validateErr == nil {
			// Validation passed — check version mismatch as a post-validation step
			if versionErr := fixVersionMismatch(); versionErr != nil {
				fmt.Fprintf(os.Stderr, "⚠️  version fix failed: %v\n", versionErr)
			}
			return nil
		}

		if attempt == maxValidationRetries {
			fmt.Println("\n⚠️  Validation still failing after max retries — review manually")
			return nil
		}

		fmt.Printf("\n🔄 Feeding validation errors back to AI (retry %d/%d)...\n\n", attempt, maxValidationRetries)

		fixPrompt, err := buildValidationFixPrompt(validateOutput)
		if err != nil {
			return fmt.Errorf("failed to build fix prompt: %w", err)
		}

		claudeArgs := []string{"-p", "--allowedTools", "Edit,Write,Bash"}
		if verbose {
			claudeArgs = append(claudeArgs, "--verbose")
		}

		cmd := exec.Command(claudePath, claudeArgs...)
		cmd.Stdin = strings.NewReader(fixPrompt)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("claude CLI failed during validation fix: %w", err)
		}
		fmt.Println()
	}
	return nil
}

// fixVersionMismatch checks if dva.yml version matches the running DVA version.
// If not, it directly updates the version field in the file (simple sed-like replacement).
func fixVersionMismatch() error {
	c, err := config.Load(".")
	if err != nil {
		return nil // can't load — skip
	}
	if c.Version == config.Version {
		return nil // already matches
	}

	fmt.Printf("🔄 Updating dva.yml version %q → %q...\n", c.Version, config.Version)

	// Direct file replacement — no need to invoke AI for a simple version bump
	raw, err := os.ReadFile(c.FilePath())
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", c.FilePath(), err)
	}

	oldVersionLine := fmt.Sprintf("version: %q", c.Version)
	newVersionLine := fmt.Sprintf("version: %q", config.Version)
	updated := strings.Replace(string(raw), oldVersionLine, newVersionLine, 1)

	if updated == string(raw) {
		// Try without quotes
		oldVersionLine = fmt.Sprintf("version: \"%s\"", c.Version)
		newVersionLine = fmt.Sprintf("version: \"%s\"", config.Version)
		updated = strings.Replace(string(raw), oldVersionLine, newVersionLine, 1)
	}

	if updated == string(raw) {
		return nil // couldn't find version line to replace
	}

	if err := os.WriteFile(c.FilePath(), []byte(updated), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", c.FilePath(), err)
	}

	fmt.Printf("✅ Version updated to %s\n", config.Version)
	return nil
}

// captureValidateOutput runs dva config validate and captures combined output.
func captureValidateOutput() (string, error) {
	selfPath, err := os.Executable()
	if err != nil {
		selfPath = "dva"
	}
	cmd := exec.Command(selfPath, "config", "validate")
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// buildValidationFixPrompt builds a focused prompt for the AI to fix validation errors.
func buildValidationFixPrompt(validateOutput string) (string, error) {
	c, err := config.Load(".")
	if err != nil {
		return "", fmt.Errorf("could not load dva.yml: %w", err)
	}

	rawConfig, err := os.ReadFile(c.FilePath())
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", c.FilePath(), err)
	}

	prompt := "# DVA Validation Fix\n\n" +
		"dva.yml에서 검출된 validation 에러를 수정하세요.\n\n" +
		"## Current dva.yml\n\n" +
		"Path: " + c.FilePath() + "\n\n" +
		"```yaml\n" + string(rawConfig) + "\n```\n\n" +
		"## Validation Output\n\n" +
		"```text\n" + validateOutput + "\n```\n\n" +
		"## Instructions\n\n" +
		"1. 위 에러/경고를 수정하세요. **최소 변경만** — 에러 수정에 집중.\n" +
		"2. 기존 interaction 이름, 구조는 최대한 유지.\n" +
		"3. 수정 후 DVA schema " + config.Version + "와 호환되어야 합니다.\n\n" +
		"Common fixes:\n" +
		"- `Additional property X is not allowed` → 스키마에 없는 필드를 삭제하거나 올바른 필드로 교체\n" +
		"- `reserved command conflict` → interaction 키 이름을 변경 (예: run → app-run)\n" +
		"- `X.type must be one of the following` → 허용된 type 값으로 변경\n" +
		"- compose name mismatch → compose 파일의 top-level name을 dva.yml project_name과 일치시킴\n\n" +
		"## DVA Library Reference\n\n" +
		libraryReferenceText + "\n"

	return prompt, nil
}

// extractGuidedWorkflow parses the bundled guided improve workflow text and writes
// individual files to targetDir. The bundle uses "--- FILE: <path> ---" markers.
func extractGuidedWorkflow(targetDir string) error {
	const marker = "--- FILE: "
	var currentFile string
	var currentLines []string

	flush := func() error {
		if currentFile == "" {
			return nil
		}
		outPath := filepath.Join(targetDir, currentFile)
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return err
		}
		return os.WriteFile(outPath, []byte(strings.Join(currentLines, "\n")), 0600)
	}

	for _, line := range strings.Split(improveGuidedWorkflowText, "\n") {
		if strings.HasPrefix(line, marker) && strings.HasSuffix(line, " ---") {
			if err := flush(); err != nil {
				return err
			}
			currentFile = strings.TrimSuffix(strings.TrimPrefix(line, marker), " ---")
			currentLines = nil
			continue
		}
		if currentFile != "" {
			currentLines = append(currentLines, line)
		}
	}
	return flush()
}

// dvaConfigExists checks whether dva.yml or dva.yaml exists in the current directory only.
// Unlike config.findConfig(), this does not walk up parent directories — scaffold targets cwd.
func dvaConfigExists() bool {
	for _, name := range []string{"dva.yml", "dva.yaml"} {
		if _, err := os.Stat(name); err == nil {
			return true
		}
	}
	return false
}
