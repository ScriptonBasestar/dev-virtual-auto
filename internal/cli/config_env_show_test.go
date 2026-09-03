//go:build !windows

// show is linux/darwin-only (supportedBridgePlatforms in config_env.go), and
// this file's terminal-gate tests need ttyPipe's syscall.Socketpair, which has
// no windows equivalent — see config_env_fixture_unix_test.go.

package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// showFaultRow exercises one row of TASK-281 §3-4-1. show's own checks (gate,
// --json, agent detection, controlling terminal) come before the shared
// preflight subset — the reverse of seal/unseal's ordering — so every row
// below that means to reach the shared subset needs withTTY set; otherwise
// the terminal gate would fail it first and the row would not be testing
// what its name says. Rows that reuse a shared preflight helper unchanged
// from seal/unseal's own coverage (env origin unknown, selector ambiguity,
// path shape) are represented by one row each, the same scoping seal's own
// fault-matrix test documents.
type showFaultRow struct {
	name              string
	build             func(t *testing.T) *bridgeFixture
	goos              string
	wantJSON          bool
	agentVar          string // set to simulate an agent-environment signal; empty means none
	withTTY           bool   // give a valid (but unread) tty so preflight can proceed past the terminal gate
	want              string
	wantDecryptCalled bool // only the rows that fail after sops ran
}

func showOriginContaminatedFixture(t *testing.T) *bridgeFixture {
	t.Helper()
	rootYAML := fmt.Sprintf(
		"version: %q\nmodules: [x]\nenv_bridge:\n  allow_seal: false\n  allow_show: true\nenv_file:\n  - {path: .env, sops_source: secrets.env.enc}\n",
		config.EnvBridgeIntroducedVersion)
	return newBridgeFixture(t, rootYAML, map[string]string{
		".sb/dva/x.yml":   "env_bridge:\n  allow_seal: false\n  allow_show: false\n",
		"secrets.env.enc": "ENC",
	})
}

func showFaultRows() []showFaultRow {
	return []showFaultRow{
		{
			name: "gate off",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, simpleBridgeYAML(".env", "secrets.env.enc"),
					map[string]string{"secrets.env.enc": "ENC"})
			},
			want: codeShowNotEnabled,
		},
		{
			name:     "--json is not supported",
			build:    defaultShowFixture,
			wantJSON: true,
			want:     codeJSONUnsupportedForShow,
		},
		{
			name:     "an agent environment is detected",
			build:    defaultShowFixture,
			agentVar: "CLAUDECODE",
			want:     codeAgentEnvironmentDetect,
		},
		{
			name:  "no controlling terminal",
			build: defaultShowFixture,
			want:  codeNoControllingTerminal,
		},
		{
			name:    "unsupported platform fails closed after the terminal gate",
			build:   defaultShowFixture,
			goos:    "windows",
			withTTY: true,
			want:    codeUnsupportedPlatform,
		},
		{
			name:    "env_bridge declared outside root contaminates the origin",
			build:   showOriginContaminatedFixture,
			withTTY: true,
			want:    codeEnvBridgeOriginNotRoot,
		},
		{
			name: "env_bridge without a satisfying version",
			build: func(t *testing.T) *bridgeFixture {
				yaml := "version: \"0.1.45\"\nenv_bridge:\n  allow_seal: false\n  allow_show: true\n" +
					"env_file:\n  - {path: .env, sops_source: secrets.env.enc}\n"
				return newBridgeFixture(t, yaml, map[string]string{"secrets.env.enc": "ENC"})
			},
			withTTY: true,
			want:    codeEnvBridgeRequiresVer,
		},
		{
			name: "no env_file declaration leaves no writable origin",
			build: func(t *testing.T) *bridgeFixture {
				yaml := fmt.Sprintf("version: %q\nenv_bridge:\n  allow_seal: false\n  allow_show: true\n", config.EnvBridgeIntroducedVersion)
				return newBridgeFixture(t, yaml, nil)
			},
			withTTY: true,
			want:    codeUnknownOrigin,
		},
		{
			name: "source equals target",
			build: func(t *testing.T) *bridgeFixture {
				yaml := fmt.Sprintf("version: %q\nenv_bridge:\n  allow_seal: false\n  allow_show: true\n"+
					"env_file:\n  - {path: .env, sops_source: ./.env}\n", config.EnvBridgeIntroducedVersion)
				return newBridgeFixture(t, yaml, map[string]string{".env": "ENC"})
			},
			withTTY: true,
			want:    codeSourceIsTarget,
		},
		{
			name: "encrypted source does not exist",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, bridgeEnabledYAML("secrets.env.enc", false, true), nil)
			},
			withTTY: true,
			want:    codeSourceMissing,
		},
		{
			name: "encrypted source is a directory",
			build: func(t *testing.T) *bridgeFixture {
				return newBridgeFixture(t, bridgeEnabledYAML("secrets.env.enc", false, true),
					map[string]string{"secrets.env.enc/inner": "x"})
			},
			withTTY: true,
			want:    codeSourceNotRegular,
		},
		{
			name: "sops is not installed",
			build: func(t *testing.T) *bridgeFixture {
				f := defaultShowFixture(t)
				f.sops.available = false
				return f
			},
			withTTY: true,
			want:    codeSopsNotFound,
		},
		{
			name: "sops decryption fails",
			build: func(t *testing.T) *bridgeFixture {
				f := defaultShowFixture(t)
				f.sops.decrypt = func(string, *os.File) error {
					return fmt.Errorf("exit status 1 while decrypting")
				}
				return f
			},
			withTTY:           true,
			want:              codeDecryptFailed,
			wantDecryptCalled: true,
		},
	}
}

