package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// stdoutHasDocument records that one of the printers below has already put bytes of a
// document on stdout. It is set when a write actually delivered bytes, not when one was
// merely attempted: a marshalling failure prints nothing and leaves it false, and so does
// a write that fails having delivered nothing. A write that fails partway does set it.
var stdoutHasDocument bool

// StdoutHasDocument reports whether a document has been printed to stdout in this process.
//
// It exists for the CLI's failure envelope: appending a second JSON document to a stdout
// that already holds one produces a stream `jq` does not read as a single object. See
// emitFailureJSON in internal/cli/root.go, the only caller.
func StdoutHasDocument() bool { return stdoutHasDocument }

// ResetStdoutDocument clears that record. Production code has no use for it — a process
// prints its document and exits — but tests share one process, and without a reset the
// first case to print would suppress every later case's envelope.
func ResetStdoutDocument() { stdoutHasDocument = false }

// printDocument writes one marshalled document to w, records whether stdout ended up
// holding bytes, and reports what the write returned.
//
// The record is keyed on the byte count rather than on the error, and that is the whole
// point of splitting this out. A write that fails after delivering part of a document has
// dirtied stdout exactly as thoroughly as one that succeeded, and the flag has one
// consumer — emitFailureJSON in internal/cli/root.go — that asks precisely whether stdout
// is dirty. Keying on err == nil instead would let a failure envelope be appended to a
// truncated document, handing a consumer two half-objects where the fix was supposed to
// leave it one whole one and an exit code.
//
// w is os.Stdout at both call sites. It is a parameter because the failure paths are hard
// to reach otherwise: a full filesystem under stdout does produce the error, but measuring
// it took a 1 MB disk image, and HFS+ returned a byte count of zero on both attempts — so
// the partial case the branch above exists for was never observed in the wild and is
// reachable only from a writer that stops short deliberately.
func printDocument(w io.Writer, s string) error {
	n, err := io.WriteString(w, s)
	if n > 0 {
		stdoutHasDocument = true
	}
	return err
}

// PrintJSON marshals data as indented JSON and prints to stdout.
//
// The error return covers marshal failures and write failures alike: encoding/json returns
// json.UnsupportedTypeError for a value it cannot encode, and printDocument returns whatever
// the write returned. PrintYAML keeps the same contract — see its comment for the one
// library difference that makes that non-trivial.
//
// os.Stdout is read here rather than captured in a package variable because several CLI
// tests capture output by assigning a pipe to it — internal/cli/show_test.go unmarshals
// what this function writes — and a writer resolved once at init would keep writing past
// them to the real one.
func PrintJSON(data any) error {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	// The trailing newline is the one fmt.Println used to supply.
	return printDocument(os.Stdout, string(bytes)+"\n")
}

// PrintYAML marshals data as YAML and prints to stdout.
//
// The error return covers marshal failures and write failures alike, matching PrintJSON.
// The non-trivial part is that gopkg.in/yaml.v3 raises an unsupported-kind condition as a
// panic rather than an error — its own recover (yaml.v3 handleErr) re-panics anything that is
// not its private yamlError type — so without the recover below a value yaml cannot encode
// terminates the process instead of reaching this return. That is why this function's error
// branch used to read as 75 percent coverage while every other function in the package reached
// full coverage: the statement was unreachable. Recovering here converts that panic into the
// same kind of returned error encoding.json hands back for the identical input. TASK-120.
func PrintYAML(data any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("yaml marshal: %v", r)
		}
	}()
	bytes, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	// yaml.Marshal already terminates its output with a newline, which is why this half of
	// the package used fmt.Print where PrintJSON used fmt.Println.
	return printDocument(os.Stdout, string(bytes))
}
