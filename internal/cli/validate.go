package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/output"
	"github.com/ScriptonBasestar/dva/internal/runner"
)

var validateStrict bool

// detectInteractionCollisionWarnings reports command names that more than one declaration
// produces.
//
// Two different YAML keys can flatten to one command name — a subcommand literally named "b c"
// under `a`, and `a` → `b` → `c` — because the tree joins path segments with a space and
// schema.json admits a space inside a key. The expansion keeps the first and drops the second,
// which used to be decided by Go's map seed and is now decided by the sorted walk. Deterministic
// is not the same as visible, and losing a declared command without a word is the part that had
// to stop. TASK-104.
//
// It asks the tree rather than re-walking c.Interaction here: the paths are the tree's to
// construct, and a second walk that agreed today would drift the first time expansion changed.
func detectInteractionCollisionWarnings(c *config.Config) []string {
	_, collisions := runner.NewInteractionTree(c.Interaction).ListWithCollisions()

	warnings := make([]string, 0, len(collisions))
	for _, col := range collisions {
		warnings = append(warnings, fmt.Sprintf(
			"%s and %s both resolve to the command %q; only the first is reachable — rename one",
			describeInteractionPath(col.Winner), describeInteractionPath(col.Loser), col.Key))
	}
	return warnings
}

// describeInteractionPath renders a command path as the dva.yml location that declares it, so the
// warning points at a line the author can edit rather than at the flattened name they never wrote.
// A segment is quoted only when it contains whitespace — which is exactly the segment that caused
// the collision, so the quoting doubles as the explanation.
func describeInteractionPath(path []string) string {
	var b strings.Builder
	b.WriteString("interaction")
	for i, seg := range path {
		if i > 0 {
			b.WriteString(".subcommands")
		}
		b.WriteString(".")
		if strings.ContainsAny(seg, " \t\n") {
			b.WriteString(strconv.Quote(seg))
		} else {
			b.WriteString(seg)
		}
	}
	return b.String()
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the syntax and schema of 'dva.yml'",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		report := newValidateReport(c)

		if err := c.Validate(); err != nil {
			return report.fail(err)
		}

		// Check compose file project name alignment
		warnings := c.ValidateComposeProjectNames()
		fix, _ := cmd.Flags().GetBool("fix")

		if fix {
			fixComposeNameWarnings(c, warnings)
		} else {
			printComposeNameWarnings(warnings)
			// Only when they were reported: --fix rewrote the files, so the mismatch no
			// longer exists and putting it in the report would describe a fixed state as
			// an outstanding warning.
			for _, w := range warnings {
				report.addComposeNameWarning(w)
			}
		}

		// Semantic warnings (version, health checks, duplicate commands, etc.)
		semanticWarnings := c.ValidateWarnings()
		for _, w := range semanticWarnings {
			fmt.Fprintf(os.Stderr, "[warn] semantic: %s\n", w)
		}
		report.add("semantic", semanticWarnings...)

		collisionWarnings := detectInteractionCollisionWarnings(c)
		for _, w := range collisionWarnings {
			fmt.Fprintf(os.Stderr, "[warn] interaction: %s\n", w)
		}
		report.add("interaction_collision", collisionWarnings...)

		driftWarnings := detectConfigDriftWarnings(c)
		printConfigDriftWarnings(driftWarnings)
		report.add("config_drift", driftWarnings...)

		suggestionWarnings := detectConfigSuggestionWarnings(c)
		printConfigSuggestionWarnings(suggestionWarnings)
		report.add("config_suggestion", suggestionWarnings...)

		if validateStrict && (len(driftWarnings) > 0 || len(semanticWarnings) > 0 || len(collisionWarnings) > 0) {
			return report.fail(fmt.Errorf("config warnings detected; review warnings above or run 'am run dva-improve'"))
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
					fmt.Fprintf(os.Stderr, "       → run: dva config validate --fix\n")
					report.add("devcontainer", "devcontainer section found but .devcontainer/devcontainer.json missing\n  → run: dva config validate --fix")
				}
			}
		}

		if jsonOutput {
			return output.PrintJSON(report)
		}
		fmt.Println("✅ dva.yml is valid")
		return nil
	},
}

func init() {
	addValidateFlags(validateCmd)
	configCmd.AddCommand(validateCmd)
}

