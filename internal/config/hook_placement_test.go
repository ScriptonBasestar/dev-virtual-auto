package config

import (
	"strings"
	"testing"
)

// TestValidateRejectsHooksWhereTheyCannotRun drives Validate() rather than
// validateHookPlacement, on purpose: the check is only worth anything if it is *wired*, and
// TASK-140 shipped a warning that worked and was never registered. Calling the helper
// directly would pass with the call in Validate deleted.
//
// Loaded through Load() because Validate() refuses a Config whose filePath is empty
// (validate.go:2), and filePath is unexported with no setter — Load is the only thing that
// sets it.
func TestValidateRejectsHooksWhereTheyCannotRun(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		wantErr string // "" means: must validate clean
	}{
		{
			// The headline case. Identical hook, one level down, was rc 0 and dead.
			name: "nested under a non-hookable parent",
			content: `version: "0.1.44"
interaction:
  db:
    command: echo db
    subcommands:
      migrate:
        before:
          - {step: backup, run: "echo BACKUP-RAN"}
        command: echo MIGRATING
`,
			// The parenthesised list is rendered by HookableCommandList, which sorts the live
			// set — it used to be a literal reading "up, down, stop, restart, build, clean,
			// logs" in both this message and the top-level one below. Spelling it out here
			// rather than calling the helper keeps the assertion independent of the code it
			// checks: a helper that returned "" would satisfy a test built from itself.
			wantErr: "interaction.db.subcommands.migrate: before/replace/after hooks run only " +
				"on a top-level hookable command (build, down, logs, restart, stop, up); " +
				"a hook nested under a subcommand never runs, whatever the subcommand is named",
		},
		{
			// A hookable parent does not lend its hookability downward. `up` needs hooks of
			// its own here or ValidateReservedCommands rejects the config first, for a
			// different reason, and the case never reaches the check under test.
			name: "nested under a hookable parent",
			content: `version: "0.1.44"
interaction:
  up:
    before:
      - {step: pre, run: "echo TOP-HOOK-RAN"}
    subcommands:
      fast:
        before:
          - {step: backup, run: "echo NESTED-BACKUP-RAN"}
        command: echo FAST
`,
			wantErr: "interaction.up.subcommands.fast: before/replace/after hooks run only",
		},
		{
			// The case that decides the rule's shape. The leaf is named `up`, so a check
			// keyed off IsHookableCommand(leafName) waves it through — while the hook is
			// exactly as unreachable as the other two. Measured before the fix:
			// `dva db up` printed DB-UP, rc 0, no LEAF-UP-BACKUP-RAN.
			name: "nested leaf whose own name is hookable",
			content: `version: "0.1.44"
interaction:
  db:
    command: echo db
    subcommands:
      up:
        before:
          - {step: backup, run: "echo LEAF-UP-BACKUP-RAN"}
        command: echo DB-UP
`,
			wantErr: "interaction.db.subcommands.up: before/replace/after hooks run only",
		},
		{
			// The pre-existing top-level rule, pinned so moving the check into a walker did
			// not change what it says. This message is quoted in TASK-169 and predates it.
			name: "top-level non-hookable is unchanged",
			content: `version: "0.1.44"
interaction:
  migrate:
    before:
      - {step: backup, run: "echo BACKUP-RAN"}
    command: echo MIGRATING
`,
			wantErr: "interaction.migrate: before/replace/after hooks are only supported on " +
				"hookable commands (build, down, logs, restart, stop, up)",
		},
		{
			// `clean` is the one name whose hooks were live config until the built-in was
			// removed, so it gets a message of its own rather than the generic one above —
			// that one reads as "you named it wrong", and here nothing is named wrong. The
			// shape is the real legacy one: hooks and no command, which only makes sense as
			// an extension of a built-in that no longer exists.
			//
			// This is the risk the restructure had to answer out loud. `stack`/`app`/`infra`
			// were never hookable, so their removal could take no working hook with it; this
			// one could, and silence here would mean a teardown step stopped running with the
			// config still validating.
			name: "clean names the removal rather than a typo",
			content: `version: "0.1.44"
interaction:
  clean:
    before:
      - {step: prune, run: "echo PRUNE-RAN"}
`,
			wantErr: "interaction.clean: the 'clean' built-in was removed — teardown is " +
				"'dva down <plan> --purge'",
		},
		{
			// Control. The whole point of hooks — a hookable built-in extended at the top
			// level — must stay legal, or this fix has broken the feature to fix its edge.
			name: "top-level hookable still validates",
			content: `version: "0.1.44"
interaction:
  build:
    before:
      - {step: pre, run: "echo PRE-BUILD"}
`,
		},
		{
			// Control. Subcommands are ordinary config; only hooks on them are rejected.
			name: "subcommands without hooks are untouched",
			content: `version: "0.1.44"
interaction:
  db:
    command: echo db
    subcommands:
      migrate:
        command: echo MIGRATING
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := loadConfigForSchemaTest(t, t.TempDir(), tc.content)
			err := cfg.Validate()

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil — this shape must keep validating", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil; the hook here can never execute, so a clean " +
					"verdict tells the author their config works when it does not")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Validate() = %q\nwant it to contain: %q", err, tc.wantErr)
			}
			// The full dotted path, not just the leaf. `migrate` alone reads as a plain
			// command name and says nothing about which nesting is the problem.
			if !strings.Contains(err.Error(), "interaction.") {
				t.Errorf("error %q does not name a config path", err)
			}
		})
	}
}

// TestHookPlacementErrorIsStableAcrossRuns pins the sort. c.Interaction is a map, so a
// config with two violations named a different one on each run before it — the TASK-128
// shape, which is worth a test here because the fix is one line and reads as decoration.
func TestHookPlacementErrorIsStableAcrossRuns(t *testing.T) {
	content := `version: "0.1.44"
interaction:
  alpha:
    command: echo a
    subcommands:
      one:
        before:
          - {step: x, run: "echo x"}
        command: echo one
  zulu:
    command: echo z
    subcommands:
      two:
        before:
          - {step: y, run: "echo y"}
        command: echo two
`
	cfg := loadConfigForSchemaTest(t, t.TempDir(), content)

	first := cfg.validateHookPlacement()
	if first == nil {
		t.Fatal("expected an error from a config with two nested hooks")
	}
	for i := range 50 {
		if got := cfg.validateHookPlacement(); got.Error() != first.Error() {
			t.Fatalf("run %d differs from run 0:\n first: %v\n got:   %v", i+1, first, got)
		}
	}
	if !strings.Contains(first.Error(), "interaction.alpha.subcommands.one") {
		t.Errorf("sorted order should report alpha before zulu, got %v", first)
	}
}