// TestConfigEnvShowFaultMatrix walks TASK-281 §3-4-1 in order (the subset
// described in showFaultRow's doc comment). Every row asserts the frozen
// code, that sops decrypted exactly when the row means it to, and that no
// plaintext ever reaches stdout or stderr — show's whole point is that its
// output goes nowhere but the controlling terminal, so a leak on a failure
// row would be as serious as one on the success path.
func TestConfigEnvShowFaultMatrix(t *testing.T) {
	for _, tt := range showFaultRows() {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.build(t)
			f.install(tt.wantJSON)
			if tt.goos != "" {
				bridgeGOOS = tt.goos
			}
			if tt.agentVar != "" {
				old, wasSet := os.LookupEnv(tt.agentVar)
				_ = os.Setenv(tt.agentVar, "1")
				t.Cleanup(func() {
					if wasSet {
						_ = os.Setenv(tt.agentVar, old)
					} else {
						_ = os.Unsetenv(tt.agentVar)
					}
				})
			}
			if tt.withTTY {
				dvaSide, _ := ttyPipe(t)
				f.withTTY(dvaSide)
			}

			var err error
			stdout, stderr := captureStreams(t, func() { err = runEnvShow("") })

			if err == nil {
				t.Fatalf("expected a failure, got success")
			}
			requireCode(t, err, tt.want)

			if got := len(f.sops.callsMade()) > 0; got != tt.wantDecryptCalled {
				t.Errorf("sops invoked = %v, want %v (calls: %v)", got, tt.wantDecryptCalled, f.sops.callsMade())
			}
			if containsAll(stdout, bridgeSecretValue) || containsAll(stderr, bridgeSecretValue) {
				t.Errorf("plaintext leaked outside the tty: stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}

	t.Run("unreadable source is blamed as a source problem", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root bypasses file permissions")
		}
		f := defaultShowFixture(t)
		if err := os.Chmod(f.path("secrets.env.enc"), 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(f.path("secrets.env.enc"), 0o600) })
		f.install(false)
		dvaSide, _ := ttyPipe(t)
		f.withTTY(dvaSide)

		var err error
		captureStreams(t, func() { err = runEnvShow("") })
		requireCode(t, err, codeSourceUnreadable)
		if len(f.sops.callsMade()) != 0 {
			t.Error("sops ran against a source DVA could not open")
		}
	})
}

// TestConfigEnvShowSuccess covers §3-4-1's success row: the decrypted bytes
// land on the tty side of the controlling-terminal descriptor and nowhere
// else — not stdout, not stderr, not a --json document.
func TestConfigEnvShowSuccess(t *testing.T) {
	f := defaultShowFixture(t)
	f.install(false)
	dvaSide, testSide := ttyPipe(t)
	f.withTTY(dvaSide)

	var err error
	stdout, stderr := captureStreams(t, func() { err = runEnvShow("") })
	if err != nil {
		t.Fatalf("runEnvShow: %v", err)
	}

	want := bridgePayload()
	buf := make([]byte, len(want))
	if _, readErr := io.ReadFull(testSide, buf); readErr != nil {
		t.Fatalf("read tty: %v", readErr)
	}
	if string(buf) != want {
		t.Errorf("tty content = %q, want %q", string(buf), want)
	}

	if containsAll(stdout, bridgeSecretValue) {
		t.Errorf("plaintext leaked to stdout: %q", stdout)
	}
	if containsAll(stderr, bridgeSecretValue) {
		t.Errorf("plaintext leaked to stderr: %q", stderr)
	}
	if len(f.sops.callsMade()) != 1 {
		t.Errorf("sops calls = %v, want exactly one decrypt", f.sops.callsMade())
	}
}

// TestConfigEnvShowNeverEmitsSecretSentinel sweeps every fault row plus the
// success path, across stdout, stderr, the error string and every filename
// in the config directory. show has no confirmation prompt and no legitimate
// place for a key name or value to appear anywhere but the tty descriptor
// this sweep does not read from, so both the value and the key name are
// checked, unlike seal's sweep which allows the name through its prompt.
func TestConfigEnvShowNeverEmitsSecretSentinel(t *testing.T) {
	type mode struct {
		name     string
		build    func(t *testing.T) *bridgeFixture
		goos     string
		wantJSON bool
		agentVar string
		withTTY  bool
		success  bool
	}
	modes := []mode{{name: "success", build: defaultShowFixture, withTTY: true, success: true}}
	for _, row := range showFaultRows() {
		modes = append(modes, mode{
			name: "failure/" + row.name, build: row.build, goos: row.goos,
			wantJSON: row.wantJSON, agentVar: row.agentVar, withTTY: row.withTTY,
		})
	}

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			f := m.build(t)
			f.install(m.wantJSON)
			if m.goos != "" {
				bridgeGOOS = m.goos
			}
			if m.agentVar != "" {
				old, wasSet := os.LookupEnv(m.agentVar)
				_ = os.Setenv(m.agentVar, "1")
				t.Cleanup(func() {
					if wasSet {
						_ = os.Setenv(m.agentVar, old)
					} else {
						_ = os.Unsetenv(m.agentVar)
					}
				})
			}
			var testSide *os.File
			if m.withTTY {
				var dvaSide *os.File
				dvaSide, testSide = ttyPipe(t)
				f.withTTY(dvaSide)
			}

			var err error
			stdout, stderr := captureStreams(t, func() { err = runEnvShow("") })

			if m.success {
				if err != nil {
					t.Fatalf("runEnvShow: %v", err)
				}
				buf := make([]byte, len(bridgePayload()))
				if _, readErr := io.ReadFull(testSide, buf); readErr != nil {
					t.Fatalf("read tty: %v", readErr)
				}
			}

			haystacks := map[string]string{"stdout": stdout, "stderr": stderr}
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
