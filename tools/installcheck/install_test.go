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

func TestInstallerOnlyTargetUsesVerifiedIsolatedDestinations(t *testing.T) {
	repo := repositoryRoot(t)
	fixture := t.TempDir()
	source := fixtureDVA(t, fixture)
	localDir := filepath.Join(fixture, "local bin")
	goDir := filepath.Join(fixture, "go bin")

	output := runInstaller(t, repo, source, "LOCAL_BIN_DIR="+localDir, "GO_BIN_DIR="+goDir)
	if !strings.Contains(output, "make install: verified local destination:") ||
		!strings.Contains(output, "make install: verified Go destination:") ||
		!strings.Contains(output, "make install: installed version evidence:") ||
		!strings.Contains(output, "commit: fixture") {
		t.Fatalf("install did not report both verified destinations and version evidence:\n%s", output)
	}

	local := filepath.Join(localDir, "dva")
	goBinary := filepath.Join(goDir, "dva")
	assertSameBinary(t, source, local)
	assertSameBinary(t, source, goBinary)
	assertSameVersion(t, source, local)
	assertSameVersion(t, source, goBinary)
}

func TestInstallerOnlyTargetRespectsIsolatedHOMEAndGOBIN(t *testing.T) {
	repo := repositoryRoot(t)
	fixture := t.TempDir()
	source := fixtureDVA(t, fixture)
	home := filepath.Join(fixture, "home")
	goDir := filepath.Join(fixture, "go", "bin")

	output := runInstaller(t, repo, source, "HOME="+home, "GOBIN="+goDir)
	if !strings.Contains(output, "make install: verified local destination:") ||
		!strings.Contains(output, "make install: verified Go destination:") {
		t.Fatalf("install did not verify the default destinations selected by HOME and GOBIN:\n%s", output)
	}
	assertSameBinary(t, source, filepath.Join(home, ".local", "bin", "dva"))
	assertSameBinary(t, source, filepath.Join(goDir, "dva"))
}

func TestInstallerOnlyTargetUsesFirstPOSIXGOPATHEntry(t *testing.T) {
	repo := repositoryRoot(t)
	fixture := t.TempDir()
	source := fixtureDVA(t, fixture)
	first := filepath.Join(fixture, "first")
	second := filepath.Join(fixture, "second")
	binDir := filepath.Join(fixture, "fake-bin")
	writeExecutable(t, filepath.Join(binDir, "go"), "#!/bin/sh\ncase \"$2\" in\n  GOBIN) printf '\\n' ;;\n  GOPATH) printf '%s\\n' \"$FAKE_GOPATH\" ;;\n  *) exit 31 ;;\nesac\n")
	localDir := filepath.Join(fixture, "local", "bin")

	runInstaller(t, repo, source,
		"LOCAL_BIN_DIR="+localDir,
		"GO_BIN_DIR=",
		"GOBIN=",
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"FAKE_GOPATH="+first+":"+second,
	)
	assertSameBinary(t, source, filepath.Join(first, "bin", "dva"))
	if _, err := os.Stat(filepath.Join(second, "bin", "dva")); !os.IsNotExist(err) {
		t.Fatalf("installer used a later GOPATH entry: stat error: %v", err)
	}
}

func TestInstallerOnlyTargetRejectsEmptyFirstPOSIXGOPATHEntry(t *testing.T) {
	repo := repositoryRoot(t)
	fixture := t.TempDir()
	source := fixtureDVA(t, fixture)
	second := filepath.Join(fixture, "second")
	binDir := filepath.Join(fixture, "fake-bin")
	writeExecutable(t, filepath.Join(binDir, "go"), "#!/bin/sh\ncase \"$2\" in\n  GOBIN) printf '\\n' ;;\n  GOPATH) printf '%s\\n' \"$FAKE_GOPATH\" ;;\n  *) exit 31 ;;\nesac\n")
	localDir := filepath.Join(fixture, "local", "bin")

	output, err := runInstallerResult(repo, source, nil,
		"LOCAL_BIN_DIR="+localDir,
		"GO_BIN_DIR=",
		"GOBIN=",
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"FAKE_GOPATH=:"+second,
	)
	if err == nil {
		t.Fatalf("install unexpectedly accepted an empty first GOPATH entry:\n%s", output)
	}
	if !strings.Contains(output, "go env GOPATH returned an empty first path") ||
		!strings.Contains(output, "replacement ledger: none; no destination was updated") {
		t.Fatalf("install did not report the empty first GOPATH entry truthfully:\n%s", output)
	}
}

