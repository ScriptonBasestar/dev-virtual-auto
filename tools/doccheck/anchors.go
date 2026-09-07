package main

import (
	"regexp"
	"strings"
	"unicode"
)

var reHeading = regexp.MustCompile(`(?m)^(#{1,6})[ \t]+(.+?)[ \t]*#*[ \t]*$`)

var (
	reHeadingLink = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	reHeadingImg  = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
)

// collectAnchors returns GitHub-style heading slug set for a markdown body.
// Fenced code blocks are ignored so headings inside fences are not anchors;
// inline code in real headings is retained (stripped only for slug text).
func collectAnchors(src string) map[string]struct{} {
	src = stripFencedRegions(src)
	anchors := make(map[string]struct{})
	seen := make(map[string]int)

	add := func(heading string) {
		base := githubAnchor(heading)
		if base == "" {
			return
		}
		n := seen[base]
		seen[base] = n + 1
		slug := base
		if n > 0 {
			slug = base + "-" + itoa(n)
		}
		anchors[slug] = struct{}{}
	}

	for _, m := range reHeading.FindAllStringSubmatch(src, -1) {
		add(stripHeadingInline(m[2]))
	}
	lines := strings.Split(src, "\n")
	for i := 1; i < len(lines); i++ {
		under := strings.TrimRight(lines[i], " \t\r")
		if under == "" || !isSetextUnderline(under) {
			continue
		}
		text := strings.TrimSpace(lines[i-1])
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		add(stripHeadingInline(text))
	}
	return anchors
}

func isSetextUnderline(s string) bool {
	if len(s) == 0 {
		return false
	}
	ch := s[0]
	if ch != '=' && ch != '-' {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] != ch {
			return false
		}
	}
	return true
}

func stripHeadingInline(s string) string {
	s = strings.TrimSpace(s)
	s = reHeadingLink.ReplaceAllString(s, "$1")
	s = reHeadingImg.ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "*", "")
	s = stripUnderscoreEmphasis(s)
	return strings.TrimSpace(s)
}

// stripUnderscoreEmphasis removes `_` delimiters only where GitHub would render
// emphasis, keeping intraword underscores (`sops_source`, `snake_case`)
// literal so heading slugs match GitHub anchors. Flanking rules follow
// CommonMark: `_` opens emphasis only when left-flanking and not intraword,
// and closes only when right-flanking and not intraword. `__strong__` runs
// match the same way, delimiter by delimiter.
func stripUnderscoreEmphasis(s string) string {
	rs := []rune(s)
	type delim struct {
		idx               int
		canOpen, canClose bool
	}
	isPunct := func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) }
	var delims []delim
	for i, r := range rs {
		if r != '_' {
			continue
		}
		prevWS, nextWS := true, true
		prevPunct, nextPunct := false, false
		if i > 0 {
			prevWS = unicode.IsSpace(rs[i-1])
			prevPunct = isPunct(rs[i-1])
		}
		if i+1 < len(rs) {
			nextWS = unicode.IsSpace(rs[i+1])
			nextPunct = isPunct(rs[i+1])
		}
		leftFlanking := !nextWS && (!nextPunct || prevWS || prevPunct)
		rightFlanking := !prevWS && (!prevPunct || nextWS || nextPunct)
		d := delim{idx: i}
		d.canOpen = leftFlanking && (!rightFlanking || prevPunct)
		d.canClose = rightFlanking && (!leftFlanking || nextPunct)
		delims = append(delims, d)
	}
	var openers []*delim
	removed := make(map[int]bool, len(delims))
	for i := range delims {
		d := &delims[i]
		if d.canClose && len(openers) > 0 {
			o := openers[len(openers)-1]
			openers = openers[:len(openers)-1]
			removed[o.idx] = true
			removed[d.idx] = true
		}
		if d.canOpen {
			openers = append(openers, d)
		}
	}
	var b strings.Builder
	b.Grow(len(rs))
	for i, r := range rs {
		if removed[i] {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// githubAnchor approximates GitHub heading anchors for this repo's docs.
func githubAnchor(heading string) string {
	var b strings.Builder
	b.Grow(len(heading))
	prevHyphen := false
	for _, r := range strings.ToLower(heading) {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			b.WriteRune(r)
			prevHyphen = false
		case r == '_' || r == '-':
			b.WriteRune(r)
			prevHyphen = r == '-'
		case unicode.IsSpace(r):
			if !prevHyphen {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
