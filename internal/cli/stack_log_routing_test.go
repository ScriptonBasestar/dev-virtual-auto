// Package cli — regression test for TASK-102 at the command level.
//
// internal/config proves DetectPlugin now resolves the runners shape on a name lookup. This
// proves the thing a user would actually notice: `dva stack log <entry>` switches on that
// return value, and when it came back "" for a modern process entry the switch fell through to
// the compose default — DVA shelled out to `docker compose logs <entry>` for a stack entry that
// has nothing to do with docker.
package cli

import (
	"strings"
	"testing"
)

// stackLogRoutingFixture declares the same plugin in both shapes plus a real compose entry, so
// one config supports the deprecated control, the case under test, and the positive control
// that keeps "docker was never called" from passing vacuously.
const stackLogRoutingFixture = `version: "0.1.44"
stack:
  legacyproc:
    order: 1
    process:
      command: sleep 1
  modernproc:
    order: 2
    default_runner: process
    runners:
      process:
        command: sleep 1
  infra:
    order: 3
    default_runner: compose
    runners:
      compose:
        files:
          - docker-compose.yml
`

func TestStackLogRoutesByResolvedPlugin(t *testing.T) {
	t.Run("process entries never reach docker", func(t *testing.T) {
		for _, entry := range []string{
			"legacyproc", // control — the deprecated shape always routed correctly
			"modernproc", // before the fix this ran `docker compose logs modernproc`
		} {
			t.Run(entry, func(t *testing.T) {
				dockerArgv := composePassthroughFixtureWith(t, stackLogRoutingFixture)

				err := stackLogCmd.RunE(stackLogCmd, []string{entry})

				// The expected outcome is showStackEntryLog failing to find the log file:
				// the fixture never ran anything, so .sb/dva/logs/<entry>.log does not exist.
				// Reaching that error is the proof the process branch was taken.
				if err == nil {
					t.Fatalf("stack log %s succeeded; expected the missing-log-file error", entry)
				}
				want := `no log file for stack entry "` + entry + `"`
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error was %q, want it to contain %q — a different error means the "+
						"process branch was not the one taken", err, want)
				}
				if argv := dockerArgv(); len(argv) > 0 {
					t.Errorf("docker was invoked with %q; a process entry must not fall through "+
						"to the compose default", argv)
				}
			})
		}
	})

	t.Run("compose entries still reach docker", func(t *testing.T) {
		// The positive control. Without it, a `stack log` that had simply stopped calling
		// docker under any circumstance would satisfy every assertion above.
		dockerArgv := composePassthroughFixtureWith(t, stackLogRoutingFixture)

		if err := stackLogCmd.RunE(stackLogCmd, []string{"infra", "--tail=5"}); err != nil {
			t.Fatalf("stack log infra: %v", err)
		}
		argv := dockerArgv()
		if len(argv) == 0 {
			t.Fatal("control failed: docker was never invoked for a compose entry, so the " +
				"assertions above prove nothing")
		}
		t.Logf("docker argv: %q", argv)
		joined := strings.Join(argv, " ")
		for _, want := range []string{"logs", "--tail=5"} {
			if !strings.Contains(joined, want) {
				t.Errorf("%q missing from docker argv %q", want, joined)
			}
		}
	})
}
