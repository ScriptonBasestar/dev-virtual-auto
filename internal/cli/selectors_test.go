package cli

import (
	"strings"
	"testing"
)

func TestRejectUnknownFlagsExplainsPathScopedFlags(t *testing.T) {
	err := rejectUnknownFlags(
		"restart",
		"a stack entry name",
		[]string{"--no-wait"},
		stackSelectorFlags,
		[]string{"--no-wait", "--var"},
	)
	if err == nil {
		t.Fatal("rejectUnknownFlags returned nil")
	}
	message := err.Error()
	if !strings.Contains(message, `--no-wait applies only after a plan name`) {
		t.Fatalf("message does not explain the plan-only flag:\n%s", message)
	}
	if strings.Contains(message, "cannot start with") {
		t.Fatalf("message still blames a name collision:\n%s", message)
	}
	if !strings.Contains(message, `dva restart <plan> --no-wait`) {
		t.Fatalf("message does not show the valid plan path:\n%s", message)
	}
}

func TestRejectUnknownFlagsSplitsFlagValue(t *testing.T) {
	err := rejectUnknownFlags(
		"restart",
		"a stack entry name",
		[]string{"--var=FOO=bar"},
		stackSelectorFlags,
		[]string{"--no-wait", "--var"},
	)
	if err == nil {
		t.Fatal("rejectUnknownFlags returned nil")
	}
	message := err.Error()
	if strings.Contains(message, "--var=FOO=bar") {
		t.Fatalf("message quotes the value as part of the flag name:\n%s", message)
	}
	if !strings.Contains(message, `unknown flag "--var"`) {
		t.Fatalf("message does not quote the split flag name:\n%s", message)
	}
	if !strings.Contains(message, `dva restart <plan> --var`) {
		t.Fatalf("message does not suggest --var on the plan path:\n%s", message)
	}
	if strings.Contains(message, `dva restart --tag`) {
		t.Fatalf("exact path-scoped match includes a merely similar flag:\n%s", message)
	}
}
