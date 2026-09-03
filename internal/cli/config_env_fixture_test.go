package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// Shared scaffolding for the TASK-246 acceptance suite.
//
// Everything in the suite drives the real preflight and the real safe writer.
// The only substitutions are the three seams the implementation exposes on
// purpose — bridgeSops, bridgeGit and bridgeGOOS — because the properties under
// test are about what DVA does around the child process, not about what sops or
// git compute. The pinned real-sops run lives in internal/integration so these
// fakes cannot drift from the binary without something failing.

const (
	// bridgeSecretKey/bridgeSecretValue is the plaintext that must never leave
	// the temporary descriptor. It is deliberately not a realistic-looking token:
	// a grep for it has to be unambiguous when it fires.
	bridgeSecretKey   = "DVA_BRIDGE_SENTINEL"
	bridgeSecretValue = "s3ntinel-9f2c4a-plaintext-must-never-surface"
)

func bridgePayload() string { return bridgeSecretKey + "=" + bridgeSecretValue + "\n" }

// --- seams -----------------------------------------------------------------

type fakeSops struct {
	available bool
	// decrypt defaults to writing the sentinel payload into out.
	decrypt func(source string, out *os.File) error
	edit    func(source string) error

	// mu guards calls. The concurrency test runs several commands at once
	// through one runner, so the recorder itself has to be safe — a fake that
	// races under -race would report on the harness instead of on the code.
	mu    sync.Mutex
	calls []string
}

func (f *fakeSops) record(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
}

// callsMade returns a copy so a caller can read it while writers are still
// running.
func (f *fakeSops) callsMade() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *fakeSops) Available() bool { return f.available }

func (f *fakeSops) Decrypt(source string, out *os.File) error {
	f.record("decrypt " + source)
	if f.decrypt != nil {
		return f.decrypt(source, out)
	}
	_, err := out.WriteString(bridgePayload())
	return err
}

func (f *fakeSops) Edit(source string) error {
	f.record("edit " + source)
	if f.edit != nil {
		return f.edit(source)
	}
	return nil
}

func okSops() *fakeSops { return &fakeSops{available: true} }

// gitAsk is one question the guard put to git.
//
// Recording the arguments is not bookkeeping: §5-4 refuses a target that git
// tracks or does not ignore, and that refusal is only worth anything if the
// question was asked about the directory the plaintext actually lands in. A fake
// that discarded its arguments made that unobservable, so a guard asking about
// `real/` while the bytes went to `decoy/` passed the suite (TASK-284 §1).
type gitAsk struct{ method, dir, target string }

type fakeGit struct {
	inside, available, tracked, ignored bool

	// mu guards asked. The concurrency test drives several commands at once
	// through one probe.
	mu    sync.Mutex
	asked []gitAsk
}

func (g *fakeGit) record(method, dir, target string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.asked = append(g.asked, gitAsk{method: method, dir: dir, target: target})
}

// askedAbout returns a copy so a caller can read it while writers still run.
func (g *fakeGit) askedAbout() []gitAsk {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]gitAsk(nil), g.asked...)
}

func (g *fakeGit) InsideRepo(dir string) bool {
	g.record("InsideRepo", dir, "")
	return g.inside
}

func (g *fakeGit) Available() bool { return g.available }

func (g *fakeGit) Tracked(dir, target string) bool {
	g.record("Tracked", dir, target)
	return g.tracked
}

func (g *fakeGit) Ignored(dir, target string) bool {
	g.record("Ignored", dir, target)
	return g.ignored
}

// okGit is the state a correctly configured project is in: inside a repository,
// git present, target ignored and untracked.
func okGit() *fakeGit { return &fakeGit{inside: true, available: true, ignored: true} }

// --- fixtures ---------------------------------------------------------------

type bridgeFixture struct {
	t    *testing.T
	dir  string
	cfg  *config.Config
	sops *fakeSops
	git  *fakeGit
}

