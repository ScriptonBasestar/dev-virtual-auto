package cli

import (
	"strings"
	"testing"
)

// TestValidatePlanDriftWarningsRenderAndPromoteUnderStrict is the end-to-end coverage
// TASK-244 criterion 4 asks for: D6 (duplicate plan declarations) and D7 (multiple
// plans without default_plan) must render under the existing "semantic" text prefix
// and JSON category — no new category — stay non-fatal by default, and be promoted
// to a non-zero exit by --strict exactly like every other semantic warning.
//
// The fixture below fires D6 and D7 together: "alpha" and "beta" declare identical
// fields (D6), and neither config sets default_plan across two-or-more plans (D7).
func TestValidatePlanDriftWarningsRenderAndPromoteUnderStrict(t *testing.T) {
	configPath := writeValidateConfigForTest(t, `version: "0.1.44"
plans:
  alpha:
    entries: []
  beta:
    entries: []
`)

	defaultRun := runValidateCommandForTest(t, configPath, "validate")
	if defaultRun.err != "" {
		t.Fatalf("default run returned an error: %s", defaultRun.err)
	}
	// validateNoticeWriter routes these through stdout in this harness (root.go writes
	// notices to os.Stdout, which runValidateCommandForTest captures as .stdout — see
	// its sibling assertions in this package for the same convention).
	if !strings.Contains(defaultRun.stdout, "[warn] semantic: plans \"alpha\" and \"beta\" declare equal") {
		t.Errorf("default run stdout missing D6 warning under [warn] semantic: prefix:\n%s", defaultRun.stdout)
	}
	if !strings.Contains(defaultRun.stdout, "[warn] semantic: 2 plans are defined (alpha, beta) but default_plan is not set") {
		t.Errorf("default run stdout missing D7 warning under [warn] semantic: prefix:\n%s", defaultRun.stdout)
	}

	strictRun := runValidateCommandForTest(t, configPath, "validate", "--strict")
	if strictRun.err == "" {
		t.Fatal("--strict run succeeded, want non-zero exit on plan drift warnings")
	}
}

// TestValidatePlanDriftWarningsUseSemanticJSONCategory pins the JSON side of the same
// contract: both new warnings land in the existing "semantic" category, not a new one.
func TestValidatePlanDriftWarningsUseSemanticJSONCategory(t *testing.T) {
	body := `version: "0.1.44"
plans:
  alpha:
    entries: []
  beta:
    entries: []
`
	out, _, err := runValidate(t, body, true)
	if err != nil {
		t.Fatalf("json run returned an error: %v", err)
	}

	doc := decodeOneDocument(t, out)
	var sawD6, sawD7 bool
	for _, w := range doc.Warnings {
		if w.Category != "semantic" {
			continue
		}
		if strings.Contains(w.Message, "declare equal environment, site, vars, endpoint_tags, and entries") {
			sawD6 = true
		}
		if strings.Contains(w.Message, "but default_plan is not set") {
			sawD7 = true
		}
	}
	if !sawD6 {
		t.Errorf("no semantic-category warning carries the D6 message; got %+v", doc.Warnings)
	}
	if !sawD7 {
		t.Errorf("no semantic-category warning carries the D7 message; got %+v", doc.Warnings)
	}
}
