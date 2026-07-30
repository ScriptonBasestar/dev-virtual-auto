// TASK-088: `dva validate --json` accepted the flag and never consulted it.
//
// Every path that a consumer would actually want a machine answer for — "is this config
// valid", "what should I fix" — printed the same 21 bytes of prose it prints without the
// flag. The only JSON anyone ever saw came from TASK-079's generic failure envelope, which
// covers a returned error and therefore fired only on the two paths where the config was
// too broken to describe.
//
// These tests pin all three properties the fix has to hold at once: stdout carries exactly
// one document under --json, stderr keeps its prose byte-for-byte in both modes, and the
// document says which key was wrong rather than handing back one long string.
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

const cleanValidateFixture = `version: "0.1.44"
interaction:
  hello:
    description: "a command with nothing to warn about"
    command: "echo hello"
`

// warnValidateFixture loads clean, so validate reaches its verdict, and warns about
// stack.*.order — the richest warning the command produces (migration URL, affected
// entries, hint), which is what makes it the useful case for the Details/Fields split.
const warnValidateFixture = `version: "0.1.44"
stack:
  infra:
    order: 1
    default_runner: compose
    runners:
      compose:
        files: [does-not-exist.yml]
`

// schemaFailValidateFixture loads, then fails c.Validate(): provision_item's legacy branch
// allows at most one property and this carries two.
const schemaFailValidateFixture = `provision:
  default:
    - echo: "legacy echo form"
      cmd: "echo both-keys-at-once"
`

// runValidate drives the real RunE in a fixture directory and returns the two streams
// apart. They have to be separated rather than combined: the whole claim of the change is
// that stdout became a document while stderr did not move, and a merged buffer cannot
// distinguish "the warnings are in the JSON" from "the warnings landed next to it".
func runValidate(t *testing.T, body string, asJSON bool) (stdout, stderr string, err error) {
	t.Helper()

	dir := t.TempDir()
	if writeErr := os.WriteFile(filepath.Join(dir, "dva.yml"), []byte(body), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	oldWd, wdErr := os.Getwd()
	if wdErr != nil {
		t.Fatal(wdErr)
	}
	if chErr := os.Chdir(dir); chErr != nil {
		t.Fatal(chErr)
	}
	oldJSON, oldStrict := jsonOutput, validateStrict
	jsonOutput, validateStrict = asJSON, false
	cfg, env = nil, nil
	t.Cleanup(func() {
		os.Chdir(oldWd)
		jsonOutput, validateStrict = oldJSON, oldStrict
		cfg, env = nil, nil
	})

	oldOut, oldErr := os.Stdout, os.Stderr
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	os.Stdout, os.Stderr = outW, errW

	// Buffered in goroutines: a warning fixture writes ~800 bytes to stderr, and a pipe
	// that nobody is draining blocks the writer once its buffer fills.
	var outBuf, errBuf bytes.Buffer
	done := make(chan struct{}, 2)
	go func() { outBuf.ReadFrom(outR); done <- struct{}{} }()
	go func() { errBuf.ReadFrom(errR); done <- struct{}{} }()

	err = validateCmd.RunE(validateCmd, nil)

	outW.Close()
	errW.Close()
	<-done
	<-done
	os.Stdout, os.Stderr = oldOut, oldErr

	return outBuf.String(), errBuf.String(), err
}

type validateDoc struct {
	Valid      bool   `json:"valid"`
	ConfigFile string `json:"config_file"`
	Warnings   []struct {
		Category string            `json:"category"`
		Message  string            `json:"message"`
		Details  []string          `json:"details"`
		Fields   map[string]string `json:"fields"`
	} `json:"warnings"`
	Errors []struct {
		Path    string `json:"path"`
		Message string `json:"message"`
	} `json:"errors"`
	Error *struct {
		Message  string `json:"message"`
		ExitCode int    `json:"exit_code"`
	} `json:"error"`
}

// decodeOneDocument is the assertion the task asks for by name: not "is this parseable"
// but "is this exactly one document". json.Decoder is what tells them apart — Unmarshal on
// a concatenation of two objects fails, but so does Unmarshal on garbage, and only a second
// Decode call distinguishes "one object" from "one object followed by another".
func decodeOneDocument(t *testing.T, out string) validateDoc {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader([]byte(out)))
	var doc validateDoc
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("stdout is not a JSON document: %v\n%s", err, out)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err == nil {
		t.Fatalf("stdout carried a second document, so no consumer can parse it as one:\n%s", out)
	}
	return doc
}

func TestValidateJSONDescribesASuccess(t *testing.T) {
	out, errOut, err := runValidate(t, cleanValidateFixture, true)
	if err != nil {
		t.Fatalf("clean fixture returned an error: %v", err)
	}

	doc := decodeOneDocument(t, out)
	if !doc.Valid {
		t.Errorf("valid = false on a clean config")
	}
	if doc.ConfigFile == "" {
		t.Errorf("config_file is empty; the document does not say what it validated")
	}
	if doc.Error != nil {
		t.Errorf("a success carries a failure envelope: %+v", doc.Error)
	}
	// The lists are present-and-empty, not absent: a consumer iterating .warnings must get
	// an empty loop, not a null.
	if !bytes.Contains([]byte(out), []byte(`"warnings": []`)) {
		t.Errorf("warnings is not an empty array on a clean config:\n%s", out)
	}
	if errOut != "" {
		t.Errorf("a clean config wrote to stderr: %q", errOut)
	}
}

