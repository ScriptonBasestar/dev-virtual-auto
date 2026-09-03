package cli

import (
	"os"
	"sort"
	"strings"
	"testing"
)

// twoEncryptedYAML has two encrypted entries and one plaintext entry, which is
// the smallest config that makes every row of the §3 selector table reachable.
const twoEncryptedYAML = "version: \"0.1.45\"\nenv_file:\n" +
	"  - {path: .env.a, sops_source: a.enc}\n" +
	"  - {path: .env.b, sops_source: b.enc}\n" +
	"  - {path: .env.plain}\n"

// TestConfigEnvCodeSetHasNotDrifted guards the closed set of §7-1. A consumer
// switching on .error.code cannot distinguish an invented code from a frozen one,
// so adding or renaming a code is a contract change and has to be a deliberate
// edit to this list, not a side effect of an edit elsewhere.
func TestConfigEnvCodeSetHasNotDrifted(t *testing.T) {
	want := []string{
		"absolute_path",
		"ambiguous_env_selector",
		"decrypt_failed",
		"empty_decrypted_output",
		"env_target_not_encrypted",
		"force_unsupported_for_edit",
		"git_unavailable",
		"invalid_dotenv_output",
		"json_unsupported_for_edit",
		"no_encrypted_env_entry",
		"path_component_symlink",
		"path_escapes_config_root",
		"permission_denied",
		"sops_not_found",
		"source_is_target",
		"source_missing",
		"source_not_regular",
		"source_unreadable",
		"target_exists",
		"target_not_ignored",
		"target_not_regular",
		"target_parent_missing",
		"target_tracked",
		"unknown_env_origin",
		"unknown_env_target",
		"unsupported_env_origin",
		"unsupported_platform",
	}
	got := append([]string(nil), envBridgeCodes...)
	sort.Strings(got)

	if len(got) != len(want) {
		t.Fatalf("code set size = %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	seen := map[string]bool{}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("code[%d] = %q, want %q", i, got[i], want[i])
		}
		if seen[got[i]] {
			t.Errorf("duplicate code %q", got[i])
		}
		seen[got[i]] = true
	}
}

// TestConfigEnvSelectorGrammar pins the §3 selector table, which is the part of
// the interface a user types. Each row is a distinct mistake with a distinct fix,
// which is why they do not share a code.
func TestConfigEnvSelectorGrammar(t *testing.T) {
	for _, tt := range []struct {
		name   string
		yaml   string
		target string
		// want is "" when the selection must succeed, in which case picked names
		// the entry it must land on.
		want   string
		picked string
	}{
		{"single entry needs no selector", simpleBridgeYAML(".env", "s.enc"), "", "", ".env"},
		{"single entry accepts its own name", simpleBridgeYAML(".env", "s.enc"), ".env", "", ".env"},
		{"several entries require a selector", twoEncryptedYAML, "", codeAmbiguousSelector, ""},
		{"an exact encrypted name selects", twoEncryptedYAML, ".env.b", "", ".env.b"},
		{"a declared but plaintext entry", twoEncryptedYAML, ".env.plain", codeTargetNotEncrypted, ""},
		{"an undeclared name is unknown", twoEncryptedYAML, ".env.zzz", codeUnknownTarget, ""},
		{
			// The zero-encrypted case outranks every selector diagnostic: with
			// nothing encrypted there is no selection problem to report, only an
			// unconfigured feature.
			"no encrypted entry outranks a bad selector",
			"version: \"0.1.45\"\nenv_file: [.env]\n", ".env.zzz", codeNoEncryptedEntry, "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newBridgeFixture(t, tt.yaml, nil)
			entry, err := selectEncryptedEntry(f.cfg, tt.target)
			if tt.want != "" {
				requireCode(t, err, tt.want)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if entry.Path != tt.picked {
				t.Errorf("selected %q, want %q", entry.Path, tt.picked)
			}
		})
	}

	t.Run("the ambiguity message lists declared strings only", func(t *testing.T) {
		f := newBridgeFixture(t, twoEncryptedYAML, nil)
		_, err := selectEncryptedEntry(f.cfg, "")
		requireCode(t, err, codeAmbiguousSelector)
		msg := err.Error()
		for _, want := range []string{".env.a", ".env.b"} {
			if !strings.Contains(msg, want) {
				t.Errorf("message does not name %q: %s", want, msg)
			}
		}
		if strings.Contains(msg, f.dir) {
			t.Errorf("message expanded a local absolute path: %s", msg)
		}
	})
}

// TestConfigEnvEditRejectsInapplicableFlags pins the two argv rows of §3-1 that
// exist only because --json is a root persistent flag every subcommand inherits.
// "Do not register it" is not available as an answer, so the contract defines
// what passing it does instead.
func TestConfigEnvEditRejectsInapplicableFlags(t *testing.T) {
	t.Run("--json is refused before an editor runs", func(t *testing.T) {
		f := defaultFixture(t)
		f.install(true)
		requireCode(t, configEnvEditCmd.RunE(configEnvEditCmd, nil), codeJSONUnsupported)
		if len(f.sops.callsMade()) != 0 {
			t.Error("an editor session was started for a command that had already been refused")
		}
	})

	t.Run("--force does not apply to edit", func(t *testing.T) {
		f := defaultFixture(t)
		f.install(false)
		if err := configEnvEditCmd.Flags().Set("force", "true"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = configEnvEditCmd.Flags().Set("force", "false") })

		requireCode(t, configEnvEditCmd.RunE(configEnvEditCmd, nil), codeForceUnsupported)
		if len(f.sops.callsMade()) != 0 {
			t.Error("an editor session was started for a command that had already been refused")
		}
	})

	// --force is registered on edit only so that the frozen code above is what a
	// user sees instead of cobra's generic unknown-flag error. It must stay out of
	// the help text, because it is not part of edit's interface.
	t.Run("--force is hidden from edit's help", func(t *testing.T) {
		flag := configEnvEditCmd.Flags().Lookup("force")
		if flag == nil {
			t.Fatal("--force is not registered on edit, so the frozen code is unreachable")
		}
		if !flag.Hidden {
			t.Error("--force is advertised on edit, which invites a user to pass a flag that is always refused")
		}
	})
}

