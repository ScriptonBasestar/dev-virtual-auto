// releaseworkflow validates the manual release boundary without publishing or deleting remote state.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const defaultRepo = "ScriptonBasestar/dva"

var (
	fullSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)
	assets  = map[string]bool{
		"checksums.txt":          true,
		"dva_linux_amd64.tar.gz": true, "dva_linux_arm64.tar.gz": true,
		"dva_darwin_amd64.tar.gz": true, "dva_darwin_arm64.tar.gz": true,
		"dva_windows_amd64.zip": true, "dva_windows_arm64.zip": true,
	}
)

func main() {
	if len(os.Args) < 2 {
		fail(errors.New("usage: releaseworkflow <preflight|postflight|clean> [flags]"))
	}
	var err error
	switch os.Args[1] {
	case "preflight":
		err = preflight(os.Args[2:])
	case "postflight":
		err = postflight(os.Args[2:])
	case "clean":
		err = clean(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q (want preflight, postflight, or clean)", os.Args[1])
	}
	fail(err)
}

type common struct {
	tag, commit, repo string
	cleanupPaths      []string
}

func commonFlags(f *flag.FlagSet) *common {
	c := &common{repo: defaultRepo}
	f.StringVar(&c.tag, "tag", "", "exact v-prefixed release tag")
	f.StringVar(&c.commit, "commit", "", "exact 40-hex release commit")
	f.Var((*stringList)(&c.cleanupPaths), "cleanup-path", "path that must not exist after the check (repeatable)")
	return c
}

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func validateCommon(c *common) error {
	if !strings.HasPrefix(c.tag, "v") || len(c.tag) == 1 {
		return fmt.Errorf("--tag %q must be v-prefixed", c.tag)
	}
	if !fullSHA.MatchString(c.commit) {
		return fmt.Errorf("--commit %q is not a full 40-hex SHA", c.commit)
	}
	if strings.Count(c.repo, "/") != 1 || strings.HasPrefix(c.repo, "/") || strings.HasSuffix(c.repo, "/") {
		return fmt.Errorf("--repo %q must be owner/repository", c.repo)
	}
	return nil
}

func preflight(args []string) error {
	f := flag.NewFlagSet("preflight", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	c := commonFlags(f)
	notes := f.String("release-notes", "", "reviewed release notes file")
	notesSHA := f.String("release-notes-sha256", "", "expected SHA-256 of release notes")
	miseFile := f.String("mise-file", ".mise.toml", "mise tool pin file")
	if err := f.Parse(args); err != nil {
		return err
	}
	if err := validateCommon(c); err != nil {
		return err
	}
	if *notes == "" || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(*notesSHA) {
		return errors.New("--release-notes and lowercase 64-hex --release-notes-sha256 are required")
	}
	if err := requireCredential(); err != nil {
		return err
	}
	if err := checkDetachedClean(); err != nil {
		return err
	}
	if err := checkOrigin(); err != nil {
		return err
	}
	if err := checkLocalTag(c.tag, c.commit); err != nil {
		return err
	}
	if err := checkVersion(c.tag); err != nil {
		return err
	}
	if err := checkNotes(*notes, *notesSHA); err != nil {
		return err
	}
	if err := checkGoReleaser(*miseFile); err != nil {
		return err
	}
	if err := remoteTagAbsent(c.tag); err != nil {
		return err
	}
	if err := releaseAbsent(c.repo, c.tag); err != nil {
		return err
	}
	if err := capabilityProbe(c.repo, c.tag, c.commit); err != nil {
		return err
	}
	if err := checkCleanup(c.cleanupPaths); err != nil {
		return err
	}
	fmt.Printf("releaseworkflow: preflight passed for %s at %s; no remote state was created\n", c.tag, c.commit)
	return nil
}

func postflight(args []string) error {
	f := flag.NewFlagSet("postflight", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	c := commonFlags(f)
	if err := f.Parse(args); err != nil {
		return err
	}
	if err := validateCommon(c); err != nil {
		return err
	}
	if err := requireCredential(); err != nil {
		return err
	}
	if err := checkOrigin(); err != nil {
		return err
	}
	if err := remoteTagTarget(c.tag, c.commit); err != nil {
		return err
	}
	if err := finalRelease(c.repo, c.tag, c.commit); err != nil {
		return err
	}
	if err := checkCleanup(c.cleanupPaths); err != nil {
		return err
	}
	fmt.Printf("releaseworkflow: postflight passed for %s at %s with the exact seven assets\n", c.tag, c.commit)
	return nil
}

func run(name string, args ...string) ([]byte, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func runStatus(name string, args ...string) ([]byte, int, error) {
	return commandStatus(exec.Command(name, args...))
}

func runGH(args ...string) ([]byte, error) {
	out, code, err := runGHStatus(args...)
	if err != nil {
		return out, err
	}
	if code != 0 {
		return out, fmt.Errorf("gh %s: exit status %d: %s", strings.Join(args, " "), code, redactCredential(strings.TrimSpace(string(out))))
	}
	return out, nil
}

func redactCredential(s string) string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return strings.ReplaceAll(s, token, "[REDACTED]")
	}
	return s
}

func runGHStatus(args ...string) ([]byte, int, error) {
	cmd := exec.Command("gh", args...)
	env := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GH_TOKEN=") &&
			!strings.HasPrefix(entry, "GITHUB_TOKEN=") &&
			!strings.HasPrefix(entry, "GH_HOST=") &&
			!strings.HasPrefix(entry, "GH_ENTERPRISE_TOKEN=") &&
			!strings.HasPrefix(entry, "GITHUB_ENTERPRISE_TOKEN=") {
			env = append(env, entry)
		}
	}
	token := os.Getenv("GITHUB_TOKEN")
	cmd.Env = append(env, "GH_HOST=github.com", "GH_TOKEN="+token, "GITHUB_TOKEN="+token)
	return commandStatus(cmd)
}

func commandStatus(cmd *exec.Cmd) ([]byte, int, error) {
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return out, exitErr.ExitCode(), nil
	}
	return out, -1, fmt.Errorf("start %s: %w", cmd.Path, err)
}