func TestInstallerOnlyTargetHandlesSameResolvedDestinationOnce(t *testing.T) {
	repo := repositoryRoot(t)
	fixture := t.TempDir()
	source := fixtureDVA(t, fixture)
	destination := filepath.Join(fixture, "shared", "bin")

	output := runInstaller(t, repo, source, "LOCAL_BIN_DIR="+destination, "GO_BIN_DIR="+destination)
	if !strings.Contains(output, "local and Go destinations are the same path; installing once") {
		t.Fatalf("install did not report the shared destination path:\n%s", output)
	}
	if strings.Contains(output, "verified Go destination:") {
		t.Fatalf("install verified the same destination twice:\n%s", output)
	}
	assertSameBinary(t, source, filepath.Join(destination, "dva"))
}

func TestInstallerOnlyTargetRejectsTargetDirectoriesBeforeReplacement(t *testing.T) {
	repo := repositoryRoot(t)
	for _, targetShape := range []string{"directory", "symlink-to-directory"} {
		t.Run(targetShape, func(t *testing.T) {
			fixture := t.TempDir()
			source := fixtureDVA(t, fixture)
			localDir := filepath.Join(fixture, "local", "bin")
			goDir := filepath.Join(fixture, "go", "bin")
			localBinary := writeExistingBinary(t, localDir)
			goTarget := filepath.Join(goDir, "dva")
			if err := os.MkdirAll(goDir, 0o755); err != nil {
				t.Fatalf("create Go fixture: %v", err)
			}
			if targetShape == "directory" {
				if err := os.Mkdir(goTarget, 0o755); err != nil {
					t.Fatalf("create Go target directory: %v", err)
				}
			} else {
				targetDirectory := filepath.Join(fixture, "symlink target")
				if err := os.Mkdir(targetDirectory, 0o755); err != nil {
					t.Fatalf("create symlink target directory: %v", err)
				}
				if err := os.Symlink(targetDirectory, goTarget); err != nil {
					t.Fatalf("create symlink to target directory: %v", err)
				}
			}

			output, err := runInstallerResult(repo, source, nil, "LOCAL_BIN_DIR="+localDir, "GO_BIN_DIR="+goDir)
			if err == nil {
				t.Fatalf("install unexpectedly succeeded with Go destination %s:\n%s", targetShape, output)
			}
			if !strings.Contains(output, "refusing Go destination directory") {
				t.Fatalf("install did not reject the Go %s before staging:\n%s", targetShape, output)
			}
			assertExistingBinary(t, localBinary)
			assertNoStageArtifacts(t, localDir)
		})
	}
}

func TestInstallerOnlyTargetRemovesRegisteredStageOnCopyFailure(t *testing.T) {
	repo := repositoryRoot(t)
	fixture := t.TempDir()
	source := fixtureDVA(t, fixture)
	localDir := filepath.Join(fixture, "local", "bin")
	goDir := filepath.Join(fixture, "go", "bin")
	localBinary := writeExistingBinary(t, localDir)
	binDir := filepath.Join(fixture, "fake-bin")
	writeExecutable(t, filepath.Join(binDir, "cp"), "#!/bin/sh\nprintf '%s\\n' 'fixture cp failure' >&2\nexit 17\n")

	output, err := runInstallerResult(repo, source, []string{"PATH=" + binDir + ":" + os.Getenv("PATH")}, "LOCAL_BIN_DIR="+localDir, "GO_BIN_DIR="+goDir)
	if err == nil {
		t.Fatalf("install unexpectedly succeeded when cp fails:\n%s", output)
	}
	if !strings.Contains(output, "fixture cp failure") {
		t.Fatalf("install did not report copy failure:\n%s", output)
	}
	assertExistingBinary(t, localBinary)
	assertNoStageArtifacts(t, localDir)
	assertNoStageArtifacts(t, goDir)
}

