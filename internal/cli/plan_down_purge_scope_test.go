package cli

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// servicesPlanFixture is a compose entry whose plan selects a subset of services — the
// shape dogfood found in four devbox repos, where `down --volumes` left named volumes and
// the network behind and each repo kept its own `docker compose down -v` interaction.
func servicesPlanFixture(t *testing.T) (*config.Config, *config.Environment) {
	t.Helper()
	c := loadTestConfig(t, `version: "0.1.44"
stack:
  infra:
    default_runner: compose
    runners:
      compose:
        files: [docker-compose.yml]
plans:
  infra:
    entries:
      - name: infra
        services: [postgres, redis]
`)
	return c, config.NewEnvironment(nil, c.FileDir(), c.FileDir())
}

// TestPlanDownPurgeTearsDownWholeProject is TASK-311 criterion 1 under --dry-run: with
// services selected, --purge runs `compose down --volumes --rmi local` for the project,
// while plain down and --volumes stay on `rm` and announce what stays.
func TestPlanDownPurgeTearsDownWholeProject(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantArgs  string
		wantNote  string
		rejectArg string
	}{
		{"purge", []string{"--purge", "--force", "--dry-run"}, "down --remove-orphans --volumes --rmi local", "", " rm "},
		{"volumes", []string{"--volumes", "--dry-run"}, "rm --force --stop --volumes postgres redis", "named volumes and the project network stay", " down "},
		{"plain", []string{"--dry-run"}, "rm --force --stop postgres redis", "the project network stay", " down "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, e := servicesPlanFixture(t)
			logs := useBufferedSlog(t)
			var err error
			stderr := captureBothStreams(t, func() { err = runPlanDown(c, planEnv(e), "infra", tc.args) })
			if err != nil {
				t.Fatalf("runPlanDown %v: %v", tc.args, err)
			}
			// slog renders args as a Go slice: [--file docker-compose.yml down --remove-orphans ...]
			out := strings.ReplaceAll(logs.String(), "]", " ]")
			if !strings.Contains(out, tc.wantArgs) {
				t.Errorf("dry-run command missing %q:\n%s", tc.wantArgs, out)
			}
			if strings.Contains(out, tc.rejectArg) {
				t.Errorf("dry-run command must not contain %q:\n%s", tc.rejectArg, out)
			}
			if tc.wantNote == "" && strings.Contains(stderr, "stay —") {
				t.Errorf("project-wide down must not print a leftovers note:\n%s", stderr)
			}
			if tc.wantNote != "" && !strings.Contains(stderr, tc.wantNote) {
				t.Errorf("stderr missing %q:\n%s", tc.wantNote, stderr)
			}
		})
	}
}