func requireCredential() error {
	if os.Getenv("GITHUB_TOKEN") == "" {
		return errors.New("caller must provide a non-empty command-scoped GITHUB_TOKEN; its value is never printed")
	}
	return nil
}

func checkDetachedClean() error {
	out, code, err := runStatus("git", "symbolic-ref", "-q", "HEAD")
	if err != nil {
		return err
	}
	if code == 0 {
		return errors.New("refusing a branch checkout; create a clean detached worktree at the release tag")
	}
	if code != 1 {
		return fmt.Errorf("cannot determine whether HEAD is detached (git symbolic-ref exit %d: %s)", code, strings.TrimSpace(string(out)))
	}
	out, err = run("git", "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(out)) != "" {
		return errors.New("worktree is not clean")
	}
	return nil
}

func checkLocalTag(tag, commit string) error {
	ref := "refs/tags/" + tag
	kind, err := run("git", "cat-file", "-t", ref)
	if err != nil {
		return fmt.Errorf("local tag %s: %w", tag, err)
	}
	if strings.TrimSpace(string(kind)) != "commit" {
		return fmt.Errorf("local tag %s must resolve directly to a commit", tag)
	}
	got, err := run("git", "rev-list", "-n1", ref)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(got)) != commit {
		return fmt.Errorf("local tag %s = %s, want %s", tag, strings.TrimSpace(string(got)), commit)
	}
	head, err := run("git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(head)) != commit {
		return fmt.Errorf("HEAD = %s, want release commit %s", strings.TrimSpace(string(head)), commit)
	}
	return nil
}

func checkOrigin() error {
	out, err := run("git", "remote", "get-url", "origin")
	if err != nil {
		return err
	}
	got := strings.TrimSpace(string(out))
	switch got {
	case "git@github.com:ScriptonBasestar/dva.git", "https://github.com/ScriptonBasestar/dva.git", "ssh://git@github.com/ScriptonBasestar/dva.git":
		return nil
	default:
		return fmt.Errorf("origin %q is not the fixed publication repository %s", got, defaultRepo)
	}
}

