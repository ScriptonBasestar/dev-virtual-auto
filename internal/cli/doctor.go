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
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check environment prerequisites and diagnose common setup issues",
	Long: `Run environment checks defined in the 'checks' section of dva.yml.
Also runs built-in checks for Docker availability and compose file existence.

Useful for diagnosing setup problems before running 'dva up' or 'dva provision'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()

		results := runDoctorChecks(c)

		if jsonOutput {
			return output.PrintJSON(map[string]any{"checks": results})
		}

		printDoctorResults(results)
		return nil
	},
}

func init() {
	doctorCmd.GroupID = "project"
	rootCmd.AddCommand(doctorCmd)
}

// runDoctorChecks runs built-in checks plus user-defined checks from dva.yml.
func runDoctorChecks(c *config.Config) []DoctorResult {
	var results []DoctorResult

	// Built-in: Docker daemon accessible
	results = append(results, checkDocker())

	// Built-in: compose files exist
	for _, f := range c.Compose.Files {
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

	return results
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

	return r
}

func checkDocker() DoctorResult {
	r := DoctorResult{Name: "Docker daemon accessible"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.Passed = exec.CommandContext(ctx, "docker", "info").Run() == nil

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
	for _, r := range results {
		if r.Passed {
			fmt.Printf("  [pass] %s\n", r.Name)
			passed++
		} else {
			fmt.Printf("  [FAIL] %s\n", r.Name)
			if r.FixHint != "" {
				fmt.Printf("         -> %s\n", r.FixHint)
			}
			failed++
		}
	}

	fmt.Printf("\n  %d passed, %d failed\n", passed, failed)
}

func condStr(cond bool, s string) string {
	if cond {
		return s
	}
	return ""
}
