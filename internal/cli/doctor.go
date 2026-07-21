package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
	"github.com/ScriptonBasestar/dva/internal/output"
)

// DoctorResult holds the outcome of a single doctor check.
type DoctorResult struct {
	Name        string `json:"name"`
	Passed      bool   `json:"passed"`
	FixHint     string `json:"fix_hint,omitempty"`
	Fixable     bool   `json:"fixable,omitempty"`
	Fixed       bool   `json:"fixed,omitempty"`
	UserDefined bool   `json:"user_defined,omitempty"`
	fixFunc     func() error
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
				if err := output.PrintJSON(map[string]any{"checks": results}); err != nil {
					return err
				}
				return doctorExitError(results)
			}

			printDoctorResults(results)
			return doctorExitError(results)
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

	// Built-in: portable daemon check, with a Linux-only socket diagnostic on failure.
	results = append(results, runDockerDoctorChecks(checkDocker, checkDockerSocketPermissions, runtime.GOOS)...)

	// Built-in: Compose project name alignment
	results = append(results, checkComposeProjectNameAlignment(c))

	// Built-in: Environment files exist
	results = append(results, checkEnvFiles(c)...)

	// Built-in: non-Compose stack entry files exist
	results = append(results, checkStackFiles(c)...)

	// Built-in: Compose files exist
	results = append(results, checkComposeFiles(c)...)

	// Built-in: Compose config resolves (catches include:/-f targets that the
	// per-file existence check above misses — e.g. compose.yaml includes a file
	// that was renamed/removed). Runs `docker compose config`, which needs no daemon.
	results = append(results, checkComposeConfigResolves(c)...)

	for _, check := range c.DoctorChecks {
		r := runSingleCheck(check, c.FileDir())
		r.UserDefined = true
		results = append(results, r)
	}

	// Built-in: devcontainer.json exists (when devcontainer section is enabled)
	if len(c.Devcontainer) > 0 && isDevcontainerEnabled(c.Devcontainer) {
		results = append(results, runSingleCheck(config.DoctorCheck{
			Name:    "devcontainer.json exists",
			Type:    "file_exists",
			Path:    ".devcontainer/devcontainer.json",
			FixHint: "Run: dva config validate --fix",
		}, c.FileDir()))
	}

	// Built-in: Check if .sb/dva is ignored in .gitignore
	results = append(results, checkGitignoreStatus(c.FileDir()))

	// Built-in: application ports are owned by processes dva tracks (not stale orphans)
	results = append(results, checkAppPortOwnership(c)...)

	return results
}

// checkAppPortOwnership flags applications whose declared port is held by a
// process dva did not start — a stale orphan from a previous run, or a child
// that outlived its tracked group. This is the condition that lets a dead app
// look "healthy" because something else is answering on its port.
func checkAppPortOwnership(c *config.Config) []DoctorResult {
	if len(c.Applications) == 0 {
		return nil
	}

	am := lifecycle.NewAppManager(c, loadEnv(c))
	conflicts := am.PortConflicts()
	if len(conflicts) == 0 {
		return []DoctorResult{{
			Name:   "Application ports owned by dva-tracked processes",
			Passed: true,
		}}
	}

	results := make([]DoctorResult, 0, len(conflicts))
	for _, pc := range conflicts {
		results = append(results, DoctorResult{
			Name:    fmt.Sprintf("App %q port %d held by a process dva did not start", pc.App, pc.Port),
			Passed:  false,
			FixHint: fmt.Sprintf("PID %d owns port %d (stale orphan?). Run 'dva app down %s' to reclaim it, then 'dva app up %s'.", pc.ForeignPID, pc.Port, pc.App, pc.App),
		})
	}
	return results
}

func runDockerDoctorChecks(
	daemonCheck func() DoctorResult,
	socketCheck func() DoctorResult,
	goos string,
) []DoctorResult {
	daemon := daemonCheck()
	results := []DoctorResult{daemon}
	if !daemon.Passed && goos == "linux" {
		results = append(results, socketCheck())
	}
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
			r.FixHint = fmt.Sprintf("Create .gitignore and add '%s/' to avoid committing transient state", config.DotDirName)
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
	res := checkDockerSocketPermissions()
	return res.Passed
}

func checkDockerSocketPermissions() DoctorResult {
	sockPath := "/var/run/docker.sock"
	_, err := os.Stat(sockPath)
	if err != nil {
		return DoctorResult{
			Name:    "Docker socket accessible",
			Passed:  false,
			FixHint: "Docker not running or socket path incorrect",
		}
	}

	// Make sure current user can open it
	f, err := os.Open(sockPath)
	if err != nil {
		return DoctorResult{
			Name:    "Docker socket permissions",
			Passed:  false,
			FixHint: "Add user to docker group or use sudo",
		}
	}
	_ = f.Close()

	return DoctorResult{
		Name:   "Docker socket permissions",
		Passed: true,
	}
}