func TestInstallerOnlyTargetPropagatesHasherFailure(t *testing.T) {
	repo := repositoryRoot(t)
	fixture := t.TempDir()
	source := fixtureDVA(t, fixture)
	localDir := filepath.Join(fixture, "local", "bin")
	goDir := filepath.Join(fixture, "go", "bin")
	localBinary := writeExistingBinary(t, localDir)
	binDir := filepath.Join(fixture, "fake-bin")
	writeExecutable(t, filepath.Join(binDir, "sha256sum"), "#!/bin/sh\nprintf '%s\\n' 'fixture hash failure' >&2\nexit 19\n")

	output, err := runInstallerResult(repo, source, []string{"PATH=" + binDir + ":" + os.Getenv("PATH")}, "LOCAL_BIN_DIR="+localDir, "GO_BIN_DIR="+goDir)
	if err == nil {
		t.Fatalf("install unexpectedly succeeded when sha256sum fails:\n%s", output)
	}
	if !strings.Contains(output, "fixture hash failure") || !strings.Contains(output, "cannot hash built binary") {
		t.Fatalf("install did not preserve the hasher failure:\n%s", output)
	}
	assertExistingBinary(t, localBinary)
	assertNoStageArtifacts(t, localDir)
}

func TestInstallerOnlyTargetReportsNoReplacementOnFirstRenameFailure(t *testing.T) {
	repo := repositoryRoot(t)
	fixture := t.TempDir()
	source := fixtureDVA(t, fixture)
	localDir := filepath.Join(fixture, "local", "bin")
	goDir := filepath.Join(fixture, "go", "bin")
	localTarget := writeExistingBinary(t, localDir)
	goTarget := writeExistingBinary(t, goDir)
	binDir := filepath.Join(fixture, "fake-bin")
	countFile := filepath.Join(fixture, "mv-count")
	realMV := writeFailingMV(t, binDir)

	output, err := runInstallerResult(repo, source, []string{
		"PATH=" + binDir + ":" + os.Getenv("PATH"),
		"REAL_MV=" + realMV,
		"MV_COUNT_FILE=" + countFile,
		"FAIL_MV_ON=1",
	}, "LOCAL_BIN_DIR="+localDir, "GO_BIN_DIR="+goDir)
	if err == nil {
		t.Fatalf("install unexpectedly succeeded when the first rename fails:\n%s", output)
	}
	if !strings.Contains(output, "fixture rename failure") ||
		!strings.Contains(output, "replacement ledger: none; no destination was updated") {
		t.Fatalf("install did not report the first rename truthfully:\n%s", output)
	}
	assertExistingBinary(t, localTarget)
	assertExistingBinary(t, goTarget)
	assertNoStageArtifacts(t, localDir)
	assertNoStageArtifacts(t, goDir)
}

