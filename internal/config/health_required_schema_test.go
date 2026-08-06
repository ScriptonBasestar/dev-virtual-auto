package config

import (
	"strings"
	"testing"
)

// TestHealthRequiredIsGone records what happened to TASK-118's option C.
//
// `health.required` was opt-in strict readiness: an app that owned its port but
// never answered its probe was promoted from [warn] to [FAIL]. It was only ever
// declarable under applications.<app>.health (and the variants beneath it), and
// only ever read by AppManager.startApp. The command-surface restructure
// (docs/43) removed both, and no equivalent exists on the plan path — this is a
// capability the restructure cost, not one it relocated.
//
// The two subtests are what is left to assert: the surviving half of the old
// contract (top-level health_checks still rejects the key, which is why the Go
// field went too), and the section's removal being reported with guidance
// rather than a bare schema rejection.
func TestHealthRequiredIsGone(t *testing.T) {
	t.Run("top_level_health_checks_still_rejects_required", func(t *testing.T) {
		tmpDir := t.TempDir()
		content := `version: "0.1.44"
health_checks:
  api:
    type: http
    url: "http://localhost:11200/healthz"
    required: true
`
		cfg := loadConfigForSchemaTest(t, tmpDir, content)
		err := cfg.Validate()
		if err == nil {
			t.Fatal("Validate() expected error for health_checks.api.required")
		}
		if !strings.Contains(err.Error(), "Additional property required is not allowed") {
			t.Fatalf("Validate() error = %v, want additional property rejection for top-level health_checks.required", err)
		}
	})

	t.Run("applications_section_is_rejected_with_guidance", func(t *testing.T) {
		tmpDir := t.TempDir()
		content := `version: "0.1.44"
applications:
  api:
    run:
      native: "sleep 1"
    health:
      type: http
      url: "http://localhost:11200/healthz"
      required: true
`
		cfg := loadConfigForSchemaTest(t, tmpDir, content)
		err := cfg.Validate()
		if err == nil {
			t.Fatal("Validate() expected error: the 'applications' section was removed")
		}
		msg := err.Error()
		if !strings.Contains(msg, "Additional property applications is not allowed") {
			t.Fatalf("Validate() error = %v, want the schema to reject 'applications'", err)
		}
		// A bare rejection would leave every pre-restructure config with no next
		// step, which is the failure removedSchemaKeys exists to prevent.
		if !strings.Contains(msg, "runners.native") {
			t.Fatalf("Validate() error = %v, want removedSchemaKeys guidance naming the replacement shape", err)
		}
	})
}