func checkComposeProjectNameAlignment(c *config.Config) DoctorResult {
	warnings := c.ValidateComposeProjectNames()
	if len(warnings) == 0 {
		return DoctorResult{
			Name:   "Compose project name alignment",
			Passed: true,
		}
	}

	w := warnings[0] // show first warning
	msg := fmt.Sprintf("Compose file %s has %s", w.File,
		condStr(w.ComposeName == "", "missing project name", fmt.Sprintf("name '%s'", w.ComposeName)))

	return DoctorResult{
		Name:    msg,
		Passed:  false,
		FixHint: fmt.Sprintf("Set 'name: %s' in %s", w.DvaName, w.File),
		Fixable: true,
		fixFunc: func() error { return c.FixComposeProjectName(w) },
	}
}

func checkEnvFiles(c *config.Config) []DoctorResult {
	var results []DoctorResult
	cfgDir := c.FileDir()

	for _, envFile := range c.AllEnvFiles() {
		if envFile == "" {
			continue
		}
		path := envFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(cfgDir, envFile)
		}

		passed := fileExists(path)
		results = append(results, DoctorResult{
			Name:    fmt.Sprintf("Environment file exists: %s", envFile),
			Passed:  passed,
			FixHint: condStr(!passed, fmt.Sprintf("Create or check path: %s", envFile)),
		})
	}

	return results
}

func checkStackFiles(c *config.Config) []DoctorResult {
	var results []DoctorResult
	cfgDir := c.FileDir()

	for name, entry := range c.Stack {
		if entry.ComposeConfig() != nil {
			continue
		}

		var files []string
		if entry.Kubectl != nil {
			files = []string{entry.Kubectl.Kubeconfig}
		}

		for _, f := range files {
			if f == "" {
				continue
			}
			path := f
			if !filepath.IsAbs(path) {
				path = filepath.Join(cfgDir, f)
			}

			passed := fileExists(path)
			results = append(results, DoctorResult{
				Name:    fmt.Sprintf("Stack file exists: %s (%s)", name, f),
				Passed:  passed,
				FixHint: condStr(!passed, fmt.Sprintf("Create or check path: %s", f)),
			})
		}
	}

	return results
}

func checkComposeFiles(c *config.Config) []DoctorResult {
	var results []DoctorResult
	seen := make(map[string]struct{})

	for _, f := range c.AllComposeFiles() {
		if f == "" {
			continue
		}

		path := f
		if !filepath.IsAbs(path) {
			path = filepath.Join(c.FileDir(), f)
		}
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		passed := fileExists(path)
		results = append(results, DoctorResult{
			Name:    fmt.Sprintf("Compose file exists: %s", f),
			Passed:  passed,
			FixHint: condStr(!passed, fmt.Sprintf("Create or check path: %s", f)),
		})
	}

	return results
}

// checkComposeConfigResolves runs `docker compose config -q` for the primary
// compose file set so a compose.yaml whose -f or include: target does not
// resolve is caught here (before `dva up`) rather than only failing at up time.
// `docker compose config` only parses/merges files and needs no daemon; the
// check is skipped when the docker CLI is absent (the daemon check reports that).
func checkComposeConfigResolves(c *config.Config) []DoctorResult {
	cc := c.PrimaryComposeConfig()
	if cc == nil || len(cc.Files) == 0 {
		return nil
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return nil
	}

	args := []string{"compose"}
	for _, f := range cc.Files {
		path := f
		if !filepath.IsAbs(path) {
			path = filepath.Join(c.FileDir(), f)
		}
		args = append(args, "-f", path)
	}
	if cc.ProjectName != "" {
		args = append(args, "--project-name", cc.ProjectName)
	}
	args = append(args, "config", "--quiet")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = c.FileDir()
	out, err := cmd.CombinedOutput()
	if err == nil {
		return []DoctorResult{{Name: "Compose config resolves", Passed: true}}
	}

	detail := firstNonEmptyLine(string(out))
	hint := "check compose.files in dva.yml and any include: paths, then run: docker compose config"
	if detail != "" {
		hint = detail + " — " + hint
	}
	return []DoctorResult{{
		Name:    "Compose config resolves",
		Passed:  false,
		FixHint: hint,
	}}
}

// firstNonEmptyLine returns the first non-blank line of s, trimmed.
func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
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
			if r.Fixable && !strings.Contains(r.FixHint, "--fix") {
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

func doctorExitError(results []DoctorResult) error {
	failed := 0
	for _, r := range results {
		if r.UserDefined && !r.Passed {
			failed++
		}
	}
	if failed == 0 {
		return nil
	}
	return fmt.Errorf("%d user check(s) failed", failed)
}

func condStr(cond bool, s string, fallback ...string) string {
	if cond {
		return s
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return ""
}
