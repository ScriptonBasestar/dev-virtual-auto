package cli

import "testing"

func TestDiscoverFlagsRegistered(t *testing.T) {
	if configDiscoverCmd.Flags().Lookup("format") == nil {
		t.Fatal("expected --format flag to be registered")
	}
	if configDiscoverCmd.Flags().Lookup("print") == nil {
		t.Fatal("expected --print flag to be registered")
	}
}

func TestJoinArgs(t *testing.T) {
	got := joinArgs([]string{"run", "dva-discover", "param.target=."})
	want := "run dva-discover param.target=."
	if got != want {
		t.Fatalf("joinArgs() = %q, want %q", got, want)
	}
}
