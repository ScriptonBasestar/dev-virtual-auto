package cli

import (
	"slices"
	"testing"
)

// TestSimilarToWeighsBothNames pins the suggestion threshold from both ends.
//
// The rule started as "the distance must be smaller than the input", which killed
// the noise it was written for — `dva restart -` offering both s1 and s2 — and left
// the mirror image open, because a long input against a one-character entry has the
// same distance-to-length ratio seen from the other side. Both rows below fail under
// the one-sided rule, in opposite directions, which is why they are asserted together.
func TestSimilarToWeighsBothNames(t *testing.T) {
	cases := []struct {
		what       string
		input      string
		candidates []string
		want       []string
	}{
		{"a real typo still suggests", "s3", []string{"s1", "s2"}, []string{"s1", "s2"}},
		{"a lone dash suggests nothing", "-", []string{"s1", "s2"}, nil},
		{"a long token does not reach a one-character entry", "wob", []string{"a", "b", "web"}, []string{"web"}},
		{"one-character entries get no suggestions at all", "c", []string{"a", "b"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.what, func(t *testing.T) {
			got := similarTo(tc.input, tc.candidates)
			if !slices.Equal(got, tc.want) {
				t.Errorf("similarTo(%q, %v) = %v, want %v", tc.input, tc.candidates, got, tc.want)
			}
		})
	}
}
