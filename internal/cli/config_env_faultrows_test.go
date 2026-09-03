package cli

import (
	"fmt"
	"os"
	"testing"
)

// The fault matrix table of TASK-245 §4-2, in the frozen check order.
//
// It lives beside the test rather than inside it because the same rows are
// replayed twice: once by TestConfigEnvAtomicWriteFaultMatrix, which asserts the
// code and the byte-for-byte survival of an existing target, and once by
// TestConfigEnvNeverEmitsSecretSentinel, which asserts that none of these
// outcomes can put decrypted bytes anywhere a user or a program would see them.
// One list keeps those two tests from covering different failure sets.

// faultExistingTarget is the content a row's pre-existing target holds. A row
// that creates one is asserting that a failure leaves it exactly this.
const faultExistingTarget = "PRE=existing-target-content\n"

type unsealFaultRow struct {
	name string
	// build returns a fixture already carrying whatever the row needs.
	build func(t *testing.T) *bridgeFixture
	force bool
	// goos overrides the platform for this row only.
	goos string
	// want is the frozen code, or "" when the row injects an operating-system
	// fault whose error is the kernel's and has no code of its own.
	want string
	// wantSopsCalled marks the rows that fail after the child ran.
	wantSopsCalled bool
}

// existingTargetFixture is the shape used by every row that has to prove a
// failure did not disturb a target that was already there.
func existingTargetFixture(t *testing.T) *bridgeFixture {
	t.Helper()
	return newBridgeFixture(t, simpleBridgeYAML(".env", "secrets.env.enc"),
		map[string]string{"secrets.env.enc": "ENC", ".env": faultExistingTarget})
}

func unsealFaultRows() []unsealFaultRow {
	return []unsealFaultRow{
		{
			name:  "unsupported platform fails closed before anything else",
			build: defaultFixture,
			goos:  "windows",
			want:  codeUnsupportedPlatform,
		},
		{
			name: "no env_file declaration leaves no writable origin",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, "version: \"0.1.45\"\n", nil)
			},
			want: codeUnknownOrigin,
		},
		{
			name: "no entry declares sops_source",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, "version: \"0.1.45\"\nenv_file: [.env]\n", nil)
			},
			want: codeNoEncryptedEntry,
		},
		{
			name: "several encrypted entries and no selector",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, twoEncryptedYAML,
					map[string]string{"a.enc": "ENC", "b.enc": "ENC"})
			},
			want: codeAmbiguousSelector,
		},
		{
			name: "source does not exist",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, simpleBridgeYAML(".env", "missing.enc"), nil)
			},
			want: codeSourceMissing,
		},
		{
			name: "source is a directory",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, simpleBridgeYAML(".env", "secrets.env.enc"),
					map[string]string{"secrets.env.enc/inner": "x"})
			},
			want: codeSourceNotRegular,
		},
		{
			name: "source and target are the same file after cleaning",
			build: func(t *testing.T) *bridgeFixture {
				// The config layer rejects an exact string match, so this is the
				// spelling that survives load and has to be caught at use time.
				return newBridgeFixture(t, simpleBridgeYAML(".env", "./.env"),
					map[string]string{".env": faultExistingTarget})
			},
			want: codeSourceIsTarget,
		},
		{
			name: "target's parent directory is absent",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, simpleBridgeYAML("nope/.env", "secrets.env.enc"),
					map[string]string{"secrets.env.enc": "ENC"})
			},
			want: codeTargetParentMissing,
		},
		{
			name: "target is a directory",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, simpleBridgeYAML(".env", "secrets.env.enc"),
					map[string]string{"secrets.env.enc": "ENC", ".env/inner": "x"})
			},
			force: true,
			want:  codeTargetNotRegular,
		},
		{
			name: "target is tracked by git",
			build: func(t *testing.T) *bridgeFixture {
				f := existingTargetFixture(t)
				f.git = fakeGit{inside: true, available: true, tracked: true}
				return f
			},
			force: true,
			want:  codeTargetTracked,
		},
		{
			name: "target is untracked but not ignored",
			build: func(t *testing.T) *bridgeFixture {
				f := defaultFixture(t)
				f.git = fakeGit{inside: true, available: true}
				return f
			},
			want: codeTargetNotIgnored,
		},
		{
			name: "inside a repository with no git binary",
			build: func(t *testing.T) *bridgeFixture {
				f := defaultFixture(t)
				f.git = fakeGit{inside: true}
				return f
			},
			want: codeGitUnavailable,
		},
		{
			name:  "existing target without --force",
			build: existingTargetFixture,
			want:  codeTargetExists,
		},
		{
			name: "sops is not installed",
			build: func(t *testing.T) *bridgeFixture {
				f := existingTargetFixture(t)
				f.sops.available = false
				return f
			},
			force: true,
			want:  codeSopsNotFound,
		},
		{
			name: "decryption fails after a partial write",
			build: func(t *testing.T) *bridgeFixture {
				f := existingTargetFixture(t)
				f.sops.decrypt = func(_ string, out *os.File) error {
					// The worst case for a writer that truncates in place
					// instead of replacing: half a payload, then a failure.
					if _, err := out.WriteString(bridgePayload()); err != nil {
						return err
					}
					return fmt.Errorf("exit status 128 while handling " + bridgeSecretKey)
				}
				return f
			},
			force:          true,
			want:           codeDecryptFailed,
			wantSopsCalled: true,
		},
		{
			name: "sops exits zero having written nothing",
			build: func(t *testing.T) *bridgeFixture {
				f := existingTargetFixture(t)
				f.sops.decrypt = func(string, *os.File) error { return nil }
				return f
			},
			force:          true,
			want:           codeEmptyOutput,
			wantSopsCalled: true,
		},
		{
			name: "decrypted output is not dotenv",
			build: func(t *testing.T) *bridgeFixture {
				f := existingTargetFixture(t)
				f.sops.decrypt = func(_ string, out *os.File) error {
					_, err := out.WriteString(bridgePayload() + bridgeSecretValue + " no equals here\n")
					return err
				}
				return f
			},
			force:          true,
			want:           codeInvalidDotenv,
			wantSopsCalled: true,
		},
		{
			name: "the temporary descriptor is closed under the writer before commit",
			build: func(t *testing.T) *bridgeFixture {
				f := existingTargetFixture(t)
				f.sops.decrypt = func(_ string, out *os.File) error {
					if _, err := out.WriteString(bridgePayload()); err != nil {
						return err
					}
					// Injected sync/close fault: everything downstream of the
					// write now fails on an already-closed descriptor.
					return out.Close()
				}
				return f
			},
			force:          true,
			wantSopsCalled: true,
		},
		{
			name: "a directory occupies the target name at commit time",
			build: func(t *testing.T) *bridgeFixture {
				f := defaultFixture(t)
				f.sops.decrypt = func(_ string, out *os.File) error {
					// Injected rename fault: the name is free during preflight
					// and occupied by a non-empty directory by commit time.
					if err := os.MkdirAll(f.path(".env/inner"), 0o755); err != nil {
						return err
					}
					_, err := out.WriteString(bridgePayload())
					return err
				}
				return f
			},
			wantSopsCalled: true,
		},
	}
}
