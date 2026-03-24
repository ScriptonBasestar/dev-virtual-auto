//go:build integration

package integration

import (
	"testing"
)

func TestValidateBasicFixture(t *testing.T) {
	c := loadFixtureConfig(t, "basic")
	if err := c.Validate(); err != nil {
		t.Errorf("basic fixture should validate: %v", err)
	}
}

func TestValidateFullStackFixture(t *testing.T) {
	c := loadFixtureConfig(t, "full-stack")
	if err := c.Validate(); err != nil {
		t.Errorf("full-stack fixture should validate: %v", err)
	}
}

func TestValidateProvisionProfilesFixture(t *testing.T) {
	c := loadFixtureConfig(t, "provision-profiles")
	if err := c.Validate(); err != nil {
		t.Errorf("provision-profiles fixture should validate: %v", err)
	}
}

func TestValidateComposeProjectNames_FullStack(t *testing.T) {
	c := loadFixtureConfig(t, "full-stack")

	// ValidateComposeProjectNames checks if compose files have matching project names
	// Since we don't have actual compose files in the fixture dir, this tests
	// the fallback behavior (no compose files found)
	warnings := c.ValidateComposeProjectNames()
	// Just verify it doesn't panic — warnings depend on compose file existence
	_ = warnings
}