// TestConfigEnvEditTouchesOnlyTheSource pins §3: edit opens the encrypted source
// and writes no target at all, which is why the staleness hint exists and why it
// goes to stderr.
func TestConfigEnvEditTouchesOnlyTheSource(t *testing.T) {
	f := defaultFixture(t)
	f.install(false)

	stdout, stderr := captureStreams(t, func() {
		if err := runEnvEdit(""); err != nil {
			t.Errorf("edit: %v", err)
		}
	})

	calls := f.sops.callsMade()
	if want := "edit " + f.path("secrets.env.enc"); len(calls) != 1 || calls[0] != want {
		t.Errorf("sops calls = %v, want exactly [%q]", calls, want)
	}
	if _, err := os.Lstat(f.path(".env")); err == nil {
		t.Error("edit created the plaintext target; only unseal writes one")
	}
	if stdout != "" {
		t.Errorf("edit wrote to stdout, which belongs to the editor session: %q", stdout)
	}
	if !strings.Contains(stderr, "unseal") {
		t.Errorf("stderr does not point at the command that refreshes the target: %q", stderr)
	}
	f.assertNoTempResidue()
}

// TestConfigEnvUnsupportedPlatformFailsClosed runs on every OS, which is the
// point: the fail-closed branch for an undeclared platform is only trustworthy if
// it is executed somewhere, and it would otherwise never run in CI.
func TestConfigEnvUnsupportedPlatformFailsClosed(t *testing.T) {
	for _, goos := range []string{"windows", "plan9", "js", "aix"} {
		t.Run(goos, func(t *testing.T) {
			f := defaultFixture(t)
			f.install(false)
			bridgeGOOS = goos

			requireCode(t, runEnvUnseal("", true), codeUnsupportedPlatform)
			requireCode(t, runEnvEdit(""), codeUnsupportedPlatform)
			if len(f.sops.callsMade()) != 0 {
				t.Errorf("sops ran on an unsupported platform: %v", f.sops.callsMade())
			}
			if _, err := os.Lstat(f.path(".env")); err == nil {
				t.Error("a target was written on an unsupported platform")
			}
		})
	}

	t.Run("the declared support set is exactly linux and darwin", func(t *testing.T) {
		got := make([]string, 0, len(supportedBridgePlatforms))
		for k, v := range supportedBridgePlatforms {
			if v {
				got = append(got, k)
			}
		}
		sort.Strings(got)
		if want := []string{"darwin", "linux"}; len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
			t.Errorf("supported platforms = %v, want %v", got, want)
		}
	})
}

// TestConfigEnvSuccessOutput pins the frozen success shapes. The stderr notice for
// a target outside any repository is a notice and not a document key, so a
// consumer diffing the JSON never sees a field appear because of where the user
// happens to keep their project.
func TestConfigEnvSuccessOutput(t *testing.T) {
	t.Run("text mode reports created then replaced", func(t *testing.T) {
		f := defaultFixture(t)
		f.install(false)

		stdout, _ := captureStreams(t, func() {
			if err := runEnvUnseal("", false); err != nil {
				t.Fatalf("unseal: %v", err)
			}
		})
		if !strings.Contains(stdout, "created") {
			t.Errorf("first run did not report a creation: %q", stdout)
		}

		stdout, _ = captureStreams(t, func() {
			if err := runEnvUnseal("", true); err != nil {
				t.Fatalf("second unseal: %v", err)
			}
		})
		if !strings.Contains(stdout, "replaced") {
			t.Errorf("second run did not report a replacement: %q", stdout)
		}
	})

	t.Run("json mode carries the declared strings", func(t *testing.T) {
		f := defaultFixture(t)
		f.install(true)

		stdout, _ := captureStreams(t, func() {
			if err := runEnvUnseal("", false); err != nil {
				t.Fatalf("unseal: %v", err)
			}
		})
		for _, want := range []string{`"action"`, `"unseal"`, `".env"`, `"secrets.env.enc"`, `"created"`} {
			if !strings.Contains(stdout, want) {
				t.Errorf("json output is missing %s: %s", want, stdout)
			}
		}
		if strings.Contains(stdout, f.dir) {
			t.Errorf("json output expanded a local absolute path: %s", stdout)
		}
	})

	t.Run("a target outside any repository gets a stderr notice, not a json key", func(t *testing.T) {
		f := defaultFixture(t)
		f.git = &fakeGit{available: true}
		f.install(true)

		stdout, stderr := captureStreams(t, func() {
			if err := runEnvUnseal("", false); err != nil {
				t.Fatalf("unseal: %v", err)
			}
		})
		if !strings.Contains(stderr, "git repository") {
			t.Errorf("stderr does not carry the outside-repository notice: %q", stderr)
		}
		if strings.Contains(stdout, "git") {
			t.Errorf("the notice leaked into the frozen json document: %s", stdout)
		}
	})
}
