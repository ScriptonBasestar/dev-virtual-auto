package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var diagnoseMaxRetries int
var diagnoseInteractive bool
var diagnosePrint bool
var diagnoseVerbose bool
var diagnoseModel string

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "Scan app environments, run dva up, and fix configuration issues via agent-mesh",
	Long: `Runs AI-assisted diagnosis of dva.yml via agent-mesh flow.

Unlike 'improve' which generates config from project structure,
'diagnose' runs the stack, collects errors, and fixes configuration issues.

Flow steps:
  1. Scan: Detect app environment requirements from source code
  2. Execute: Run dva up and collect errors/logs
  3. Diagnose: AI classifies problems (auto-fix / user-decision / out-of-scope)
  4. Fix: Apply auto-fixes to dva.yml
  5. Validate: Re-validate configuration

Requires 'am' (agent-mesh) CLI in PATH.
Use --print to output the am run command without executing.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !dvaConfigExists() {
			return fmt.Errorf("dva.yml not found — run 'dva init' or 'dva config improve' first")
		}
		if diagnosePrint {
			return printDiagnoseAmCommand()
		}
		return runAmDiagnose()
	},
}

// runAmDiagnose executes the dva-diagnose flow via agent-mesh CLI.
func runAmDiagnose() error {
	amPath, err := findAmCLI()
	if err != nil {
		return err
	}

	amArgs := buildAmDiagnoseArgs()

	fmt.Println("Running AI diagnosis via agent-mesh...")
	fmt.Println("  Flow: scan → execute → diagnose → fix → validate")
	fmt.Println()

	return execAm(amPath, amArgs)
}

// printDiagnoseAmCommand outputs the am run command for diagnosis.
func printDiagnoseAmCommand() error {
	amArgs := buildAmDiagnoseArgs()
	fmt.Printf("am %s\n", strings.Join(amArgs, " "))
	return nil
}

// buildAmDiagnoseArgs constructs arguments for am run dva-diagnose.
func buildAmDiagnoseArgs() []string {
	args := []string{"run", "dva-diagnose"}

	if diagnoseVerbose {
		args = append(args, "--verbose")
	}

	if diagnoseMaxRetries != 3 {
		args = append(args, fmt.Sprintf("param.max_retries=%d", diagnoseMaxRetries))
	}

	return args
}

func init() {
	diagnoseCmd.Flags().BoolVar(&diagnosePrint, "print", false, "Output the am run command to stdout (for manual use)")
	diagnoseCmd.Flags().BoolVarP(&diagnoseVerbose, "verbose", "v", false, "Show detailed progress")
	diagnoseCmd.Flags().IntVar(&diagnoseMaxRetries, "max-retries", 3, "Maximum feedback loop iterations")
	// Keep --interactive and --model flags for backward compatibility (ignored, handled by flow)
	diagnoseCmd.Flags().BoolVarP(&diagnoseInteractive, "interactive", "i", false, "Interactive mode (handled by agent-mesh flow)")
	diagnoseCmd.Flags().StringVar(&diagnoseModel, "model", "", "Model hint (passed to agent-mesh flow)")
	_ = diagnoseCmd.Flags().MarkHidden("interactive")
	_ = diagnoseCmd.Flags().MarkHidden("model")
	improveCmd.AddCommand(diagnoseCmd)
}
