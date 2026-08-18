// Corpus-wide checks. Every other rule here reads one field at a time, which is the right
// unit for a defect that lives inside a field. A fact computed independently in four
// separate flows is a different shape: each copy is correct on its own, and only the set
// can be wrong.

package main

import (
	"fmt"
	"regexp"
	"strings"
)

// corpusField is a shell field together with the file it came from. The per-file scan
// carries no path because it never needs one; a cross-file rule names two locations in a
// single finding and does.
type corpusField struct {
	path  string
	field shellField
}

type corpusFinding struct {
	path string
	line int
	rule string
	msg  string
}

// configProbes selects the fields that decide whether the target already has a config.
// The signature is a `-f 'dva.yml'` test and a boolean word emitted, and both halves are
// needed: `config_path`, the `CONFIG=` selectors and `DVA_FILE=` all run the same test but
// answer *which* file it is, naming a file instead of emitting true or false, while
// `has_compose` and `has_dockerfile` sit in the same context map emitting the same two
// words about a different question.
//
// Deliberately not part of the signature: the `dva.yaml` alternative. Requiring it was
// measured to hold the rule silent on the drift that costs the most -- a copy edited down
// to test `dva.yml` alone stops matching, leaves the set, and the scan goes green while
// that flow now misses every project using the other spelling. The only trace was the
// probe count falling from four to three in a summary line nobody diffs.
//
// The emit is matched as a word a command prints, not as a substring of the field. `|| true`
// appears in shell that has nothing to do with this fact, and a bare `strings.Contains` for
// it pulled two unrelated yq blocks into the set.
// reBoolEmit matches a command printing the literal flag: `printf true`, `echo false`,
// quoted or not.
var reBoolEmit = regexp.MustCompile(`(printf|echo)[ \t]+["']?(true|false)`)

func configProbes(fields []corpusField) []corpusField {
	var out []corpusField
	for _, cf := range fields {
		text := cf.field.node.Value
		if !strings.Contains(text, "-f 'dva.yml'") {
			continue
		}
		if !reBoolEmit.MatchString(text) {
			continue
		}
		out = append(out, cf)
	}
	return out
}

// checkConfigProbe requires every config-presence probe in the corpus to be spelled the
// same way.
//
// Four flows compute this flag: dva-diagnose, dva-improve, and the guided 00-analyze and
// 30-configure stages. They cannot share one producer -- they are separate flows, and the
// guided stages are each runnable on their own, so a flag handed down as a pipeline
// parameter would be absent exactly when a stage is run directly, and the backup it gates
// would skip in silence. Four copies is the honest shape. Drift between them is not.
//
// The drift that matters is not cosmetic. Swap this field's `printf` for an `echo` and the
// value gains a trailing newline am does not trim, so `== 'true'` is false for every input
// and the gate goes inert while the run still exits 0 -- the same silent fail-open the gate
// rules exist to catch, reintroduced one copy at a time.
//
// Every copy is measured against the first in sorted path order, so editing that one
// reports the other three rather than itself. Three findings, one edit, and the text names
// the location to compare against -- the alternative, picking the majority spelling, would
// bless a drift the moment it reached three copies.
func checkConfigProbe(probes []corpusField) []corpusFinding {
	if len(probes) < 2 {
		return nil
	}
	first := probes[0]
	want := normalizeField(first.field.node.Value)

	var out []corpusFinding
	for _, cf := range probes[1:] {
		if normalizeField(cf.field.node.Value) == want {
			continue
		}
		out = append(out, corpusFinding{
			path: cf.path,
			line: probeLine(cf),
			rule: "config-probe-drift",
			msg: fmt.Sprintf(
				"the config-presence probe is spelled two ways in the corpus: this field disagrees "+
					"with the one at %s:%d. One fact needs one spelling -- a copy that drifts to "+
					"`echo` gains a trailing newline am does not trim, and its gate then compares "+
					"false for every input while the run still exits 0. If the two are meant to "+
					"answer different questions they are different facts and need different names.",
				first.path, probeLine(first)),
		})
	}
	return out
}

func probeLine(cf corpusField) int {
	return lineOf(cf.field, cf.field.node.Value, 0)
}

// normalizeField compares fields on their code. Comments are dropped so that documenting
// one copy is not reported as drift, and surrounding blank space is trimmed because a
// block scalar carries the newline that ends it.
func normalizeField(text string) string {
	return strings.TrimSpace(blankComments(text))
}
