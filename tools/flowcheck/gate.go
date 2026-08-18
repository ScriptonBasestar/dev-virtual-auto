package main

import (
	"fmt"
	"regexp"
)

var (
	// A filter inside a `when:` reference defeats the gate: measured against am cb8b4ce,
	// `{{ref | trim}} == 'true'` and `{{ref | trim}} == 'false'` both ran the step.
	reGateFilter = regexp.MustCompile(`\{\{[^}]*\|[^}]*\}\}`)

	// An unquoted `true`/`false` operand parses as a boolean while the template renders to
	// a string, and a string never equals a boolean. `!= true` therefore ran the step for
	// every value and `== true` skipped it for every value. `== 'true'` is correct, so the
	// quote is what the rule looks for.
	reGateBareBool = regexp.MustCompile(`(==|!=)[ \t]*(true|false)\b`)

	reTemplate = regexp.MustCompile(`\{\{`)

	// The `<step>.<key>` form of a reference: what a gate reads, and what any other field
	// interpolates. Both need the producer resolved, from opposite ends.
	reStepRef = regexp.MustCompile(`\{\{[ \t]*([A-Za-z_][A-Za-z0-9_-]*)\.([A-Za-z_][A-Za-z0-9_-]*)[ \t]*\}\}`)

	// A flag emitted with `echo` carries a trailing newline. am does not trim before
	// comparing, so the value renders as "true\n" and never equals 'true' -- the same
	// silent fail-open as an unquoted operand, reached from the other end.
	reEchoFlag = regexp.MustCompile(`\becho\b[ \t]+["']?(true|false)["']?`)
)

// checkGate runs the rules for `when:` expressions. Both defects below are silent: the
// gate does not error, the step simply runs (or does not) regardless of the value, and
// the pipeline still exits 0 — so neither `am validate` nor a passing run reveals them.
//
// Only expressions carrying a template are checked. `when: "true == true"` is a literal
// comparison and the evaluator handles it correctly.
func checkGate(f shellField, s *scan) {
	text := f.node.Value
	if !reTemplate.MatchString(text) {
		return
	}

	if m := reGateFilter.FindStringIndex(text); m != nil {
		s.add("gate-filter", lineOf(f, text, m[0]),
			"when: a filter inside the reference defeats the gate — the step runs whatever "+
				"the value and whatever the operator. Drop the filter; `printf` in the "+
				"producing step emits no trailing newline to trim.")
	}

	if m := reGateBareBool.FindStringSubmatchIndex(text); m != nil {
		op, lit := text[m[2]:m[3]], text[m[4]:m[5]]
		s.add("gate-operand", lineOf(f, text, m[0]),
			"when: `%s %s` compares a rendered string against a YAML boolean, which can never "+
				"be equal — `!= %s` runs the step for every value and `== %s` skips it for "+
				"every value. Quote the operand: `%s '%s'`.", op, lit, lit, lit, op, lit)
	}

	checkGateProducers(f, text, s)
}

// checkGateProducers follows each `{{step.key}}` a gate reads back to the shell that
// produces it. Quoting the operand is not sufficient on its own: the comparison is exact,
// so a producer that appends a newline defeats a correctly written gate. The finding is
// reported at the producer, which is where the fix goes and routinely hundreds of lines
// from the gate. References the file does not define — `param.*`, or a step in another
// file — are skipped rather than guessed at.
func checkGateProducers(f shellField, text string, s *scan) {
	for _, m := range reStepRef.FindAllStringSubmatch(text, -1) {
		ref := m[1] + "." + m[2]
		p, ok := s.producers[ref]
		if !ok || s.gateRefsSeen[ref] {
			continue
		}
		body := blankComments(p.Value)
		em := reEchoFlag.FindStringSubmatchIndex(body)
		if em == nil {
			continue
		}
		if s.gateRefsSeen == nil {
			s.gateRefsSeen = map[string]bool{}
		}
		s.gateRefsSeen[ref] = true
		s.add("gate-producer-newline", lineOf(shellField{node: p, name: ref}, body, em[0]),
			"context.%s: feeds the `when:` gate at line %d but emits its flag with `echo`, "+
				"which appends a newline. am does not trim, so the value renders as %q and "+
				"never equals '%s' — the gate runs for every value under `!=` and skips for "+
				"every value under `==`. Emit with `printf %s` instead.",
			m[2], f.node.Line, body[em[2]:em[3]]+"\n", body[em[2]:em[3]], body[em[2]:em[3]])
	}
}

