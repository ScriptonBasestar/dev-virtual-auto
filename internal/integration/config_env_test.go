//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestConfigEnvRealSOPS drives TASK-246 through the pinned sops binary.
//
// The unit suite in internal/cli proves what DVA does around the child process
// by substituting a fake for it. That is the only way to inject a create, sync
// or rename fault, but it can never prove the two things this test exists for:
// that the argv DVA builds is argv sops accepts, and that DVA's own exit
// contract holds when the real binary exits with a code of its own. sops exits
// 128 when it cannot get a data key and 200 when an edit session leaves the file
// unchanged; neither may reach the caller, because TASK-245 §7-3 fixes every
// failure at exit 1.
func TestConfigEnvRealSOPS(t *testing.T) {
	t.Run("unseal writes exactly what sops decrypts", func(t *testing.T) {
		f := newSopsFixture(t, realSopsPlaintext)

		code, stdout, stderr := f.run(f.keyFile, "config", "env", "unseal", "--json")
		if code != 0 {
			t.Fatalf("exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}

		got, err := os.ReadFile(f.file(".env"))
		if err != nil {
			t.Fatalf("target was not written: %v", err)
		}
		// Byte equality, not key-by-key: --input-type/--output-type dotenv is
		// stated on both sides precisely so the shape survives, and " # not a
		// comment" inside quotes is where an inferred type would differ.
		if string(got) != realSopsPlaintext {
			t.Errorf("target = %q, want %q", string(got), realSopsPlaintext)
		}

		info, err := os.Stat(f.file(".env"))
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("target mode = %o, want 600", perm)
		}

		var doc map[string]any
		if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
			t.Fatalf("stdout is not the frozen json document: %v\n%s", err, stdout)
		}
		assertNoSentinel(t, "stdout", stdout)
		assertNoSentinel(t, "stderr", stderr)
		f.noResidue()
	})

	// The failure that the fake cannot stage honestly: a real key error, with
	// sops' real exit code and its real stderr.
	t.Run("a decryption failure is exit 1, not sops' 128", func(t *testing.T) {
		f := newSopsFixture(t, realSopsPlaintext)
		const existing = "PRE=existing-target-content\n"
		writeAll(t, f.dir, map[string]string{".env": existing})

		code, stdout, stderr := f.run(f.file("no-such-identity.txt"),
			"config", "env", "unseal", "--force", "--json")
		if code != 1 {
			t.Errorf("exit = %d, want 1 — sops' own code must not be propagated", code)
		}
		if got := errorCodeOf(t, stdout); got != "decrypt_failed" {
			t.Errorf("error code = %q, want decrypt_failed\nstdout: %s", got, stdout)
		}

		b, err := os.ReadFile(f.file(".env"))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != existing {
			t.Errorf("target = %q, want it untouched at %q", string(b), existing)
		}
		assertNoSentinel(t, "stdout", stdout)
		assertNoSentinel(t, "stderr", stderr)
		f.noResidue()
	})

	// sops encrypts an empty dotenv file happily and decrypts it back to zero
	// bytes with a zero exit, so "the child succeeded" and "the child produced a
	// usable target" are genuinely different questions on the real binary too.
	t.Run("an empty decryption is refused rather than written", func(t *testing.T) {
		f := newSopsFixture(t, "")

		code, stdout, stderr := f.run(f.keyFile, "config", "env", "unseal", "--json")
		if code != 1 {
			t.Fatalf("exit = %d, want 1\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
		if got := errorCodeOf(t, stdout); got != "empty_decrypted_output" {
			t.Errorf("error code = %q, want empty_decrypted_output\nstdout: %s", got, stdout)
		}
		if _, err := os.Lstat(f.file(".env")); err == nil {
			t.Error("an empty target was created")
		}
		f.noResidue()
	})

	// EDITOR=true leaves the file unchanged, which sops reports by exiting 200.
	// It is a cancellation, not a decryption failure, and the frozen code set has
	// no separate name for it — but the exit code is fixed either way.
	t.Run("a cancelled edit is exit 1, not sops' 200", func(t *testing.T) {
		f := newSopsFixture(t, realSopsPlaintext)
		before, err := os.ReadFile(f.file("secrets.env.enc"))
		if err != nil {
			t.Fatal(err)
		}

		code, stdout, stderr := f.run(f.keyFile, "config", "env", "edit")
		if code != 1 {
			t.Errorf("exit = %d, want 1 — sops' own code must not be propagated", code)
		}
		if _, err := os.Lstat(f.file(".env")); err == nil {
			t.Error("edit created a plaintext target; only unseal writes one")
		}

		after, err := os.ReadFile(f.file("secrets.env.enc"))
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != string(before) {
			t.Error("a cancelled edit rewrote the encrypted source")
		}
		assertNoSentinel(t, "stdout", stdout)
		assertNoSentinel(t, "stderr", stderr)
		f.noResidue()
	})
}

// TestConfigEnvGatedCommandsRealBinary exercises seal and show's own gate,
// --json and version checks through the real dva binary and real config
// loading — the properties the fake-driven unit suite cannot prove: that the
// real binary's exit code and .error.code hold with the real preflight in
// front of it.
//
// Only rows 1-2 of TASK-281 §3-3-1/§3-4-1 (gate, --json) are checked here
// against the JSON envelope. Every later row, including the version check, is
// unreachable through --json on the real CLI: seal and show both check
// jsonOutput at step 2, strictly before origin/version/platform/selector/etc.
// run, so the envelope emitFailureJSONFor writes can only ever carry a gate
// or a json-unsupported code for these two commands — never anything past
// them. That ordering is TASK-281's own contract (§3-3, §3-4-1: "gate before
// --json before everything else"), not a gap in this test. The version-gate
// subtests below therefore check the real binary's exit code and side
// effects without a machine-readable code; TestConfigEnvSealFaultMatrix and
// TestConfigEnvShowFaultMatrix already pin the exact code for this row
// through the fake-driven suite, which the ordering rule above is exactly
// why it CAN pin it (those tests do not go through --json's early return).
//
// seal and show diverge past that point too: seal has no agent-detection or
// controlling-terminal step ahead of its version check, so the real binary
// reaches that check directly. show's ordering puts agent-detection and the
// terminal gate first, and this harness gives the child no controlling
// terminal at all (no pty), so show's row here instead pins that its own
// terminal gate fires deterministically against the real binary — the one
// property the fake-driven suite, which substitutes a seam for the terminal,
// cannot prove. show's version check itself is already pinned there.
//
// A full real-sops round trip through seal is covered separately, by
// TestConfigEnvSealRealSOPSRoundTrip: EnvBridgeVersionSatisfied requires a
// declared version: at least EnvBridgeIntroducedVersion (0.1.48), which would
// otherwise be unreachable here, since checkConfigVersion refuses to load any
// config whose declared version: is newer than the running dva's own
// compiled-in config.Version — the same dva.yml field feeds both checks. That
// round trip's dvaBinary is therefore built with config.Version=0.1.48
// injected via -ldflags (config_env_helpers_test.go), rather than by bumping
// the global compiled default the rest of this suite still runs against.
//
// show's own real-sops path stays uncovered here and by that round trip both:
// its decrypted output is reachable only through bridgeOpenTTY's real
// os.OpenFile("/dev/tty", ...) call, which requires the child to have an
// actual controlling terminal. Nothing in this harness allocates a pty, so no
// test anywhere — unit or integration — currently drives show's real
// bridgeSops.Decrypt call end to end; the unit suite's ttyPipe seam fakes
// bridgeSops too (see config_env_fixture_test.go's fakeSops), and this file's
// own terminal-gate subtest below stops at the tty gate, before sops would
// ever run.
func TestConfigEnvGatedCommandsRealBinary(t *testing.T) {
	t.Run("seal and show are off by default against a real binary", func(t *testing.T) {
		f := newSopsFixture(t, realSopsPlaintext)

		for _, tt := range []struct {
			args     []string
			wantCode string
		}{
			{[]string{"config", "env", "seal", "--yes", "--json"}, "seal_not_enabled"},
			{[]string{"config", "env", "show", "--json"}, "show_not_enabled"},
		} {
			code, stdout, stderr := f.run(f.keyFile, tt.args...)
			if code != 1 {
				t.Errorf("%v: exit = %d, want 1\nstdout: %s\nstderr: %s", tt.args, code, stdout, stderr)
			}
			if got := errorCodeOf(t, stdout); got != tt.wantCode {
				t.Errorf("%v: error code = %q, want %q\nstdout: %s", tt.args, got, tt.wantCode, stdout)
			}
			if _, err := os.Lstat(f.file(".env")); err == nil {
				t.Errorf("%v: a disabled command wrote the plaintext target", tt.args)
			}
			assertNoSentinel(t, "stdout", stdout)
			assertNoSentinel(t, "stderr", stderr)
		}
		f.noResidue()
	})

	t.Run("--json is refused even once the gate is on", func(t *testing.T) {
		f := newSopsFixtureWithEnvBridge(t, realSopsPlaintext, "0.1.45", true, true)

		for _, tt := range []struct {
			args     []string
			wantCode string
		}{
			{[]string{"config", "env", "seal", "--yes", "--json"}, "json_unsupported_for_seal"},
			{[]string{"config", "env", "show", "--json"}, "json_unsupported_for_show"},
		} {
			code, stdout, stderr := f.run(f.keyFile, tt.args...)
			if code != 1 {
				t.Errorf("%v: exit = %d, want 1\nstdout: %s\nstderr: %s", tt.args, code, stdout, stderr)
			}
			if got := errorCodeOf(t, stdout); got != tt.wantCode {
				t.Errorf("%v: error code = %q, want %q\nstdout: %s", tt.args, got, tt.wantCode, stdout)
			}
			if _, err := os.Lstat(f.file(".env")); err == nil {
				t.Errorf("%v: a refused command wrote the plaintext target", tt.args)
			}
			assertNoSentinel(t, "stdout", stdout)
			assertNoSentinel(t, "stderr", stderr)
		}
		f.noResidue()
	})

	// seal has no agent-detection or tty step ahead of the version check, so
	// the real binary reaches it directly and this subtest pins it end to end.
	// show's own ordering (§3-4-1) puts agent-detection and the controlling-
	// terminal gate ahead of the version check, and neither is reachable
	// honestly through this harness: f.run() spawns dva with buffer-backed
	// stdio and no pty, so show's tty gate fires deterministically before the
	// version check ever would, real controlling terminal or not. Forcing a
	// real controlling terminal onto the child just for this one row would
	// mean adding pty machinery for something the fake-driven
	// TestConfigEnvShowFaultMatrix's "env_bridge without a satisfying version"
	// case already pins exactly, through the real in-process ttyPipe seam —
	// so show's row here instead asserts the terminal gate itself fires
	// against the real binary, which is the one property that row can prove
	// that the fake-driven suite cannot.
	t.Run("seal: env_bridge without a satisfying version refuses before sops runs", func(t *testing.T) {
		f := newSopsFixtureWithEnvBridge(t, realSopsPlaintext, "0.1.45", true, true)
		// This suite runs under Claude Code itself, so CLAUDECODE is genuinely
		// set in the outer environment f.run() inherits via os.Environ().
		// seal's own ordering has no agent-detection step, so this does not
		// change what seal's row exercises here — it is scrubbed anyway for
		// parity with the show subtest below and in case a future ordering
		// change adds one.
		for _, name := range []string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "AI_AGENT"} {
			old, wasSet := os.LookupEnv(name)
			_ = os.Unsetenv(name)
			t.Cleanup(func() {
				if wasSet {
					_ = os.Setenv(name, old)
				}
			})
		}

		code, stdout, stderr := f.run(f.keyFile, "config", "env", "seal", "--yes")
		if code != 1 {
			t.Errorf("exit = %d, want 1\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
		if !strings.Contains(stderr, "satisfying version") {
			t.Errorf("stderr does not name the version problem: %q", stderr)
		}
		if _, err := os.Lstat(f.file(".env")); err == nil {
			t.Error("a version-refused seal wrote the plaintext target")
		}
		assertNoSentinel(t, "stdout", stdout)
		assertNoSentinel(t, "stderr", stderr)
		f.noResidue()
	})

	t.Run("show: the terminal gate fires before sops runs, with no controlling terminal to satisfy it", func(t *testing.T) {
		f := newSopsFixtureWithEnvBridge(t, realSopsPlaintext, "0.1.45", true, true)
		for _, name := range []string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "AI_AGENT"} {
			old, wasSet := os.LookupEnv(name)
			_ = os.Unsetenv(name)
			t.Cleanup(func() {
				if wasSet {
					_ = os.Setenv(name, old)
				}
			})
		}

		code, stdout, stderr := f.run(f.keyFile, "config", "env", "show")
		if code != 1 {
			t.Errorf("exit = %d, want 1\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
		if !strings.Contains(stderr, "no controlling terminal") {
			t.Errorf("stderr does not name the terminal problem: %q", stderr)
		}
		if _, err := os.Lstat(f.file(".env")); err == nil {
			t.Error("a terminal-refused show wrote the plaintext target")
		}
		assertNoSentinel(t, "stdout", stdout)
		assertNoSentinel(t, "stderr", stderr)
		f.noResidue()
	})
}

// TestConfigEnvSealRealSOPSRoundTrip drives seal through the pinned sops
// binary the same way TestConfigEnvRealSOPS already drives unseal, and closes
// the gap TestConfigEnvGatedCommandsRealBinary's own doc comment names: with
// dvaBinary now built with config.Version=0.1.48 (see
// config_env_helpers_test.go's ldflags), a dva.yml can satisfy
// EnvBridgeVersionSatisfied and checkConfigVersion at once, so seal's real
// preflight, real confirmation skip and real sops encrypt can all run
// end to end instead of stopping at the version gate.
//
// The round trip is proven through unseal, not show. show's decrypted output
// is intentionally reachable only through bridgeOpenTTY's real
// os.OpenFile("/dev/tty", ...) call (see runEnvShow), which has no seam for
// an external process the way the in-process fake in config_env_show_test.go
// does — resolving it needs a real controlling terminal (a pty) attached to
// the child, which nothing in this harness currently allocates. Feeding
// seal's own output back through unseal still proves the property criterion 9
// is actually after: that a source seal produced with the real sops binary
// decrypts, with the real sops binary, back to the exact plaintext seal read.
func TestConfigEnvSealRealSOPSRoundTrip(t *testing.T) {
	f := newSealFixture(t, realSopsPlaintext, "0.1.48", true, true)

	code, _, stderr := f.run(f.keyFile, "config", "env", "seal", "--yes")
	if code != 0 {
		t.Fatalf("seal exit = %d, want 0\nstderr: %s", code, stderr)
	}
	assertNoSentinel(t, "stderr", stderr)

	enc, err := os.ReadFile(f.file("secrets.env.enc"))
	if err != nil {
		t.Fatalf("seal did not create the encrypted source: %v", err)
	}
	if strings.Contains(string(enc), realSopsSentinel) {
		t.Fatal("encrypted source holds the plaintext sentinel unencrypted")
	}

	// seal never deletes the plaintext target it read.
	if got, err := os.ReadFile(f.file(".env")); err != nil {
		t.Fatalf("plaintext target was removed: %v", err)
	} else if string(got) != realSopsPlaintext {
		t.Errorf("plaintext target = %q, want it left unchanged at %q", string(got), realSopsPlaintext)
	}
	f.noResidue()

	// The round trip: overwrite the plaintext with --force and confirm what
	// comes back byte-for-byte matches what seal encrypted, through the same
	// real sops binary seal itself used.
	code, stdout, stderr := f.run(f.keyFile, "config", "env", "unseal", "--force", "--json")
	if code != 0 {
		t.Fatalf("unseal exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	got, err := os.ReadFile(f.file(".env"))
	if err != nil {
		t.Fatalf("unseal did not write the target: %v", err)
	}
	if string(got) != realSopsPlaintext {
		t.Errorf("round-tripped plaintext = %q, want %q", string(got), realSopsPlaintext)
	}
	assertNoSentinel(t, "stdout", stdout)
	assertNoSentinel(t, "stderr", stderr)
	f.noResidue()
}

// errorCodeOf reads .error.code off the root envelope. TASK-245 §7-1 makes it an
// additive key on the document every failing command already emits, so a
// consumer switching on it never has to know which command failed.
func errorCodeOf(t *testing.T, stdout string) string {
	t.Helper()
	var doc struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not json: %v\n%s", err, stdout)
	}
	return doc.Error.Code
}

// assertNoSentinel is the one assertion repeated everywhere, success and failure
// alike: the plaintext reaches the target file and nowhere else. Real sops makes
// it stronger than the fake can — the bytes here were genuinely encrypted, so a
// leak would be a leak of something sops itself produced.
func assertNoSentinel(t *testing.T, stream, content string) {
	t.Helper()
	if strings.Contains(content, realSopsSentinel) {
		t.Errorf("decrypted plaintext surfaced on %s: %s", stream, content)
	}
}
