// releasecheck validates the local GoReleaser snapshot contract without publishing anything.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

var platforms = []platform{
	{os: "linux", arch: "amd64", ext: ".tar.gz"},
	{os: "linux", arch: "arm64", ext: ".tar.gz"},
	{os: "darwin", arch: "amd64", ext: ".tar.gz"},
	{os: "darwin", arch: "arm64", ext: ".tar.gz"},
	{os: "windows", arch: "amd64", ext: ".zip"},
	{os: "windows", arch: "arm64", ext: ".zip"},
}

type platform struct {
	os   string
	arch string
	ext  string
}

func main() {
	if len(os.Args) < 2 {
		fail(errors.New("usage: releasecheck <version|artifacts> [flags]"))
	}

	var err error
	switch os.Args[1] {
	case "snapshot-version":
		err = printSnapshotVersion(os.Args[2:])
	case "binary":
		err = checkBinary(os.Args[2:])
	case "stamping":
		err = checkStamping(os.Args[2:])
	case "version":
		err = checkVersion(os.Args[2:])
	case "artifacts":
		err = checkArtifacts(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q (want snapshot-version, stamping, binary, version, or artifacts)", os.Args[1])
	}
	fail(err)
}

func printSnapshotVersion(args []string) error {
	flags := flag.NewFlagSet("snapshot-version", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	tag := flags.String("tag", "", "exact Git tag, if any")
	commit := flags.String("commit", "", "full Git commit")
	shortCommit := flags.String("short-commit", "", "Git abbreviated commit")
	if err := flags.Parse(args); err != nil {
		return err
	}
	version, err := snapshotVersion(*tag, *commit, *shortCommit)
	if err != nil {
		return err
	}
	fmt.Println(version)
	return nil
}

func snapshotVersion(tag, commit, shortCommit string) (string, error) {
	if !fullCommitRE.MatchString(commit) {
		return "", fmt.Errorf("commit %q is not a full 40-hex SHA", commit)
	}
	if shortCommit == "" || !strings.HasPrefix(commit, shortCommit) || !regexp.MustCompile(`^[0-9a-f]+$`).MatchString(shortCommit) {
		return "", fmt.Errorf("short commit %q must be a hexadecimal prefix of full commit %q", shortCommit, commit)
	}
	base := "0.0.0"
	if tag != "" {
		if !strings.HasPrefix(tag, "v") || len(tag) == 1 {
			return "", fmt.Errorf("tag %q must be v-prefixed", tag)
		}
		base = strings.TrimPrefix(tag, "v")
	}
	return base + "-SNAPSHOT-" + shortCommit, nil
}

var fullCommitRE = regexp.MustCompile(`^[0-9a-f]{40}$`)

func checkBinary(args []string) error {
	flags := flag.NewFlagSet("binary", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	binary := flags.String("binary", "", "path to executable")
	commit := flags.String("commit", "", "expected full Git commit")
	version := flags.String("version", "", "expected version, or snapshot prefix with --snapshot")
	snapshot := flags.Bool("snapshot", false, "require a snapshot version prefix")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *binary == "" || *commit == "" || *version == "" {
		return errors.New("--binary, --commit, and --version are required")
	}
	if !fullCommitRE.MatchString(*commit) {
		return fmt.Errorf("expected commit %q is not a full 40-hex SHA", *commit)
	}
	output, err := exec.Command(*binary, "version").Output()
	if err != nil {
		return fmt.Errorf("run %s version: %w", *binary, err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) != 3 {
		return fmt.Errorf("unexpected version output from %s: %q", *binary, output)
	}
	gotVersion := strings.TrimPrefix(lines[0], "dva version ")
	if gotVersion != *version {
		return fmt.Errorf("version = %q, want %q", gotVersion, *version)
	}
	if *snapshot && !strings.Contains(gotVersion, "-SNAPSHOT-") {
		return fmt.Errorf("version %q is not a snapshot", gotVersion)
	}
	gotCommit := strings.TrimPrefix(lines[1], "commit: ")
	if gotCommit != *commit || !fullCommitRE.MatchString(gotCommit) {
		return fmt.Errorf("commit = %q, want full SHA %q", gotCommit, *commit)
	}
	date := strings.TrimPrefix(lines[2], "build date: ")
	parsed, err := time.Parse(time.RFC3339, date)
	if err != nil || !strings.HasSuffix(date, "Z") || parsed.Location() != time.UTC {
		return fmt.Errorf("build date %q must be UTC RFC3339 with Z", date)
	}
	fmt.Printf("releasecheck: %s has stamped version, full commit, and UTC build date\n", *binary)
	return nil
}

func checkStamping(args []string) error {
	flags := flag.NewFlagSet("stamping", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	makefile := flags.String("makefile", "Makefile", "path to Makefile")
	config := flags.String("config", ".goreleaser.yml", "path to GoReleaser config")
	if err := flags.Parse(args); err != nil {
		return err
	}
	makeData, err := os.ReadFile(*makefile)
	if err != nil {
		return fmt.Errorf("read Makefile %s: %w", *makefile, err)
	}
	configData, err := os.ReadFile(*config)
	if err != nil {
		return fmt.Errorf("read GoReleaser config %s: %w", *config, err)
	}
	for _, field := range []string{"Version", "Commit", "BuildDate"} {
		makeFlag := fmt.Sprintf("-X $(MODULE)/internal/config.%s=$(%s)", field, strings.ToUpper(field))
		if field == "BuildDate" {
			makeFlag = "-X $(MODULE)/internal/config.BuildDate=$(BUILD_DATE)"
		}
		if !strings.Contains(string(makeData), makeFlag) {
			return fmt.Errorf("makefile is missing build stamp %q", makeFlag)
		}
		goreleaserValue := fmt.Sprintf("{{.%s}}", field)
		if field == "BuildDate" {
			goreleaserValue = "{{.Date}}"
		}
		goreleaserFlag := fmt.Sprintf("-X github.com/ScriptonBasestar/dva/internal/config.%s=%s", field, goreleaserValue)
		if !strings.Contains(string(configData), goreleaserFlag) {
			return fmt.Errorf("GoReleaser config is missing release stamp %q", goreleaserFlag)
		}
	}
	fmt.Println("releasecheck: Makefile and GoReleaser stamp Version, Commit, and BuildDate")
	return nil
}

func fail(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "releasecheck: ERROR: %v\n", err)
	os.Exit(1)
}

func checkVersion(args []string) error {
	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	versionFile := flags.String("version-file", "internal/config/version.go", "path to version.go")
	tag := flags.String("tag", "", "exact Git tag, if this is a tagged release")
	if err := flags.Parse(args); err != nil {
		return err
	}
	version, err := versionFromFile(*versionFile)
	if err != nil {
		return err
	}
	if *tag == "" {
		fmt.Printf("releasecheck: snapshot mode; internal/config.Version=%s (no exact tag)\n", version)
		return nil
	}
	want := "v" + version
	if *tag != want {
		return fmt.Errorf("tag %q must match internal/config.Version as %q", *tag, want)
	}
	fmt.Printf("releasecheck: tag %s matches internal/config.Version\n", *tag)
	return nil
}

func versionFromFile(path string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return "", fmt.Errorf("parse version source %s: %w", path, err)
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 || value.Names[0].Name != "Version" || len(value.Values) != 1 {
				continue
			}
			literal, ok := value.Values[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return "", fmt.Errorf("version in %s must be a string literal", path)
			}
			version := strings.Trim(literal.Value, "\"")
			if version == "" {
				return "", fmt.Errorf("version in %s is empty", path)
			}
			return version, nil
		}
	}
	return "", fmt.Errorf("version variable not found in %s", path)
}

func checkArtifacts(args []string) error {
	flags := flag.NewFlagSet("artifacts", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dist := flags.String("dist", "dist", "GoReleaser dist directory")
	if err := flags.Parse(args); err != nil {
		return err
	}

	checksums, err := readChecksums(filepath.Join(*dist, "checksums.txt"))
	if err != nil {
		return err
	}
	wantArchives := expectedArchiveNames()
	if err := sameNameSet("checksums.txt", checksums, wantArchives); err != nil {
		return err
	}
	actualArchives, err := topLevelArchives(*dist)
	if err != nil {
		return err
	}
	if err := sameNameSet("top-level archives", actualArchives, wantArchives); err != nil {
		return err
	}
	for name := range wantArchives {
		want, ok := checksums[name]
		if !ok {
			return fmt.Errorf("checksums.txt is missing %s", name)
		}
		got, err := fileSHA256(filepath.Join(*dist, name))
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("checksum mismatch for %s: got %s, want %s", name, got, want)
		}
	}
	fmt.Printf("releasecheck: verified %d platform archives and checksums in %s\n", len(platforms), *dist)
	return nil
}

func expectedArchiveNames() map[string]struct{} {
	names := make(map[string]struct{}, len(platforms))
	for _, target := range platforms {
		names[fmt.Sprintf("dva_%s_%s%s", target.os, target.arch, target.ext)] = struct{}{}
	}
	return names
}

func topLevelArchives(dist string) (map[string]struct{}, error) {
	entries, err := os.ReadDir(dist)
	if err != nil {
		return nil, fmt.Errorf("read dist directory %s: %w", dist, err)
	}
	archives := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() {
			continue // GoReleaser's per-build directories are expected metadata inputs.
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".zip") {
			archives[name] = struct{}{}
		}
	}
	return archives, nil
}

func sameNameSet(label string, actual any, want map[string]struct{}) error {
	got := make(map[string]struct{})
	switch values := actual.(type) {
	case map[string]string:
		for name := range values {
			got[name] = struct{}{}
		}
	case map[string]struct{}:
		got = values
	default:
		return fmt.Errorf("internal error: unsupported %s set", label)
	}
	if len(got) == len(want) {
		identical := true
		for name := range want {
			if _, ok := got[name]; !ok {
				identical = false
				break
			}
		}
		if identical {
			return nil
		}
	}
	return fmt.Errorf("%s names = %s, want exactly %s", label, sortedNames(got), sortedNames(want))
}

func sortedNames(names map[string]struct{}) string {
	values := make([]string, 0, len(names))
	for name := range names {
		values = append(values, name)
	}
	slices.Sort(values)
	return strings.Join(values, ", ")
}

func readChecksums(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read checksums %s: %w", path, err)
	}
	checksums := make(map[string]string)
	for lineNo, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || len(fields[0]) != sha256.Size*2 {
			return nil, fmt.Errorf("invalid checksum line %d in %s", lineNo+1, path)
		}
		if _, err := hex.DecodeString(fields[0]); err != nil {
			return nil, fmt.Errorf("invalid checksum line %d in %s: %w", lineNo+1, path, err)
		}
		if _, exists := checksums[fields[1]]; exists {
			return nil, fmt.Errorf("duplicate checksum for %s", fields[1])
		}
		checksums[fields[1]] = fields[0]
	}
	return checksums, nil
}

func fileSHA256(path string) (digest string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read archive %s: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			digest = ""
			err = fmt.Errorf("close archive %s: %w", path, closeErr)
		}
	}()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash archive %s: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