func TestInstallerOnlyTargetRollsBackEarlierReplacementAfterLaterRenameFailure(t *testing.T) {
	repo := repositoryRoot(t)
	fixture := t.TempDir()
	source := fixtureDVA(t, fixture)
	localDir := filepath.Join(fixture, "local", "bin")
	goDir := filepath.Join(fixture, "go", "bin")
	localTarget := filepath.Join(localDir, "dva")
	writeExecutableMode(t, localTarget, "old local binary", 0o701)
	localBefore := readFile(t, localTarget)
	localMode := fileMode(t, localTarget)
	goTarget := writeExistingBinary(t, goDir)
	binDir := filepath.Join(fixture, "fake-bin")
	countFile := filepath.Join(fixture, "mv-count")
	realMV := writeFailingMV(t, binDir)

	output, err := runInstallerResult(repo, source, []string{
		"PATH=" + binDir + ":" + os.Getenv("PATH"),
		"REAL_MV=" + realMV,
		"MV_COUNT_FILE=" + countFile,
		"FAIL_MV_ON=2",
	}, "LOCAL_BIN_DIR="+localDir, "GO_BIN_DIR="+goDir)
	if err == nil {
		t.Fatalf("install unexpectedly succeeded when the second rename fails:\n%s", output)
	}
	if !strings.Contains(output, "fixture rename failure") ||
		!strings.Contains(output, "rollback succeeded") ||
		!strings.Contains(output, "completed replacement ledger:") ||
		!strings.Contains(output, "rollback ledger: restored local destination") {
		t.Fatalf("install did not report a successful rollback truthfully:\n%s", output)
	}
	assertFileBytesAndMode(t, localTarget, localBefore, localMode)
	assertExistingBinary(t, goTarget)
	assertNoInstallerArtifacts(t, localDir)
	assertNoInstallerArtifacts(t, goDir)
}

func TestInstallerOnlyTargetRemovesNewEarlierDestinationOnRollback(t *testing.T) {
	repo := repositoryRoot(t)
	fixture := t.TempDir()
	source := fixtureDVA(t, fixture)
	localDir := filepath.Join(fixture, "local", "bin")
	goDir := filepath.Join(fixture, "go", "bin")
	localTarget := filepath.Join(localDir, "dva")
	goTarget := writeExistingBinary(t, goDir)
	binDir := filepath.Join(fixture, "fake-bin")
	countFile := filepath.Join(fixture, "mv-count")
	realMV := writeFailingMV(t, binDir)

	output, err := runInstallerResult(repo, source, []string{
		"PATH=" + binDir + ":" + os.Getenv("PATH"),
		"REAL_MV=" + realMV,
		"MV_COUNT_FILE=" + countFile,
		"FAIL_MV_ON=2",
	}, "LOCAL_BIN_DIR="+localDir, "GO_BIN_DIR="+goDir)
	if err == nil {
		t.Fatalf("install unexpectedly succeeded when the second rename fails:\n%s", output)
	}
	if !strings.Contains(output, "rollback succeeded") ||
		!strings.Contains(output, "rollback ledger: removed newly created local destination") {
		t.Fatalf("install did not report removal of the newly created destination:\n%s", output)
	}
	if _, err := os.Stat(localTarget); !os.IsNotExist(err) {
		t.Fatalf("rollback left a newly created local destination: %v", err)
	}
	assertExistingBinary(t, goTarget)
	assertNoInstallerArtifacts(t, localDir)
	assertNoInstallerArtifacts(t, goDir)
}

func TestInstallerOnlyTargetReportsRollbackFailureAndCleansBackup(t *testing.T) {
	repo := repositoryRoot(t)
	fixture := t.TempDir()
	source := fixtureDVA(t, fixture)
	localDir := filepath.Join(fixture, "local", "bin")
	goDir := filepath.Join(fixture, "go", "bin")
	localTarget := writeExistingBinary(t, localDir)
	goTarget := writeExistingBinary(t, goDir)
	binDir := filepath.Join(fixture, "fake-bin")
	countFile := filepath.Join(fixture, "mv-count")
	realMV := writeFailingMV(t, binDir)

	output, err := runInstallerResult(repo, source, []string{
		"PATH=" + binDir + ":" + os.Getenv("PATH"),
		"REAL_MV=" + realMV,
		"MV_COUNT_FILE=" + countFile,
		"FAIL_MV_ON=2,3",
	}, "LOCAL_BIN_DIR="+localDir, "GO_BIN_DIR="+goDir)
	if err == nil {
		t.Fatalf("install unexpectedly succeeded when rollback rename fails:\n%s", output)
	}
	if !strings.Contains(output, "atomic replacement failed") ||
		!strings.Contains(output, "rollback failed") ||
		!strings.Contains(output, "rollback failed: cannot restore local destination") {
		t.Fatalf("install did not distinguish rollback failure:\n%s", output)
	}
	assertSameBinary(t, source, localTarget)
	assertExistingBinary(t, goTarget)
	assertNoInstallerArtifacts(t, localDir)
	assertNoInstallerArtifacts(t, goDir)
}

