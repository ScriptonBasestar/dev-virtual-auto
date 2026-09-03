package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfigEnvAnchorsWriteToTargetDirectory covers TASK-284: every write-side
// operation goes through a handle on the target's own directory, so no component
// of the declared path is re-resolved by name after preflight approved it.
func TestConfigEnvAnchorsWriteToTargetDirectory(t *testing.T) {
	subFixture := func(t *testing.T) *bridgeFixture {
		t.Helper()
		return newBridgeFixture(t, simpleBridgeYAML("sub/.env", "secrets.env.enc"),
			map[string]string{"secrets.env.enc": "ENC", "sub/.keep": ""})
	}

	// §1. The temp used to be created at the config root under a name with no
	// directory component, which is what handed `sub` back to the kernel to
	// re-resolve at rename time.
	t.Run("the temporary for a subdirectory target is created in that subdirectory", func(t *testing.T) {
		f := subFixture(t)
		var atRoot, inSub []string
		f.sops.decrypt = func(_ string, out *os.File) error {
			atRoot, inSub = ownedTempsIn(t, f.dir), ownedTempsIn(t, f.path("sub"))
			_, err := out.WriteString(bridgePayload())
			return err
		}
		f.install(false)

		captureStreams(t, func() {
			if err := runEnvUnseal("", false); err != nil {
				t.Fatalf("unseal: %v", err)
			}
		})
		if len(atRoot) != 0 {
			t.Errorf("temporary created at the config root: %v", atRoot)
		}
		if len(inSub) != 1 {
			t.Errorf("temporaries in the target's own directory = %v, want exactly one", inSub)
		}
		if b, err := os.ReadFile(f.path("sub/.env")); err != nil || string(b) != bridgePayload() {
			t.Errorf("sub/.env = %q, %v; want the decrypted payload", string(b), err)
		}
		f.assertNoTempResidue()
	})

	// §2. syncDir flushes through the anchor, so what it flushes is the directory
	// the rename created an entry in. Whether an fsync reached the platter is not
	// observable from a test; that the handle refers to `sub` and not to the
	// config root is, and it is the part that was wrong.
	t.Run("the anchor is the directory the rename lands in", func(t *testing.T) {
		f := subFixture(t)
		anchor := f.anchorFor("sub/.env")

		if want := f.path("sub"); anchor.dir.dir != want {
			t.Errorf("anchor directory = %q, want %q", anchor.dir.dir, want)
		}
		if anchor.leaf != ".env" {
			t.Errorf("anchor leaf = %q, want %q", anchor.leaf, ".env")
		}
		held, err := anchor.dir.root.Stat(".")
		if err != nil {
			t.Fatal(err)
		}
		onDisk, err := os.Stat(f.path("sub"))
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(held, onDisk) {
			t.Error("the anchor handle does not refer to the target's own directory")
		}
		if err := anchor.dir.syncDir(); err != nil {
			t.Errorf("syncDir through the anchor: %v", err)
		}
	})

	// §1, git half. The guard's verdict is only worth something if it was asked
	// about the directory the plaintext lands in; before this it was asked about a
	// path derived from the config root while the bytes could land elsewhere.
	t.Run("the git guard is asked about the directory the bytes land in", func(t *testing.T) {
		f := subFixture(t)
		f.install(false)

		captureStreams(t, func() {
			if err := runEnvUnseal("", false); err != nil {
				t.Fatalf("unseal: %v", err)
			}
		})
		asked := f.git.askedAbout()
		if len(asked) == 0 {
			t.Fatal("the git guard asked nothing")
		}
		landed := f.path("sub")
		for _, a := range asked {
			if a.dir != landed {
				t.Errorf("%s asked about %q, want the target's own directory %q", a.method, a.dir, landed)
			}
			if a.method != "InsideRepo" && a.target != ".env" {
				t.Errorf("%s asked about target %q, want %q", a.method, a.target, ".env")
			}
		}
	})

	// §3. A stray temp now sits beside its target and carries its name, so the
	// glob forms projects actually write for an env file cover it too. An exact
	// name rule still does not — the residue is bounded, not eliminated, which is
	// the same thing §8-4 already says about SIGKILL cleanup.
	t.Run("a temporary is covered by the ignore globs the target is", func(t *testing.T) {
		token, err := randomToken()
		if err != nil {
			t.Fatal(err)
		}
		name := tempName(".env", os.Getpid(), token)
		for _, pattern := range []string{".env*", ".env.*", "*.tmp"} {
			ok, err := filepath.Match(pattern, name)
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				t.Errorf("temporary %q is not covered by the ignore pattern %q", name, pattern)
			}
		}
	})

	// §5. The directory fsync after the rename is the one step that can fail once
	// the target has already been replaced. unseal's help text promises that "any
	// failure leaves an existing target byte-for-byte unchanged", so this failure
	// has to say plainly that it did not.
	t.Run("a post-rename failure does not claim the target is unchanged", func(t *testing.T) {
		err := error(&postRenameError{target: "sub/.env", err: errors.New("input/output error")})
		msg := err.Error()
		for _, want := range []string{"sub/.env", "was replaced", "input/output error"} {
			if !strings.Contains(msg, want) {
				t.Errorf("message %q does not mention %q", msg, want)
			}
		}
		if strings.Contains(msg, "unchanged") {
			t.Errorf("message %q claims the target is unchanged", msg)
		}

		// The reason runEnvUnseal has to test for this type before it tests for
		// a permission fault: the wrapped cause satisfies both, and the
		// permission mapping's whole meaning is "nothing was written".
		perm := error(&postRenameError{target: ".env", err: fs.ErrPermission})
		if !errors.Is(perm, fs.ErrPermission) {
			t.Fatal("premise gone: a post-rename failure no longer unwraps to its cause")
		}
		if _, ok := errors.AsType[*postRenameError](perm); !ok {
			t.Fatal("a post-rename failure is not recognizable as one")
		}
	})
}
