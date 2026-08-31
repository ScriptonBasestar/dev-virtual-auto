package main

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testCommit = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestPreflightPassesWithoutPublishing(t *testing.T) {
	dir := fakeCommands(t, `
case "$*" in
  "symbolic-ref -q HEAD") exit 1 ;;
  "status --porcelain") exit 0 ;;
  "cat-file -t refs/tags/v1.2.3") echo commit ;;
  "rev-list -n1 refs/tags/v1.2.3"|"rev-parse HEAD") echo aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa ;;
  "ls-remote --exit-code --tags origin refs/tags/v1.2.3") exit 2 ;;
  *) echo "unexpected git: $*" >&2; exit 9 ;;
esac`, `
case "$*" in
  "api --include repos/ScriptonBasestar/dva/releases/tags/v1.2.3") echo 'HTTP/2.0 404 Not Found' >&2; exit 1 ;;
  "api --method POST repos/ScriptonBasestar/dva/releases/generate-notes -f tag_name=v1.2.3 -f target_commitish=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") echo '{}' ;;
  *) echo "unexpected gh: $*" >&2; exit 9 ;;
esac`, `echo 'goreleaser version 2.12.7'`)
	notes, digest := fixtureNotes(t, dir)
	if err := preflight([]string{"--tag", "v1.2.3", "--commit", testCommit, "--release-notes", notes, "--release-notes-sha256", digest, "--mise-file", filepath.Join(dir, ".mise.toml"), "--cleanup-path", filepath.Join(dir, "gone")}); err != nil {
		t.Fatalf("preflight: %v", err)
	}
}

func TestPreflightRefusesBranchCheckoutBeforeRemoteProbe(t *testing.T) {
	dir := fakeCommands(t, `
case "$*" in "symbolic-ref -q HEAD") echo refs/heads/master ;; *) echo SHOULD_NOT_RUN >&2; exit 9;; esac`, "exit 9", "exit 9")
	notes, digest := fixtureNotes(t, dir)
	err := preflight([]string{"--tag", "v1.2.3", "--commit", testCommit, "--release-notes", notes, "--release-notes-sha256", digest, "--mise-file", filepath.Join(dir, ".mise.toml")})
	if err == nil || !strings.Contains(err.Error(), "branch checkout") || strings.Contains(err.Error(), "SHOULD_NOT_RUN") {
		t.Fatalf("error = %v", err)
	}
}

func TestPreflightRefusesUnknownDetachedState(t *testing.T) {
	dir := fakeCommands(t, `echo 'not a git repository' >&2; exit 128`, "exit 9", "exit 9")
	notes, digest := fixtureNotes(t, dir)
	err := preflight([]string{"--tag", "v1.2.3", "--commit", testCommit, "--release-notes", notes, "--release-notes-sha256", digest, "--mise-file", filepath.Join(dir, ".mise.toml")})
	if err == nil || !strings.Contains(err.Error(), "cannot determine whether HEAD is detached") {
		t.Fatalf("error = %v", err)
	}
}

func TestPreflightRefusesExistingRemoteTag(t *testing.T) {
	dir := fakeCommands(t, `
case "$*" in
 "symbolic-ref -q HEAD") exit 1;; "status --porcelain") ;; "cat-file -t refs/tags/v1.2.3") echo commit;;
 "rev-list -n1 refs/tags/v1.2.3"|"rev-parse HEAD") echo aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa;;
 "ls-remote --exit-code --tags origin refs/tags/v1.2.3") echo exists;; *) exit 9;; esac`, "exit 9", "echo 'goreleaser version 2.12.7'")
	notes, digest := fixtureNotes(t, dir)
	err := preflight([]string{"--tag", "v1.2.3", "--commit", testCommit, "--release-notes", notes, "--release-notes-sha256", digest, "--mise-file", filepath.Join(dir, ".mise.toml")})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v", err)
	}
}