func TestInstallerOnlyTargetReportsCompletedLedgerAfterVerificationFailure(t *testing.T) {
	repo := repositoryRoot(t)
	fixture := t.TempDir()
	source := fixtureDVA(t, fixture)
	localDir := filepath.Join(fixture, "local", "bin")
	goDir := filepath.Join(fixture, "go", "bin")
	localTarget := writeExistingBinary(t, localDir)
	goTarget := writeExistingBinary(t, goDir)
	binDir := filepath.Join(fixture, "fake-bin")
	countFile := filepath.Join(fixture, "sha-count")
	realShasum, err := exec.LookPath("shasum")
	if err != nil {
		t.Fatalf("locate shasum: %v", err)
	}
	writeExecutable(t, filepath.Join(binDir, "sha256sum"), "#!/bin/sh\ncount=$(cat \"$SHA_COUNT_FILE\" 2>/dev/null || printf 0)\ncount=$((count + 1))\nprintf '%s' \"$count\" > \"$SHA_COUNT_FILE\"\nif [ \"$count\" -eq 4 ]; then\n  printf '%s\\n' 'fixture post-replacement hash failure' >&2\n  exit 37\nfi\nexec \"$REAL_SHASUM\" -a 256 \"$1\"\n")

	output, err := runInstallerResult(repo, source, []string{
		"PATH=" + binDir + ":" + os.Getenv("PATH"),
		"REAL_SHASUM=" + realShasum,
		"SHA_COUNT_FILE=" + countFile,
	}, "LOCAL_BIN_DIR="+localDir, "GO_BIN_DIR="+goDir)
	if err == nil {
		t.Fatalf("install unexpectedly succeeded when post-replacement verification fails:\n%s", output)
	}
	if !strings.Contains(output, "fixture post-replacement hash failure") ||
		!strings.Contains(output, "cannot hash local destination after replacement") ||
		!strings.Contains(output, "completed replacement ledger:") ||
		!strings.Contains(output, "local/bin/dva, ") {
		t.Fatalf("install did not report completed replacements after verification failure:\n%s", output)
	}
	assertSameBinary(t, source, localTarget)
	assertSameBinary(t, source, goTarget)
	assertNoStageArtifacts(t, localDir)
	assertNoStageArtifacts(t, goDir)
}

