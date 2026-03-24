package cli

import (
	"path/filepath"
	"testing"
)

func TestResolveInfraPath_Absolute(t *testing.T) {
	got := resolveInfraPath("/opt/infra/pg", "/home/user/project")
	if got != "/opt/infra/pg" {
		t.Errorf("resolveInfraPath = %q, want /opt/infra/pg", got)
	}
}

func TestResolveInfraPath_Relative(t *testing.T) {
	got := resolveInfraPath("infra/pg", "/home/user/project")
	want := filepath.Join("/home/user/project", "infra/pg")
	if got != want {
		t.Errorf("resolveInfraPath = %q, want %q", got, want)
	}
}
