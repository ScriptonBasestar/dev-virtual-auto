package main

import "strings"

type verifyBinding struct {
	Line int
	Span string
}

// extractVerifyBindings declares the mechanical binding population shared by
// every verify-binding gate: task criteria only, fenced regions removed, the
// first inline-code span after verify:, and no human-only bindings or table rows.
func extractVerifyBindings(body string) []verifyBinding {
	body = stripFencedRegions(body)
	var bindings []verifyBinding
	offset := 0
	for _, line := range splitKeepEnds(body) {
		match := verifyCriterionRe.FindStringIndex(line)
		if match == nil {
			offset += len(line)
			continue
		}
		rest := strings.TrimSpace(line[match[1]:])
		if strings.HasPrefix(rest, "human —") {
			offset += len(line)
			continue
		}
		spanLoc := inlineCodeSpanRe.FindStringIndex(rest)
		if spanLoc == nil {
			offset += len(line)
			continue
		}
		span := rest[spanLoc[0]+1 : spanLoc[1]-1]
		spanOffset := offset + match[1] + spanLoc[0]
		if inTableRow(body, spanOffset) {
			offset += len(line)
			continue
		}
		bindings = append(bindings, verifyBinding{Line: lineAt(body, offset), Span: span})
		offset += len(line)
	}
	return bindings
}
