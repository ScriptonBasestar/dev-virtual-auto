package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// The four named acceptance tests of TASK-246. Shared scaffolding lives in
// config_env_fixture_test.go, the fault matrix rows in config_env_faultrows_test.go,
// and the selector, argv and output tables in config_env_grammar_test.go.

// --- criterion 4: path and swap safety --------------------------------------

// TestConfigEnvRejectsPathSwap covers TASK-245 §5-3 and §8-2.
//
// The declaration-shape rows are the cheap half. The swap rows are the reason the
// command holds one os.Root handle from preflight through rename: os.Root blocks
// escapes but deliberately follows symlinks that stay inside the root, so
// containment alone would let a target be redirected between the check and the
// write. Each swap row mutates the filesystem from inside the decrypt callback —
// that is, after every preflight gate has passed — and then asserts where the
// bytes actually landed.
func TestConfigEnvRejectsPathSwap(t *testing.T) {
	t.Run("declaration shapes", func(t *testing.T) {
		symlinkFixture := func(link, dest string, mkdir bool) func(*testing.T, string) {
			return func(t *testing.T, dir string) {
				if mkdir {
					if err := os.MkdirAll(filepath.Join(dir, dest), 0o755); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(filepath.Join(dir, dest), filepath.Join(dir, link)); err != nil {
					t.Fatal(err)
				}
			}
		}
		for _, tt := range []struct {
			name           string
			target, source string
			setup          func(*testing.T, string)
			want           string
		}{
			{name: "absolute target", target: "/etc/passwd", source: "secrets.env.enc", want: codeAbsolutePath},
			{name: "absolute source", target: ".env", source: "/etc/shadow", want: codeAbsolutePath},
			{name: "target escapes the config root", target: "../outside.env", source: "secrets.env.enc", want: codePathEscapes},
			{name: "source escapes the config root", target: ".env", source: "../secrets.env.enc", want: codePathEscapes},
			{
				name: "target is itself a symlink", target: ".env", source: "secrets.env.enc",
				setup: symlinkFixture(".env", "real.env", false), want: codePathComponentSymlnk,
			},
			{
				name: "target parent is a symlink", target: "link/.env", source: "secrets.env.enc",
				setup: symlinkFixture("link", "real", true), want: codePathComponentSymlnk,
			},
			{
				name: "source is a symlink", target: ".env", source: "link.enc",
				setup: symlinkFixture("link.enc", "secrets.env.enc", false), want: codePathComponentSymlnk,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				f := newBridgeFixture(t, simpleBridgeYAML(tt.target, tt.source),
					map[string]string{"secrets.env.enc": "ENC"})
				if tt.setup != nil {
					tt.setup(t, f.dir)
				}
				f.install(false)

				requireCode(t, runEnvUnseal("", true), tt.want)
				if len(f.sops.callsMade()) != 0 {
					t.Errorf("sops was invoked for a path the preflight must refuse: %v", f.sops.callsMade())
				}
				f.assertNoTempResidue()
			})
		}
	})

	// The config directory is reached through a symlink that is repointed while
	// the child is "running". The rename is handle-relative, so it must land in
	// the directory the handle resolved at open time, not in whatever the name now
	// points at.
	t.Run("config directory symlink repointed mid-run", func(t *testing.T) {
		base, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		original, decoy, link := filepath.Join(base, "original"), filepath.Join(base, "decoy"), filepath.Join(base, "link")
		for _, d := range []string{original, decoy} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		writeFixtureFiles(t, original, map[string]string{
			config.FileName:   simpleBridgeYAML(".env", "secrets.env.enc"),
			"secrets.env.enc": "ENC",
		})
		if err := os.Symlink(original, link); err != nil {
			t.Fatal(err)
		}

		c, err := config.Load(link)
		if err != nil {
			t.Fatalf("config.Load: %v", err)
		}
		f := &bridgeFixture{t: t, dir: link, cfg: c, sops: okSops(), git: okGit()}
		f.sops.decrypt = func(_ string, out *os.File) error {
			// The swap: by the time this returns, `link` names a different
			// directory than the one preflight validated.
			if err := os.Remove(link); err != nil {
				return err
			}
			if err := os.Symlink(decoy, link); err != nil {
				return err
			}
			_, err := out.WriteString(bridgePayload())
			return err
		}
		f.install(false)

		captureStreams(t, func() {
			if err := runEnvUnseal("", false); err != nil {
				t.Errorf("unseal: %v", err)
			}
		})
		if _, err := os.Lstat(filepath.Join(original, ".env")); err != nil {
			t.Errorf("write did not land in the originally resolved directory: %v", err)
		}
		if _, err := os.Lstat(filepath.Join(decoy, ".env")); err == nil {
			t.Error("write followed the repointed symlink into the decoy directory")
		}
	})

	// The target itself is replaced by a symlink to a file outside the root after
	// preflight. rename(2) replaces the link, it does not follow it, so the
	// outside file must be untouched.
	t.Run("target replaced by an outward symlink mid-run", func(t *testing.T) {
		f := defaultFixture(t)
		outside := filepath.Join(t.TempDir(), "victim")
		const victimContent = "VICTIM=untouched\n"
		if err := os.WriteFile(outside, []byte(victimContent), 0o600); err != nil {
			t.Fatal(err)
		}
		f.sops.decrypt = func(_ string, out *os.File) error {
			if err := os.Symlink(outside, f.path(".env")); err != nil {
				return err
			}
			_, err := out.WriteString(bridgePayload())
			return err
		}
		f.install(false)

		captureStreams(t, func() {
			if err := runEnvUnseal("", false); err != nil {
				t.Errorf("unseal: %v", err)
			}
		})
		if got := f.readTarget(); got != bridgePayload() {
			t.Errorf(".env = %q, want the decrypted payload", got)
		}
		info, err := os.Lstat(f.path(".env"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			t.Error(".env is still a symlink; the rename followed it instead of replacing it")
		}
		b, err := os.ReadFile(outside)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != victimContent {
			t.Errorf("a file outside the config root was written through the symlink: %q", string(b))
		}
	})

	// The target's parent directory is swapped for another directory mid-run. The
	// handle refers to the original inode, so the write must follow the inode and
	// not the name — and whatever the outcome, it must never be a reported success
	// over a partial file.
	t.Run("target parent directory swapped mid-run", func(t *testing.T) {
		f := newBridgeFixture(t, simpleBridgeYAML("sub/.env", "secrets.env.enc"),
			map[string]string{"secrets.env.enc": "ENC", "sub/.keep": ""})
		decoy, originalSub, moved := f.path("decoy"), f.path("sub"), f.path("sub-moved")
		if err := os.MkdirAll(decoy, 0o755); err != nil {
			t.Fatal(err)
		}
		f.sops.decrypt = func(_ string, out *os.File) error {
			if err := os.Rename(originalSub, moved); err != nil {
				return err
			}
			if err := os.Rename(decoy, originalSub); err != nil {
				return err
			}
			_, err := out.WriteString(bridgePayload())
			return err
		}
		f.install(false)

		var err error
		captureStreams(t, func() { err = runEnvUnseal("", false) })
		if err != nil {
			// A refusal is an acceptable outcome; a silent wrong-directory write
			// or a truncated file is not.
			t.Logf("commit refused after the parent swap: %v", err)
		}
		for _, dir := range []string{originalSub, moved} {
			if b, readErr := os.ReadFile(filepath.Join(dir, ".env")); readErr == nil && string(b) != bridgePayload() {
				t.Errorf("%s/.env is not the complete payload: %q", dir, string(b))
			}
		}
	})
}

// --- criterion 5: the fault matrix ------------------------------------------

// TestConfigEnvAtomicWriteFaultMatrix walks TASK-245 §4-2 in order.
//
// Every row asserts three things at once: the exact frozen code, that an existing
// target is byte-for-byte what it was, and that no temporary artifact survives.
// Asserting them together is the point — a fault that produces the right code
// while leaving a half-written file is still a failure of this contract.
func TestConfigEnvAtomicWriteFaultMatrix(t *testing.T) {
	for _, tt := range unsealFaultRows() {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.build(t)
			f.install(false)
			if tt.goos != "" {
				bridgeGOOS = tt.goos
			}

			hadTarget := false
			if b, err := os.ReadFile(f.path(".env")); err == nil {
				hadTarget = string(b) == faultExistingTarget
			}

			var err error
			captureStreams(t, func() { err = runEnvUnseal("", tt.force) })

			if err == nil {
				t.Fatalf("expected a failure, got success")
			}
			if tt.want != "" {
				requireCode(t, err, tt.want)
			}
			if got := len(f.sops.callsMade()) > 0; got != tt.wantSopsCalled {
				t.Errorf("sops invoked = %v, want %v (calls: %v)", got, tt.wantSopsCalled, f.sops.callsMade())
			}
			if hadTarget {
				if got := f.readTarget(); got != faultExistingTarget {
					t.Errorf("existing target was modified by a failed run: %q", got)
				}
			}
			f.assertNoTempResidue()
		})
	}

	t.Run("unreadable source is blamed as a source problem", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root bypasses file permissions")
		}
		f := defaultFixture(t)
		if err := os.Chmod(f.path("secrets.env.enc"), 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(f.path("secrets.env.enc"), 0o600) })
		f.install(false)

		requireCode(t, runEnvUnseal("", false), codeSourceUnreadable)
		if len(f.sops.callsMade()) != 0 {
			t.Error("sops ran against a source DVA could not open")
		}
	})

	// Injected create fault: the temporary cannot be created at all.
	t.Run("an unwritable config directory is blamed as permission denied", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root bypasses file permissions")
		}
		f := defaultFixture(t)
		if err := os.Chmod(f.dir, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(f.dir, 0o755) })
		f.install(false)

		requireCode(t, runEnvUnseal("", false), codePermissionDenied)
	})

	// §8-5. Recovery is deliberately narrow, so this test's job is mostly to prove
	// what it does *not* remove: TASK-245 §8-4 refuses to claim SIGKILL or
	// power-loss cleanup, and an over-eager sweep would be a worse promise than no
	// promise at all.
	t.Run("owned stale temporaries are reclaimed and nothing else is", func(t *testing.T) {
		f := defaultFixture(t)
		old := time.Now().Add(-2 * staleTempAge)
		stale, fresh, staleDir := tempName(1234, "aaaa"), tempName(1234, "bbbb"), tempName(1234, "cccc")
		const foreign = "editor-backup.tmp"

		writeFixtureFiles(t, f.dir, map[string]string{stale: "x", fresh: "x", foreign: "x"})
		if err := os.MkdirAll(f.path(staleDir), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, n := range []string{stale, foreign, staleDir} {
			if err := os.Chtimes(f.path(n), old, old); err != nil {
				t.Fatal(err)
			}
		}
		f.install(false)

		captureStreams(t, func() {
			if err := runEnvUnseal("", false); err != nil {
				t.Errorf("unseal: %v", err)
			}
		})

		if _, err := os.Lstat(f.path(stale)); err == nil {
			t.Error("an owned, aged, regular temporary was not reclaimed")
		}
		for _, n := range []string{fresh, foreign, staleDir} {
			if _, err := os.Lstat(f.path(n)); err != nil {
				t.Errorf("recovery removed %s, which it does not own: %v", n, err)
			}
		}
	})
}

