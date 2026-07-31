package cli

import (
	"regexp"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/output"
)

// validateReport is what `dva validate --json` puts on stdout.
//
// TASK-088: the flag was accepted and never consulted. What JSON a consumer got came
// entirely from TASK-079's generic failure envelope, which stringifies whatever error the
// command returns and therefore covered only the two failing paths — the warning and
// success paths printed 21 bytes of prose whether or not --json was passed. That is the
// inverse of what the command is for: an assistant asking "is this config fine, and what
// should I fix?" got a machine answer only when the config was too broken to load.
//
// stderr is deliberately untouched. The prose warnings still go there byte-for-byte, in
// both modes; only stdout switches from the verdict line to this document, which is what
// keeps `dva validate --json 2>/dev/null | jq` parseable without moving human output.
type validateReport struct {
	Valid      bool              `json:"valid"`
	ConfigFile string            `json:"config_file,omitempty"`
	Warnings   []validateWarning `json:"warnings"`
	Errors     []validateError   `json:"errors"`

	// Error preserves TASK-079's envelope shape — `.error.message` and `.error.exit_code`
	// — for consumers already reading a failed `dva validate --json`. Printing this
	// document suppresses the envelope (see fail), so without this key that contract
	// would break silently on exactly the paths it was written for.
	Error *validateErrorEnvelope `json:"error,omitempty"`
}

type validateErrorEnvelope struct {
	Message  string `json:"message"`
	ExitCode int    `json:"exit_code"`
}

// validateWarning carries the prose apart rather than re-typed. Message is the warning's
// first line and Details holds the rest verbatim, so nothing the operator would have read
// is lost. Fields is a convenience over Details, never a replacement for it.
type validateWarning struct {
	Category string            `json:"category"`
	Message  string            `json:"message"`
	Details  []string          `json:"details,omitempty"`
	Fields   map[string]string `json:"fields,omitempty"`
}

type validateError struct {
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

// detailFieldPattern matches the `Key: value` convention the warning producers in
// internal/config follow on continuation lines — "Migration guide: https://…",
// "Affected entries: infra", "Hint: …".
//
// It is applied only to already-dedented detail lines, so the nested YAML inside the
// no-plans warning's "Example:" block cannot match: those lines keep leading spaces, and
// "Example:" itself has no value after the colon. A producer that stops using the
// convention loses its Fields entry and nothing else — Details still carries the text,
// which is why extraction is allowed to be a convention rather than a contract.
var detailFieldPattern = regexp.MustCompile(`^([A-Z][A-Za-z ]*): (.+)$`)

// schemaErrorPattern matches one line of config.Validate's schema failure list, e.g.
// "  - provision.default.0: Must have at most 1 properties".
var schemaErrorPattern = regexp.MustCompile(`^\s*-\s+(\S+):\s+(.+)$`)

// newValidateReport starts an optimistic report with both lists non-nil, so a clean config
// serializes `"warnings": []` rather than `"warnings": null`. A consumer that pipes into
// `jq '.warnings | length'` gets 0 either way, but one that iterates gets an empty loop
// instead of a null-iteration error, and the two lists then have one type across all four
// exit paths instead of two.
func newValidateReport(c *config.Config) validateReport {
	return validateReport{
		Valid:      true,
		ConfigFile: c.FilePath(),
		Warnings:   []validateWarning{},
		Errors:     []validateError{},
	}
}

// add appends free-text warnings under one category. Called with the same slices the
// prose printers consume, immediately after them, so the document cannot list a warning
// the operator was not also shown.
func (r *validateReport) add(category string, texts ...string) {
	for _, text := range texts {
		r.Warnings = append(r.Warnings, newValidateWarning(category, text))
	}
}

// addComposeNameWarning records the one warning class that arrives already structured.
// Its message is built by the same helper the printer uses, so the two cannot drift.
func (r *validateReport) addComposeNameWarning(w config.ComposeNameWarning) {
	lines := composeNameWarningLines(w)
	warning := newValidateWarning("compose_name", strings.Join(lines, "\n  "))
	if warning.Fields == nil {
		warning.Fields = map[string]string{}
	}
	warning.Fields["file"] = w.File
	warning.Fields["dva_name"] = w.DvaName
	if w.ComposeName != "" {
		warning.Fields["compose_name"] = w.ComposeName
	}
	r.Warnings = append(r.Warnings, warning)
}

// fail renders err into the report and hands it back unchanged for the normal error path.
//
// The document is printed here rather than by the caller because ordering is what keeps
// stdout to a single document: output.PrintJSON records the write, and emitFailureJSON in
// root.go yields when output.StdoutHasDocument() reports one. Returning the error still
// produces the `ERROR:` line on stderr and exit 1.
func (r *validateReport) fail(err error) error {
	if !jsonOutput {
		return err
	}
	r.Valid = false
	r.Errors = parseValidateErrors(err)
	r.Error = &validateErrorEnvelope{Message: err.Error(), ExitCode: 1}
	_ = output.PrintJSON(*r)
	return err
}

func newValidateWarning(category, text string) validateWarning {
	lines := strings.Split(text, "\n")
	w := validateWarning{Category: category, Message: strings.TrimSpace(lines[0])}
	for _, line := range lines[1:] {
		detail := strings.TrimPrefix(line, "  ")
		w.Details = append(w.Details, detail)
		if m := detailFieldPattern.FindStringSubmatch(detail); m != nil {
			if w.Fields == nil {
				w.Fields = map[string]string{}
			}
			w.Fields[strings.ReplaceAll(strings.ToLower(m[1]), " ", "_")] = m[2]
		}
	}
	return w
}

// parseValidateErrors splits a schema failure into one entry per offending key, so a
// consumer learns *which* key was wrong instead of receiving the whole report as one
// string. Any error that does not carry that list — a Go-level validation failure, or
// --strict's warning error — yields a single pathless entry, which is why the fallback is
// not an error case.
func parseValidateErrors(err error) []validateError {
	var out []validateError
	for line := range strings.SplitSeq(err.Error(), "\n") {
		if m := schemaErrorPattern.FindStringSubmatch(line); m != nil {
			out = append(out, validateError{Path: m[1], Message: m[2]})
		}
	}
	if len(out) == 0 {
		return []validateError{{Message: err.Error()}}
	}
	return out
}
