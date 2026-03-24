package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
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
	fmt.Println("✅ Claude Code finished. Validating...")

	// Auto-validate
	validateExec := exec.Command("dva", "config", "validate")
	validateExec.Stdout = os.Stdout
	validateExec.Stderr = os.Stderr
	if err := validateExec.Run(); err != nil {
		fmt.Println("⚠️  Validation failed — review the generated dva.yml")
	}

	return nil
}

func init() {
	improveCmd.Flags().BoolVar(&improvePrint, "print", false, "Output the improvement prompt to stdout (for manual use)")
	improveCmd.Flags().BoolVar(&improveDocsOnly, "docs-only", false, "Only regenerate CLAUDE.md/AGENTS.md (dva.yml unchanged)")
	improveCmd.Flags().BoolVarP(&improveVerbose, "verbose", "v", false, "Show detailed progress during AI execution")
	configCmd.AddCommand(improveCmd)
}
