package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// The entry-name slot sits one position after the plan-name slot, and runPlanBuild's own
// comment has always claimed the two read a token the same way. TASK-218 changed the
// plan-name slot to read a lone "-" as a name; until these tests landed, the entry-name
// slot still read it as a flag, so the claimed parity was false in the direction the
// comment asserts. Measured before the fix, with an entry named "-" inside plan multi:
//
//	dva build multi -   ERROR: plan "multi" builds 2 entries, so - cannot be routed to
//	                           one of them; name it: dva build multi <-|s2>
//
// The suggestion offers "-" and the same message refuses it. One of those two sentences
// had to go; TASK-218 kept the suggestion, because config validation accepts the entry
// (`dva validate` exits 0 on that fixture) and nothing else in dva treats "-" as reserved.

const dashEntryBuildConfig = `version: "0.1.44"
stack:
  "-":
    default_runner: native
    runners:
      native:
        build: touch built-dash
        run: ./dash
  s2:
    default_runner: native
    runners:
      native:
        build: touch built-s2
        run: ./s2
plans:
  multi:
    entries:
      - name: "-"
        order: 1
      - name: s2
        order: 2
`

// TestPlanBuildRoutesToAnEntryNamedWithALoneDash binds build.go's entry-name slot.
//
// The two subtests are the two halves of the predicate and must both hold: adopting
// isFlagToken has to make "-" reachable WITHOUT making a real flag reachable. Without the
// second subtest the site could be "fixed" by deleting the guard outright.
func TestPlanBuildRoutesToAnEntryNamedWithALoneDash(t *testing.T) {
	t.Run("a lone dash names the entry", func(t *testing.T) {
		c := loadTestConfig(t, dashEntryBuildConfig)
		e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

		if err := runPlanBuild(c, planEnv(e), "multi", []string{"-"}); err != nil {
			t.Fatalf(`build multi -: %v; the plan declares an entry named "-" and the message that refused it offered that very name`, err)
		}

		// Which entry ran is the assertion, not merely that something did: routing to the
		// wrong one, or to both, would also exit 0.
		if _, err := os.Stat(filepath.Join(c.FileDir(), "built-dash")); err != nil {
			t.Errorf(`the entry named "-" did not build: %v`, err)
		}
		if _, err := os.Stat(filepath.Join(c.FileDir(), "built-s2")); err == nil {
			t.Errorf(`naming one entry built s2 as well; the entry-name slot selected nothing and fell through to the build-everything path`)
		}
	})

	t.Run("a real flag is still not an entry name", func(t *testing.T) {
		c := loadTestConfig(t, dashEntryBuildConfig)
		e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

		err := runPlanBuild(c, planEnv(e), "multi", []string{"--no-cache"})

		if err == nil {
			t.Fatal("--no-cache was routed as an entry name; the length test is what keeps flags out of this slot")
		}
		if !strings.Contains(err.Error(), "cannot be routed") {
			t.Errorf("want the ambiguity error a multi-target plan owes a flag it cannot place, got: %v", err)
		}
	})
}

const dashEntryLogConfig = `version: "0.1.44"
stack:
  "-":
    default_runner: native
    runners:
      native:
        run: ./dash
  s2:
    default_runner: native
    runners:
      native:
        run: ./s2
plans:
  multi:
    entries:
      - name: "-"
      - name: s2
`

// TestPlanLogsRoutesToAnEntryNamedWithALoneDash binds logs.go's entry-name slot, which is
// the same line as build.go's and therefore needs its own assertion: one test written from
// the defect report would leave the other site revertible with the suite green.
func TestPlanLogsRoutesToAnEntryNamedWithALoneDash(t *testing.T) {
	t.Run("a lone dash names the entry", func(t *testing.T) {
		c := loadTestConfig(t, dashEntryLogConfig)
		e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())
		writeEntryLog(t, c, "-", "DASH_ENTRY_LOG\n")
		writeEntryLog(t, c, "s2", "S2_ENTRY_LOG\n")

		var err error
		out := captureStdout(t, func() { err = runPlanLogs(c, planEnv(e), "multi", []string{"-"}) })

		if err != nil {
			t.Fatalf(`logs multi -: %v`, err)
		}
		if !strings.Contains(out, "DASH_ENTRY_LOG") {
			t.Errorf(`the log of the entry named "-" was not printed:\n%s`, out)
		}
		if strings.Contains(out, "S2_ENTRY_LOG") {
			t.Errorf("another entry's log was printed instead of the one named:\n%s", out)
		}
	})

	t.Run("a real flag is still not an entry name", func(t *testing.T) {
		c := loadTestConfig(t, dashEntryLogConfig)
		e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

		err := runPlanLogs(c, planEnv(e), "multi", []string{"-f"})

		if err == nil {
			t.Fatal("-f was routed as an entry name")
		}
		if !strings.Contains(err.Error(), "name one") {
			t.Errorf("want the ambiguity error, got: %v", err)
		}
	})
}
