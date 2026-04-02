package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

var improvePrint bool
var improveDocsOnly bool
var improveVerbose bool
var improveRecursive bool
var improveRewrite bool
var improveInteractive bool
var improveModel string

var improveCmd = &cobra.Command{
	Use:   "improve",
	Short: "Review and improve the current dva.yml via agent-mesh workflow",
	Long: `Runs AI-assisted improvement of dva.yml via agent-mesh flow.

If dva.yml does not exist, it scaffolds one first (auto-detecting project type),
then runs AI improvement on it. If dva.yml already exists, it improves in place.

Use --rewrite to rebuild dva.yml from scratch based on project analysis.
Use --recursive to also improve dva.yml in detected sub-projects.
Use --interactive to run the guided 5-stage improve pipeline.
Use --print to show the am run command that would be executed.
Use --docs-only to only regenerate CLAUDE.md/AGENTS.md (dva.yml unchanged).

Flags --print, --docs-only, --interactive are mutually exclusive (first match wins).
--interactive cannot be combined with --recursive.

Requires 'am' (agent-mesh) CLI in PATH.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if improveInteractive && improveRecursive {
			return fmt.Errorf("--interactive and --recursive cannot be combined")
		}
		if improvePrint {
			return printAmRunCommand()
		}
		if improveDocsOnly {
			return runAIDocsOnly()
		}
		if improveInteractive {
			return runAmImproveGuided()
		}
		if err := runAmImprove(); err != nil {
			return err
		}
		if improveRecursive {
			return runAmImproveRecursive()
		}
		return nil
	},
}

// runAmImprove executes the dva-improve flow via agent-mesh CLI.
func runAmImprove() error {
	// Scaffold dva.yml if missing
	if !dvaConfigExists() {
		fmt.Println("No dva.yml found — scaffolding initial configuration...")
		if _, err := scaffoldDvaYml(".", ""); err != nil {
			return fmt.Errorf("failed to scaffold dva.yml: %w", err)
		}
		fmt.Println()
	}

	amPath, err := findAmCLI()
	if err != nil {
		return err
	}

	mode := "preserve"
	if improveRewrite {
		mode = "rewrite"
	}

	amArgs := buildAmArgs("dva-improve", map[string]string{
		"mode": mode,
	})

	fmt.Println("Running AI improvement via agent-mesh...")
	fmt.Println()

	return execAm(amPath, amArgs)
}

// runAmImproveGuided executes the guided 5-stage improve pipeline via agent-mesh.
func runAmImproveGuided() error {
	// Scaffold dva.yml if missing
	if !dvaConfigExists() {
		fmt.Println("No dva.yml found — scaffolding initial configuration...")
		if _, err := scaffoldDvaYml(".", ""); err != nil {
			return fmt.Errorf("failed to scaffold dva.yml: %w", err)
		}
		fmt.Println()
	}

	amPath, err := findAmCLI()
	if err != nil {
		return err
	}

	amArgs := buildAmArgs("dva-improve-guided", nil)

	fmt.Println("Running guided improve pipeline via agent-mesh...")
	fmt.Println("  Pipeline: analyze → verify (approval) → transform → configure → execute")
	fmt.Println()

	return execAm(amPath, amArgs)
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

	fmt.Printf("Generated %s\n", guidePath)
	return nil
}

// runAmImproveRecursive finds sub-projects with dva.yml and runs improve in each.
func runAmImproveRecursive() error {
	var subs []subInfo
	scanForSubprojects(".", 0, 3, &subs)

	var targets []string
	for _, sp := range subs {
		for _, name := range []string{config.FileName, config.FileNameAlt} {
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

	amPath, err := findAmCLI()
	if err != nil {
		return err
	}

	mode := "preserve"
	if improveRewrite {
		mode = "rewrite"
	}

	for _, dir := range targets {
		fmt.Printf("\nImproving %s/dva.yml...\n", dir)

		absDir, err := filepath.Abs(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not resolve %s: %v\n", dir, err)
			continue
		}

		amArgs := buildAmArgs("dva-improve", map[string]string{
			"mode":   mode,
			"target": absDir,
		})

		if err := execAm(amPath, amArgs); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", dir, err)
		}
	}

	return nil
}

// printAmRunCommand outputs the am run command that would be executed.
func printAmRunCommand() error {
	mode := "preserve"
	if improveRewrite {
		mode = "rewrite"
	}

	flowName := "dva-improve"
	if improveInteractive {
		flowName = "dva-improve-guided"
	}

	amArgs := buildAmArgs(flowName, map[string]string{
		"mode": mode,
	})

	fmt.Printf("am %s\n", strings.Join(amArgs, " "))
	return nil
}

// --- Agent-Mesh CLI helpers ---

// findAmCLI locates the agent-mesh CLI binary.
func findAmCLI() (string, error) {
	amPath, err := exec.LookPath("am")
	if err != nil {
		return "", fmt.Errorf("agent-mesh CLI (am) not found in PATH.\n  Install: https://github.com/user/agent-mesh\n  Or use 'dva config improve --print' to see the command")
	}
	return amPath, nil
}

// buildAmArgs constructs arguments for am run.
func buildAmArgs(flowName string, params map[string]string) []string {
	args := []string{"run", flowName}

	if improveVerbose {
		args = append(args, "--verbose")
	}

	for k, v := range params {
		if v != "" {
			args = append(args, fmt.Sprintf("param.%s=%s", k, v))
		}
	}

	return args
}

// execAm runs the agent-mesh CLI with given arguments, streaming output to stdout/stderr.
func execAm(amPath string, args []string) error {
	cmd := exec.Command(amPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("agent-mesh CLI failed: %w", err)
	}
	return nil
}

// dvaConfigExists checks whether dva.yml or dva.yaml exists in the current directory only.
func dvaConfigExists() bool {
	for _, name := range []string{config.FileName, config.FileNameAlt} {
		if _, err := os.Stat(name); err == nil {
			return true
		}
	}
	return false
}

func init() {
	improveCmd.Flags().BoolVar(&improvePrint, "print", false, "Output the am run command to stdout (for manual use)")
	improveCmd.Flags().BoolVar(&improveDocsOnly, "docs-only", false, "Only regenerate CLAUDE.md/AGENTS.md (dva.yml unchanged)")
	improveCmd.Flags().BoolVarP(&improveVerbose, "verbose", "v", false, "Show detailed progress during execution")
	improveCmd.Flags().BoolVar(&improveRecursive, "recursive", false, "Also improve dva.yml in detected sub-projects")
	improveCmd.Flags().BoolVar(&improveRewrite, "rewrite", false, "Rewrite dva.yml from scratch based on project analysis (ignores existing structure)")
	improveCmd.Flags().BoolVarP(&improveInteractive, "interactive", "i", false, "Run guided 5-stage improve pipeline (with user approval gate)")
	improveCmd.Flags().StringVar(&improveModel, "model", "", "Model hint (passed to agent-mesh flow)")
	configCmd.AddCommand(improveCmd)
}
