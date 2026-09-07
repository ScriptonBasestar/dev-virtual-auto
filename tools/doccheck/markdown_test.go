package main

import (
	"strings"
	"testing"
)

func TestHeadingAnchor_githubStyle(t *testing.T) {
	cases := map[string]string{
		"Real Heading":    "real-heading",
		"11. Migration":   "11-migration",
		"Foo & Bar":       "foo-bar",
		"Hello, World!":   "hello-world",
		"already-slug":    "already-slug",
		"  Spaced  Out  ": "spaced-out",
		"UPPER case":      "upper-case",
		"path/to/file":    "pathtofile",
		"under_score":     "under_score",
		"dots...ok":       "dotsok",
	}
	for heading, want := range cases {
		got := githubAnchor(heading)
		if got != want {
			t.Errorf("githubAnchor(%q)=%q want %q", heading, got, want)
		}
	}
}

func TestExtractLinks_skipsCode(t *testing.T) {
	src := "# H\n\n" +
		"Good [a](a.md) and ![img](pic.png).\n" +
		"Bad inline `[x](no.md)`.\n" +
		"```md\n[y](no2.md)\n```\n" +
		"Ref [z][ref]\n\n[ref]: target.md\n"
	links := extractLinks(src)
	targets := make([]string, 0, len(links))
	for _, l := range links {
		targets = append(targets, l.Target)
	}
	joined := strings.Join(targets, ",")
	if !strings.Contains(joined, "a.md") {
		t.Fatalf("missing a.md in %v", targets)
	}
	if !strings.Contains(joined, "pic.png") {
		t.Fatalf("missing pic.png in %v", targets)
	}
	if !strings.Contains(joined, "target.md") {
		t.Fatalf("missing reference target.md in %v", targets)
	}
	for _, bad := range []string{"no.md", "no2.md"} {
		if strings.Contains(joined, bad) {
			t.Fatalf("code link %q should be suppressed, got %v", bad, targets)
		}
	}
}

// Given headings carrying underscores, When inline markup is stripped, Then
// only paired `_..._` emphasis is removed and intraword identifiers like
// `sops_source` survive exactly as GitHub renders them (TASK-326).
func TestStripHeadingInline_keepsIntrawordUnderscore(t *testing.T) {
	cases := map[string]string{
		"sops_source":              "sops_source",
		"snake_case and _italics_": "snake_case and italics",
		"__init__":                 "init",
		"_a_b_":                    "a_b",
		"trailing_":                "trailing_",
		"env_file Handling":        "env_file Handling",
	}
	for in, want := range cases {
		if got := stripHeadingInline(in); got != want {
			t.Errorf("stripHeadingInline(%q)=%q want %q", in, got, want)
		}
	}
}

// Given a real heading whose identifier carries an intraword underscore, When
// anchors are collected, Then the slug equals the anchor GitHub derives from
// the same heading (`#sops_source-bridge`, `#snake_case`).
func TestCollectAnchors_githubSlugForUnderscoreHeading(t *testing.T) {
	a := collectAnchors("## sops_source bridge\n")
	if _, ok := a["sops_source-bridge"]; !ok {
		t.Fatalf("expected sops_source-bridge in %v", a)
	}
	if _, ok := collectAnchors("## snake_case\n")["snake_case"]; !ok {
		t.Fatal("expected snake_case anchor")
	}
}

// Given a real heading with inline code, When anchors are collected, Then the
// slug is still present (inline code in headings is legitimate).
func TestCollectAnchors_keepsInlineCodeInHeading(t *testing.T) {
	src := "# Use `foo` flag\n"
	a := collectAnchors(src)
	if _, ok := a["use-foo-flag"]; !ok {
		t.Fatalf("expected use-foo-flag in %v", a)
	}
}

// Given CommonMark fence closed by a longer run, When links after the fence
// are extracted, Then prose links are still checked.
func TestExtractLinks_longerClosingFence(t *testing.T) {
	src := "# H\n\n" +
		"```\n" +
		"[fake](missing.md)\n" +
		"````\n\n" +
		"Real [ok](ok.md).\n"
	links := extractLinks(src)
	var targets []string
	for _, l := range links {
		targets = append(targets, l.Target)
	}
	joined := strings.Join(targets, ",")
	if strings.Contains(joined, "missing.md") {
		t.Fatalf("fenced link leaked: %v", targets)
	}
	if !strings.Contains(joined, "ok.md") {
		t.Fatalf("prose link after longer close missing: %v", targets)
	}
}

// Given CommonMark fence closed with up to three leading spaces, When links
// after the fence are extracted, Then prose links are still checked.
func TestExtractLinks_indentedClosingFence(t *testing.T) {
	src := "# H\n\n" +
		"```\n" +
		"[fake](missing.md)\n" +
		"   ```\n\n" +
		"Real [ok](ok.md).\n"
	links := extractLinks(src)
	var targets []string
	for _, l := range links {
		targets = append(targets, l.Target)
	}
	joined := strings.Join(targets, ",")
	if strings.Contains(joined, "missing.md") {
		t.Fatalf("fenced link leaked: %v", targets)
	}
	if !strings.Contains(joined, "ok.md") {
		t.Fatalf("prose link after indented close missing: %v", targets)
	}
}
