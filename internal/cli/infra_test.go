package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
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

func TestInfraServiceLocation_GitOnlyUsesCacheDir(t *testing.T) {
	cfgDir := filepath.Join(string(filepath.Separator), "home", "user", "project")
	svc := config.InfraConfig{
		Git: "https://github.com/example/infra.git",
		Ref: "main",
	}

	got, err := infraServiceLocation(svc, "gitsvc", cfgDir)
	if err != nil {
		t.Fatalf("infraServiceLocation: %v", err)
	}

	want := filepath.Join(cfgDir, config.DotDirName, "infra", "gitsvc")
	if got != want {
		t.Errorf("location = %q, want %q", got, want)
	}
	if got == cfgDir {
		t.Error("git-only service must never resolve to cfgDir")
	}
	if !strings.HasSuffix(got, filepath.Join(".sb", "dva", "infra", "gitsvc")) {
		t.Errorf("location %q must end with .sb/dva/infra/gitsvc", got)
	}
}

func TestInfraServiceLocation_PathOnlyRelative(t *testing.T) {
	cfgDir := filepath.Join(string(filepath.Separator), "home", "user", "project")
	svc := config.InfraConfig{Path: "infra/pg"}

	got, err := infraServiceLocation(svc, "pg", cfgDir)
	if err != nil {
		t.Fatalf("infraServiceLocation: %v", err)
	}

	want := filepath.Join(cfgDir, "infra", "pg")
	if got != want {
		t.Errorf("location = %q, want %q", got, want)
	}
}

func TestInfraServiceLocation_PathOnlyAbsolute(t *testing.T) {
	cfgDir := filepath.Join(string(filepath.Separator), "home", "user", "project")
	svc := config.InfraConfig{Path: "/opt/infra/pg"}

	got, err := infraServiceLocation(svc, "pg", cfgDir)
	if err != nil {
		t.Fatalf("infraServiceLocation: %v", err)
	}
	if got != "/opt/infra/pg" {
		t.Errorf("location = %q, want /opt/infra/pg", got)
	}
}

func TestInfraServiceLocation_PathResolvesToCfgDir_Error(t *testing.T) {
	cfgDir := filepath.Join(string(filepath.Separator), "home", "user", "project")
	// "." joins to cfgDir
	svc := config.InfraConfig{Path: "."}

	_, err := infraServiceLocation(svc, "bad", cfgDir)
	if err == nil {
		t.Fatal("expected error when path resolves to cfgDir")
	}
	if !strings.Contains(err.Error(), "project directory") {
		t.Errorf("error = %q, want mention of project directory", err)
	}
}

func TestInfraServiceLocation_EmptyPathNoGit_Error(t *testing.T) {
	cfgDir := filepath.Join(string(filepath.Separator), "home", "user", "project")
	svc := config.InfraConfig{}

	_, err := infraServiceLocation(svc, "empty", cfgDir)
	if err == nil {
		t.Fatal("expected error when neither git nor path is set")
	}
}

func TestInfraServiceLocation_GitOnlyNeverEqualsCfgDir(t *testing.T) {
	// Regression for TASK-049: empty path + git must not fall through to cfgDir.
	cfgDir := "/tmp/dva-project"
	svc := config.InfraConfig{Git: "https://example.com/r.git"}

	got, err := infraServiceLocation(svc, "svc", cfgDir)
	if err != nil {
		t.Fatalf("infraServiceLocation: %v", err)
	}
	if filepath.Clean(got) == filepath.Clean(cfgDir) {
		t.Fatalf("git-only location must not equal cfgDir; got %q", got)
	}
}
