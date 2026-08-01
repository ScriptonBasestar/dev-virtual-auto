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
	for _, ch := range []string{"**", "__", "*", "_"} {
		s = strings.ReplaceAll(s, ch, "")
	}
	return strings.TrimSpace(s)
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
