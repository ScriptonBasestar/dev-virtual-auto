package lifecycle

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestComposeConfigError_Error(t *testing.T) {
	cause := errors.New("exit status 1")
	e := &ComposeConfigError{
		Files:  []string{"deploy/local/compose.yaml"},
		Detail: "open /repo/deploy/local/compose.nexus.yaml: no such file or directory",
		cause:  cause,
	}

	msg := e.Error()

	// Names it as a config problem, surfaces docker's own cause, and offers a hint.
	for _, want := range []string{
		"compose config is invalid",
		"no such file or directory",
		"include:",
		"docker compose config",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, want it to contain %q", msg, want)
		}
	}
}

func TestComposeConfigError_Error_MultilineDetailUsesFirstLine(t *testing.T) {
	e := &ComposeConfigError{
		Detail: "open /repo/compose.nexus.yaml: no such file\nvalidating ...: extra noise",
	}
	// The summary line (before the first hint) must not carry the trailing noise.
	summary := strings.SplitN(e.Error(), "\n", 2)[0]
	if strings.Contains(summary, "extra noise") {
		t.Errorf("summary line leaked multi-line detail: %q", summary)
	}
	if !strings.Contains(summary, "no such file") {
		t.Errorf("summary line dropped the cause: %q", summary)
	}
}

func TestComposeConfigError_Unwrap(t *testing.T) {
	cause := errors.New("boom")
	e := &ComposeConfigError{Detail: "x", cause: cause}

	if !errors.Is(e, cause) {
		t.Errorf("errors.Is(e, cause) = false, want true")
	}

	var target *ComposeConfigError
	if !errors.As(fmt.Errorf("wrapped: %w", e), &target) {
		t.Errorf("errors.As did not recover *ComposeConfigError through a wrap")
	}
}

func TestFirstLine(t *testing.T) {
	cases := map[string]string{
		"single line":         "single line",
		"first\nsecond":       "first",
		"  trimmed  \nsecond": "trimmed",
		"":                    "",
	}
	for in, want := range cases {
		if got := firstLine(in); got != want {
			t.Errorf("firstLine(%q) = %q, want %q", in, got, want)
		}
	}
}