func TestPreflightRefusesUnknownRemoteState(t *testing.T) {
	dir := fakeCommands(t, `
case "$*" in
 "symbolic-ref -q HEAD") exit 1;; "status --porcelain") ;; "cat-file -t refs/tags/v1.2.3") echo commit;;
 "rev-list -n1 refs/tags/v1.2.3"|"rev-parse HEAD") echo aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa;;
 "ls-remote --exit-code --tags origin refs/tags/v1.2.3") echo 'network unavailable' >&2; exit 128;; *) exit 9;; esac`, "exit 9", "echo 'goreleaser version 2.12.7'")
	notes, digest := fixtureNotes(t, dir)
	err := preflight([]string{"--tag", "v1.2.3", "--commit", testCommit, "--release-notes", notes, "--release-notes-sha256", digest, "--mise-file", filepath.Join(dir, ".mise.toml")})
	if err == nil || !strings.Contains(err.Error(), "cannot determine") {
		t.Fatalf("error = %v", err)
	}
}

func TestPreflightNeverLeaksCredentialValue(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "secret-value-must-not-escape")
	fakeCommands(t, "exit 9", "echo \"credential rejected: $GH_TOKEN\" >&2; exit 1", "exit 9")
	_, err := runGH("api", "user")
	if err == nil || strings.Contains(err.Error(), "secret-value-must-not-escape") {
		t.Fatalf("credential leaked in error: %v", err)
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("credential was not visibly redacted: %v", err)
	}
}

func TestReleaseAbsentNeverLeaksCredentialValue(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "secret-value-must-not-escape")
	fakeCommands(t, "exit 9", "echo \"credential rejected: $GH_TOKEN\" >&2; exit 1", "exit 9")
	err := releaseAbsent(defaultRepo, "v1.2.3")
	if err == nil || strings.Contains(err.Error(), "secret-value-must-not-escape") {
		t.Fatalf("credential leaked in error: %v", err)
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("credential was not visibly redacted: %v", err)
	}
}

func TestPreflightOverridesAmbientGHToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "wrong-ambient-token")
	t.Setenv("GH_HOST", "example.invalid")
	t.Setenv("GH_ENTERPRISE_TOKEN", "wrong-enterprise-token")
	dir := fakeCommands(t, `
case "$*" in
  "symbolic-ref -q HEAD") exit 1 ;;
  "status --porcelain") exit 0 ;;
  "cat-file -t refs/tags/v1.2.3") echo commit ;;
  "rev-list -n1 refs/tags/v1.2.3"|"rev-parse HEAD") echo aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa ;;
  "ls-remote --exit-code --tags origin refs/tags/v1.2.3") exit 2 ;;
  *) exit 9 ;;
esac`, `
test "$GH_TOKEN" = provided || { echo wrong-token >&2; exit 9; }
test "$GITHUB_TOKEN" = provided || { echo wrong-token >&2; exit 9; }
test "$GH_HOST" = github.com || { echo wrong-host >&2; exit 9; }
test -z "$GH_ENTERPRISE_TOKEN" || { echo enterprise-token-leaked >&2; exit 9; }
case "$*" in
  "api --include repos/ScriptonBasestar/dva/releases/tags/v1.2.3") echo 'HTTP/2.0 404 Not Found' >&2; exit 1 ;;
  "api --method POST repos/ScriptonBasestar/dva/releases/generate-notes -f tag_name=v1.2.3 -f target_commitish=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") echo '{}' ;;
  *) exit 9 ;;
esac`, `echo 'goreleaser version 2.12.7'`)
	notes, digest := fixtureNotes(t, dir)
	if err := preflight([]string{"--tag", "v1.2.3", "--commit", testCommit, "--release-notes", notes, "--release-notes-sha256", digest, "--mise-file", filepath.Join(dir, ".mise.toml")}); err != nil {
		t.Fatalf("preflight: %v", err)
	}
}

