package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/runner"
)

var validateStrict bool

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the syntax and schema of 'dva.yml'",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()

		if err := c.Validate(); err != nil {
			return err
		}

		// Check compose file project name alignment
		warnings := c.ValidateComposeProjectNames()
		fix, _ := cmd.Flags().GetBool("fix")

		if fix {
			fixComposeNameWarnings(c, warnings)
		} else {
			printComposeNameWarnings(warnings)
		}

		// Semantic warnings (version, health checks, duplicate commands, etc.)
		semanticWarnings := c.ValidateWarnings()
		for _, w := range semanticWarnings {
			fmt.Fprintf(os.Stderr, "[warn] %s\n", w)
		}

		driftWarnings := detectConfigDriftWarnings(c)
		printConfigDriftWarnings(driftWarnings)
		printConfigSuggestionWarnings(detectConfigSuggestionWarnings(c))

		if validateStrict && (len(driftWarnings) > 0 || len(semanticWarnings) > 0) {
			return fmt.Errorf("config warnings detected; review warnings above or run 'dva config improve --print'")
		}

		// Check devcontainer sync
		if len(c.Devcontainer) > 0 && isDevcontainerEnabled(c.Devcontainer) {
			dcPath := filepath.Join(c.FileDir(), ".devcontainer", "devcontainer.json")
			if _, err := os.Stat(dcPath); os.IsNotExist(err) {
				if fix {
					if err := writeDevcontainerFiles(c.Devcontainer, c.AllComposeFiles(), c.FileDir()); err != nil {
						fmt.Fprintf(os.Stderr, "[error] devcontainer: %v\n", err)
					} else {
						fmt.Fprintf(os.Stderr, "[fixed] created .devcontainer/devcontainer.json\n")
					}
				} else {
					fmt.Fprintf(os.Stderr, "[warn] devcontainer section found but .devcontainer/devcontainer.json missing\n")
					fmt.Fprintf(os.Stderr, "       → run: dva add devcontainer  (or dva config validate --fix)\n")
				}
			}
		}

		fmt.Println("✅ dva.yml is valid")
		return nil
	},
}

func init() {
	validateCmd.Flags().Bool("fix", false, "Auto-fix compose file project name mismatches")
	validateCmd.Flags().BoolVar(&validateStrict, "strict", false, "Fail validation when config drift warnings are detected")
	configCmd.AddCommand(validateCmd)
}

// printComposeNameWarnings prints warnings about compose file name mismatches to stderr.
func printComposeNameWarnings(warnings []config.ComposeNameWarning) {
	for _, w := range warnings {
		if w.ComposeName == "" {
			fmt.Fprintf(os.Stderr, "[warn] %s: missing top-level 'name: %s'\n", w.File, w.DvaName)
			fmt.Fprintf(os.Stderr, "       Running 'docker compose up' directly will use the directory name as project,\n")
			fmt.Fprintf(os.Stderr, "       causing port conflicts with dva. Fix: add 'name: %s' to %s\n", w.DvaName, w.File)
		} else {
			fmt.Fprintf(os.Stderr, "[warn] %s: name '%s' differs from dva.yml project_name '%s'\n", w.File, w.ComposeName, w.DvaName)
			fmt.Fprintf(os.Stderr, "       Fix: change 'name: %s' to 'name: %s' in %s\n", w.ComposeName, w.DvaName, w.File)
		}
	}
}

// fixComposeNameWarnings auto-fixes compose file name mismatches.
func fixComposeNameWarnings(c *config.Config, warnings []config.ComposeNameWarning) {
	for _, w := range warnings {
		if err := c.FixComposeProjectName(w); err != nil {
			fmt.Fprintf(os.Stderr, "[error] failed to fix %s: %v\n", w.File, err)
		} else {
			fmt.Fprintf(os.Stderr, "[fixed] %s: set 'name: %s'\n", w.File, w.DvaName)
		}
	}
}

func printConfigDriftWarnings(warnings []string) {
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "[warn] config drift: %s\n", warning)
	}
}

func detectConfigDriftWarnings(c *config.Config) []string {
	var warnings []string

	detectedCompose := detectComposeFilesInDir(c.FileDir())
	if len(detectedCompose) > 0 {
		configured := normalizeRelativePaths(c.AllComposeFiles())
		if !sameStringSlice(configured, detectedCompose) {
			warnings = append(warnings,
				fmt.Sprintf("compose.files is %s but detected root compose files are %s; review whether dva.yml is tracking the current project layout",
					formatList(configured), formatList(detectedCompose)))
		}
	}

	availableServices := configuredComposeServices(c)
	if len(availableServices) == 0 {
		return warnings
	}

	tree := runner.NewInteractionTree(c.Interaction)
	for name, cmd := range tree.List() {
		if cmd.Service == "" {
			continue
		}
		if !availableServices[cmd.Service] {
			warnings = append(warnings,
				fmt.Sprintf("interaction %q references compose service %q, but configured compose files expose %s",
					name, cmd.Service, formatList(sortedSetKeys(availableServices))))
		}
	}

	return warnings
}

func printConfigSuggestionWarnings(warnings []string) {
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "[warn] config suggestion: %s\n", warning)
	}
}

func detectConfigSuggestionWarnings(c *config.Config) []string {
	commandSet := map[string]bool{}
	for name := range runner.NewInteractionTree(c.Interaction).List() {
		commandSet[name] = true
	}

	candidates := map[string]string{}
	for _, target := range extractDocumentedMakefileTargetNamesInDir(c.FileDir()) {
		candidates[target] = "Makefile"
	}
	for _, script := range extractPackageScriptNamesInDir(c.FileDir()) {
		if _, exists := candidates[script]; !exists {
			candidates[script] = "package.json"
		}
	}

	var names []string
	for name := range candidates {
		names = append(names, name)
	}
	sort.Strings(names)

	var warnings []string
	for _, name := range names {
		if commandSet[name] {
			continue
		}
		warnings = append(warnings,
			fmt.Sprintf("%s defines %q but no DVA interaction with the same name exists; consider adding a direct mapping if it is part of the developer workflow",
				candidates[name], name))
	}

	return warnings
}