// skipPromptFields names the fields where an unrendered `{{step.key}}` is worse than
// wrong-looking. An `instruction` hands the literal to a model, which answers around it
// instead of failing; a `file:` path or body writes it to disk, where it outlives the
// run. Everywhere else the literal at least stays inside the pipeline.
var skipPromptFields = map[string]bool{
	"instruction":  true,
	"prompt":       true,
	"file.path":    true,
	"file.content": true,
	"file.from":    true,
	"file.to":      true,
	"src":          true,
}

// checkSkipPropagation enforces the two halves of the `when:` contract that describe what
// happens *after* a gate closes, measured against am cb8b4ce:
//
//   - A skip propagates only into dependents that carry a `when:` of their own. A
//     dependent without one runs, and so does everything below it.
//   - A skipped step's key never renders. The reference reaches the consumer as the
//     literal text `{{step.key}}`, and the pipeline still exits 0.
//
// So a consumer that reads `{{G.key}}` is safe only when a skip of G is guaranteed to
// reach it: some `depends_on` path G → … → consumer on which every step past G is gated.
// A gate on the consumer alone is not enough — a step that reads a gated key without
// depending on it was measured running with the literal in hand.
func checkSkipPropagation(s *scan) {
	seen := map[string]bool{}
	for _, st := range s.steps {
		for _, r := range st.refs {
			p := s.step(r.producer)
			if p == nil || !p.gated || p.id == st.id {
				continue
			}
			s.skippableRefs++
			if s.skipReaches(st.id, r.producer, map[string]bool{}) {
				continue
			}
			key := st.id + "\x00" + r.producer + "." + r.key + "\x00" + r.field
			if seen[key] {
				continue
			}
			seen[key] = true

			ref := r.producer + "." + r.key
			rule := "gate-skip-leak"
			tail := fmt.Sprintf("the reference reaches this field as the literal text "+
				"`{{%s}}` and the run still exits 0", ref)
			if skipPromptFields[r.field] {
				rule = "gate-skip-prompt"
				tail = fmt.Sprintf("the literal text `{{%s}}` is handed to the model, or "+
					"written to disk, as if it were the value — so the step produces a "+
					"confident answer about a reference rather than failing, and the run "+
					"exits 0", ref)
			}
			s.add(rule, r.line,
				"%s: reads `{{%s}}`, whose step `%s` carries a `when:` (line %d) and can be "+
					"skipped. A skip propagates only into dependents carrying a `when:` of their "+
					"own, and `%s` is not on such a chain from `%s` — so it runs anyway and %s. "+
					"Gate this step too and `depends_on: [%s]`, or read the key through a step "+
					"that already does.",
				r.field, ref, p.id, p.line, st.id, p.id, tail, p.id)
		}
	}
}

// skipReaches reports whether a skip of producer propagates all the way down to consumer.
// Propagation stops at the first step without a `when:`, so every node on the path — the
// consumer included — has to be gated.
func (s *scan) skipReaches(consumer, producer string, seen map[string]bool) bool {
	c := s.step(consumer)
	if c == nil || !c.gated || seen[consumer] {
		return false
	}
	seen[consumer] = true
	for _, d := range c.dependsOn {
		if d == producer || s.skipReaches(d, producer, seen) {
			return true
		}
	}
	return false
}
