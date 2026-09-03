//go:build !windows

// seal is linux/darwin-only (supportedBridgePlatforms in config_env.go), and
// this file's confirmation-prompt tests need ttyPipe's syscall.Socketpair,
// which has no windows equivalent — see config_env_fixture_unix_test.go.

package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// sealFaultRow exercises one row of TASK-281 §3-3-1. Rows that reuse a shared
// preflight helper unchanged from unseal's own coverage (env_file origin
// writability's unsupported-origin branch, target selection, path shape,
// git tracked/ignored — seal explicitly skips the git checks, §3-3) are not
// repeated here; only one representative row (env origin unknown, row 6)
// stands in for that whole shared block, to pin that seal reaches it at all
// and in the right order. Every row genuinely new to seal — the gate, origin
// and version checks, create-only source handling, plaintext reading and key
// extraction, the .sops.yaml ancestor walk, confirmation and the encrypt
// step — has its own row.
type sealFaultRow struct {
	name  string
	build func(t *testing.T) *bridgeFixture
	goos  string
	// yes bypasses confirmation for every row that is not itself testing
	// confirmation (those live in TestConfigEnvSealConfirmation instead).
	yes      bool
	wantJSON bool
	want     string
	// wantEncryptCalled marks the rows that fail after sops ran.
	wantEncryptCalled bool
	// sourcePath is the declared sops_source to check for after the run.
	// Empty defaults to "secrets.env.enc"; "-" skips the check (used by the
	// source-is-target row, whose declared source is the target itself).
	sourcePath string
	// expectSourcePresent is true only for the create-only refusal, whose
	// whole point is that the pre-existing source is untouched.
	expectSourcePresent bool
}

func sealOriginContaminatedFixture(t *testing.T) *bridgeFixture {
	t.Helper()
	rootYAML := fmt.Sprintf(
		"version: %q\nmodules: [x]\nenv_bridge:\n  allow_seal: true\n  allow_show: false\nenv_file:\n  - {path: .env, sops_source: secrets.env.enc}\n",
		config.EnvBridgeIntroducedVersion)
	return newBridgeFixture(t, rootYAML, map[string]string{
		".sb/dva/x.yml": "env_bridge:\n  allow_seal: false\n  allow_show: false\n",
		".env":          bridgePayload(),
		".sops.yaml":    sopsCreationRuleYAML,
	})
}

