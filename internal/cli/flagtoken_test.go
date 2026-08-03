package cli

import (
	"strings"
	"testing"
)

// TestSplitFlagToken pins the token grammar itself, including the one case that is not about
// flags at all: `dva run` forwards positional KEY=value pairs to interaction commands, and
// splitting those would turn `PORT=8080` into a flag named `PORT`.
func TestSplitFlagToken(t *testing.T) {
	cases := []struct {
		in       string
		name     string
		value    string
		hasValue bool
	}{
		{"--debug", "--debug", "", false},
		{"--debug=true", "--debug", "true", true},
		{"-M=dev", "-M", "dev", true},
		{"--tag=a,b", "--tag", "a,b", true},
		{"--mode=", "--mode", "", true},
		{"infra", "infra", "", false},
		{"PORT=8080", "PORT=8080", "", false},
		{"--", "--", "", false},
		{"-", "-", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			name, value, hasValue := splitFlagToken(tc.in)
			if name != tc.name || value != tc.value || hasValue != tc.hasValue {
				t.Errorf("splitFlagToken(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.in, name, value, hasValue, tc.name, tc.value, tc.hasValue)
			}
		})
	}
}

// TestRootFlagShapesNeverReachDocker is the criterion test for TASK-145: every token shape a
// root flag can be written in, in both the position cobra leaves it in.
//
// "Pre-command" and "post-command" are not a distinction cobra preserves for these commands.
// When the target sets DisableFlagParsing, cobra locates it and hands back every token that
// is not part of the command path — so `dva --debug=true stack log infra` arrives at RunE as
// ["--debug=true", "infra"], with the flag ahead of the entry name. That is the shape
// measured leaking, and it is what these rows drive. Nothing about the fix is
// position-sensitive, which is exactly what the rows are here to keep true.
func TestRootFlagShapesNeverReachDocker(t *testing.T) {
	cases := []struct {
		name string
		run  func(args []string) error
		args []string
		// absent must not appear in docker's argv as a whole token.
		absent []string
		// present must, or the fix broke the passthrough it exists to protect.
		present   []string
		wantDebug bool
		wantJSON  bool
	}{
		{
			name:      "bare, pre-command",
			run:       func(a []string) error { return stackLogCmd.RunE(stackLogCmd, a) },
			args:      []string{"--debug", "infra", "--tail=5"},
			absent:    []string{"--debug"},
			present:   []string{"logs", "--tail=5"},
			wantDebug: true,
		},
		{
			name:      "bare, post-command",
			run:       func(a []string) error { return stackLogCmd.RunE(stackLogCmd, a) },
			args:      []string{"infra", "--debug", "--tail=5"},
			absent:    []string{"--debug"},
			present:   []string{"logs", "--tail=5"},
			wantDebug: true,
		},
		{
			name:      "=value, pre-command",
			run:       func(a []string) error { return stackLogCmd.RunE(stackLogCmd, a) },
			args:      []string{"--debug=true", "infra", "--tail=5"},
			absent:    []string{"--debug=true", "--debug"},
			present:   []string{"logs", "--tail=5"},
			wantDebug: true,
		},
		{
			name:      "=value, post-command",
			run:       func(a []string) error { return stackLogCmd.RunE(stackLogCmd, a) },
			args:      []string{"infra", "--debug=true", "--tail=5"},
			absent:    []string{"--debug=true", "--debug"},
			present:   []string{"logs", "--tail=5"},
			wantDebug: true,
		},
		{
			// --debug=false is the shape that proves the value is read rather than the name
			// merely matched: a fix that stripped the token and set debug unconditionally
			// would pass every other row here.
			name:      "=false is obeyed, not just stripped",
			run:       func(a []string) error { return stackLogCmd.RunE(stackLogCmd, a) },
			args:      []string{"infra", "--debug=false"},
			absent:    []string{"--debug=false", "--debug"},
			present:   []string{"logs"},
			wantDebug: false,
		},
		{
			name:     "=value on the raw compose passthrough",
			run:      func(a []string) error { return composeCmd.RunE(composeCmd, a) },
			args:     []string{"--json=true", "logs", "--tail=5"},
			absent:   []string{"--json=true", "--json"},
			present:  []string{"logs", "--tail=5"},
			wantJSON: true,
		},
		{
			// Criterion 3. Before the fix this reached docker as `logs -- --tail=5`: the
			// terminator survived and the literal it was protecting did not.
			name:      "terminator forwards a DVA-spelled token",
			run:       func(a []string) error { return stackLogCmd.RunE(stackLogCmd, a) },
			args:      []string{"infra", "--", "--debug", "--tail=5"},
			absent:    []string{"--"},
			present:   []string{"logs", "--debug", "--tail=5"},
			wantDebug: false,
		},
		{
			name:      "terminator protects the =value shape too",
			run:       func(a []string) error { return logsCmd.RunE(logsCmd, a) },
			args:      []string{"--", "--debug=true"},
			absent:    []string{"--"},
			present:   []string{"logs", "--debug=true"},
			wantDebug: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			argv := composePassthroughFixture(t)

			if err := tc.run(tc.args); err != nil {
				t.Fatalf("%v returned %v", tc.args, err)
			}

			got := argv()
			t.Logf("docker invocations (%d): %v", len(got), got)
			if len(got) != 1 {
				t.Fatalf("docker invoked %d times, want 1 — no argv to assert on", len(got))
			}
			line := got[0]

			for _, a := range tc.absent {
				if hasArg(line, a) {
					t.Errorf("%s reached docker: %q", a, line)
				}
			}
			for _, p := range tc.present {
				if !hasArg(line, p) {
					t.Errorf("%s did not survive into %q — the passthrough is broken", p, line)
				}
			}
			if debug != tc.wantDebug {
				t.Errorf("debug = %v, want %v — the flag was consumed without being applied", debug, tc.wantDebug)
			}
			if jsonOutput != tc.wantJSON {
				t.Errorf("jsonOutput = %v, want %v", jsonOutput, tc.wantJSON)
			}
		})
	}
}

