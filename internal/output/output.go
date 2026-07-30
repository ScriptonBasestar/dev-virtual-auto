package output

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// stdoutHasDocument records that one of the printers below has already put a document on
// stdout. It is set only on a successful write, so a marshalling failure — which prints
// nothing — leaves stdout still empty as far as callers are concerned.
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

// PrintJSON marshals data as indented JSON and prints to stdout.
func PrintJSON(data any) error {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(bytes))
	stdoutHasDocument = true
	return nil
}

// PrintYAML marshals data as YAML and prints to stdout.
func PrintYAML(data any) error {
	bytes, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	fmt.Print(string(bytes))
	stdoutHasDocument = true
	return nil
}
