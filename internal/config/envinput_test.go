package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The sentinels below are what "content-free diagnostics" is checked against. A
// diagnostic that names either of them has leaked the contents of a file the user
// declared precisely because it is not meant to be read aloud.
const (
	secretKey   = "DVA_TEST_SENTINEL_KEY"
	secretValue = "dva-test-sentinel-value"
)

// TestEnvInputStates walks every state class in TASK-247 §2 against one report model,
// including the ordering cases: a success between two failures, and a success before a
// failure. Both matter, because the whole point of the report is that it does not stop
// at the first bad file and does not keep the good one's values.
func TestEnvInputStates(t *testing.T) {
	for _, tt := range []struct {
		name        string
		files       map[string]string
		dirs        []string
		declaration any
		wantState   EnvInputState
		wantReasons []string
	}{
		{
			name:        "single loaded",
			files:       map[string]string{".env": secretKey + "=" + secretValue + "\n"},
			declaration: ".env",
			wantState:   EnvInputComplete,
		},
		{
			name:        "optional missing is skipped, not failed",
			declaration: ".env.local",
			wantState:   EnvInputCompleteWithSkips,
		},
		{
			name:        "required missing",
			declaration: map[string]any{"files": []any{".env"}, "required": true},
			wantState:   EnvInputIncomplete,
			wantReasons: []string{"missing required file"},
		},
		{
			name:        "optional unreadable still fails",
			dirs:        []string{".env"},
			declaration: ".env",
			wantState:   EnvInputIncomplete,
			wantReasons: []string{"cannot read file"},
		},
		{
			name:        "malformed line is reported by number only",
			files:       map[string]string{".env": "# comment\n\n" + secretKey + "=" + secretValue + "\nnot an assignment\n"},
			declaration: ".env",
			wantState:   EnvInputIncomplete,
			wantReasons: []string{"invalid dotenv syntax at line 4"},
		},
		{
			// The §6 ordering fixture: loaded, then required missing, then malformed,
			// with an optional miss in between. Every declaration is examined and every
			// failure is reported, in declaration order.
			name: "loaded then required missing then malformed",
			files: map[string]string{
				"a.env": secretKey + "=" + secretValue + "\n",
				"d.env": "OK=1\nbroken\n",
			},
			declaration: map[string]any{"files": []any{
				map[string]any{"path": "a.env"},
				map[string]any{"path": "b.env", "required": true},
				map[string]any{"path": "c.env"},
				map[string]any{"path": "d.env"},
			}},
			wantState:   EnvInputIncomplete,
			wantReasons: []string{"missing required file", "invalid dotenv syntax at line 2"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, body := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range tt.dirs {
				if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
					t.Fatal(err)
				}
			}

			env := NewEnvironment(nil, dir, dir)
			report := ApplyEnvFiles(tt.declaration, dir, env)

			if report.State != tt.wantState {
				t.Fatalf("State = %q, want %q (entries %+v)", report.State, tt.wantState, report.Entries)
			}

			var gotReasons []string
			for _, f := range report.Failures() {
				gotReasons = append(gotReasons, f.Reason())
			}
			if strings.Join(gotReasons, "|") != strings.Join(tt.wantReasons, "|") {
				t.Errorf("reasons = %v, want %v", gotReasons, tt.wantReasons)
			}

			// Atomicity: on any failure not one env-file-derived value survives, even
			// from a file that loaded perfectly before the failing one.
			if report.Incomplete() {
				if got, ok := env.Vars[secretKey]; ok {
					t.Errorf("%s = %q survived an incomplete report; the merge must be all-or-nothing", secretKey, got)
				}
			} else if len(tt.files) > 0 && tt.wantState == EnvInputComplete {
				if env.Vars[secretKey] != secretValue {
					t.Errorf("%s = %q, want the loaded value on a complete report", secretKey, env.Vars[secretKey])
				}
			}

			// No diagnostic may carry a key, a value, or a count of what merged first.
			msg := report.Message()
			for _, leak := range []string{secretKey, secretValue} {
				if strings.Contains(msg, leak) {
					t.Errorf("Message() leaked %q:\n%s", leak, msg)
				}
			}
		})
	}
}

// TestEnvInputReportListsEveryFailingPathAsWritten pins the display rule: a relative
// declaration stays relative. Expanding it would put the machine's checkout path in
// every diagnostic, and the relative form is the one the user has to go and edit.
func TestEnvInputReportListsEveryFailingPathAsWritten(t *testing.T) {
	dir := t.TempDir()
	report := InspectEnvFiles(map[string]any{"files": []any{"missing/one.env", "missing/two.env"}, "required": true}, dir)

	msg := report.Message()
	want := "environment inputs are incomplete\n  - missing/one.env: missing required file\n  - missing/two.env: missing required file"
	if msg != want {
		t.Fatalf("Message() =\n%s\nwant\n%s", msg, want)
	}
	if strings.Contains(msg, dir) {
		t.Errorf("Message() expanded a relative declaration to an absolute path:\n%s", msg)
	}
}

// TestLoadEnvFileKeepsSuccessfulPrecedence pins that the rewrite changed only failure
// behavior: later files still win over earlier ones, and interpolation still runs.
func TestLoadEnvFileKeepsSuccessfulPrecedence(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.env"), []byte("A=first\nB=${A}-derived\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.env"), []byte("A=second\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	env := NewEnvironment(nil, dir, dir)
	if err := LoadEnvFile([]any{"a.env", "b.env"}, dir, env); err != nil {
		t.Fatalf("LoadEnvFile: %v", err)
	}
	if env.Vars["A"] != "second" {
		t.Errorf("A = %q, want the later file to win", env.Vars["A"])
	}
	if env.Vars["B"] != "first-derived" {
		t.Errorf("B = %q, want interpolation against the value in scope when it was declared", env.Vars["B"])
	}
}
