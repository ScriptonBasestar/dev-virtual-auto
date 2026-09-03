package agentdeny

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ScriptonBasestar/dva/internal/skillinstall"
)

// RuntimeClaudeCode is the only runtime with an implemented, tested deploy target as of
// TASK-286; see docs/agent-deny-rules.md's coverage table for every other runtime's
// researched-but-not-implemented or no-known-mechanism status.
const RuntimeClaudeCode = "claude-code"

// Options supplies filesystem roots and scope, reusing skillinstall.Scope directly so
// `dva agent-deny` takes the same --scope user|project a `dva skill` install does. Empty
// roots are resolved from the process environment, exactly as skillinstall.Options does.
type Options struct {
	Scope       skillinstall.Scope
	HomeDir     string
	ProjectRoot string
	StateRoot   string
	DryRun      bool
	Version     string
}

type Result struct {
	Scope        skillinstall.Scope  `json:"scope"`
	Destinations []DestinationResult `json:"destinations"`
}

// DestinationResult reports one runtime target's outcome. Status is one of:
// "installed" | "up-to-date" | "would-install" (dry-run) | "absent" (no DVA receipt) |
// "drifted" (a DVA-owned pattern is missing — user edited or removed it) |
// "recovery-required" (settings file is not valid JSON, or uninstall found drift and
// refused to guess) | "uninstalled" | "would-uninstall" (dry-run) | "not-installed".
type DestinationResult struct {
	Runtime     string   `json:"runtime"`
	Destination string   `json:"destination"`
	Status      string   `json:"status"`
	Detail      string   `json:"detail,omitempty"`
	Patterns    []string `json:"patterns,omitempty"`
}

func desiredPatterns() []string {
	seen := map[string]bool{}
	var patterns []string
	for _, command := range GatedCommands {
		for _, pattern := range command.Patterns() {
			if !seen[pattern] {
				seen[pattern] = true
				patterns = append(patterns, pattern)
			}
		}
	}
	sort.Strings(patterns)
	return patterns
}

func resolve(options Options) (Options, error) {
	if options.Scope != skillinstall.ScopeUser && options.Scope != skillinstall.ScopeProject {
		return Options{}, fmt.Errorf("agent-deny scope must be %q or %q", skillinstall.ScopeUser, skillinstall.ScopeProject)
	}
	if options.HomeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Options{}, fmt.Errorf("resolve home directory: %w", err)
		}
		options.HomeDir = home
	}
	home, err := filepath.Abs(options.HomeDir)
	if err != nil {
		return Options{}, err
	}
	options.HomeDir = home
	if options.ProjectRoot == "" {
		project, err := os.Getwd()
		if err != nil {
			return Options{}, err
		}
		options.ProjectRoot = project
	}
	project, err := filepath.Abs(options.ProjectRoot)
	if err != nil {
		return Options{}, err
	}
	options.ProjectRoot = project
	if options.StateRoot == "" {
		if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
			options.StateRoot = filepath.Join(xdg, "dva")
		} else {
			options.StateRoot = filepath.Join(home, ".local", "state", "dva")
		}
	}
	state, err := filepath.Abs(options.StateRoot)
	if err != nil {
		return Options{}, err
	}
	options.StateRoot = state
	if options.Version == "" {
		options.Version = "unknown"
	}
	return options, nil
}

// claudeCodeDestination returns the .claude/settings.json path for scope — the only
// destination this package resolves today, kept as its own function so a second runtime
// target can be added beside it without reshaping Install/Status/Uninstall.
func claudeCodeDestination(scope skillinstall.Scope, home, project string) (string, error) {
	switch scope {
	case skillinstall.ScopeUser:
		return filepath.Join(home, ".claude", "settings.json"), nil
	case skillinstall.ScopeProject:
		return filepath.Join(project, ".claude", "settings.json"), nil
	default:
		return "", fmt.Errorf("unsupported agent-deny scope %q", scope)
	}
}

func readFileIfExists(path string) ([]byte, error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return contents, err
}

