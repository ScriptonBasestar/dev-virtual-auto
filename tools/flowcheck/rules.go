package main

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// finding is one rule violation, anchored to the YAML node that produced it.
type finding struct {
	rule string
	line int
	msg  string
}

// shellField is a scalar the flow runtime hands to /bin/sh: a step `action`, or any
// value under a step `context`. Prompt bodies are deliberately excluded. They are
// prose *about* dva, and scanning them is what made an earlier hand audit report
// `dva repo` and `dva commands` as real commands and miss the one that mattered.
type shellField struct {
	node *yaml.Node
	name string
}

type scan struct {
	shells   []shellField
	gates    []shellField
	findings []finding
	dvaCalls int
	// reportFields counts fields that read a tmp/ JSON artifact with jq, not individual
	// jq invocations. One field may hold several; the rule's unit is the field, because
	// that is what carries (or lacks) the single-object guard.
	reportFields int
	// producers indexes every `context:` value by the `<step id>.<key>` name a `when:`
	// reference uses, so a gate can be checked against the shell that feeds it. A gate is
	// only as sound as its producer, and the two sit hundreds of lines apart.
	producers map[string]*yaml.Node
	// gateRefsSeen keeps one producer from being reported once per gate that reads it.
	// `validate_pass1.is_valid` feeds three gates; the defect is in the producer, once.
	gateRefsSeen map[string]bool
	// steps holds one record per `id:`-bearing mapping, in document order, and stepIdx
	// names them. Together they are the `depends_on` graph the skip rules walk.
	steps   []*stepNode
	stepIdx map[string]int
	// cur is the step whose subtree walk is inside, so a scalar nested under `file:` is
	// still attributed to the step that owns it.
	cur *stepNode
	// skippableRefs counts references whose producer carries a `when:` — the population
	// the skip rules actually judge. Without it a clean run cannot be told apart from a
	// run where nothing was in scope, which is how an inert gate survives a green build.
	skippableRefs int
}

// stepNode is what the skip rules need to know about a step: whether a skip can start at
// it (`gated`), what a skip can reach from it (`dependsOn`), and what it reads (`refs`).
type stepNode struct {
	id        string
	line      int
	gated     bool
	dependsOn []string
	refs      []stepRef
}

// stepRef is one `{{producer.key}}` occurrence inside a step, tagged with the field that
// holds it. The field decides how bad an unrendered reference is, not whether it happens.
type stepRef struct {
	producer string
	key      string
	field    string
	line     int
}

func (s *scan) add(rule string, line int, format string, args ...any) {
	s.findings = append(s.findings, finding{rule: rule, line: line, msg: fmt.Sprintf(format, args...)})
}

// walk descends the document, collecting shell fields and checking the rules that are
// about structure rather than shell text. parentKey is the mapping key that owns n.
func walk(n *yaml.Node, parentKey string, s *scan) {
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range n.Content {
			walk(c, parentKey, s)
		}
	case yaml.MappingNode:
		stepID := scalarField(n, "id")
		prev := s.cur
		if stepID != "" {
			s.cur = s.newStep(stepID, n.Line)
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Value == "context" && stepID != "" && v.Kind == yaml.MappingNode {
				for j := 0; j+1 < len(v.Content); j += 2 {
					if v.Content[j+1].Kind == yaml.ScalarNode {
						if s.producers == nil {
							s.producers = map[string]*yaml.Node{}
						}
						s.producers[stepID+"."+v.Content[j].Value] = v.Content[j+1]
					}
				}
			}
			switch k.Value {
			case "exit_if_empty":
				s.add("exit-if-empty", k.Line,
					"`exit_if_empty` ends the pipeline *successfully*, so a missing prerequisite "+
						"produces a clean run that did nothing — indistinguishable from a run with "+
						"nothing to do. Exit non-zero from the step instead.")
			case "action":
				if v.Kind == yaml.ScalarNode {
					s.shells = append(s.shells, shellField{node: v, name: "action"})
				}
			case "parameters":
				checkParameters(v, s)
			case "when":
				if v.Kind == yaml.ScalarNode {
					s.gates = append(s.gates, shellField{node: v, name: "when"})
					if s.cur != nil {
						s.cur.gated = true
					}
				}
			case "depends_on":
				if s.cur != nil {
					s.cur.dependsOn = append(s.cur.dependsOn, scalarList(v)...)
				}
			}
			if parentKey == "context" && v.Kind == yaml.ScalarNode {
				s.shells = append(s.shells, shellField{node: v, name: "context." + k.Value})
			}
			// A gate's own references are the gate, not a consumer of one, and `id` names a
			// step rather than reading from it.
			if v.Kind == yaml.ScalarNode && k.Value != "when" && k.Value != "id" {
				s.collectRefs(shellField{node: v, name: qualify(parentKey, k.Value)})
			}
			walk(v, k.Value, s)
		}
		s.cur = prev
	}
}

