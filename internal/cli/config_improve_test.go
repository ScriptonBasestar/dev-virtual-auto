package cli

import (
	"testing"
)

func TestImproveInteractiveFlagRegistered(t *testing.T) {
	f := improveCmd.Flags().Lookup("interactive")
	if f == nil {
		t.Fatal("expected --interactive flag to be registered")
	}
	if f.Shorthand != "i" {
		t.Errorf("expected shorthand 'i', got %q", f.Shorthand)
	}
	if f.DefValue != "false" {
		t.Errorf("expected default false, got %q", f.DefValue)
	}
}

func TestImproveAllFlagsRegistered(t *testing.T) {
	flags := []string{"print", "docs-only", "verbose", "recursive", "rewrite", "interactive"}
	for _, name := range flags {
		if improveCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected flag --%s to be registered", name)
		}
	}
}

func TestBuildAmArgs(t *testing.T) {
	// Reset global state for test
	origVerbose := improveVerbose
	defer func() { improveVerbose = origVerbose }()

	improveVerbose = false
	args := buildAmArgs("dva-improve", map[string]string{
		"mode": "preserve",
	})
	if len(args) < 2 {
		t.Fatal("expected at least 2 args")
	}
	if args[0] != "run" {
		t.Errorf("expected first arg 'run', got %q", args[0])
	}
	if args[1] != "dva-improve" {
		t.Errorf("expected second arg 'dva-improve', got %q", args[1])
	}

	// Check param is included
	found := false
	for _, a := range args {
		if a == "param.mode=preserve" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected param.mode=preserve in args: %v", args)
	}
}

func TestBuildAmArgsVerbose(t *testing.T) {
	origVerbose := improveVerbose
	defer func() { improveVerbose = origVerbose }()

	improveVerbose = true
	args := buildAmArgs("dva-improve", nil)

	found := false
	for _, a := range args {
		if a == "--verbose" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --verbose in args: %v", args)
	}
}

func TestDvaConfigExists(t *testing.T) {
	// In test context without dva.yml, should return false
	// (depends on working directory, so just verify it doesn't panic)
	_ = dvaConfigExists()
}
