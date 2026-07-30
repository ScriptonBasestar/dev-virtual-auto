package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
)

const (
	defaultIgnoreSection = "# ignore ScriptonBasestar tmp files"
)

func defaultIgnorePath() string {
	return config.DotDirName + "/"
}

// ensureGitignore ensures that the .gitignore file contains the necessary entries for DVA.
// Returns true if it was updated or already present.
func ensureGitignore(configDir string) (bool, error) {
	gitignorePath := filepath.Join(configDir, ".gitignore")
	ignorePath := defaultIgnorePath()

	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		// No .gitignore, creating a new one
		content := fmt.Sprintf("%s\n%s\n", defaultIgnoreSection, ignorePath)
		if err := os.WriteFile(gitignorePath, []byte(content), 0644); err != nil {
			return false, fmt.Errorf("failed to create .gitignore: %w", err)
		}
		return true, nil
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		return false, fmt.Errorf("failed to read .gitignore: %w", err)
	}

	content := string(data)
	if isDvaIgnored(content) {
		return true, nil
	}

	// Not ignored, append to the end
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return false, fmt.Errorf("failed to open .gitignore for appending: %w", err)
	}
	defer func() { _ = f.Close() }()

	if !strings.HasSuffix(content, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return false, err
		}
	}

	ignoreBlock := fmt.Sprintf("\n%s\n%s\n", defaultIgnoreSection, ignorePath)
	if _, err := f.WriteString(ignoreBlock); err != nil {
		return false, fmt.Errorf("failed to append to .gitignore: %w", err)
	}

	return true, nil
}

// isDvaIgnored checks if the config directory is already ignored in the gitignore content.
//
// Matching the full path literally is not enough. DotDirName is a two-segment path
// (".sb/dva"), and git excludes an entire subtree when any ancestor directory is listed, so
// a .gitignore saying ".sb/" already ignores ".sb/dva". The literal check called that
// unignored and told the user to add a rule they did not need — on the better-written
// config, since ".sb/" covers DVA's whole dot directory rather than one child.
//
// Negations have to be read, and they do not behave the same way at every depth. Verified
// against git with the paths on disk:
//
//	.sb/ + !.sb/dva/   → still ignored. A path cannot be re-included once a parent
//	                     directory is excluded, so negating a descendant loses.
//	.sb/ + !.sb/        → NOT ignored. The exclusion and the negation name the same
//	                     directory, no ancestor of it is excluded, so ordinary
//	                     last-matching-pattern-wins applies and the negation wins.
//
// So each ancestor is resolved on its own, outermost first, and the first one that comes out
// excluded is decisive. Reading the file for a single match and stopping — as this did
// before — reports "already ignored" for the second case above and suppresses the warning
// while git leaves the markers committable.
//
// This still stops short of implementing gitignore semantics: globs (".sb/*"), non-root
// anchoring ("**/.sb/"), and .git/info/exclude stay uninterpreted. Every one of those gaps
// makes DVA warn about a path that is in fact ignored, which is the harmless direction — an
// extra warning, never a silently committed marker.
func isDvaIgnored(content string) bool {
	lines := strings.Split(content, "\n")
	for _, prefix := range ancestorsAndSelf(config.DotDirName) {
		if lastMatchExcludes(lines, pathSpellings(prefix)) {
			return true
		}
	}
	return false
}

// lastMatchExcludes applies gitignore's last-matching-pattern-wins rule to one path: it
// reports whether the last line naming that path excludes it rather than negates it, and
// false when no line names it at all.
func lastMatchExcludes(lines []string, forms map[string]bool) bool {
	excluded := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		pattern := strings.TrimPrefix(line, "!")
		if forms[pattern] {
			excluded = line == pattern
		}
	}
	return excluded
}

// pathSpellings returns the .gitignore lines that name path, in each spelling git treats
// alike: bare, trailing slash, and root-anchored with a leading slash.
func pathSpellings(path string) map[string]bool {
	forms := make(map[string]bool, 4)
	if path == "" {
		return forms
	}
	for _, form := range []string{path, path + "/", "/" + path, "/" + path + "/"} {
		forms[form] = true
	}
	return forms
}

// ancestorsAndSelf lists dir's path prefixes outermost first: ".sb/dva" → [".sb", ".sb/dva"].
func ancestorsAndSelf(dir string) []string {
	var prefixes []string
	parts := strings.Split(dir, "/")
	for i := range parts {
		if prefix := strings.Join(parts[:i+1], "/"); prefix != "" {
			prefixes = append(prefixes, prefix)
		}
	}
	return prefixes
}

// ignoreRulesCovering returns every .gitignore line that names dir or one of its ancestors,
// in every spelling. It is the union of what isDvaIgnored evaluates; isDvaIgnored keeps the
// prefixes separate because a negation resolves differently at each depth.
func ignoreRulesCovering(dir string) map[string]bool {
	rules := make(map[string]bool)
	for _, prefix := range ancestorsAndSelf(dir) {
		for form := range pathSpellings(prefix) {
			rules[form] = true
		}
	}
	return rules
}

// checkGitignoreForWarning checks if the config directory is ignored and prints a warning if not.
// This is used for normal command execution.
func checkGitignoreForWarning(configDir string) {
	gitignorePath := filepath.Join(configDir, ".gitignore")

	// If .gitignore doesn't exist, we might not be in a git repo or user doesn't care.
	// But if we have a .git directory, we should probably warn.
	if _, err := os.Stat(filepath.Join(configDir, ".git")); os.IsNotExist(err) {
		return
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		// Not ignored since we can't read it or it doesn't exist
		if !os.IsNotExist(err) {
			return
		}
	}

	if isDvaIgnored(string(data)) {
		return
	}

	fmt.Fprintf(os.Stderr, "⚠️  [warn] %s/ is not in your .gitignore. Transient markers might be committed.\n", config.DotDirName)
	fmt.Fprintf(os.Stderr, "         Run 'dva doctor --fix' to auto-fix or add '%s/' to .gitignore manually.\n\n", config.DotDirName)
}