func detectComposeFilesInDir(dir string) []string {
	candidates := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	var found []string
	for _, name := range candidates {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			found = append(found, name)
		}
	}

	for _, name := range []string{"docker-compose.override.yml", "docker-compose.override.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			found = append(found, name)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return found
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "docker-compose.") &&
			(strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) &&
			!contains(found, name) {
			found = append(found, name)
		}
	}

	if len(found) > 1 {
		primary := []string{}
		rest := []string{}
		for _, file := range found {
			switch filepath.Base(file) {
			case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
				primary = append(primary, file)
			default:
				rest = append(rest, file)
			}
		}
		found = append(primary, rest...)
	}

	return found
}

func configuredComposeServices(c *config.Config) map[string]bool {
	services := map[string]bool{}
	for _, file := range c.AllComposeFiles() {
		path := file
		if !filepath.IsAbs(path) {
			path = filepath.Join(c.FileDir(), file)
		}
		for _, service := range extractComposeServices(path) {
			services[service] = true
		}
	}
	return services
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func normalizeRelativePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		out = append(out, filepath.ToSlash(path))
	}
	return out
}

func sortedSetKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func formatList(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}

func extractDocumentedMakefileTargetNamesInDir(dir string) []string {
	makefilePath := filepath.Join(dir, "Makefile")
	targets := extractDocumentedTargetNamesFromMakefiles(makefilePath)
	sort.Strings(targets)
	return targets
}

// extractDocumentedTargetNamesFromMakefiles follows include directives and
// extracts target names (without descriptions) from documented targets.
func extractDocumentedTargetNamesFromMakefiles(path string) []string {
	seen := map[string]bool{}
	var targets []string
	collectDocumentedTargetNames(path, seen, &targets)
	return targets
}

func collectDocumentedTargetNames(path string, seen map[string]bool, targets *[]string) {
	absPath, _ := filepath.Abs(path)
	if seen[absPath] {
		return
	}
	seen[absPath] = true

	data, err := os.ReadFile(path)
	if err != nil {
		matches, globErr := filepath.Glob(path)
		if globErr != nil || len(matches) == 0 {
			return
		}
		for _, m := range matches {
			collectDocumentedTargetNames(m, seen, targets)
		}
		return
	}

	dir := filepath.Dir(path)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Follow include/-include directives
		if strings.HasPrefix(trimmed, "include ") || strings.HasPrefix(trimmed, "-include ") {
			includePath := strings.TrimPrefix(trimmed, "-include ")
			includePath = strings.TrimPrefix(includePath, "include ")
			includePath = strings.TrimSpace(includePath)
			if !filepath.IsAbs(includePath) {
				includePath = filepath.Join(dir, includePath)
			}
			matches, globErr := filepath.Glob(includePath)
			if globErr == nil && len(matches) > 0 {
				for _, m := range matches {
					collectDocumentedTargetNames(m, seen, targets)
				}
			} else {
				collectDocumentedTargetNames(includePath, seen, targets)
			}
			continue
		}

		// Extract target: ## description lines
		if strings.Contains(line, "##") && !strings.HasPrefix(line, "#") &&
			!strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 || strings.HasPrefix(parts[0], ".") {
				continue
			}
			target := strings.TrimSpace(parts[0])
			if target != "" && !shouldIgnoreMakefileTarget(target) {
				*targets = append(*targets, target)
			}
		}
	}
}

// shouldIgnoreMakefileTarget returns true for Makefile targets that are meta/infra
// targets unlikely to be useful as DVA interactions.
func shouldIgnoreMakefileTarget(name string) bool {
	ignoredTargets := map[string]bool{
		// Meta targets
		"help": true, "all": true, "default": true,
		// DVA reserved commands — overlap with built-in DVA commands
		"stop": true, "up": true, "down": true, "restart": true,
		"ps": true, "run": true, "logs": true, "build": true, "clean": true,
		// Generic infra targets that overlap with DVA modes/stack
		"infra-up": true, "infra-down": true, "infra-start": true, "infra-stop": true,
		// Generic setup/dependency targets handled by provision
		"deps": true, "install": true, "prepare": true, "setup": true,
		"install-hooks": true,
		// Documentation targets
		"docs": true, "docs-build": true, "docs-serve": true,
	}
	if ignoredTargets[name] {
		return true
	}

	// Compose lifecycle suffixes: e.g., dev-full-up, e2e-down, app-logs
	// DVA handles these natively via modes and `dva up/down/logs` commands
	for _, suffix := range []string{"-up", "-down", "-stop", "-restart", "-logs", "-ps"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}

	return false
}

func extractPackageScriptNamesInDir(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	var scripts []string
	for name := range pkg.Scripts {
		if shouldIgnorePackageScript(name) {
			continue
		}
		scripts = append(scripts, name)
	}
	sort.Strings(scripts)
	return scripts
}

func shouldIgnorePackageScript(name string) bool {
	if name == "" {
		return true
	}
	if strings.HasPrefix(name, "pre") && len(name) > 3 {
		return true
	}
	if strings.HasPrefix(name, "post") && len(name) > 4 {
		return true
	}
	switch name {
	case "prepare":
		return true
	default:
		return false
	}
}
