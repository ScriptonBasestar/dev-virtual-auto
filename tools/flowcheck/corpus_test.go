package main

import (
	"strings"
	"testing"
)

// The spelling the corpus actually ships, in all four flows that ask the question.
const configProbeShell = "cd '{{param.target}}' && { [ -f 'dva.yml' ] || [ -f 'dva.yaml' ]; } && printf true || printf false"

type fileDoc struct{ path, body string }

// contextDoc wraps context values in the smallest flow that carries them. The corpus rule
// reads fields the same walk produces for a real file, so the cases are built by running
// the real scan rather than by hand-assembling nodes.
func contextDoc(values ...string) string {
	var doc strings.Builder
	doc.WriteString("steps:\n  - id: s\n    type: context\n    context:\n")
	for i, v := range values {
		doc.WriteString("      k")
		doc.WriteByte(byte('0' + i))
		doc.WriteString(": |\n")
		for l := range strings.SplitSeq(v, "\n") {
			doc.WriteString("        ")
			doc.WriteString(l)
			doc.WriteString("\n")
		}
	}
	return doc.String()
}

// inlineDoc carries the same value as a plain scalar instead of a block scalar. The two
// styles differ by the newline `|` puts at the end, and the map holding the probes already
// mixes them -- `dva_version:` one line above the diagnose probe is written this way.
func inlineDoc(value string) string {
	return "steps:\n  - id: s\n    type: context\n    context:\n      k0: \"" + value + "\"\n"
}

func corpusOf(t *testing.T, files ...fileDoc) []corpusField {
	t.Helper()
	var out []corpusField
	for _, f := range files {
		s, err := checkBytes([]byte(f.body), map[string]bool{})
		if err != nil {
			t.Fatalf("%s: %v", f.path, err)
		}
		for _, sf := range s.shells {
			out = append(out, corpusField{path: f.path, field: sf})
		}
	}
	return out
}

