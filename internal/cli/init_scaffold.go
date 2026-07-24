package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ScriptonBasestar/dva/internal/config"
)

var errComposeFileNotFound = errors.New("no Docker Compose file detected")

// scaffoldDvaYml creates a dva.yml in the given directory if one doesn't exist.
// Returns true if a file was created.
func scaffoldDvaYml(dir, tmpl string) (bool, error) {
	target := filepath.Join(dir, config.FileName)
	if _, err := os.Stat(target); err == nil {
		fmt.Printf("⏭  dva.yml already exists in %s (skipped)\n", dir)
		return false, nil
	}

	if len(detectComposeFilesIn(dir)) == 0 {
		return false, fmt.Errorf(`%w in %s; dva.yml was not created
  DVA init only scaffolds projects with a Compose file in the current directory.
  For non-standard or multi-project layouts, run:
    am run dva-improve param.mode=rewrite
  Or create dva.yml manually, then run:
    dva config validate`, errComposeFileNotFound, dir)
	}

	if tmpl == "" {
		tmpl = detectTemplateIn(dir)
	}

	content := generateConfigIn(dir, tmpl)
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		return false, fmt.Errorf("failed to write %s: %w", target, err)
	}

	fmt.Printf("✅ Created %s (template: %s)\n", target, tmpl)

	if updated, err := ensureGitignore(dir); err == nil && updated {
		fmt.Printf("📎 Updated .gitignore to ignore %s/\n", config.DotDirName)
	}

	return true, nil
}