func checkVersion(tag string) error {
	_, err := run("go", "run", "./tools/releasecheck", "version", "--tag", tag)
	if err != nil {
		return fmt.Errorf("release version identity: %w", err)
	}
	return nil
}

func checkNotes(path, want string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read release notes %s: %w", path, err)
	}
	got := sha256.Sum256(b)
	if hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("release notes SHA-256 = %x, want %s", got, want)
	}
	return nil
}

func checkGoReleaser(miseFile string) error {
	b, err := os.ReadFile(miseFile)
	if err != nil {
		return fmt.Errorf("read %s: %w", miseFile, err)
	}
	m := regexp.MustCompile(`(?m)^goreleaser\s*=\s*"([^"]+)"\s*$`).FindStringSubmatch(string(b))
	if len(m) != 2 {
		return fmt.Errorf("%s does not pin goreleaser", miseFile)
	}
	out, err := run("goreleaser", "--version")
	if err != nil {
		return err
	}
	if !strings.Contains(string(out), m[1]) {
		return fmt.Errorf("goreleaser --version does not contain pinned version %q", m[1])
	}
	return nil
}

func remoteTagAbsent(tag string) error {
	out, code, err := runStatus("git", "ls-remote", "--exit-code", "--tags", "origin", "refs/tags/"+tag)
	if err != nil {
		return err
	}
	if code == 0 {
		return fmt.Errorf("remote tag %s already exists", tag)
	}
	if code == 2 && strings.TrimSpace(string(out)) == "" {
		return nil
	}
	return fmt.Errorf("cannot determine whether remote tag %s exists (git ls-remote exit %d: %s)", tag, code, strings.TrimSpace(string(out)))
}
func releaseAbsent(repo, tag string) error {
	out, code, err := runGHStatus("api", "--include", "repos/"+repo+"/releases/tags/"+tag)
	if err != nil {
		return err
	}
	if code == 0 {
		return fmt.Errorf("GitHub release %s already exists", tag)
	}
	if code != 0 && regexp.MustCompile(`(?m)^HTTP/\S+ 404(?: |\r?$)`).Match(out) {
		return nil
	}
	return fmt.Errorf("cannot determine whether GitHub release %s exists (gh api exit %d: %s)", tag, code, redactCredential(strings.TrimSpace(string(out))))
}

func capabilityProbe(repo, tag, commit string) error {
	_, err := runGH("api", "--method", "POST", "repos/"+repo+"/releases/generate-notes", "-f", "tag_name="+tag, "-f", "target_commitish="+commit)
	if err != nil {
		return fmt.Errorf("non-persisting GitHub generate-notes write-capability probe failed: %w", err)
	}
	return nil
}

func remoteTagTarget(tag, commit string) error {
	out, err := run("git", "ls-remote", "--tags", "origin", "refs/tags/"+tag, "refs/tags/"+tag+"^{}")
	if err != nil {
		return err
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == commit {
			return nil
		}
	}
	return fmt.Errorf("remote tag %s does not target %s", tag, commit)
}

type release struct {
	TagName    string `json:"tagName"`
	Target     string `json:"targetCommitish"`
	Draft      bool   `json:"isDraft"`
	Prerelease bool   `json:"isPrerelease"`
	Assets     []struct {
		Name  string `json:"name"`
		State string `json:"state"`
		Size  int64  `json:"size"`
	} `json:"assets"`
}