// composeNameWarningLines renders one compose-name warning as its headline followed by
// its continuation lines, without any of the prefixes either consumer adds.
//
// It exists so the prose on stderr and the JSON in validateReport are the same sentences
// by construction rather than by two authors agreeing (TASK-088). The alternative — a
// second set of format strings in validate_json.go — is the shape that produced the
// note-printed-four-different-ways defect in TASK-086.
func composeNameWarningLines(w config.ComposeNameWarning) []string {
	if w.ComposeName == "" {
		return []string{
			fmt.Sprintf("%s: missing top-level 'name: %s'", w.File, w.DvaName),
			"Running 'docker compose up' directly will use the directory name as project,",
			fmt.Sprintf("causing port conflicts with dva. Fix: add 'name: %s' to %s", w.DvaName, w.File),
		}
	}
	return []string{
		fmt.Sprintf("%s: name '%s' differs from dva.yml project_name '%s'", w.File, w.ComposeName, w.DvaName),
		fmt.Sprintf("Fix: change 'name: %s' to 'name: %s' in %s", w.ComposeName, w.DvaName, w.File),
	}
}

// printComposeNameWarnings prints warnings about compose file name mismatches to stderr.
func printComposeNameWarnings(warnings []config.ComposeNameWarning) {
	for _, w := range warnings {
		lines := composeNameWarningLines(w)
		fmt.Fprintf(os.Stderr, "[warn] semantic: %s\n", lines[0])
		for _, detail := range lines[1:] {
			fmt.Fprintf(os.Stderr, "       %s\n", detail)
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
		// Not merely an optimization: this is what stops a compose file built entirely out
		// of `include:` from producing a false positive on every interaction, because
		// configuredComposeServices cannot see through `include:`. See its doc comment.
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
	allCommands := runner.NewInteractionTree(c.Interaction).List()
	commandSet := map[string]bool{}
	for name := range allCommands {
		commandSet[name] = true
	}

	// Build subcommand coverage set: for "app:build ce" → also match "build-ce"
	// This detects when a Makefile target like "build-ce" is already covered by a
	// DVA interaction subcommand under a different parent name.
	subcommandCoverage := map[string]bool{}
	for fullPath := range allCommands {
		parts := strings.Split(fullPath, " ")
		if len(parts) < 2 {
			continue
		}
		// Strip namespace prefix from parent name ("app:build" → "build")
		baseName := parts[0]
		if idx := strings.LastIndex(baseName, ":"); idx >= 0 {
			baseName = baseName[idx+1:]
		}
		// "app:build ce" → "build-ce", "test all" → "test-all"
		subParts := append([]string{baseName}, parts[1:]...)
		subcommandCoverage[strings.Join(subParts, "-")] = true
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
		if subcommandCoverage[name] {
			continue
		}
		if matchesSuggestionIgnore(name, c.SuggestionIgnore) {
			continue
		}
		warnings = append(warnings,
			fmt.Sprintf("%s defines %q but no DVA interaction with the same name exists; consider adding a direct mapping if it is part of the developer workflow",
				candidates[name], name))
	}

	return warnings
}

// matchesSuggestionIgnore returns true if name matches any glob pattern in the
// suggestion_ignore list from dva.yml.
func matchesSuggestionIgnore(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}
	return false
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
		return deduplicateComposeFiles(dir, found)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if (strings.HasPrefix(name, "docker-compose.") || strings.HasPrefix(name, "compose.")) &&
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

	return deduplicateComposeFiles(dir, found)
}

// configuredComposeServices returns the services declared directly in the configured
// compose files.
//
// It does NOT resolve compose `include:`, so a file that only pulls services in via
// `include:` contributes nothing. That is a real gap — 14 configs in the measured corpus
// declare services this function cannot see — and TASK-068 chose to leave it rather than
// paper over it. What keeps the gap from becoming a false positive is the empty-set early
// return in detectConfigDriftWarnings, not anything here: when `include:` is all a project
// uses, this returns an empty map and the interaction-service comparison is skipped
// wholesale. Stated because that is a load-bearing dependency between two functions that
// neither of them declared, and it survives only as long as nobody refactors either half.
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
	left := append([]string(nil), a...)
	right := append([]string(nil), b...)
	sort.Strings(left)
	sort.Strings(right)
	for i := range left {
		if left[i] != right[i] {
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
		"ps": true, "run": true, config.LogsDirName: true, "build": true, "clean": true,
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
	for _, suffix := range []string{"-up", "-down", "-stop", "-restart", "-logs", "-ps", "-build"} {
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
