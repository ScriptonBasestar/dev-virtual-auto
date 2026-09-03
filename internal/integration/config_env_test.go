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
