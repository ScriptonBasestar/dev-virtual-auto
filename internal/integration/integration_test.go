//go:build integration

package integration

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// testdataDir returns the absolute path to testdata/fixtures.
func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", "fixtures")
}

// loadFixtureConfig loads a dva.yml from a named fixture directory.
func loadFixtureConfig(t *testing.T, fixtureName string) *config.Config {
	t.Helper()
	dir := filepath.Join(testdataDir(), fixtureName)
	c, err := config.Load(dir)
	if err != nil {
		t.Fatalf("failed to load fixture %q: %v", fixtureName, err)
	}
	return c
}

// loadFixtureConfigErr loads a fixture and expects an error.
func loadFixtureConfigErr(t *testing.T, fixtureName string) error {
	t.Helper()
	dir := filepath.Join(testdataDir(), fixtureName)
	_, err := config.Load(dir)
	return err
}
