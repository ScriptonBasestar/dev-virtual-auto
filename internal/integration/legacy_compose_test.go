//go:build integration

package integration

import (
	"testing"
)

// TestLoadLegacyComposeFixture covers the legacy stack.<entry>.compose loading
// path that ComposeConfig() still supports. The other fixtures use the modern
// runners.compose shape, so this is the only fixture-driven coverage of the
// back-compat path (unit tests in internal/config cover it too).
// The fixture is not expected to pass Validate() — schema.json rejects the
// legacy shape by design.
func TestLoadLegacyComposeFixture(t *testing.T) {
	c := loadFixtureConfig(t, "legacy-compose")

	if c.ComposeProjectName() != "legacy-test" {
		t.Errorf("ProjectName = %q, want %q", c.ComposeProjectName(), "legacy-test")
	}
	files := c.AllComposeFiles()
	if len(files) != 1 || files[0] != "compose.yml" {
		t.Errorf("Compose.Files = %v, want [compose.yml]", files)
	}
}