func finalRelease(repo, tag, commit string) error {
	out, err := runGH("release", "view", tag, "--repo", repo, "--json", "tagName,targetCommitish,isDraft,isPrerelease,assets")
	if err != nil {
		return err
	}
	var r release
	if err := json.Unmarshal(out, &r); err != nil {
		return fmt.Errorf("parse GitHub release response: %w", err)
	}
	if r.TagName != tag || r.Target != commit || r.Draft || r.Prerelease {
		return fmt.Errorf("release must be final and target %s (tag=%q target=%q draft=%t prerelease=%t)", commit, r.TagName, r.Target, r.Draft, r.Prerelease)
	}
	got := map[string]bool{}
	for _, a := range r.Assets {
		if a.State != "uploaded" || a.Size <= 0 {
			return fmt.Errorf("release asset %s must be uploaded and non-empty (state=%q size=%d)", a.Name, a.State, a.Size)
		}
		got[a.Name] = true
	}
	if len(r.Assets) != len(assets) || len(got) != len(assets) {
		return fmt.Errorf("release assets = %v, want exactly seven expected assets", sorted(got))
	}
	for a := range assets {
		if !got[a] {
			return fmt.Errorf("release assets missing %s", a)
		}
	}
	return verifyReleaseDownloads(repo, tag)
}

func verifyReleaseDownloads(repo, tag string) error {
	dir, err := os.MkdirTemp("", "dva-release-postflight-")
	if err != nil {
		return fmt.Errorf("create release verification directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(dir)
	}()

	names := sorted(assets)
	args := []string{"release", "download", tag, "--repo", repo, "--dir", dir}
	for _, name := range names {
		args = append(args, "--pattern", name)
	}
	if _, err := runGH(args...); err != nil {
		return fmt.Errorf("download release assets for checksum verification: %w", err)
	}
	return verifyDownloadedChecksums(dir)
}

func verifyDownloadedChecksums(dir string) error {
	b, err := os.ReadFile(filepath.Join(dir, "checksums.txt"))
	if err != nil {
		return fmt.Errorf("read downloaded checksums.txt: %w", err)
	}
	want := make(map[string]string, len(assets)-1)
	for lineNo, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(fields[0]) {
			return fmt.Errorf("checksums.txt line %d is not '<sha256> <asset>'", lineNo+1)
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name == "checksums.txt" || !assets[name] {
			return fmt.Errorf("checksums.txt names unexpected asset %q", name)
		}
		if _, exists := want[name]; exists {
			return fmt.Errorf("checksums.txt repeats asset %q", name)
		}
		want[name] = fields[0]
	}
	if len(want) != len(assets)-1 {
		return fmt.Errorf("checksums.txt covers %d archives, want %d", len(want), len(assets)-1)
	}
	for name, digest := range want {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read downloaded asset %s: %w", name, err)
		}
		got := sha256.Sum256(b)
		if hex.EncodeToString(got[:]) != digest {
			return fmt.Errorf("downloaded asset %s SHA-256 does not match checksums.txt", name)
		}
	}
	return nil
}

func clean(args []string) error {
	f := flag.NewFlagSet("clean", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return errors.New("clean accepts no arguments")
	}
	if err := checkRepositoryRoot(); err != nil {
		return err
	}
	if err := checkOrigin(); err != nil {
		return err
	}
	paths := []string{"dist", "bin", "tmp"}
	for _, name := range paths {
		info, err := os.Lstat(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect cleanup path %s: %w", name, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("refusing cleanup path %s: expected a real directory", name)
		}
	}
	for _, name := range paths {
		if err := os.RemoveAll(name); err != nil {
			return fmt.Errorf("remove cleanup path %s: %w", name, err)
		}
	}
	fmt.Println("releaseworkflow: removed repository-local dist, bin, and tmp outputs")
	return nil
}

func checkRepositoryRoot() error {
	out, err := run("git", "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("resolve repository root: %w", err)
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	if err != nil {
		return fmt.Errorf("resolve repository root path: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve current directory: %w", err)
	}
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		return fmt.Errorf("resolve current directory path: %w", err)
	}
	if cwd != root {
		return fmt.Errorf("refusing cleanup outside repository root (cwd=%s root=%s)", cwd, root)
	}
	return nil
}
func sorted(m map[string]bool) []string {
	r := make([]string, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}
func checkCleanup(paths []string) error {
	for _, p := range paths {
		if _, err := os.Lstat(filepath.Clean(p)); err == nil {
			return fmt.Errorf("cleanup path still exists: %s", p)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect cleanup path %s: %w", p, err)
		}
	}
	return nil
}
func fail(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "releaseworkflow: ERROR: %v\n", err)
		os.Exit(1)
	}
}