func sealFaultRows() []sealFaultRow {
	return []sealFaultRow{
		{
			name: "gate off",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, simpleBridgeYAML(".env", "secrets.env.enc"),
					map[string]string{".env": bridgePayload()})
			},
			yes:  true,
			want: codeSealNotEnabled,
		},
		{
			name:     "--json is not supported",
			build:    defaultSealFixture,
			yes:      true,
			wantJSON: true,
			want:     codeJSONUnsupportedForSeal,
		},
		{
			name:  "unsupported platform fails closed before anything else",
			build: defaultSealFixture,
			goos:  "windows",
			yes:   true,
			want:  codeUnsupportedPlatform,
		},
		{
			name:  "env_bridge declared outside root contaminates the origin",
			build: sealOriginContaminatedFixture,
			yes:   true,
			want:  codeEnvBridgeOriginNotRoot,
		},
		{
			name: "env_bridge without a satisfying version",
			build: func(t *testing.T) *bridgeFixture {
				yaml := "version: \"0.1.45\"\nenv_bridge:\n  allow_seal: true\n  allow_show: false\n" +
					"env_file:\n  - {path: .env, sops_source: secrets.env.enc}\n"
				return newBridgeFixture(t, yaml, map[string]string{".env": bridgePayload(), ".sops.yaml": sopsCreationRuleYAML})
			},
			yes:  true,
			want: codeEnvBridgeRequiresVer,
		},
		{
			name: "no env_file declaration leaves no writable origin",
			build: func(t *testing.T) *bridgeFixture {
				yaml := fmt.Sprintf("version: %q\nenv_bridge:\n  allow_seal: true\n  allow_show: false\n", config.EnvBridgeIntroducedVersion)
				return newBridgeFixture(t, yaml, nil)
			},
			yes:  true,
			want: codeUnknownOrigin,
		},
		{
			name: "source equals target",
			build: func(t *testing.T) *bridgeFixture {
				yaml := fmt.Sprintf("version: %q\nenv_bridge:\n  allow_seal: true\n  allow_show: false\n"+
					"env_file:\n  - {path: .env, sops_source: ./.env}\n", config.EnvBridgeIntroducedVersion)
				return newBridgeFixture(t, yaml, map[string]string{".env": bridgePayload(), ".sops.yaml": sopsCreationRuleYAML})
			},
			yes:        true,
			want:       codeSourceIsTarget,
			sourcePath: "-",
		},
		{
			name: "source already exists — seal is create-only, no --force",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, bridgeEnabledYAML("secrets.env.enc", true, false),
					map[string]string{".env": bridgePayload(), "secrets.env.enc": "ENC-ALREADY", ".sops.yaml": sopsCreationRuleYAML})
			},
			yes:                 true,
			want:                codeSourceExists,
			expectSourcePresent: true,
		},
		{
			name: "source's parent directory is absent",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, bridgeEnabledYAML("nope/secrets.env.enc", true, false),
					map[string]string{".env": bridgePayload(), ".sops.yaml": sopsCreationRuleYAML})
			},
			yes:        true,
			want:       codeSourceParentMissing,
			sourcePath: "nope/secrets.env.enc",
		},
		{
			name: "plaintext target does not exist",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, bridgeEnabledYAML("secrets.env.enc", true, false),
					map[string]string{".sops.yaml": sopsCreationRuleYAML})
			},
			yes:  true,
			want: codeSealTargetMissing,
		},
		{
			name: "plaintext target is a directory",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, bridgeEnabledYAML("secrets.env.enc", true, false),
					map[string]string{".env/inner": "x", ".sops.yaml": sopsCreationRuleYAML})
			},
			yes:  true,
			want: codeTargetNotRegular,
		},
		{
			name: "plaintext target is not valid dotenv",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, bridgeEnabledYAML("secrets.env.enc", true, false),
					map[string]string{".env": "not a valid dotenv line\n", ".sops.yaml": sopsCreationRuleYAML})
			},
			yes:  true,
			want: codeInvalidDotenvInput,
		},
		{
			name: "plaintext target declares no assignments",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, bridgeEnabledYAML("secrets.env.enc", true, false),
					map[string]string{".env": "", ".sops.yaml": sopsCreationRuleYAML})
			},
			yes:  true,
			want: codeEmptyPlaintextInput,
		},
		{
			name: "no .sops.yaml is reachable from the source",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, bridgeEnabledYAML("secrets.env.enc", true, false),
					map[string]string{".env": bridgePayload()})
			},
			yes:  true,
			want: codeSopsCreationRuleMissing,
		},
		{
			name: "sops is not installed",
			build: func(t *testing.T) *bridgeFixture {
				f := defaultSealFixture(t)
				f.sops.available = false
				return f
			},
			yes:  true,
			want: codeSopsNotFound,
		},
		{
			name: "sops reports no matching creation rule",
			build: func(t *testing.T) *bridgeFixture {
				f := defaultSealFixture(t)
				f.sops.encrypt = func(string, *os.File, *os.File) error { return errSopsCreationRuleMismatch }
				return f
			},
			yes:               true,
			want:              codeSopsCreationRuleMissing,
			wantEncryptCalled: true,
		},
		{
			name: "sops encryption fails for another reason",
			build: func(t *testing.T) *bridgeFixture {
				f := defaultSealFixture(t)
				f.sops.encrypt = func(string, *os.File, *os.File) error {
					return fmt.Errorf("exit status 1 while handling " + bridgeSecretKey)
				}
				return f
			},
			yes:               true,
			want:              codeEncryptFailed,
			wantEncryptCalled: true,
		},
		{
			// TASK-281 §3-3-1 row 28: unlike unseal, seal collapses every
			// temp write/rename/fsync failure into permission_denied, with no
			// postRenameError distinction. This row forces the fault at
			// commit time, after a successful encrypt, to prove that
			// collapse rather than merely asserting it from the source.
			name: "the temp descriptor is closed under the writer before commit",
			build: func(t *testing.T) *bridgeFixture {
				f := defaultSealFixture(t)
				f.sops.encrypt = func(_ string, in, out *os.File) error {
					if _, err := io.Copy(out, in); err != nil {
						return err
					}
					return out.Close()
				}
				return f
			},
			yes:               true,
			want:              codePermissionDenied,
			wantEncryptCalled: true,
		},
	}
}

// TestConfigEnvSealFaultMatrix walks TASK-281 §3-3-1 in order (the subset
// described in sealFaultRow's doc comment). Every row asserts the frozen
// code, that sops ran exactly when the row means it to, that a source the
// row did not mean to create was not created, and that no temp artifact
// survives — the same three-part discipline TestConfigEnvAtomicWriteFaultMatrix
// applies to unseal.
func TestConfigEnvSealFaultMatrix(t *testing.T) {
	for _, tt := range sealFaultRows() {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.build(t)
			f.install(tt.wantJSON)
			if tt.goos != "" {
				bridgeGOOS = tt.goos
			}

			var err error
			captureStreams(t, func() { err = runEnvSeal("", tt.yes) })

			if err == nil {
				t.Fatalf("expected a failure, got success")
			}
			requireCode(t, err, tt.want)

			if got := len(f.sops.callsMade()) > 0; got != tt.wantEncryptCalled {
				t.Errorf("sops invoked = %v, want %v (calls: %v)", got, tt.wantEncryptCalled, f.sops.callsMade())
			}

			sp := tt.sourcePath
			if sp == "" {
				sp = "secrets.env.enc"
			}
			if sp != "-" {
				_, statErr := os.Stat(f.path(sp))
				exists := statErr == nil
				if exists != tt.expectSourcePresent {
					t.Errorf("source %s exists = %v, want %v", sp, exists, tt.expectSourcePresent)
				}
			}
			f.assertNoTempResidue()
		})
	}
}