// newStep registers a step and returns it. A duplicate `id:` keeps the first record, so
// the graph stays single-valued; am rejects duplicates itself, and guessing which one a
// reference means would report the defect twice under a different rule.
func (s *scan) newStep(id string, line int) *stepNode {
	if i, ok := s.stepIdx[id]; ok {
		return s.steps[i]
	}
	if s.stepIdx == nil {
		s.stepIdx = map[string]int{}
	}
	st := &stepNode{id: id, line: line}
	s.stepIdx[id] = len(s.steps)
	s.steps = append(s.steps, st)
	return st
}

// step resolves an id, or nil for a reference this file does not define — `param.*`, or a
// step in another flow. Those are skipped rather than guessed at.
func (s *scan) step(id string) *stepNode {
	if i, ok := s.stepIdx[id]; ok {
		return s.steps[i]
	}
	return nil
}

// collectRefs records every `{{step.key}}` a field interpolates, against the step that
// owns the field.
func (s *scan) collectRefs(f shellField) {
	if s.cur == nil {
		return
	}
	text := f.node.Value
	for _, m := range reStepRef.FindAllStringSubmatchIndex(text, -1) {
		s.cur.refs = append(s.cur.refs, stepRef{
			producer: text[m[2]:m[3]],
			key:      text[m[4]:m[5]],
			field:    f.name,
			line:     lineOf(f, text, m[0]),
		})
	}
}

// qualify names a field the way a reader would locate it: `file.path`, not `path`. Steps
// sit directly under `steps:`, which is a container rather than a field.
func qualify(parentKey, key string) string {
	if parentKey == "" || parentKey == "steps" {
		return key
	}
	return parentKey + "." + key
}

// scalarList reads `depends_on` in either spelling — `[a, b]` or a block sequence — and
// tolerates the single-scalar form.
func scalarList(n *yaml.Node) []string {
	var out []string
	switch n.Kind {
	case yaml.ScalarNode:
		out = append(out, n.Value)
	case yaml.SequenceNode:
		for _, c := range n.Content {
			if c.Kind == yaml.ScalarNode {
				out = append(out, c.Value)
			}
		}
	}
	return out
}

// checkParameters enforces flow.schema.json's Parameter contract, where `default` is a
// string and `enum` holds strings. YAML resolves a bare `true` to a boolean, so
// `enum: [true, false]` type-fails validation while looking perfectly reasonable.
//
// Both spellings are accepted: this corpus writes `parameters:` as a sequence of maps
// carrying a `name:` field, while the schema also allows a mapping keyed by name.
func checkParameters(params *yaml.Node, s *scan) {
	switch params.Kind {
	case yaml.SequenceNode:
		for _, spec := range params.Content {
			checkParameterSpec(paramName(spec), spec, s)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(params.Content); i += 2 {
			checkParameterSpec(params.Content[i].Value, params.Content[i+1], s)
		}
	}
}

// scalarField reads a scalar field off a mapping, or "" when it is absent or not a
// scalar. Used to recover the step `id` that owns a `context:` block.
func scalarField(n *yaml.Node, key string) string {
	if n.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key && n.Content[i+1].Kind == yaml.ScalarNode {
			return n.Content[i+1].Value
		}
	}
	return ""
}

// paramName reads the `name:` field of a sequence-style parameter, for the message only.
func paramName(spec *yaml.Node) string {
	if spec.Kind != yaml.MappingNode {
		return "?"
	}
	for i := 0; i+1 < len(spec.Content); i += 2 {
		if spec.Content[i].Value == "name" {
			return spec.Content[i+1].Value
		}
	}
	return "?"
}

func checkParameterSpec(name string, spec *yaml.Node, s *scan) {
	if spec.Kind != yaml.MappingNode {
		return
	}
	for j := 0; j+1 < len(spec.Content); j += 2 {
		fk, fv := spec.Content[j], spec.Content[j+1]
		switch fk.Value {
		case "default":
			if fv.Kind == yaml.ScalarNode && fv.Tag != "!!str" {
				s.add("param-type", fv.Line,
					"parameter %q: default %s is %s, but flow.schema.json requires a string — quote it",
					name, fv.Value, yamlType(fv.Tag))
			}
		case "enum":
			for _, item := range fv.Content {
				if item.Tag != "!!str" {
					s.add("param-type", item.Line,
						"parameter %q: enum value %s is %s, but flow.schema.json requires strings — quote it",
						name, item.Value, yamlType(item.Tag))
				}
			}
		}
	}
}

func yamlType(tag string) string { return strings.TrimPrefix(tag, "!!") }

// blankComments replaces whole-line shell comments with blank lines, preserving line
// numbering. Comments do not execute, and the fix for a defect routinely explains that
// defect in a comment directly above the code — a scanner that reads them reports the
// explanation as the bug.
func blankComments(text string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimLeft(l, " \t"), "#") {
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}

// lineOf maps a byte offset inside a scalar back to a file line. A block scalar's
// node.Line is the `key: |` line, so its content starts on the next one; every other
// style sits on the line node.Line already names. Most `context:` entries in this corpus
// are single-line quoted scalars, so treating them as block scalars misreports every
// finding on them by one line.
func lineOf(f shellField, text string, offset int) int {
	start := f.node.Line
	if f.node.Style == yaml.LiteralStyle || f.node.Style == yaml.FoldedStyle {
		start++
	}
	return start + strings.Count(text[:offset], "\n")
}
