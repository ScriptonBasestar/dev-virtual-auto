package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/output"
)

// TestFailureJSONEnvelopeReachesAProgram covers TASK-079: --json used to describe successes
// only, so a failed command left stdout empty and the exit code was the whole message. The
// name contains "JSON" on purpose — the task's criterion runs `-run JSON`.
func TestFailureJSONEnvelopeReachesAProgram(t *testing.T) {
	const msg = "no applications declared, so there is no 'myapp' to start"

	t.Run("a failure under --json is parseable and carries the message", func(t *testing.T) {
		defer restoreJSONState(t)()

		out := captureOutput(t, func() { emitFailureJSON(msg) })

		var doc struct {
			Error struct {
				Message  string `json:"message"`
				ExitCode int    `json:"exit_code"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(out), &doc); err != nil {
			t.Fatalf("stdout is not parseable JSON: %v; got %q", err, out)
		}
		// The same string stderr prints, not a summary of it: everything TASK-073 and
		// TASK-074 put into these messages has to survive the trip.
		if doc.Error.Message != msg {
			t.Errorf("message = %q, want %q", doc.Error.Message, msg)
		}
		if doc.Error.ExitCode != 1 {
			t.Errorf("exit_code = %d, want 1", doc.Error.ExitCode)
		}
	})

	t.Run("nothing goes to stdout without the flag", func(t *testing.T) {
		defer restoreJSONState(t)()
		jsonOutput = false

		if out := captureOutput(t, func() { emitFailureJSON(msg) }); out != "" {
			t.Errorf("emitted %q to stdout with --json unset; the human path owns stderr alone", out)
		}
	})

	// The case that decides the design. dva doctor --json exits 1 with a full {"checks": …}
	// document whose "passed": false already IS the failure; appending an envelope would put
	// two concatenated documents on stdout, which no consumer reads as one object.
	t.Run("it yields to a document already on stdout", func(t *testing.T) {
		defer restoreJSONState(t)()

		out := captureOutput(t, func() {
			_ = output.PrintJSON(map[string]any{"checks": []string{"already answered"}})
			emitFailureJSON(msg)
		})

		if strings.Contains(out, `"error"`) {
			t.Errorf("appended an envelope to a stdout that already had a document:\n%s", out)
		}
		// Not just "no error key" — the stream must still be exactly one document, which is
		// the property a second Unmarshal of the whole buffer actually tests.
		var single any
		if err := json.Unmarshal([]byte(out), &single); err != nil {
			t.Errorf("stdout is no longer a single JSON document: %v\n%s", err, out)
		}
	})
}

// restoreJSONState turns the envelope on and gives the test a clean stdout record, then
// returns the func that puts both globals back. Both are process-wide, and a case that
// leaked either would silently decide the next case's outcome.
func restoreJSONState(t *testing.T) func() {
	t.Helper()
	oldJSON := jsonOutput
	jsonOutput = true
	output.ResetStdoutDocument()
	return func() {
		jsonOutput = oldJSON
		output.ResetStdoutDocument()
	}
}