// TestConfigEnvSealConfirmation covers rows 25-26: no controlling terminal
// without --yes refuses with confirmation_required and creates nothing; a
// controlling terminal that declines cancels without a code and creates
// nothing either.
func TestConfigEnvSealConfirmation(t *testing.T) {
	t.Run("no controlling terminal and no --yes", func(t *testing.T) {
		f := defaultSealFixture(t)
		f.install(false)

		var err error
		captureStreams(t, func() { err = runEnvSeal("", false) })

		requireCode(t, err, codeConfirmationRequired)
		if len(f.sops.callsMade()) != 0 {
			t.Errorf("sops was invoked before confirmation: %v", f.sops.callsMade())
		}
		if _, statErr := os.Stat(f.path("secrets.env.enc")); statErr == nil {
			t.Error("a refused seal created the source")
		}
	})

	t.Run("a controlling terminal present and the user declines", func(t *testing.T) {
		f := defaultSealFixture(t)
		f.install(false)
		dvaSide, testSide := ttyPipe(t)
		f.withTTY(dvaSide)

		errCh := make(chan error, 1)
		go func() {
			var err error
			captureStreams(t, func() { err = runEnvSeal("", false) })
			errCh <- err
		}()

		scanner := bufio.NewScanner(testSide)
		if !scanner.Scan() {
			t.Fatalf("read prompt: %v", scanner.Err())
		}
		prompt := scanner.Text()
		if !containsAll(prompt, "1 key", bridgeSecretKey) {
			t.Errorf("prompt = %q, want the key count and name", prompt)
		}
		if _, err := testSide.Write([]byte("n\n")); err != nil {
			t.Fatalf("write reply: %v", err)
		}

		err := <-errCh
		if err == nil {
			t.Fatal("expected the decline to fail the command")
		}
		if errorCode(err) != "" {
			t.Errorf("a declined confirmation carries a code (%q); §3-3-1 row 26 says it should not", errorCode(err))
		}
		if len(f.sops.callsMade()) != 0 {
			t.Errorf("sops was invoked after a decline: %v", f.sops.callsMade())
		}
		if _, statErr := os.Stat(f.path("secrets.env.enc")); statErr == nil {
			t.Error("a declined seal created the source")
		}
	})
}

// TestConfigEnvSealSuccess covers row 29: every check passes, the source is
// created, and — unlike unseal's target — the plaintext target is left in
// place; seal never deletes what it read.
func TestConfigEnvSealSuccess(t *testing.T) {
	f := defaultSealFixture(t)
	f.install(false)

	var err error
	stdout, _ := captureStreams(t, func() { err = runEnvSeal("", true) })
	if err != nil {
		t.Fatalf("runEnvSeal: %v", err)
	}
	if !containsAll(stdout, ".env", "secrets.env.enc") {
		t.Errorf("stdout = %q, want it to name the source and target", stdout)
	}

	b, statErr := os.ReadFile(f.path("secrets.env.enc"))
	if statErr != nil {
		t.Fatalf("read source: %v", statErr)
	}
	if string(b) != bridgePayload() {
		t.Errorf("source content = %q, want the fake encrypt's passthrough of the plaintext", string(b))
	}

	if _, statErr := os.Stat(f.path(".env")); statErr != nil {
		t.Errorf("plaintext target was removed: %v", statErr)
	}
	f.assertNoTempResidue()
}

func containsAll(haystack string, needles ...string) bool {
	for _, n := range needles {
		if !strings.Contains(haystack, n) {
			return false
		}
	}
	return true
}

// TestConfigEnvSealNeverEmitsSecretSentinel sweeps every fault row (run with
// --yes so no interactive prompt blocks it) plus the success path, across
// stdout, stderr, the error string and every filename in the config
// directory. Seal's confirmation prompt is the one place a key *name*
// legitimately appears — TestConfigEnvSealConfirmation already pins that —
// and this sweep does not touch tty content; the decrypted *value* must
// never appear anywhere else, on failure or success.
func TestConfigEnvSealNeverEmitsSecretSentinel(t *testing.T) {
	type mode struct {
		name  string
		build func(t *testing.T) *bridgeFixture
		goos  string
	}
	modes := []mode{{name: "success", build: defaultSealFixture}}
	for _, row := range sealFaultRows() {
		modes = append(modes, mode{name: "failure/" + row.name, build: row.build, goos: row.goos})
	}

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			f := m.build(t)
			f.install(false)
			if m.goos != "" {
				bridgeGOOS = m.goos
			}

			var err error
			stdout, stderr := captureStreams(t, func() { err = runEnvSeal("", true) })

			haystacks := map[string]string{"stdout": stdout, "stderr": stderr}
			if err != nil {
				haystacks["error string"] = err.Error()
			}
			for _, name := range f.names() {
				haystacks["filename "+name] = name
			}
			for where, hay := range haystacks {
				if strings.Contains(hay, bridgeSecretValue) {
					t.Errorf("%s leaked the plaintext value: %q", where, hay)
				}
			}
		})
	}
}
