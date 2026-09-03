//go:build integration

package integration

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// Scaffolding for TestConfigEnvRealSOPS.
//
// Everything here exists to answer one question the unit suite cannot: is the
// argv DVA builds the argv sops accepts, and does DVA's own exit contract hold
// when the real binary exits with a code of its own. So nothing in this file
// fakes a child process — it resolves the real binaries, builds the real dva,
// and runs them.

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

var (
	dvaBinaryOnce sync.Once
	dvaBinaryPath string
	dvaBinaryErr  error
)

// dvaBinary builds cmd/dva once per test run.
//
// It compiles directly rather than through `make build`, which also regenerates
// checked-in files: a test must not rewrite the working tree it is being run
// against, and `make check-generate` would then be reporting on the test's edits
// instead of the author's.
func dvaBinary(t *testing.T) string {
	t.Helper()
	dvaBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "dva-bin-")
		if err != nil {
			dvaBinaryErr = err
			return
		}
		dvaBinaryPath = filepath.Join(dir, "dva")
		// EnvBridgeIntroducedVersion (0.1.48) is above config.Version's compiled
		// default (0.1.47 as of this writing), so any dva.yml that satisfies
		// EnvBridgeVersionSatisfied would otherwise fail checkConfigVersion
		// against this test binary before env_bridge is ever reached — the same
		// deadlock TestConfigEnvGatedCommandsRealBinary's doc comment describes.
		// Injecting a newer Version here, exactly the way the real release build
		// injects Commit/BuildDate, unblocks that combination for tests without
		// bumping the global compiled default or touching the Makefile's release
		// build.
		cmd := exec.Command("go", "build",
			"-ldflags", "-X github.com/ScriptonBasestar/dva/internal/config.Version=0.1.48",
			"-o", dvaBinaryPath, "./cmd/dva")
		cmd.Dir = repoRoot()
		if out, err := cmd.CombinedOutput(); err != nil {
			dvaBinaryErr = err
			t.Logf("go build: %s", out)
		}
	})
	if dvaBinaryErr != nil {
		t.Fatalf("building cmd/dva: %v", dvaBinaryErr)
	}
	return dvaBinaryPath
}

// realTool resolves name to an executable that works regardless of the working
// directory it is later run from.
//
// The indirection is not incidental. Version managers put shims on PATH that
// resolve the real binary from the *current* directory's configuration, and DVA
// runs sops with its own working directory — the config directory under test —
// where no such configuration exists. A shim would fail there for a reason that
// has nothing to do with what is being asserted, so the real path is resolved
// once, from the repository, and handed to the child directly.
func realTool(t *testing.T, name string) string {
	t.Helper()
	if mise, err := exec.LookPath("mise"); err == nil {
		cmd := exec.Command(mise, "which", name)
		cmd.Dir = repoRoot()
		if out, err := cmd.Output(); err == nil {
			if p := strings.TrimSpace(string(out)); p != "" {
				return p
			}
		}
	}
	p, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s is not installed; the pinned versions are in .mise.toml and in the "+
			"config-env-platform CI job, and this test only means anything against them", name)
	}
	return p
}