// --- criterion 6: concurrency and durability --------------------------------

// TestConfigEnvConcurrentWriters pins what atomic replacement does and does not
// promise.
//
// It does not promise mutual exclusion: two --force runs both complete and the
// later rename wins. What it promises is that no observer ever sees a partial
// file — every byte sequence the target holds is one writer's complete payload —
// and that a run which loses the race leaves nothing behind. Asserting the
// stronger property would be asserting a lock the contract never specified, and
// the honest reading of "serialize or one fails explicitly" is that no update is
// ever lost mid-file, not that no writer is ever overtaken.
func TestConfigEnvConcurrentWriters(t *testing.T) {
	t.Run("O_EXCL prevents two writers from sharing a temporary", func(t *testing.T) {
		f := defaultFixture(t)
		root, err := openEnvRoot(f.dir)
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()

		name := tempName(os.Getpid(), "fixed")
		w, err := root.newSafeWriter(name)
		if err != nil {
			t.Fatalf("first writer: %v", err)
		}
		defer w.Abort()

		if _, err := root.newSafeWriter(name); err == nil {
			t.Fatal("a second writer claimed a temporary that was already held")
		}
	})

	// The regression this pins: names used to be derived from time.Now().UnixNano(),
	// which darwin reports at microsecond resolution, so two writers starting in
	// the same microsecond produced the same name and the O_EXCL create failed with
	// a bare "file exists" carrying none of the frozen codes.
	t.Run("temporary names do not collide when generated back to back", func(t *testing.T) {
		seen := map[string]bool{}
		for range 10000 {
			token, err := randomToken()
			if err != nil {
				t.Fatal(err)
			}
			n := tempName(os.Getpid(), token)
			if seen[n] {
				t.Fatalf("temporary name collision: %s", n)
			}
			seen[n] = true
		}
	})

	t.Run("a taken temporary name is retried, not reported", func(t *testing.T) {
		f := defaultFixture(t)
		root, err := openEnvRoot(f.dir)
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()

		var writers []*safeWriter
		t.Cleanup(func() {
			for _, w := range writers {
				w.Abort()
			}
		})
		for i := range 16 {
			w, err := root.newTemp()
			if err != nil {
				t.Fatalf("newTemp %d: %v", i, err)
			}
			writers = append(writers, w)
		}
		names := map[string]bool{}
		for _, w := range writers {
			if names[w.name] {
				t.Fatalf("newTemp handed out %s twice", w.name)
			}
			names[w.name] = true
		}
	})

	t.Run("the target is never torn by concurrent replacement", func(t *testing.T) {
		const writers = 8
		f := defaultFixture(t)

		payloads := make([]string, writers)
		valid := map[string]bool{}
		for i := range payloads {
			// Long enough that a non-atomic write would interleave visibly.
			payloads[i] = fmt.Sprintf("%s=%s-%02d\n%s", bridgeSecretKey, bridgeSecretValue, i,
				strings.Repeat("PAD="+strings.Repeat("x", 120)+"\n", 40))
			valid[payloads[i]] = true
		}

		var mu sync.Mutex
		var next int
		f.sops.decrypt = func(_ string, out *os.File) error {
			mu.Lock()
			i := next
			next++
			mu.Unlock()
			_, err := out.WriteString(payloads[i%writers])
			return err
		}
		f.install(false)

		// A reader running alongside the writers: the assertion is about what an
		// observer can see, so something has to be observing.
		stop := make(chan struct{})
		torn := make(chan string, 1)
		var reader sync.WaitGroup
		reader.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				if b, err := os.ReadFile(f.path(".env")); err == nil && !valid[string(b)] {
					select {
					case torn <- string(b):
					default:
					}
					return
				}
			}
		})

		var wg sync.WaitGroup
		errs := make([]error, writers)
		captureStreams(t, func() {
			for i := range writers {
				wg.Go(func() {
					errs[i] = runEnvUnseal("", true)
				})
			}
			wg.Wait()
		})
		close(stop)
		reader.Wait()

		select {
		case s := <-torn:
			t.Errorf("observed a target that is not any writer's complete payload (%d bytes)", len(s))
		default:
		}
		for i, err := range errs {
			if err != nil {
				t.Errorf("writer %d: %v", i, err)
			}
		}
		if got := f.readTarget(); !valid[got] {
			t.Errorf("final target is not any single writer's complete payload (%d bytes)", len(got))
		}
		f.assertNoTempResidue()
	})

	t.Run("a failed run leaves an existing target byte-for-byte", func(t *testing.T) {
		f := existingTargetFixture(t)
		before, err := os.Stat(f.path(".env"))
		if err != nil {
			t.Fatal(err)
		}
		f.sops.decrypt = func(_ string, out *os.File) error {
			if _, err := out.WriteString("PARTIAL=hal"); err != nil {
				return err
			}
			return fmt.Errorf("exit status 128")
		}
		f.install(false)

		if err := runEnvUnseal("", true); err == nil {
			t.Fatal("expected a failure")
		}
		if got := f.readTarget(); got != faultExistingTarget {
			t.Errorf(".env = %q, want %q", got, faultExistingTarget)
		}
		after, err := os.Stat(f.path(".env"))
		if err != nil {
			t.Fatal(err)
		}
		if !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size() {
			t.Error("the existing target's metadata changed during a failed run")
		}
		f.assertNoTempResidue()
	})

	t.Run("a created target is not readable by other users", func(t *testing.T) {
		f := defaultFixture(t)
		f.install(false)
		captureStreams(t, func() {
			if err := runEnvUnseal("", false); err != nil {
				t.Fatalf("unseal: %v", err)
			}
		})
		info, err := os.Stat(f.path(".env"))
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			t.Errorf("target mode = %04o, want no group or other bits", perm)
		}
	})
}

