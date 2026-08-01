package main

import (
	"regexp"
	"strings"
)

// linkRef is a markdown link or image target extracted from prose (not code).
type linkRef struct {
	Target string
	Line   int // 1-based
}

var (
	reInlineLink = regexp.MustCompile(`!?\[(?:[^\]\\]|\\.)*\]\(\s*(?:<([^>]+)>|([^)\s]+))\s*(?:"[^"]*"|'[^']*'|\([^)]*\))?\s*\)`)
	reRefDef     = regexp.MustCompile(`(?m)^[ \t]{0,3}\[([^\]]+)\]:\s*(?:<([^>]+)>|(\S+))`)
)

// stripCodeRegions blanks fenced and inline code (keeps newlines) so link
// regexes cannot match inside code.
func stripCodeRegions(src string) string {
	return stripInlineCode(stripFencedRegions(src))
}

// stripFencedRegions blanks CommonMark fenced code blocks only (not inline
// code), preserving newlines. Used by link extraction and anchor collection.
func stripFencedRegions(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	lines := splitKeepEnds(src)
	i := 0
	for i < len(lines) {
		line := lines[i]
		openLen, openCh, ok := fenceOpenInfo(line)
		if !ok {
			b.WriteString(line)
			i++
			continue
		}
		// Blank opening fence line through closing fence line (or EOF).
		start := i
		i++
		for i < len(lines) {
			if fenceCloses(lines[i], openCh, openLen) {
				i++
				break
			}
			i++
		}
		for j := start; j < i; j++ {
			writeBlankKeepingNewlines(&b, lines[j])
		}
	}
	return b.String()
}

func stripInlineCode(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	i := 0
	for i < len(src) {
		if src[i] == '`' {
			n := 1
			for i+n < len(src) && src[i+n] == '`' {
				n++
			}
			open := src[i : i+n]
			closeIdx := findInlineClose(src[i+n:], open)
			if closeIdx < 0 {
				b.WriteByte(src[i])
				i++
				continue
			}
			end := i + n + closeIdx + n
			writeBlankKeepingNewlines(&b, src[i:end])
			i = end
			continue
		}
		b.WriteByte(src[i])
		i++
	}
	return b.String()
}

// splitKeepEnds splits on \n but retains the newline on each piece (last may lack it).
func splitKeepEnds(src string) []string {
	if src == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			out = append(out, src[start:i+1])
			start = i + 1
		}
	}
	if start < len(src) {
		out = append(out, src[start:])
	}
	return out
}

// fenceOpenInfo reports opener run length and marker if line opens a fence.
func fenceOpenInfo(line string) (n int, ch byte, ok bool) {
	s := strings.TrimRight(line, "\r\n")
	j := 0
	spaces := 0
	for j < len(s) && s[j] == ' ' && spaces < 3 {
		j++
		spaces++
	}
	if j >= len(s) {
		return 0, 0, false
	}
	ch = s[j]
	if ch != '`' && ch != '~' {
		return 0, 0, false
	}
	n = 0
	for j+n < len(s) && s[j+n] == ch {
		n++
	}
	if n < 3 {
		return 0, 0, false
	}
	// Info string may follow; backtick fences cannot contain backticks in info.
	rest := s[j+n:]
	if ch == '`' && strings.Contains(rest, "`") {
		return 0, 0, false
	}
	return n, ch, true
}

// fenceCloses reports whether line is a CommonMark closing fence for openCh/openLen.
// Close may be longer than open and may have up to three leading spaces.
func fenceCloses(line string, openCh byte, openLen int) bool {
	s := strings.TrimRight(line, "\r\n")
	j := 0
	spaces := 0
	for j < len(s) && s[j] == ' ' && spaces < 3 {
		j++
		spaces++
	}
	if j >= len(s) || s[j] != openCh {
		return false
	}
	n := 0
	for j+n < len(s) && s[j+n] == openCh {
		n++
	}
	if n < openLen {
		return false
	}
	// Remainder of line must be only spaces/tabs.
	for k := j + n; k < len(s); k++ {
		if s[k] != ' ' && s[k] != '\t' {
			return false
		}
	}
	return true
}

func findInlineClose(rest, open string) int {
	n := len(open)
	for i := 0; i+n <= len(rest); i++ {
		if rest[i:i+n] != open {
			continue
		}
		if i+n < len(rest) && rest[i+n] == '`' {
			continue
		}
		return i
	}
	return -1
}

func writeBlankKeepingNewlines(b *strings.Builder, s string) {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			b.WriteByte('\n')
		} else {
			b.WriteByte(' ')
		}
	}
}

// extractLinks returns markdown link/image destinations from non-code prose.
func extractLinks(src string) []linkRef {
	clean := stripCodeRegions(src)
	var out []linkRef
	for _, m := range reInlineLink.FindAllStringSubmatchIndex(clean, -1) {
		dest := submatch(clean, m, 1)
		if dest == "" {
			dest = submatch(clean, m, 2)
		}
		dest = strings.TrimSpace(dest)
		if dest == "" {
			continue
		}
		out = append(out, linkRef{Target: dest, Line: lineAt(clean, m[0])})
	}
	for _, m := range reRefDef.FindAllStringSubmatchIndex(clean, -1) {
		dest := submatch(clean, m, 2)
		if dest == "" {
			dest = submatch(clean, m, 3)
		}
		dest = strings.TrimSpace(dest)
		if i := strings.IndexAny(dest, " \t"); i >= 0 {
			dest = dest[:i]
		}
		if dest == "" {
			continue
		}
		out = append(out, linkRef{Target: dest, Line: lineAt(clean, m[0])})
	}
	return out
}

func submatch(s string, idx []int, group int) string {
	a, b := idx[group*2], idx[group*2+1]
	if a < 0 || b < 0 {
		return ""
	}
	return s[a:b]
}

func lineAt(s string, offset int) int {
	if offset < 0 {
		return 1
	}
	return strings.Count(s[:offset], "\n") + 1
}
