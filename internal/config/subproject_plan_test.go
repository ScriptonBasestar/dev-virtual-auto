package config

import "testing"

func TestCloneImportedPlanCopiesEndpointTags(t *testing.T) {
	original := &PlanConfig{EndpointTags: []string{"app"}}

	cloned := cloneImportedPlan(original, "/tmp/subproject")
	cloned.EndpointTags[0] = "changed"

	if got := original.EndpointTags[0]; got != "app" {
		t.Errorf("original endpoint tag = %q, want app", got)
	}
}
