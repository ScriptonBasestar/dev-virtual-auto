package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

var improvePrint bool
var improveDocsOnly bool
var improveVerbose bool

var improveCmd = &cobra.Command{
	Use:   "improve",
	Short: "Review and improve the current dva.yml via AI",
	Long: `Runs AI-assisted improvement of the current dva.yml.

Default behavior: executes Claude Code CLI with the improvement prompt.
Use --print to output the prompt to stdout for manual use.
Use --docs-only to only regenerate CLAUDE.md/AGENTS.md (dva.yml unchanged).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if improvePrint {
			return generateAndPrintImprovePrompt()
		}
		if improveDocsOnly {
			return runAIDocsOnly()
		}
		return runAIImprove()
	},
}

// runAIImprove generates the improve prompt and executes it via Claude Code CLI.
func runAIImprove() error {
	prog := newProgress(improveVerbose)

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

func init() {
	improveCmd.Flags().BoolVar(&improvePrint, "print", false, "Output the improvement prompt to stdout (for manual use)")
	improveCmd.Flags().BoolVar(&improveDocsOnly, "docs-only", false, "Only regenerate CLAUDE.md/AGENTS.md (dva.yml unchanged)")
	improveCmd.Flags().BoolVarP(&improveVerbose, "verbose", "v", false, "Show detailed progress during AI execution")
	configCmd.AddCommand(improveCmd)
}

const maxValidationRetries = 3

// runValidationFeedbackLoop runs dva config validate after AI finishes.
// If validation fails, it feeds the errors back to the AI for fixing, up to maxValidationRetries.
func runValidationFeedbackLoop(claudePath string, verbose bool) error {
	for attempt := 1; attempt <= maxValidationRetries; attempt++ {
		fmt.Printf("🔍 Validating dva.yml (attempt %d/%d)...\n", attempt, maxValidationRetries)

		validateOutput, validateErr := captureValidateOutput()

		if validateOutput != "" {
			fmt.Print(validateOutput)
		}

		if validateErr == nil {
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