func TestInstallerOnlyTargetDoesNotWriteCheckout(t *testing.T) {
	repo := repositoryRoot(t)
	before := gitStatus(t, repo)
	fixture := t.TempDir()
	source := fixtureDVA(t, fixture)
	runInstaller(t, repo, source, "LOCAL_BIN_DIR="+filepath.Join(fixture, "local", "bin"), "GO_BIN_DIR="+filepath.Join(fixture, "go", "bin"))
	if after := gitStatus(t, repo); after != before {
		t.Fatalf("installer-only target changed the checkout:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestMakeHelpDoesNotResolveGoBin(t *testing.T) {
	repo := repositoryRoot(t)
	fixture := t.TempDir()
	marker := filepath.Join(fixture, "go-was-called")
	binDir := filepath.Join(fixture, "fake-bin")
	writeExecutable(t, filepath.Join(binDir, "go"), "#!/bin/sh\nprintf called > \""+marker+"\"\nexit 23\n")

	command := exec.Command("make", "-C", repo, "help")
	command.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"))
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("make help failed with a deliberately failing go command: %v\n%s", err, output)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("make help resolved GO_BIN_DIR by calling go; marker stat error: %v", err)
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

func fixtureDVA(t *testing.T, directory string) string {
	t.Helper()
	path := filepath.Join(directory, "fixture-dva")
	writeExecutable(t, path, "#!/bin/sh\nif [ \"${1:-}\" = version ]; then\n  printf '%s\\n' 'dva version fixture' 'commit: fixture' 'build date: fixture'\n  exit 0\nfi\nprintf '%s\\n' 'fixture supports only version' >&2\nexit 2\n")
	return path
}

func writeExecutable(t *testing.T, path, body string) {
	writeExecutableMode(t, path, body, 0o755)
}

func writeExecutableMode(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create executable directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatalf("write executable %q: %v", path, err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod executable %q: %v", path, err)
	}
}

func writeExistingBinary(t *testing.T, directory string) string {
	t.Helper()
	path := filepath.Join(directory, "dva")
	writeExecutable(t, path, "keep the working binary")
	return path
}

func writeFailingMV(t *testing.T, directory string) string {
	t.Helper()
	realMV, err := exec.LookPath("mv")
	if err != nil {
		t.Fatalf("locate real mv: %v", err)
	}
	writeExecutable(t, filepath.Join(directory, "mv"), "#!/bin/sh\ncount=$(cat \"$MV_COUNT_FILE\" 2>/dev/null || printf 0)\ncount=$((count + 1))\nprintf '%s' \"$count\" > \"$MV_COUNT_FILE\"\ncase \",$FAIL_MV_ON,\" in *\",$count,\"*)\n  printf '%s\\n' 'fixture rename failure' >&2\n  exit 29\n  ;;\nesac\nexec \"$REAL_MV\" \"$@\"\n")
	return realMV
}

func runInstaller(t *testing.T, repository, source string, arguments ...string) string {
	t.Helper()
	output, err := runInstallerResult(repository, source, nil, arguments...)
	if err != nil {
		t.Fatalf("make install-binary failed: %v\n%s", err, output)
	}
	return output
}

func runInstallerResult(repository, source string, environment []string, arguments ...string) (string, error) {
	arguments = append([]string{"-C", repository, "install-binary", "INSTALL_SOURCE=" + source}, arguments...)
	command := exec.Command("make", arguments...)
	command.Env = append(os.Environ(), environment...)
	output, err := command.CombinedOutput()
	return string(output), err
}

func assertSameBinary(t *testing.T, wantPath, gotPath string) {
	t.Helper()
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read fixture binary %q: %v", wantPath, err)
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

func assertExistingBinary(t *testing.T, path string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read existing binary after failed install: %v", err)
	}
	if string(got) != "keep the working binary" {
		t.Fatalf("failed install replaced existing binary: got %q", got)
	}
}

func assertNoStageArtifacts(t *testing.T, directory string) {
	assertNoInstallerArtifacts(t, directory)
}

func assertNoInstallerArtifacts(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatalf("read stage directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".dva-install.") || strings.HasPrefix(entry.Name(), ".dva-install-backup.") {
			t.Fatalf("installer artifact remained after failure: %s", filepath.Join(directory, entry.Name()))
		}
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	return contents
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	return info.Mode().Perm()
}

func assertFileBytesAndMode(t *testing.T, path string, want []byte, wantMode os.FileMode) {
	t.Helper()
	if got := readFile(t, path); string(got) != string(want) {
		t.Fatalf("restored bytes differ: got %q want %q", got, want)
	}
	if gotMode := fileMode(t, path); gotMode != wantMode {
		t.Fatalf("restored mode differs: got %04o want %04o", gotMode, wantMode)
	}
}

func gitStatus(t *testing.T, repository string) string {
	t.Helper()
	output, err := exec.Command("git", "-C", repository, "status", "--porcelain=v1", "--untracked-files=all").CombinedOutput()
	if err != nil {
		t.Fatalf("read checkout status: %v\n%s", err, output)
	}
	return string(output)
}

func versionOutput(t *testing.T, binary string) string {
	t.Helper()
	output, err := exec.Command(binary, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("run %q version: %v\n%s", binary, err, output)
	}
	return string(output)
}
