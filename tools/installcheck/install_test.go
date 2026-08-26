package installcheck

import (
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallUsesVerifiedIsolatedDestinations(t *testing.T) {
	repo := repositoryRoot(t)
	fixture := t.TempDir()
	localDir := filepath.Join(fixture, "local", "bin")
	goDir := filepath.Join(fixture, "go", "bin")

	output := runMake(t, repo, localDir, goDir)
	if !strings.Contains(output, "make install: verified local destination:") ||
		!strings.Contains(output, "make install: verified Go destination:") ||
		!strings.Contains(output, "make install: installed version evidence:") ||
		!strings.Contains(output, "commit:") {
		t.Fatalf("install did not report both verified destinations and version evidence:\n%s", output)
	}

	built := filepath.Join(repo, "bin", "dva")
	local := filepath.Join(localDir, "dva")
	goBinary := filepath.Join(goDir, "dva")
	assertSameBinary(t, built, local)
	assertSameBinary(t, built, goBinary)
	assertSameVersion(t, built, local)
	assertSameVersion(t, built, goBinary)
}

func TestInstallRespectsIsolatedHOMEAndGOBIN(t *testing.T) {
	repo := repositoryRoot(t)
	fixture := t.TempDir()
	home := filepath.Join(fixture, "home")
	goDir := filepath.Join(fixture, "go", "bin")

	output := runMakeArguments(t, repo, "HOME="+home, "GOBIN="+goDir)
	if !strings.Contains(output, "make install: verified local destination:") ||
		!strings.Contains(output, "make install: verified Go destination:") {
		t.Fatalf("install did not verify the default destinations selected by HOME and GOBIN:\n%s", output)
	}
	assertSameBinary(t, filepath.Join(repo, "bin", "dva"), filepath.Join(home, ".local", "bin", "dva"))
	assertSameBinary(t, filepath.Join(repo, "bin", "dva"), filepath.Join(goDir, "dva"))
}

func TestInstallHandlesSameResolvedDestinationOnce(t *testing.T) {
	repo := repositoryRoot(t)
	destination := filepath.Join(t.TempDir(), "shared", "bin")

	output := runMake(t, repo, destination, destination)
	if !strings.Contains(output, "local and Go destinations are the same path; installing once") {
		t.Fatalf("install did not report the shared destination path:\n%s", output)
	}
	if strings.Contains(output, "verified Go destination:") {
		t.Fatalf("install verified the same destination twice:\n%s", output)
	}
	assertSameBinary(t, filepath.Join(repo, "bin", "dva"), filepath.Join(destination, "dva"))
}

func TestInstallPreparesAllDestinationsBeforeReplacingEither(t *testing.T) {
	repo := repositoryRoot(t)
	fixture := t.TempDir()
	localDir := filepath.Join(fixture, "local", "bin")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("create local fixture: %v", err)
	}
	localBinary := filepath.Join(localDir, "dva")
	const original = "keep the working binary"
	if err := os.WriteFile(localBinary, []byte(original), 0o755); err != nil {
		t.Fatalf("write existing local binary: %v", err)
	}

	goDir := filepath.Join(fixture, "go", "bin")
	if err := os.MkdirAll(goDir, 0o755); err != nil {
		t.Fatalf("create Go fixture: %v", err)
	}
	if err := os.Chmod(goDir, 0o500); err != nil {
		t.Fatalf("make Go fixture unwritable: %v", err)
	}
	output, err := runMakeResult(repo, localDir, goDir)
	if err == nil {
		t.Fatalf("install unexpectedly succeeded with an unwritable Go destination:\n%s", output)
	}
	if !strings.Contains(output, "staged and verified") || !strings.Contains(output, "cannot stage candidate") {
		t.Fatalf("install did not report the second-destination staging failure:\n%s", output)
	}
	got, err := os.ReadFile(localBinary)
	if err != nil {
		t.Fatalf("read existing local binary after failed install: %v", err)
	}
	if string(got) != original {
		t.Fatalf("failed install replaced the existing local binary: got %q want %q", got, original)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "Makefile")); err != nil {
		t.Fatalf("resolve repository root %q: %v", root, err)
	}
	return root
}

func runMake(t *testing.T, repository, localDir, goDir string) string {
	t.Helper()
	output, err := runMakeResult(repository, localDir, goDir)
	if err != nil {
		t.Fatalf("make install failed: %v\n%s", err, output)
	}
	return output
}

func runMakeResult(repository, localDir, goDir string) (string, error) {
	return runMakeArgumentsResult(repository, "LOCAL_BIN_DIR="+localDir, "GO_BIN_DIR="+goDir)
}

func runMakeArguments(t *testing.T, repository string, arguments ...string) string {
	t.Helper()
	output, err := runMakeArgumentsResult(repository, arguments...)
	if err != nil {
		t.Fatalf("make install failed: %v\n%s", err, output)
	}
	return output
}

func runMakeArgumentsResult(repository string, arguments ...string) (string, error) {
	arguments = append([]string{"-C", repository, "install"}, arguments...)
	command := exec.Command("make", arguments...)
	output, err := command.CombinedOutput()
	return string(output), err
}

func assertSameBinary(t *testing.T, wantPath, gotPath string) {
	t.Helper()
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read built binary %q: %v", wantPath, err)
	}
	got, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("read installed binary %q: %v", gotPath, err)
	}
	wantSHA := sha256.Sum256(want)
	gotSHA := sha256.Sum256(got)
	if wantSHA != gotSHA {
		t.Fatalf("installed binary SHA-256 differs: got %x want %x", gotSHA, wantSHA)
	}
}

func assertSameVersion(t *testing.T, wantPath, gotPath string) {
	t.Helper()
	want := versionOutput(t, wantPath)
	got := versionOutput(t, gotPath)
	if got != want {
		t.Fatalf("installed binary version differs:\ninstalled:\n%s\nbuilt:\n%s", got, want)
	}
}

func versionOutput(t *testing.T, binary string) string {
	t.Helper()
	output, err := exec.Command(binary, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("run %q version: %v\n%s", binary, err, output)
	}
	return string(output)
}
