//go:build integration

package integration

import (
	"strings"
	"testing"
)

// TestLegacyComposeFixtureIsRejected pins the loader's refusal of the legacy
// stack.<entry>.compose shape at the fixture level.
//
// This test used to assert the opposite: that Load() still accepted the shape for
// back-compat. 553c478 removed that support on purpose — schema.json had always
// rejected it, so a loader that accepted it made the schema rule unenforceable — but the
// commit only touched internal/config, and `make test` does not run this suite. It has
// been failing since, and `make test-integration` is a gating CI step.
//
// Asserting the rejection rather than deleting the fixture keeps the shape covered from
// the outside: if the loader ever silently starts accepting it again, the disagreement
// with schema.json reappears here instead of in someone's config.
func TestLegacyComposeFixtureIsRejected(t *testing.T) {
	err := loadFixtureConfigErr(t, "legacy-compose")
	if err == nil {
		t.Fatal("Load(legacy-compose) error = nil, want a rejection of the legacy compose shape")
	}
	if !strings.Contains(err.Error(), "compose must be declared under runners.compose") {
		t.Errorf("error = %q, want it to name the legacy compose shape", err)
	}
	// The value of the error is that it is actionable, so check it still prints the
	// replacement instead of only naming the fault.
	if !strings.Contains(err.Error(), "default_runner: compose") {
		t.Errorf("error = %q, want it to show the replacement shape", err)
	}
}