// toolPATH returns a PATH whose first entry contains only the resolved tools, so
// that DVA's own exec.LookPath finds a real binary from any working directory.
// The inherited PATH is kept after it: git and the rest still have to resolve.
func toolPATH(t *testing.T, tools map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, target := range tools {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	return dir + string(os.PathListSeparator) + os.Getenv("PATH")
}

// sopsFixture is a config directory holding a real age identity, a real
// sops-encrypted source, and a git repository that ignores the target — the
// state a correctly configured project is actually in.
type sopsFixture struct {
	t       *testing.T
	dir     string
	keyFile string
	path    string
	env     []string
}

const (
	realSopsSentinel = "s3ntinel-9f2c4a-plaintext-must-never-surface"
	// Deliberately awkward: an unquoted value truncating at " #" and a quoted one
	// keeping it is exactly where a dotenv round trip can silently differ, so the
	// fixture carries both.
	realSopsPlaintext = "DVA_BRIDGE_SENTINEL=" + realSopsSentinel + "\n" +
		"QUOTED=\"has spaces\"\n" +
		"HASHY=\"tok-123 # not a comment\"\n"
)

// newSopsFixture encrypts plaintext with a throwaway identity generated for this
// test only. No key material is committed: an identity that lives for the length
// of one t.TempDir cannot leak into the repository.
func newSopsFixture(t *testing.T, plaintext string) *sopsFixture {
	t.Helper()
	sops, keygen, git := realTool(t, "sops"), realTool(t, "age-keygen"), realTool(t, "git")

	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	keyFile := filepath.Join(dir, "age-identity.txt")

	out, err := exec.Command(keygen, "-o", keyFile).CombinedOutput()
	if err != nil {
		t.Fatalf("age-keygen: %v: %s", err, out)
	}
	pub := agePublicKey(t, string(out))

	plainFile := filepath.Join(dir, "plain.env")
	if err := os.WriteFile(plainFile, []byte(plaintext), 0o600); err != nil {
		t.Fatal(err)
	}
	enc, err := exec.Command(sops, "encrypt", "--input-type", "dotenv",
		"--output-type", "dotenv", "--age", pub, plainFile).Output()
	if err != nil {
		t.Fatalf("sops encrypt: %v", err)
	}
	if err := os.Remove(plainFile); err != nil {
		t.Fatal(err)
	}

	writeAll(t, dir, map[string]string{
		"secrets.env.enc": string(enc),
		"dva.yml": "version: \"0.1.45\"\nenv_file:\n" +
			"  - {path: .env, sops_source: secrets.env.enc}\n",
		".gitignore": ".env\nage-identity.txt\n",
	})
	initRepo(t, git, dir, "dva.yml", ".gitignore", "secrets.env.enc")

	return &sopsFixture{
		t: t, dir: dir, keyFile: keyFile,
		path: toolPATH(t, map[string]string{"sops": sops, "git": git}),
	}
}

// newSopsFixtureWithEnvBridge is newSopsFixture's counterpart for seal/show:
// the same real age identity, real sops-encrypted source and git repository,
// but dva.yml also declares env_bridge and a matching .sops.yaml creation
// rule. version is the config's own declared version: string, taken as a
// parameter rather than baked in — TestConfigEnvGatedCommandsRealBinary needs
// one that leaves EnvBridgeVersionSatisfied false without also tripping
// checkConfigVersion against the running dva's own compiled-in version.
func newSopsFixtureWithEnvBridge(t *testing.T, plaintext, version string, allowSeal, allowShow bool) *sopsFixture {
	t.Helper()
	sops, keygen, git := realTool(t, "sops"), realTool(t, "age-keygen"), realTool(t, "git")

	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	keyFile := filepath.Join(dir, "age-identity.txt")

	out, err := exec.Command(keygen, "-o", keyFile).CombinedOutput()
	if err != nil {
		t.Fatalf("age-keygen: %v: %s", err, out)
	}
	pub := agePublicKey(t, string(out))

	plainFile := filepath.Join(dir, "plain.env")
	if err := os.WriteFile(plainFile, []byte(plaintext), 0o600); err != nil {
		t.Fatal(err)
	}
	enc, err := exec.Command(sops, "encrypt", "--input-type", "dotenv",
		"--output-type", "dotenv", "--age", pub, plainFile).Output()
	if err != nil {
		t.Fatalf("sops encrypt: %v", err)
	}
	if err := os.Remove(plainFile); err != nil {
		t.Fatal(err)
	}

	writeAll(t, dir, map[string]string{
		"secrets.env.enc": string(enc),
		"dva.yml": fmt.Sprintf("version: %q\nenv_bridge:\n  allow_seal: %t\n  allow_show: %t\n"+
			"env_file:\n  - {path: .env, sops_source: secrets.env.enc}\n", version, allowSeal, allowShow),
		".sops.yaml": fmt.Sprintf("creation_rules:\n  - path_regex: \\.enc$\n    age: %s\n", pub),
		".gitignore": ".env\nage-identity.txt\n",
	})
	initRepo(t, git, dir, "dva.yml", ".gitignore", "secrets.env.enc")

	return &sopsFixture{
		t: t, dir: dir, keyFile: keyFile,
		path: toolPATH(t, map[string]string{"sops": sops, "git": git}),
	}
}

// newSealFixture is newSopsFixtureWithEnvBridge's counterpart for seal itself:
// seal is create-only (codeSourceExists, no --force — see config_env_seal.go),
// so its fixture must start with a plaintext target ready to encrypt and no
// source yet, the reverse of every other sopsFixture constructor here, which
// all start from an already-encrypted source. version is a parameter for the
// same reason newSopsFixtureWithEnvBridge takes one: the caller picks a value
// that satisfies EnvBridgeVersionSatisfied without also tripping
// checkConfigVersion against the running dva's own compiled-in version.
func newSealFixture(t *testing.T, plaintext, version string, allowSeal, allowShow bool) *sopsFixture {
	t.Helper()
	sops, keygen, git := realTool(t, "sops"), realTool(t, "age-keygen"), realTool(t, "git")

	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	keyFile := filepath.Join(dir, "age-identity.txt")

	out, err := exec.Command(keygen, "-o", keyFile).CombinedOutput()
	if err != nil {
		t.Fatalf("age-keygen: %v: %s", err, out)
	}
	pub := agePublicKey(t, string(out))

	writeAll(t, dir, map[string]string{
		".env": plaintext,
		"dva.yml": fmt.Sprintf("version: %q\nenv_bridge:\n  allow_seal: %t\n  allow_show: %t\n"+
			"env_file:\n  - {path: .env, sops_source: secrets.env.enc}\n", version, allowSeal, allowShow),
		".sops.yaml": fmt.Sprintf("creation_rules:\n  - path_regex: \\.enc$\n    age: %s\n", pub),
		".gitignore": ".env\nage-identity.txt\n",
	})
	initRepo(t, git, dir, "dva.yml", ".gitignore", ".sops.yaml")

	return &sopsFixture{
		t: t, dir: dir, keyFile: keyFile,
		path: toolPATH(t, map[string]string{"sops": sops, "git": git}),
	}
}

// agePublicKey pulls the recipient out of age-keygen's report. The private half
// is never read by this process; only sops ever opens the identity file.
func agePublicKey(t *testing.T, out string) string {
	t.Helper()
	for line := range strings.SplitSeq(out, "\n") {
		if _, rest, ok := strings.Cut(line, "Public key: "); ok {
			return strings.TrimSpace(rest)
		}
	}
	t.Fatalf("age-keygen reported no public key: %s", out)
	return ""
}

func writeAll(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// initRepo makes the fixture a real repository so the git guard is decided by
// git itself. The identity is set locally because a runner with no global
// git config would otherwise fail at commit time. tracked names the files to
// commit — callers whose source does not exist yet (seal's fixture) pass a
// set that omits it, since `git add` on a missing path is itself a fatal
// error, not a no-op.
func initRepo(t *testing.T, git, dir string, tracked ...string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"config", "user.email", "dva-test@example.invalid"},
		{"config", "user.name", "dva test"},
		append([]string{"add"}, tracked...),
		{"commit", "--quiet", "-m", "fixture"},
	} {
		cmd := exec.Command(git, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

// run executes dva in the fixture directory and returns its exit code with the
// two streams kept apart, because which stream a message lands on is part of
// what the contract fixes.
func (f *sopsFixture) run(keyFile string, args ...string) (code int, stdout, stderr string) {
	f.t.Helper()
	cmd := exec.Command(dvaBinary(f.t), args...)
	cmd.Dir = f.dir
	cmd.Env = append(os.Environ(),
		"PATH="+f.path,
		"SOPS_AGE_KEY_FILE="+keyFile,
		"EDITOR=true",
	)
	var outBuf, errBuf strings.Builder
	cmd.Stdout, cmd.Stderr = &outBuf, &errBuf
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			f.t.Fatalf("running dva: %v", err)
		}
		code = exitErr.ExitCode()
	}
	return code, outBuf.String(), errBuf.String()
}

func (f *sopsFixture) file(rel string) string { return filepath.Join(f.dir, rel) }

// noResidue asserts the invariant that outlives every individual outcome: a
// temporary file is either committed or removed, never left.
func (f *sopsFixture) noResidue() {
	f.t.Helper()
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		f.t.Fatal(err)
	}
	for _, e := range entries {
		// TASK-284 moved the marker from the front of the name to the middle:
		// a temp now carries its target's name first, so that a stray one falls
		// under the same ignore rule the target does.
		if strings.Contains(e.Name(), ".dva-env-") && strings.HasSuffix(e.Name(), ".tmp") {
			f.t.Errorf("temporary artifact left behind: %s", e.Name())
		}
	}
}
