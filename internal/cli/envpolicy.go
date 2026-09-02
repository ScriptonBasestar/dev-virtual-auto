package cli

import (
	"github.com/ScriptonBasestar/dva/internal/config"
)

// This file holds the shared renderings of TASK-247's env-input contract. They live
// together so the frozen strings and JSON keys have one definition rather than one
// per route: `status`, `logs` and doctor each choose a policy, but none of them
// gets to spell the contract differently.
//
// Nothing here prints a variable name, a value, or how many files merged before a
// failure. The report carries only the configured path, the required flag and the
// failure kind, and these renderers cannot add what the report does not hold.

// envPartialJSON is the `environment` block every partial observation document
// carries. State is the literal "partial" rather than the report's own state,
// because this block is only ever emitted for an incomplete report and the
// observation contract names that value.
func envPartialJSON(r *config.EnvInputReport) map[string]any {
	failures := make([]map[string]any, 0, len(r.Failures()))
	for _, f := range r.Failures() {
		failures = append(failures, map[string]any{
			"file":     f.File,
			"required": f.Required,
			"kind":     string(f.Kind),
		})
	}
	return map[string]any{
		"state":    "partial",
		"failures": failures,
	}
}

// envNotQueriedJSON is the `runtime` block of a partial observation document. The
// point of the pair is that a consumer never has to infer absence: `queried:false`
// says the backend was not asked, so an empty result is not an empty stack.
func envNotQueriedJSON() map[string]any {
	return map[string]any{
		"queried": false,
		"reason":  "environment_incomplete",
	}
}

// envErrorJSON is the in-document error of a partial observation. It carries the
// bare sentence, not the per-file list: the list is already in the environment
// block, and repeating it inside a document that holds it structurally would give
// a consumer two sources for one fact.
func envErrorJSON() map[string]any {
	return map[string]any{
		"message":   "environment inputs are incomplete",
		"exit_code": 1,
	}
}

// envIncompleteError is the fail-closed error for execution, teardown and the exit
// status of partial observation. Its message is the frozen human block, so the root
// handler's stderr rendering and the JSON envelope both get the file list without
// either of them assembling it.
func envIncompleteError(r *config.EnvInputReport) error {
	return r.Err()
}
