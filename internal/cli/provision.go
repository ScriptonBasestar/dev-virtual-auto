package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/output"
)

var provisionList bool

var provisionCmd = &cobra.Command{
	Use:   "provision [PROFILE]",
	Short: "Execute the provisioning steps defined in 'dva.yml'",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()

		// --list: show available profiles and exit
		if provisionList {
			return listProvisionProfiles(c)
		}

		e := loadEnv(c)

		requested := "default"
		explicit := len(args) > 0
		if explicit {
			requested = args[0]
		}

		profile, steps, err := resolveProvisionProfile(c.Provision.Profiles, c.Provision.DefaultProfile, requested, explicit)
		if err != nil {
			return err
		}

		if dryRun {
			fmt.Printf("🔍 DRY RUN — showing execution plan for profile: %s\n\n", profile)
		} else {
			fmt.Printf("🚀 Running provision profile: %s\n\n", profile)
		}

		// Execute steps with parallel batch support
		batches := groupParallelBatches(steps)
		stepOffset := 0
		for _, batch := range batches {
			if len(batch) == 1 || !batch[0].Parallel {
				// Sequential execution
				for _, step := range batch {
					if err := executeProvisionStep(e, c, step, stepOffset, len(steps), dryRun); err != nil {
						return err
					}
					stepOffset++
				}
			} else {
				// Parallel execution
				if err := executeParallelBatch(e, c, batch, stepOffset, len(steps), dryRun); err != nil {
					return err
				}
				stepOffset += len(batch)
			}
		}

		if dryRun {
			fmt.Println("\n🔍 Dry run complete — no commands were executed.")
		} else {
			// Write provision marker so `dva up` knows this profile was run
			writeProvisionMarker(c.FileDir(), profile)
			fmt.Println("\n✅ Provision complete!")
		}
		return nil
	},
}

func init() {
	provisionCmd.Flags().BoolVarP(&provisionList, "list", "l", false, "List available provision profiles")
}

