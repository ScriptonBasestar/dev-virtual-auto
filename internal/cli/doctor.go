package cli

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/output"
)

// DoctorResult holds the outcome of a single doctor check.
type DoctorResult struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	FixHint string `json:"fix_hint,omitempty"`
	Fixable bool   `json:"fixable"`
	Fixed   bool   `json:"fixed,omitempty"`
	fixFunc func() error // built-in fix function (unexported)
}

var doctorFix bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check environment prerequisites and diagnose common setup issues",
	Long: `Run environment checks defined in the 'checks' section of dva.yml.
Also runs built-in checks for Docker availability and compose file existence.

Useful for diagnosing setup problems before running 'dva up' or 'dva provision'.
Use --fix to automatically resolve fixable issues.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()

		results := runDoctorChecks(c)

		if doctorFix {
			applyDoctorFixes(results)
		}

		if jsonOutput {
			return output.PrintJSON(map[string]any{"checks": results})
		}

		printDoctorResults(results)
		return nil
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Automatically fix issues that can be resolved")
	doctorCmd.GroupID = "project"
	rootCmd.AddCommand(doctorCmd)
}

// runDoctorChecks runs built-in checks plus user-defined checks from dva.yml.
func runDoctorChecks(c *config.Config) []DoctorResult {
	var results []DoctorResult

	// Built-in: Docker daemon accessible
	results = append(results, checkDocker())

	// Built-in: compose files exist
	for _, f := range c.AllComposeFiles() {
		path := f
		if !filepath.IsAbs(path) {
			path = filepath.Join(c.FileDir(), f)
		}
		passed := fileExists(path)
		results = append(results, DoctorResult{
			Name:    fmt.Sprintf("Compose file exists: %s", f),
			Passed:  passed,
			FixHint: condStr(!passed, fmt.Sprintf("Create or check path: %s", f)),
		})
	}

	// User-defined checks
	for _, check := range c.DoctorChecks {
		results = append(results, runSingleCheck(check, c.FileDir()))
	}

	// Built-in: devcontainer.json exists (when devcontainer section is enabled)
	if len(c.Devcontainer) > 0 && isDevcontainerEnabled(c.Devcontainer) {
		results = append(results, runSingleCheck(config.DoctorCheck{
			Name:    "devcontainer.json exists",
			Type:    "file_exists",
			Path:    ".devcontainer/devcontainer.json",
			FixHint: "Run: dva add devcontainer",
		}, c.FileDir()))
	}

	// Built-in: Check if .sb/dva is ignored in .gitignore
	results = append(results, checkGitignoreStatus(c.FileDir()))

	return results
}

// applyDoctorFixes attempts to fix all failed checks that have a fix function or command.
func applyDoctorFixes(results []DoctorResult) {
	for i := range results {
		r := &results[i]
		if r.Passed || !r.Fixable {
			continue
		}
		if r.fixFunc != nil {
			if err := r.fixFunc(); err != nil {
				fmt.Fprintf(os.Stderr, "  [fix-err] %s: %v\n", r.Name, err)
			} else {
				r.Fixed = true
				r.Passed = true
			}
		}
	}
}

func checkGitignoreStatus(configDir string) DoctorResult {
	r := DoctorResult{Name: fmt.Sprintf("%s/ is ignored in .gitignore", config.DotDirName)}

	gitignorePath := filepath.Join(configDir, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			r.Passed = false
			r.Fixable = true
			r.FixHint = fmt.Sprintf("Create .gitignore and add %s/ or run 'dva doctor --fix'", config.DotDirName)
			r.fixFunc = func() error {
				_, err := ensureGitignore(configDir)
				return err
			}
			return r
		}
		r.Passed = false
		return r
	}

	if isDvaIgnored(string(data)) {
		r.Passed = true
	} else {
		r.Passed = false
		r.Fixable = true
		r.FixHint = fmt.Sprintf("Add '%s/' to .gitignore to avoid committing transient state", config.DotDirName)
		r.fixFunc = func() error {
			_, err := ensureGitignore(configDir)
			return err
		}
	}

	return r
}

func runSingleCheck(check config.DoctorCheck, configDir string) DoctorResult {
	r := DoctorResult{
		Name:    check.Name,
		FixHint: check.FixHint,
	}

	switch check.Type {
	case "file_exists":
		path := check.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(configDir, path)
		}
		r.Passed = fileExists(path)

	case "command":
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		r.Passed = exec.CommandContext(ctx, "sh", "-c", check.Command).Run() == nil

	case "docker_socket":
		r.Passed = isDockerSocketAccessible()

	default:
		r.Passed = false
		r.FixHint = fmt.Sprintf("Unknown check type: %s", check.Type)
	}

	// Clear fix hint on success
	if r.Passed {
		r.FixHint = ""
	}

	// If check has a fix command, mark as fixable
	if !r.Passed && check.Fix != "" {
		r.Fixable = true
		fixCmd := check.Fix
		r.fixFunc = func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "sh", "-c", fixCmd)
			cmd.Dir = configDir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
	}

	return r
}

func checkDocker() DoctorResult {
	r := DoctorResult{Name: "Docker daemon accessible"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	r.Passed = cmd.Run() == nil

	if !r.Passed {
		r.FixHint = "Start Docker Desktop or ensure dockerd is running"
	}
	return r
}

func isDockerSocketAccessible() bool {
	sockPath := "/var/run/docker.sock"
	if _, err := os.Stat(sockPath); err != nil {
		return false
	}
	// Try connecting to verify it's alive
	conn, err := net.DialTimeout("unix", sockPath, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func printDoctorResults(results []DoctorResult) {
	fmt.Println("Environment Checks:")
	fmt.Println()

	passed := 0
	failed := 0
	fixed := 0
	for _, r := range results {
		if r.Fixed {
			fmt.Printf("  [fixed] %s\n", r.Name)
			fixed++
			passed++
		} else if r.Passed {
			fmt.Printf("  [pass] %s\n", r.Name)
			passed++
		} else {
			fmt.Printf("  [FAIL] %s\n", r.Name)
			if r.FixHint != "" {
				fmt.Printf("         -> %s\n", r.FixHint)
			}
			if r.Fixable {
				fmt.Printf("         -> Run 'dva doctor --fix' to auto-fix\n")
			}
			failed++
		}
	}

	summary := fmt.Sprintf("\n  %d passed, %d failed", passed, failed)
	if fixed > 0 {
		summary += fmt.Sprintf(" (%d auto-fixed)", fixed)
	}
	fmt.Println(summary)
}

func condStr(cond bool, s string) string {
	if cond {
		return s
	}
	return ""
}