// --- criterion 7: no secret ever surfaces -----------------------------------

// TestConfigEnvNeverEmitsSecretSentinel is the leak test.
//
// It is deliberately indiscriminate: the success path, every row of the fault
// matrix, both output modes, and then a sweep of stdout, stderr, the JSON failure
// envelope, the error string, and every filename in the config directory. The
// property is that the decrypted bytes travel from the sops child into the 0600
// temporary descriptor and are read back only by a validator that returns a count
// and a line number, so no code path could surface them — this test's job is to
// keep it that way as the code changes.
func TestConfigEnvNeverEmitsSecretSentinel(t *testing.T) {
	type mode struct {
		name  string
		build func(t *testing.T) *bridgeFixture
		force bool
		goos  string
	}
	modes := []mode{{name: "success", build: defaultFixture}}
	// Every failure the matrix defines, including the three that read the
	// decrypted bytes back: empty output, invalid dotenv, and a failed commit.
	for _, row := range unsealFaultRows() {
		modes = append(modes, mode{name: "failure/" + row.name, build: row.build, force: row.force, goos: row.goos})
	}

	for _, m := range modes {
		for _, wantJSON := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%v", m.name, wantJSON), func(t *testing.T) {
				f := m.build(t)
				f.install(wantJSON)
				if m.goos != "" {
					bridgeGOOS = m.goos
				}

				var err error
				stdout, stderr := captureStreams(t, func() { err = runEnvUnseal("", m.force) })

				// The JSON failure envelope is produced by Execute rather than by
				// the RunE, so it is rendered here the way the real path does.
				var envelope string
				if err != nil && wantJSON {
					envelope, _ = captureStreams(t, func() { emitFailureJSONFor(err) })
				}

				haystacks := map[string]string{"stdout": stdout, "stderr": stderr, "envelope": envelope}
				if err != nil {
					haystacks["error string"] = err.Error()
				}
				for _, name := range f.names() {
					haystacks["filename "+name] = name
				}
				for where, hay := range haystacks {
					if strings.Contains(hay, bridgeSecretValue) {
						t.Errorf("%s leaked the decrypted value: %q", where, hay)
					}
					if strings.Contains(hay, bridgeSecretKey) {
						t.Errorf("%s leaked a decrypted key name: %q", where, hay)
					}
				}
			})
		}
	}

	t.Run("a temporary left by a crash names nothing about its content", func(t *testing.T) {
		token, err := randomToken()
		if err != nil {
			t.Fatal(err)
		}
		name := tempName(4242, token)
		if strings.Contains(name, bridgeSecretKey) || strings.Contains(name, bridgeSecretValue) {
			t.Fatalf("temporary name derived from content: %s", name)
		}
		if !strings.HasPrefix(name, envTempPrefix) || !strings.HasSuffix(name, envTempSuffix) {
			t.Fatalf("temporary name %q is outside the owned namespace recovery recognizes", name)
		}
	})
}
