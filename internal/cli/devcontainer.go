package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// isDevcontainerEnabled reports whether the devcontainer section is active.
// Absent `enabled` field defaults to true; only explicit false disables it.
func isDevcontainerEnabled(dc map[string]any) bool {
	v, ok := dc["enabled"]
	if !ok {
		return true
	}
	b, ok := v.(bool)
	return !ok || b
}

// writeDevcontainerFiles creates .devcontainer/devcontainer.json from the config map.
// baseDir is the directory containing dva.yml (used to resolve the output path).
func writeDevcontainerFiles(dc map[string]any, composeFiles []string, baseDir string) error {
	dir := filepath.Join(baseDir, ".devcontainer")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create .devcontainer/: %w", err)
	}

	data, err := generateDevcontainerJSON(dc, composeFiles)
	if err != nil {
		return fmt.Errorf("failed to generate devcontainer.json: %w", err)
	}

	path := filepath.Join(dir, "devcontainer.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// generateDevcontainerJSON converts a dva.yml devcontainer map to devcontainer.json bytes.
func generateDevcontainerJSON(dc map[string]any, composeFiles []string) ([]byte, error) {
	out := make(map[string]any)
	for k, v := range dc {
		if k == "enabled" {
			continue // DVA-only field, not part of devcontainer spec
		}
		out[k] = v
	}

	// Expand feature shorthand keys to full ghcr.io URIs
	if features, ok := out["features"]; ok {
		out["features"] = expandFeatures(features)
	}

	// If no image or dockerfile specified, link to the compose setup.
	// devcontainer.json paths are relative to .devcontainer/, so prepend "../"
	// for paths that are relative to the project root.
	_, hasImage := out["image"]
	_, hasDockerfile := out["dockerFile"]
	if !hasImage && !hasDockerfile {
		if _, hasCompose := out["dockerComposeFile"]; !hasCompose {
			if len(composeFiles) > 0 {
				out["dockerComposeFile"] = toDevcontainerRelative(composeFiles[0])
			}
		}
	}

	return json.MarshalIndent(out, "", "  ")
}

// expandFeatures expands shorthand feature names to full devcontainer feature URIs.
// Keys already containing "/" are treated as full URIs and kept as-is.
func expandFeatures(features any) map[string]any {
	m, ok := features.(map[string]any)
	if !ok {
		return nil
	}
	expanded := make(map[string]any, len(m))
	for k, v := range m {
		if strings.Contains(k, "/") {
			expanded[k] = v
		} else {
			expanded["ghcr.io/devcontainers/features/"+k+":latest"] = v
		}
	}
	return expanded
}

// toDevcontainerRelative adjusts a path (relative to project root) so it is
// relative to the .devcontainer/ subdirectory.
// Absolute paths are returned unchanged.
func toDevcontainerRelative(p string) string {
	if filepath.IsAbs(p) || strings.HasPrefix(p, "..") {
		return p
	}
	return "../" + p
}