// TestParseDvaFlagValueShapes covers the fourth token shape, `--flag value`, alongside its
// `=` twin. parseDvaFlags is where the value-taking flags live; its output never reaches an
// external argv, so it gets a unit table rather than a row above.
func TestParseDvaFlagValueShapes(t *testing.T) {
	oldDebug, oldJSON, oldDryRun := debug, jsonOutput, dryRun
	t.Cleanup(func() { debug, jsonOutput, dryRun = oldDebug, oldJSON, oldDryRun })

	cases := []struct {
		name         string
		in           []string
		mode         string
		env          string
		includeTags  []string
		filtered     []string
		wantDebug    bool
		wantDryRun   bool
		wantFiltered string
		wantErr      bool
	}{
		{name: "separate value", in: []string{"--mode", "native", "pg"}, mode: "native", wantFiltered: "pg"},
		{name: "inline value", in: []string{"--mode=native", "pg"}, mode: "native", wantFiltered: "pg"},
		{name: "short inline", in: []string{"-M=native", "-E=stg"}, mode: "native", env: "stg"},
		{name: "list value splits", in: []string{"--tag=a,b"}, includeTags: []string{"a", "b"}},
		{name: "bare bool", in: []string{"--debug", "pg"}, wantDebug: true, wantFiltered: "pg"},
		{name: "bool with value", in: []string{"--debug=true", "pg"}, wantDebug: true, wantFiltered: "pg"},
		{name: "bool with false value", in: []string{"--debug=false", "pg"}, wantDebug: false, wantFiltered: "pg"},
		{name: "dry-run with value", in: []string{"--dry-run=true", "pg"}, wantDryRun: true, wantFiltered: "pg"},
		{
			// The terminator is kept here, unlike on the passthrough path: this output stays
			// inside DVA and the callers that reject unknown flags have always rejected a
			// stray `--`.
			name:         "terminator stops the scan and is kept",
			in:           []string{"--mode=native", "--", "--mode=other", "--debug"},
			mode:         "native",
			wantFiltered: "-- --mode=other --debug",
		},
		{
			// Not a boolean, so rejected here. It used to land in filtered "for the caller's
			// own rejectUnknownFlags to name" — which 5 of the 12 call sites do not have, and
			// on `dva build` filtered is docker's argv. TASK-172.
			name:    "malformed bool is rejected, not passed down",
			in:      []string{"--debug=notabool", "pg"},
			wantErr: true,
		},
		{
			// The rejection is the flag's, not the position's: a malformed value anywhere in
			// DVA's run of flags is still DVA's to name.
			name:    "malformed bool after a positional is still rejected",
			in:      []string{"pg", "--json=maybe"},
			wantErr: true,
		},
		{
			// Past the terminator it is not DVA's flag at all, so it stays a passthrough token
			// and no error is raised.
			name:         "malformed bool past the terminator is not ours",
			in:           []string{"--", "--debug=notabool"},
			wantFiltered: "-- --debug=notabool",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			debug, jsonOutput, dryRun = false, false, false
			mode, env, includeTags, _, filtered, err := parseDvaFlags(tc.in)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseDvaFlags(%v) = %q, nil — a malformed value must not reach a caller",
						tc.in, strings.Join(filtered, " "))
				}
				if !strings.Contains(err.Error(), "invalid boolean value") {
					t.Errorf("error = %q, want it to name the problem", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDvaFlags(%v) returned an unexpected error: %v", tc.in, err)
			}

			if mode != tc.mode {
				t.Errorf("mode = %q, want %q", mode, tc.mode)
			}
			if env != tc.env {
				t.Errorf("env = %q, want %q", env, tc.env)
			}
			if strings.Join(includeTags, ",") != strings.Join(tc.includeTags, ",") {
				t.Errorf("includeTags = %v, want %v", includeTags, tc.includeTags)
			}
			if strings.Join(filtered, " ") != tc.wantFiltered {
				t.Errorf("filtered = %q, want %q", strings.Join(filtered, " "), tc.wantFiltered)
			}
			if debug != tc.wantDebug {
				t.Errorf("debug = %v, want %v", debug, tc.wantDebug)
			}
			if dryRun != tc.wantDryRun {
				t.Errorf("dryRun = %v, want %v", dryRun, tc.wantDryRun)
			}
		})
	}
}

// TestMalformedBoolIsRejectedAtTheBoundary is the other half of "never silently accepted".
//
// consumeRootPersistentFlags is the last code that knows `--debug=notabool` is DVA's, so it
// is the last chance to say so in DVA's words. Forwarded, it would come back as docker
// complaining about a flag docker does not have.
func TestMalformedBoolIsRejectedAtTheBoundary(t *testing.T) {
	oldDebug, oldJSON := debug, jsonOutput
	t.Cleanup(func() { debug, jsonOutput = oldDebug, oldJSON })

	for _, in := range [][]string{
		{"--debug=notabool", "logs"},
		{"logs", "--json=maybe"},
	} {
		got, err := consumeRootPersistentFlags(in)
		if err == nil {
			t.Errorf("consumeRootPersistentFlags(%v) = %v, nil — a malformed value must not "+
				"reach the external argv", in, got)
			continue
		}
		if !strings.Contains(err.Error(), "invalid boolean value") {
			t.Errorf("consumeRootPersistentFlags(%v) error = %q, want it to name the problem", in, err)
		}
	}
}