// Install ensures every GatedCommands pattern is present in the Claude Code deny array
// at the resolved destination, adding only what is missing and leaving every other key
// and every pre-existing array entry (DVA-owned or not) untouched.
func Install(options Options) (Result, error) {
	resolved, err := resolve(options)
	if err != nil {
		return Result{}, err
	}
	destination, err := claudeCodeDestination(resolved.Scope, resolved.HomeDir, resolved.ProjectRoot)
	if err != nil {
		return Result{}, err
	}
	want := desiredPatterns()
	receiptFile := receiptPath(resolved.StateRoot, destination)
	record, hadReceipt, err := readReceipt(receiptFile)
	if err != nil {
		return Result{}, fmt.Errorf("read receipt for %s: %w", destination, err)
	}

	existing, err := readFileIfExists(destination)
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", destination, err)
	}
	newContents, finalDeny, changed, err := mergeClaudeCodeDeny(existing, want)
	if err != nil {
		return Result{
			Scope: resolved.Scope,
			Destinations: []DestinationResult{{
				Runtime: RuntimeClaudeCode, Destination: destination,
				Status: "recovery-required", Detail: err.Error(),
			}},
		}, nil
	}

	// A pattern only becomes DVA-owned if this install actually added it — not merely
	// because it is desired. Claiming ownership of a pattern the user already had before
	// ever running install (typed by hand, or added by some other tool) would let a
	// later uninstall delete an entry DVA never wrote, the exact clobber class
	// criterion 5 exists to prevent. preMergeDeny is `existing` re-parsed rather than
	// carried from mergeClaudeCodeDeny's return, so it reflects the array before this
	// call's additions.
	preMergeDeny, err := claudeCodeDenyArray(existing)
	if err != nil {
		return Result{}, fmt.Errorf("re-read pre-merge deny array for %s: %w", destination, err)
	}
	preMergeSet := make(map[string]bool, len(preMergeDeny))
	for _, pattern := range preMergeDeny {
		preMergeSet[pattern] = true
	}
	var newlyAdded []string
	for _, pattern := range want {
		if !preMergeSet[pattern] {
			newlyAdded = append(newlyAdded, pattern)
		}
	}
	ownedPatterns := newlyAdded
	if hadReceipt {
		ownedPatterns, _ = unionAppend(record.Patterns, newlyAdded)
	}
	sort.Strings(ownedPatterns)

	status := "up-to-date"
	if changed {
		status = "installed"
	}
	if resolved.DryRun {
		if changed {
			status = "would-install"
		}
		return Result{Scope: resolved.Scope, Destinations: []DestinationResult{{
			Runtime: RuntimeClaudeCode, Destination: destination, Status: status, Patterns: finalDeny,
		}}}, nil
	}

	if changed {
		if err := writeSettingsFile(destination, newContents); err != nil {
			return Result{}, fmt.Errorf("write %s: %w", destination, err)
		}
	}
	newReceipt := receipt{
		Schema: receiptSchema, Scope: resolved.Scope, Runtime: RuntimeClaudeCode,
		Destination: destination, Patterns: ownedPatterns, Version: resolved.Version,
	}
	if !hadReceipt || !stringsEqual(record.Patterns, ownedPatterns) || record.Version != resolved.Version {
		if err := writeReceipt(receiptFile, newReceipt); err != nil {
			return Result{}, fmt.Errorf("write receipt for %s: %w", destination, err)
		}
	}

	return Result{Scope: resolved.Scope, Destinations: []DestinationResult{{
		Runtime: RuntimeClaudeCode, Destination: destination, Status: status, Patterns: finalDeny,
	}}}, nil
}

// Status reports whether the resolved destination is installed, drifted (a DVA-owned
// pattern is missing), or absent (no DVA receipt) without writing anything.
func Status(options Options) (Result, error) {
	resolved, err := resolve(options)
	if err != nil {
		return Result{}, err
	}
	destination, err := claudeCodeDestination(resolved.Scope, resolved.HomeDir, resolved.ProjectRoot)
	if err != nil {
		return Result{}, err
	}
	receiptFile := receiptPath(resolved.StateRoot, destination)
	record, hadReceipt, err := readReceipt(receiptFile)
	if err != nil {
		return Result{}, fmt.Errorf("read receipt for %s: %w", destination, err)
	}
	if !hadReceipt {
		return Result{Scope: resolved.Scope, Destinations: []DestinationResult{{
			Runtime: RuntimeClaudeCode, Destination: destination, Status: "absent",
		}}}, nil
	}

	contents, err := readFileIfExists(destination)
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", destination, err)
	}
	if contents == nil {
		return Result{Scope: resolved.Scope, Destinations: []DestinationResult{{
			Runtime: RuntimeClaudeCode, Destination: destination, Status: "drifted",
			Detail: "settings file no longer exists",
		}}}, nil
	}
	currentDeny, err := claudeCodeDenyArray(contents)
	if err != nil {
		return Result{Scope: resolved.Scope, Destinations: []DestinationResult{{
			Runtime: RuntimeClaudeCode, Destination: destination, Status: "recovery-required", Detail: err.Error(),
		}}}, nil
	}
	have := map[string]bool{}
	for _, pattern := range currentDeny {
		have[pattern] = true
	}
	var missing []string
	for _, pattern := range record.Patterns {
		if !have[pattern] {
			missing = append(missing, pattern)
		}
	}
	if len(missing) > 0 {
		return Result{Scope: resolved.Scope, Destinations: []DestinationResult{{
			Runtime: RuntimeClaudeCode, Destination: destination, Status: "drifted",
			Detail:   fmt.Sprintf("%d of %d DVA-owned deny patterns are missing (locally removed or edited): %v", len(missing), len(record.Patterns), missing),
			Patterns: currentDeny,
		}}}, nil
	}
	return Result{Scope: resolved.Scope, Destinations: []DestinationResult{{
		Runtime: RuntimeClaudeCode, Destination: destination, Status: "installed", Patterns: currentDeny,
	}}}, nil
}