func TestPostflightRequiresFinalExactSevenAssetsAndCleanup(t *testing.T) {
	assetJSON := `{"tagName":"v1.2.3","targetCommitish":"` + testCommit + `","isDraft":false,"isPrerelease":false,"assets":[{"name":"checksums.txt","state":"uploaded","size":1},{"name":"dva_linux_amd64.tar.gz","state":"uploaded","size":1},{"name":"dva_linux_arm64.tar.gz","state":"uploaded","size":1},{"name":"dva_darwin_amd64.tar.gz","state":"uploaded","size":1},{"name":"dva_darwin_arm64.tar.gz","state":"uploaded","size":1},{"name":"dva_windows_amd64.zip","state":"uploaded","size":1},{"name":"dva_windows_arm64.zip","state":"uploaded","size":1}]}`
	dir := fakeCommands(t, `case "$*" in "ls-remote --tags origin refs/tags/v1.2.3 refs/tags/v1.2.3^{}") echo 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa refs/tags/v1.2.3';; *) exit 9;; esac`, `
case "$*" in
  "release view v1.2.3 --repo ScriptonBasestar/dva --json tagName,targetCommitish,isDraft,isPrerelease,assets") echo '`+assetJSON+`' ;;
  "release download v1.2.3"*)
    while [ "$#" -gt 0 ]; do [ "$1" = --dir ] && { shift; out=$1; }; shift; done
    cp "$RELEASE_ASSET_FIXTURE"/* "$out" ;;
  *) exit 9 ;;
esac`, "exit 9")
	fixture := filepath.Join(dir, "release-assets")
	writeReleaseAssets(t, fixture)
	t.Setenv("RELEASE_ASSET_FIXTURE", fixture)
	if err := postflight([]string{"--tag", "v1.2.3", "--commit", testCommit, "--cleanup-path", filepath.Join(dir, "gone")}); err != nil {
		t.Fatalf("postflight: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "leftover"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := postflight([]string{"--tag", "v1.2.3", "--commit", testCommit, "--cleanup-path", filepath.Join(dir, "leftover")}); err == nil || !strings.Contains(err.Error(), "cleanup path") {
		t.Fatalf("cleanup error = %v", err)
	}
}

func TestDownloadedChecksumsRejectCorruptAsset(t *testing.T) {
	dir := t.TempDir()
	writeReleaseAssets(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "dva_linux_amd64.tar.gz"), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyDownloadedChecksums(dir); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("checksum error = %v", err)
	}
}

func TestCleanRequiresRepositoryRootAndRefusesSymlink(t *testing.T) {
	dir := fakeCommands(t, `
case "$*" in
  "rev-parse --show-toplevel") printf '%s\n' "$CLEAN_REPOSITORY_ROOT" ;;
  *) exit 9 ;;
esac`, "exit 9", "exit 9")
	repo := filepath.Join(dir, "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLEAN_REPOSITORY_ROOT", repo)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	for _, name := range []string{"dist", "bin", "tmp"} {
		if err := os.Mkdir(filepath.Join(repo, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(repo, "bin")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(repo, "outside"), filepath.Join(repo, "bin")); err != nil {
		t.Fatal(err)
	}
	if err := clean(nil); err == nil || !strings.Contains(err.Error(), "expected a real directory") {
		t.Fatalf("symlink error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "dist")); err != nil {
		t.Fatalf("preflight deleted dist before refusing bin: %v", err)
	}

	otherRoot := filepath.Join(dir, "another-root")
	if err := os.Mkdir(otherRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLEAN_REPOSITORY_ROOT", otherRoot)
	if err := clean(nil); err == nil || !strings.Contains(err.Error(), "outside repository root") {
		t.Fatalf("root error = %v", err)
	}

	t.Setenv("CLEAN_REPOSITORY_ROOT", repo)
	if err := os.Remove(filepath.Join(repo, "bin")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := clean(nil); err != nil {
		t.Fatalf("clean: %v", err)
	}
	for _, name := range []string{"dist", "bin", "tmp"} {
		if _, err := os.Lstat(filepath.Join(repo, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("cleanup path %s still exists: %v", name, err)
		}
	}
}

func fakeCommands(t *testing.T, git, gh, goreleaser string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	git = `if [ "$*" = "remote get-url origin" ]; then echo git@github.com:ScriptonBasestar/dva.git; exit 0; fi
` + git
	for name, body := range map[string]string{"git": git, "gh": gh, "goreleaser": goreleaser, "go": "exit 0"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".mise.toml"), []byte("[tools]\ngoreleaser = \"2.12.7\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GITHUB_TOKEN", "provided")
	return dir
}

func fixtureNotes(t *testing.T, dir string) (string, string) {
	t.Helper()
	path := filepath.Join(dir, "notes.md")
	data := []byte("reviewed notes\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return path, fmtHex(sum[:])
}

func writeReleaseAssets(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var checksums strings.Builder
	for name := range assets {
		if name == "checksums.txt" {
			continue
		}
		data := []byte("release asset: " + name + "\n")
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		checksums.WriteString(fmtHex(sum[:]) + "  " + name + "\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(checksums.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}
func fmtHex(b []byte) string {
	const hex = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2], out[i*2+1] = hex[v>>4], hex[v&15]
	}
	return string(out)
}