// groupParallelBatches groups steps into batches. Consecutive steps with
// Parallel=true form a single batch; non-parallel steps are each their own batch.
func groupParallelBatches(steps []config.ProvisionItem) [][]config.ProvisionItem {
	var batches [][]config.ProvisionItem
	var current []config.ProvisionItem

	for _, step := range steps {
		if step.Parallel {
			current = append(current, step)
		} else {
			if len(current) > 0 {
				batches = append(batches, current)
				current = nil
			}
			batches = append(batches, []config.ProvisionItem{step})
		}
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

// executeProvisionStep runs a single provision step sequentially.
func executeProvisionStep(e *config.Environment, c *config.Config, step config.ProvisionItem, index, total int, dryRun bool) error {
	if step.Step != "" {
		fmt.Printf("  [%d/%d] %s\n", index+1, total, step.Step)
	}

	if step.Note != "" {
		fmt.Println()
		for _, line := range strings.Split(step.Note, "\n") {
			fmt.Printf("    %s\n", line)
		}
		fmt.Println()
	}

	// Execute compose-aware commands
	if len(step.ComposeUp) > 0 {
		composeArgs := append([]string{"up", "-d"}, step.ComposeUp...)
		if dryRun {
			composeCmd, args := buildComposeArgs(e, c, composeArgs)
			fmt.Printf("    [dry-run] $ %s %s\n", composeCmd, strings.Join(args, " "))
		} else {
			if err := runProvisionCompose(e, c, step.Step, composeArgs); err != nil {
				return err
			}
		}
		return nil
	}

	if step.ComposeExec != "" {
		composeArgs := append([]string{"exec"}, strings.Fields(step.ComposeExec)...)
		if dryRun {
			composeCmd, args := buildComposeArgs(e, c, composeArgs)
			fmt.Printf("    [dry-run] $ %s %s\n", composeCmd, strings.Join(args, " "))
		} else {
			if err := runProvisionCompose(e, c, step.Step, composeArgs); err != nil {
				return err
			}
		}
		return nil
	}

	if step.ComposeRun != "" {
		composeArgs := append([]string{"run"}, strings.Fields(step.ComposeRun)...)
		if dryRun {
			composeCmd, args := buildComposeArgs(e, c, composeArgs)
			fmt.Printf("    [dry-run] $ %s %s\n", composeCmd, strings.Join(args, " "))
		} else {
			if err := runProvisionCompose(e, c, step.Step, composeArgs); err != nil {
				return err
			}
		}
		return nil
	}

	// Execute raw commands
	cmds := step.RunCommands()
	for _, cmdStr := range cmds {
		if dryRun {
			fmt.Printf("    [dry-run] $ %s\n", cmdStr)
		} else {
			fmt.Printf("    $ %s\n", cmdStr)
			if err := runShellCommand(cmdStr); err != nil {
				return fmt.Errorf("provision step '%s' failed: %w", step.Step, err)
			}
		}
	}

	// Legacy format: echo
	if step.Echo != "" {
		fmt.Printf("    %s\n", step.Echo)
	}

	// Legacy format: cmd
	if step.Cmd != "" {
		if dryRun {
			fmt.Printf("    [dry-run] $ %s\n", step.Cmd)
		} else {
			fmt.Printf("    $ %s\n", step.Cmd)
			if err := runShellCommand(step.Cmd); err != nil {
				return fmt.Errorf("provision command failed: %w", err)
			}
		}
	}
	return nil
}

// executeParallelBatch runs a batch of parallel steps concurrently.
// Output is buffered per step and printed atomically to avoid interleaving.
func executeParallelBatch(e *config.Environment, c *config.Config, batch []config.ProvisionItem, startIndex, total int, dryRun bool) error {
	fmt.Printf("  ⚡ Running %d steps in parallel...\n", len(batch))

	type result struct {
		index  int
		output string
		err    error
	}

	results := make([]result, len(batch))
	var wg sync.WaitGroup

	for i, step := range batch {
		wg.Add(1)
		go func(idx int, s config.ProvisionItem) {
			defer wg.Done()
			var buf bytes.Buffer
			stepLabel := fmt.Sprintf("[%d/%d]", startIndex+idx+1, total)

			if s.Step != "" {
				fmt.Fprintf(&buf, "  %s %s\n", stepLabel, s.Step)
			}

			if dryRun {
				// Dry run: just show what would run
				if len(s.ComposeUp) > 0 {
					composeArgs := append([]string{"up", "-d"}, s.ComposeUp...)
					composeCmd, args := buildComposeArgs(e, c, composeArgs)
					fmt.Fprintf(&buf, "    [dry-run] $ %s %s\n", composeCmd, strings.Join(args, " "))
				} else if s.ComposeExec != "" {
					composeArgs := append([]string{"exec"}, strings.Fields(s.ComposeExec)...)
					composeCmd, args := buildComposeArgs(e, c, composeArgs)
					fmt.Fprintf(&buf, "    [dry-run] $ %s %s\n", composeCmd, strings.Join(args, " "))
				} else if s.ComposeRun != "" {
					composeArgs := append([]string{"run"}, strings.Fields(s.ComposeRun)...)
					composeCmd, args := buildComposeArgs(e, c, composeArgs)
					fmt.Fprintf(&buf, "    [dry-run] $ %s %s\n", composeCmd, strings.Join(args, " "))
				} else {
					for _, cmdStr := range s.RunCommands() {
						fmt.Fprintf(&buf, "    [dry-run] $ %s\n", cmdStr)
					}
					if s.Cmd != "" {
						fmt.Fprintf(&buf, "    [dry-run] $ %s\n", s.Cmd)
					}
				}
				if s.Echo != "" {
					fmt.Fprintf(&buf, "    %s\n", s.Echo)
				}
				results[idx] = result{index: idx, output: buf.String()}
				return
			}

			// Actual execution: compose commands use exec.Command directly
			var err error
			if len(s.ComposeUp) > 0 {
				err = runProvisionCompose(e, c, s.Step, append([]string{"up", "-d"}, s.ComposeUp...))
			} else if s.ComposeExec != "" {
				err = runProvisionCompose(e, c, s.Step, append([]string{"exec"}, strings.Fields(s.ComposeExec)...))
			} else if s.ComposeRun != "" {
				err = runProvisionCompose(e, c, s.Step, append([]string{"run"}, strings.Fields(s.ComposeRun)...))
			} else {
				for _, cmdStr := range s.RunCommands() {
					fmt.Fprintf(&buf, "    $ %s\n", cmdStr)
					if err = runShellCommand(cmdStr); err != nil {
						err = fmt.Errorf("provision step '%s' failed: %w", s.Step, err)
						break
					}
				}
				if err == nil && s.Cmd != "" {
					fmt.Fprintf(&buf, "    $ %s\n", s.Cmd)
					if err = runShellCommand(s.Cmd); err != nil {
						err = fmt.Errorf("provision command failed: %w", err)
					}
				}
			}
			if s.Echo != "" {
				fmt.Fprintf(&buf, "    %s\n", s.Echo)
			}
			results[idx] = result{index: idx, output: buf.String(), err: err}
		}(i, step)
	}

	wg.Wait()

	// Print output in order and collect errors
	var errs []error
	for _, r := range results {
		if r.output != "" {
			fmt.Print(r.output)
		}
		if r.err != nil {
			errs = append(errs, r.err)
		}
	}

	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return fmt.Errorf("parallel provision failed:\n  %s", strings.Join(msgs, "\n  "))
	}
	return nil
}

// resolveProvisionProfile resolves which provision profile to use.
// Priority: exact match → default_profile alias → single-profile auto → error.
func resolveProvisionProfile(provision map[string][]config.ProvisionItem, defaultProfile string, requested string, explicit bool) (string, []config.ProvisionItem, error) {
	// Direct lookup
	if steps, ok := provision[requested]; ok {
		return requested, steps, nil
	}

	// No provision defined at all
	if len(provision) == 0 {
		return "", nil, fmt.Errorf("no provision commands defined in dva.yml")
	}

	// Implicit fallbacks only (user did not specify a profile name)
	if requested == "default" && !explicit {
		// default_profile alias (explicit config, works with any number of profiles)
		if defaultProfile != "" {
			if steps, ok := provision[defaultProfile]; ok {
				fmt.Fprintf(os.Stderr, "⚠ Profile 'default' not found, using '%s' (default_profile)\n\n", defaultProfile)
				return defaultProfile, steps, nil
			}
			fmt.Fprintf(os.Stderr, "⚠ default_profile '%s' not found in provision profiles — ignoring\n\n", defaultProfile)
		}

		// Auto-fallback: exactly 1 profile
		if len(provision) == 1 {
			for k, v := range provision {
				fmt.Fprintf(os.Stderr, "⚠ Profile 'default' not found, using '%s' (only available profile)\n\n", k)
				return k, v, nil
			}
		}
	}

	// Build available list
	available := make([]string, 0, len(provision))
	for k := range provision {
		available = append(available, k)
	}
	sort.Strings(available)

	// "Did you mean?" suggestion
	var suggestions []string
	for _, name := range available {
		if levenshtein(requested, name) <= 2 {
			suggestions = append(suggestions, name)
		}
	}

	msg := fmt.Sprintf("provision profile '%s' not found. Available: %s", requested, strings.Join(available, ", "))
	if len(suggestions) > 0 {
		msg += "\n\nDid you mean?"
		for _, s := range suggestions {
			msg += fmt.Sprintf("\n  dva provision %s", s)
		}
	}

	return "", nil, fmt.Errorf("%s", msg)
}

// listProvisionProfiles prints provision profiles in the requested format.
func listProvisionProfiles(c *config.Config) error {
	if len(c.Provision.Profiles) == 0 {
		fmt.Println("No provision profiles defined.")
		return nil
	}

	keys := make([]string, 0, len(c.Provision.Profiles))
	for k := range c.Provision.Profiles {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if jsonOutput {
		return printProvisionJSON(c.Provision.Profiles, c.Provision.DefaultProfile, keys)
	}
	return printProvisionTable(c.Provision.Profiles, c.Provision.DefaultProfile, keys)
}

func printProvisionTable(provision map[string][]config.ProvisionItem, defaultProfile string, keys []string) error {
	maxName := len("PROFILE")
	for _, k := range keys {
		if len(k) > maxName {
			maxName = len(k)
		}
	}
	if defaultProfile != "" {
		maxName += 2 // room for " *" suffix
	}

	fmt.Printf("%-*s  %5s  %s\n", maxName, "PROFILE", "STEPS", "FIRST STEP")
	for _, k := range keys {
		steps := provision[k]
		desc := firstStepDescription(steps)
		display := k
		if k == defaultProfile {
			display = k + " *"
		}
		fmt.Printf("%-*s  %5d  %s\n", maxName, display, len(steps), desc)
	}
	if defaultProfile != "" {
		fmt.Printf("\n* default profile\n")
	}
	return nil
}

func printProvisionJSON(provision map[string][]config.ProvisionItem, defaultProfile string, keys []string) error {
	profiles := make(map[string]any, len(keys))
	for _, k := range keys {
		steps := provision[k]
		profiles[k] = map[string]any{
			"steps":      len(steps),
			"first_step": firstStepDescription(steps),
		}
	}
	result := map[string]any{"profiles": profiles}
	if defaultProfile != "" {
		result["default_profile"] = defaultProfile
	}
	return output.PrintJSON(result)
}

func firstStepDescription(steps []config.ProvisionItem) string {
	if len(steps) == 0 {
		return ""
	}
	s := steps[0]
	if s.Step != "" {
		return s.Step
	}
	if s.Raw != "" {
		return s.Raw
	}
	if s.Echo != "" {
		return s.Echo
	}
	return ""
}

// runProvisionCompose builds and runs a compose command for a provision step.
// Uses exec.Command directly instead of shell to avoid command injection.
func runProvisionCompose(e *config.Environment, c *config.Config, stepName string, args []string) error {
	composeCmd, composeArgs := buildComposeArgs(e, c, args)
	fmt.Printf("    $ %s %s\n", composeCmd, strings.Join(composeArgs, " "))

	cmd := exec.Command(composeCmd, composeArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = e.EnvSlice()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("provision step '%s' failed: %w", stepName, err)
	}
	return nil
}

func runShellCommand(cmdStr string) error {
	var c *exec.Cmd

	// Platform-specific shell selection
	switch runtime.GOOS {
	case "windows":
		// Use PowerShell for better compatibility
		c = exec.Command("powershell", "-Command", cmdStr)
	default:
		// Unix-like systems (Linux, macOS, BSD, etc.)
		c = exec.Command("sh", "-c", cmdStr)
	}

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// clearProvisionMarkers removes all provision marker files from .sb/dva/.
// Called by `dva clean --volumes` so that provision suggestions reappear after a reset.
func clearProvisionMarkers(configDir string) {
	markerDir := filepath.Join(configDir, config.DotDirName)
	entries, err := os.ReadDir(markerDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "provisioned-") {
			os.Remove(filepath.Join(markerDir, e.Name()))
		}
	}
}

// writeProvisionMarker creates a marker file indicating that a provision
// profile has been run. Used by `dva up` to skip provision suggestions.
func writeProvisionMarker(configDir, profile string) {
	markerDir := filepath.Join(configDir, config.DotDirName)
	if err := os.MkdirAll(markerDir, 0755); err != nil {
		return
	}
	markerFile := filepath.Join(markerDir, "provisioned-"+profile)
	if err := os.WriteFile(markerFile, []byte(""), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "[warn] could not write provision marker: %v\n", err)
	}
}
