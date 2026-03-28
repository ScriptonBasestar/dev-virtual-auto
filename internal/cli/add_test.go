package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestDetectPrimaryService_WithComposeFile(t *testing.T) {
	tmpDir := t.TempDir()
	compose := filepath.Join(tmpDir, "docker-compose.yml")
	content := "services:\n  web:\n    image: nginx\n  db:\n    image: postgres\n"
	os.WriteFile(compose, []byte(content), 0644)

	c := &config.Config{
		Lifecycle: []config.LifecycleEntry{
			{
				Name: "compose", Plugin: "compose", Order: 10,
				Compose: &config.ComposePluginConfig{
					Files: []string{compose},
				},
			},
		},
	}

	got := detectPrimaryService(c)
	if got != "web" {
		t.Errorf("detectPrimaryService = %q, want 'web'", got)
	}
}

func TestDetectPrimaryService_NoComposeFiles(t *testing.T) {
	c := &config.Config{}
	got := detectPrimaryService(c)
	if got != "app" {
		t.Errorf("detectPrimaryService = %q, want 'app'", got)
	}
}

func TestDetectPrimaryService_MissingFile(t *testing.T) {
	c := &config.Config{
		Lifecycle: []config.LifecycleEntry{
			{
				Name: "compose", Plugin: "compose", Order: 10,
				Compose: &config.ComposePluginConfig{
					Files: []string{"/nonexistent/compose.yml"},
				},
			},
		},
	}
	got := detectPrimaryService(c)
	if got != "app" {
		t.Errorf("detectPrimaryService = %q, want 'app' fallback", got)
	}
}