// Uninstall removes only the DVA-owned patterns recorded in the receipt, and only when
// every one of them is still present verbatim; any drift refuses the whole operation
// rather than guessing which entries are still safe to remove, matching
// internal/skillinstall's fail-closed handling of a locally modified install.
func Uninstall(options Options) (Result, error) {
	resolved, err := resolve(options)
	if err != nil {
		return Result{}, err
	}
	destination, err := claudeCodeDestination(resolved.Scope, resolved.HomeDir, resolved.ProjectRoot)
	if err != nil {
		return Result{}, err
	}
	receiptFile := receiptPath(resolved.StateRoot, destination)
	record, hadReceipt, err := readReceipt(receiptFile)
	if err != nil {
		return Result{}, fmt.Errorf("read receipt for %s: %w", destination, err)
	}
	if !hadReceipt {
		return Result{Scope: resolved.Scope, Destinations: []DestinationResult{{
			Runtime: RuntimeClaudeCode, Destination: destination, Status: "not-installed",
		}}}, nil
	}

	contents, err := readFileIfExists(destination)
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", destination, err)
	}
	if contents == nil {
		if !resolved.DryRun {
			if err := removeReceipt(receiptFile); err != nil {
				return Result{}, fmt.Errorf("remove receipt for %s: %w", destination, err)
			}
		}
		return Result{Scope: resolved.Scope, Destinations: []DestinationResult{{
			Runtime: RuntimeClaudeCode, Destination: destination, Status: "uninstalled",
			Detail: "settings file was already gone",
		}}}, nil
	}

	currentDeny, err := claudeCodeDenyArray(contents)
	if err != nil {
		return Result{Scope: resolved.Scope, Destinations: []DestinationResult{{
			Runtime: RuntimeClaudeCode, Destination: destination, Status: "recovery-required", Detail: err.Error(),
		}}}, nil
	}
	have := map[string]bool{}
	for _, pattern := range currentDeny {
		have[pattern] = true
	}
	var missing []string
	for _, pattern := range record.Patterns {
		if !have[pattern] {
			missing = append(missing, pattern)
		}
	}
	if len(missing) > 0 {
		return Result{Scope: resolved.Scope, Destinations: []DestinationResult{{
			Runtime: RuntimeClaudeCode, Destination: destination, Status: "recovery-required",
			Detail: fmt.Sprintf("%d of %d DVA-owned deny patterns were locally modified or removed; refusing to guess which are still safe to remove — restore them or delete the receipt manually, then retry: %v", len(missing), len(record.Patterns), missing),
		}}}, nil
	}

	if resolved.DryRun {
		return Result{Scope: resolved.Scope, Destinations: []DestinationResult{{
			Runtime: RuntimeClaudeCode, Destination: destination, Status: "would-uninstall", Patterns: record.Patterns,
		}}}, nil
	}

	newContents, remaining, removed, err := removeClaudeCodeDeny(contents, record.Patterns)
	if err != nil {
		return Result{}, fmt.Errorf("remove deny patterns from %s: %w", destination, err)
	}
	if removed > 0 {
		if err := writeSettingsFile(destination, newContents); err != nil {
			return Result{}, fmt.Errorf("write %s: %w", destination, err)
		}
	}
	if err := removeReceipt(receiptFile); err != nil {
		return Result{}, fmt.Errorf("remove receipt for %s: %w", destination, err)
	}
	return Result{Scope: resolved.Scope, Destinations: []DestinationResult{{
		Runtime: RuntimeClaudeCode, Destination: destination, Status: "uninstalled", Patterns: remaining,
	}}}, nil
}

func stringsEqual(a, b []string) bool {
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
