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
func isDvaIgnored(content string) bool {
	dir := config.DotDirName
	dirSlash := dir + "/"
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == dir || line == dirSlash {
			return true
		}
	}
	return false
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