// TestConfigProbeDrift covers the one fact four separate flows each compute for
// themselves. They cannot share a producer -- dva-diagnose and dva-improve are unrelated
// flows, and the two guided stages each run standalone, so a flag passed down as a
// pipeline parameter would go missing exactly when a stage is run on its own and the
// backup it gates would skip in silence. The duplicate is the honest shape; this rule is
// what keeps the copies from drifting apart unnoticed.
func TestConfigProbeDrift(t *testing.T) {
	const fires = "config-probe-drift"

	tests := []struct {
		name  string
		files []fileDoc
		want  []string
	}{{
		name: "the four shipped copies agree",
		files: []fileDoc{
			{"a.yaml", contextDoc(configProbeShell)},
			{"b.yaml", contextDoc(configProbeShell)},
			{"c.yaml", contextDoc(configProbeShell)},
			{"d.yaml", contextDoc(configProbeShell)},
		},
		want: nil,
	}, {
		// The drift that matters. am does not trim, so this copy renders "true\n" and its
		// gate is false for every input -- while the run still exits 0.
		name: "one copy drifts to echo",
		files: []fileDoc{
			{"a.yaml", contextDoc(configProbeShell)},
			{"b.yaml", contextDoc(strings.ReplaceAll(configProbeShell, "printf", "echo"))},
		},
		want: []string{fires},
	}, {
		name: "two copies drift, two findings",
		files: []fileDoc{
			{"a.yaml", contextDoc(configProbeShell)},
			{"b.yaml", contextDoc(strings.ReplaceAll(configProbeShell, "printf", "echo"))},
			{"c.yaml", contextDoc(strings.ReplaceAll(configProbeShell, "||", "&&"))},
		},
		want: []string{fires, fires},
	}, {
		// The drift that costs the most, and the reason the `dva.yaml` alternative is not
		// part of the probe signature: a copy edited down to one filename must stay in the
		// set and disagree, not quietly leave it and take the scan green with it.
		name: "a copy stops testing the other filename",
		files: []fileDoc{
			{"a.yaml", contextDoc(configProbeShell)},
			{"b.yaml", contextDoc("cd '{{param.target}}' && [ -f 'dva.yml' ] && printf true || printf false")},
		},
		want: []string{fires},
	}, {
		// Abbreviated from deterministic_check_1 in dva-improve.yaml: it opens on the same
		// `-f 'dva.yml'` test and carries `|| true` further down as error suppression. That
		// is not a flag being emitted, and matching the bare word pulled this field and one
		// like it into the set.
		name: "shell that merely mentions true is not a probe",
		files: []fileDoc{
			{"a.yaml", contextDoc(configProbeShell)},
			{"b.yaml", contextDoc(
				"DVA_FILE=$([ -f 'dva.yml' ] && echo dva.yml || echo dva.yaml)\n" +
					"yq e '.stack | keys' \"$DVA_FILE\" 2>/dev/null || true",
			)},
		},
		want: nil,
	}, {
		// Nothing to disagree with. dva-discover asks the question nowhere.
		name: "a single copy is not drift",
		files: []fileDoc{
			{"a.yaml", contextDoc(configProbeShell)},
		},
		want: nil,
	}, {
		// Same code, written in the other scalar style. A block scalar ends in a newline and
		// a plain one does not, so comparing raw text would call this drift and point at a
		// difference the shell cannot see.
		name: "the same probe as a plain scalar is not drift",
		files: []fileDoc{
			{"a.yaml", contextDoc(configProbeShell)},
			{"b.yaml", inlineDoc(configProbeShell)},
		},
		want: nil,
	}, {
		// dva-discover asks the question nowhere at all. An empty set has no first copy to
		// measure the others against.
		name: "no copies at all",
		files: []fileDoc{
			{"a.yaml", contextDoc("dva validate 2>&1 | tail -40")},
		},
		want: nil,
	}, {
		// Documenting one copy is not drift. The comparison is on the code.
		name: "a comment on one copy is not drift",
		files: []fileDoc{
			{"a.yaml", contextDoc(configProbeShell)},
			{"b.yaml", contextDoc("# both names, because either is a legal config\n" + configProbeShell)},
		},
		want: nil,
	}, {
		// `has_compose` is the neighbouring field in 00-analyze.yaml's own context map. It
		// ends in the same `printf true || printf false` and is a different question, so the
		// probe has to be recognised by what it tests, not only by what it emits.
		name: "the sibling boolean probes are a different fact",
		files: []fileDoc{
			{"a.yaml", contextDoc(configProbeShell)},
			{"b.yaml", contextDoc(
				"cd '{{param.target}}' && ls compose.yaml compose.yml docker-compose.yml docker-compose.yaml 2>/dev/null >/dev/null && printf true || printf false",
				"cd '{{param.target}}' && [ -f 'Dockerfile' ] && printf true || printf false",
			)},
		},
		want: nil,
	}, {
		// Copied verbatim out of dva-improve.yaml, where all three sit in the same steps as
		// a probe and test the same two filenames -- but they answer *which* file it is,
		// not whether one exists. Reporting them against each other would be reporting
		// three different facts as one. The first two are held out by the boolean half of
		// the selector, the third by the filename half.
		name: "the fields naming a file are a different fact",
		files: []fileDoc{
			{"a.yaml", contextDoc(configProbeShell)},
			{"b.yaml", contextDoc(
				"cd '{{param.target}}' && { [ -f 'dva.yml' ] && echo 'dva.yml' || { [ -f 'dva.yaml' ] && echo 'dva.yaml' || echo ''; }; }",
				"if [ -f 'dva.yml' ]; then CONFIG='dva.yml'; elif [ -f 'dva.yaml' ]; then CONFIG='dva.yaml'; fi",
				"DVA_FILE=$([ -f 'dva.yml' ] && echo dva.yml || echo dva.yaml)",
			)},
		},
		want: nil,
	}}

	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.name, " ", "_"), func(t *testing.T) {
			found := checkConfigProbe(configProbes(corpusOf(t, tt.files...)))

			var got []string
			for _, f := range found {
				got = append(got, f.rule)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("rules = %v, want %v (findings: %+v)", got, tt.want, found)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("rules = %v, want %v", got, tt.want)
				}
			}
			// A finding has to say where the other copy is, or it cannot be acted on.
			for _, f := range found {
				if f.path == "" || f.line == 0 || !strings.Contains(f.msg, "a.yaml:") {
					t.Fatalf("finding does not locate both copies: %+v", f)
				}
			}
		})
	}
}