// newBridgeFixture writes a config directory and loads it. Paths are resolved
// through EvalSymlinks so that assertions comparing a written location against an
// expected one are not defeated by /var -> /private/var on darwin.
func newBridgeFixture(t *testing.T, yaml string, files map[string]string) *bridgeFixture {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	writeFixtureFiles(t, dir, files)
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(dir)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return &bridgeFixture{t: t, dir: dir, cfg: c, sops: okSops(), git: okGit()}
}

func writeFixtureFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// simpleBridgeYAML is the one-encrypted-entry config almost every row uses.
func simpleBridgeYAML(target, source string) string {
	return fmt.Sprintf("version: \"0.1.45\"\nenv_file:\n  - {path: %s, sops_source: %s}\n", target, source)
}

// defaultFixture is the happy path: one encrypted entry, an encrypted source on
// disk, no target yet.
func defaultFixture(t *testing.T) *bridgeFixture {
	t.Helper()
	return newBridgeFixture(t, simpleBridgeYAML(".env", "secrets.env.enc"),
		map[string]string{"secrets.env.enc": "ENC"})
}

// install swaps the package globals for the duration of the test.
func (f *bridgeFixture) install(wantJSON bool) {
	f.t.Helper()
	oldSops, oldGit, oldGOOS := bridgeSops, bridgeGit, bridgeGOOS
	bridgeSops, bridgeGit = f.sops, f.git
	if !supportedBridgePlatforms[bridgeGOOS] {
		// The suite must run identically on a developer machine and in CI; a row
		// that is not specifically about platform support should never take the
		// unsupported branch because of where the test happens to run.
		bridgeGOOS = "linux"
	}
	withEnvPolicyGlobals(f.t, f.cfg, wantJSON)
	f.t.Cleanup(func() { bridgeSops, bridgeGit, bridgeGOOS = oldSops, oldGit, oldGOOS })
}

func (f *bridgeFixture) path(rel string) string { return filepath.Join(f.dir, rel) }

// anchorFor opens the config root and the target anchor the way preflight does,
// and closes both when the test ends. Tests that drive the safe writer directly
// need the pair, because a writer is created through the anchor and validated
// against the config root it was opened from.
func (f *bridgeFixture) anchorFor(declared string) *targetAnchor {
	f.t.Helper()
	root, err := openEnvRoot(f.dir)
	if err != nil {
		f.t.Fatalf("openEnvRoot: %v", err)
	}
	f.t.Cleanup(root.Close)
	a, err := root.openTargetAnchor(declared)
	if err != nil {
		f.t.Fatalf("openTargetAnchor(%s): %v", declared, err)
	}
	f.t.Cleanup(a.Close)
	return a
}

// readTarget reads the declared plaintext target. Every fixture in the suite
// declares the same one; a row that needs a different path reads it directly.
func (f *bridgeFixture) readTarget() string {
	f.t.Helper()
	b, err := os.ReadFile(f.path(".env"))
	if err != nil {
		f.t.Fatalf("read .env: %v", err)
	}
	return string(b)
}

// names lists the config directory.
func (f *bridgeFixture) names() []string {
	f.t.Helper()
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		f.t.Fatalf("ReadDir: %v", err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// assertNoTempResidue walks the whole fixture, not just its top level: since
// TASK-284 a temp is created beside its target, so a `sub/.env` entry leaves its
// residue in `sub` and a top-level scan would report success for a directory it
// never looked in.
func (f *bridgeFixture) assertNoTempResidue() {
	f.t.Helper()
	err := filepath.WalkDir(f.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && isOwnedTemp(d.Name()) {
			f.t.Errorf("temporary artifact left behind: %s", path)
		}
		return nil
	})
	if err != nil {
		f.t.Fatalf("walk %s: %v", f.dir, err)
	}
}

// ownedTempsIn lists the temporaries DVA recognizes as its own in one directory.
// A missing directory reads as empty, so a caller can ask about a location the
// implementation should never have created anything in.
func ownedTempsIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("ReadDir %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if isOwnedTemp(e.Name()) {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// requireCode asserts the exact frozen code. Matching the message instead would
// pin prose the contract explicitly allows to be reworded.
func requireCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %q, got nil", want)
	}
	if got := errorCode(err); got != want {
		t.Fatalf("code = %q, want %q (message: %v)", got, want, err)
	}
}