func TestValidateJSONCarriesWarningsAsData(t *testing.T) {
	out, errOut, err := runValidate(t, warnValidateFixture, true)
	if err != nil {
		t.Fatalf("warning fixture returned an error: %v", err)
	}

	doc := decodeOneDocument(t, out)
	// Printed, not just asserted: an empty array satisfying "no failure" is the exact
	// vacuous pass this test exists to prevent.
	t.Logf("warnings in document: %d", len(doc.Warnings))
	if len(doc.Warnings) == 0 {
		t.Fatalf("the config that produced %d bytes of stderr prose produced no warnings in the document:\n%s", len(errOut), out)
	}
	if !doc.Valid {
		t.Errorf("valid = false, but warnings alone do not make a config invalid (exit stays 0)")
	}

	var order *struct {
		Category string            `json:"category"`
		Message  string            `json:"message"`
		Details  []string          `json:"details"`
		Fields   map[string]string `json:"fields"`
	}
	for i := range doc.Warnings {
		if bytes.Contains([]byte(doc.Warnings[i].Message), []byte("stack.*.order")) {
			order = &doc.Warnings[i]
		}
	}
	if order == nil {
		t.Fatalf("no warning mentions stack.*.order; got %+v", doc.Warnings)
	}
	if order.Category != "semantic" {
		t.Errorf("category = %q, want semantic", order.Category)
	}
	if len(order.Details) == 0 {
		t.Errorf("the warning lost its continuation lines, which is where the migration URL lives")
	}
	// Fields is convenience over Details. It is asserted on the one producer that follows
	// the `Key: value` convention, because that is the only thing that makes the
	// convenience honest.
	if order.Fields["affected_entries"] != "infra" {
		t.Errorf("fields[affected_entries] = %q, want infra; fields = %v", order.Fields["affected_entries"], order.Fields)
	}
	if order.Fields["migration_guide"] == "" {
		t.Errorf("fields lost the migration guide; fields = %v", order.Fields)
	}
}

func TestValidateJSONNamesTheOffendingKey(t *testing.T) {
	out, _, err := runValidate(t, schemaFailValidateFixture, true)
	if err == nil {
		t.Fatalf("the schema-failure fixture returned nil; exit code would be 0")
	}

	doc := decodeOneDocument(t, out)
	if doc.Valid {
		t.Errorf("valid = true on a config that failed schema validation")
	}
	if len(doc.Errors) == 0 {
		t.Fatalf("errors is empty on a failure:\n%s", out)
	}
	found := false
	for _, e := range doc.Errors {
		if e.Path == "provision.default.0" {
			found = true
		}
		if e.Message == "" {
			t.Errorf("error entry has no message: %+v", e)
		}
	}
	if !found {
		t.Errorf("no error names provision.default.0; got %+v", doc.Errors)
	}
	// TASK-079's contract: a consumer reading .error.message on a failed command keeps
	// working. Losing this is the silent break the report could have caused.
	if doc.Error == nil || doc.Error.Message == "" || doc.Error.ExitCode != 1 {
		t.Errorf("the TASK-079 envelope keys are gone or wrong: %+v", doc.Error)
	}
}

// TestValidateWithoutJSONIsUnchanged is the other half of the change: the human path is
// supposed to be untouched. Comparing against the literal bytes rather than "contains"
// makes an accidental extra line fail here rather than in a user's terminal.
func TestValidateWithoutJSONIsUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name       string
		fixture    string
		wantStdout string
		wantErrOut bool
	}{
		{"clean", cleanValidateFixture, "✅ dva.yml is valid\n", false},
		{"warnings", warnValidateFixture, "✅ dva.yml is valid\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, errOut, err := runValidate(t, tc.fixture, false)
			if err != nil {
				t.Fatalf("returned an error: %v", err)
			}
			if out != tc.wantStdout {
				t.Errorf("stdout = %q, want %q", out, tc.wantStdout)
			}
			if (errOut != "") != tc.wantErrOut {
				t.Errorf("stderr non-empty = %v, want %v; got %q", errOut != "", tc.wantErrOut, errOut)
			}
		})
	}
}

// TestComposeNameWarningPathsShareOneSource pins the reason composeNameWarningLines exists.
// The prose and the document are built from the same slice, so a change to the wording
// cannot land in one and miss the other — the failure mode TASK-086 spent a whole task on.
func TestComposeNameWarningPathsShareOneSource(t *testing.T) {
	for _, tc := range []struct {
		name        string
		composeName string
	}{
		{"missing name", ""},
		{"differing name", "other-project"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := config.ComposeNameWarning{
				File:        "docker-compose.yml",
				DvaName:     "dva-project",
				ComposeName: tc.composeName,
			}
			lines := composeNameWarningLines(w)

			var report validateReport
			report.addComposeNameWarning(w)
			if len(report.Warnings) != 1 {
				t.Fatalf("got %d warnings, want 1", len(report.Warnings))
			}
			got := report.Warnings[0]

			if got.Message != lines[0] {
				t.Errorf("message = %q, want the headline %q", got.Message, lines[0])
			}
			if len(got.Details) != len(lines)-1 {
				t.Fatalf("details = %v, want the %d continuation lines %v", got.Details, len(lines)-1, lines[1:])
			}
			for i, want := range lines[1:] {
				if got.Details[i] != want {
					t.Errorf("details[%d] = %q, want %q", i, got.Details[i], want)
				}
			}
			if got.Fields["file"] != w.File || got.Fields["dva_name"] != w.DvaName {
				t.Errorf("structured fields lost: %v", got.Fields)
			}
		})
	}
}
