package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckVersionRequiresTagToMatchVersion(t *testing.T) {
	versionFile := writeVersionFile(t, "0.1.44")
	if err := checkVersion([]string{"--version-file", versionFile, "--tag", "v0.1.44"}); err != nil {
		t.Fatalf("checkVersion matching tag: %v", err)
	}
	err := checkVersion([]string{"--version-file", versionFile, "--tag", "v0.1.43"})
	if err == nil || !strings.Contains(err.Error(), "must match") {
		t.Fatalf("checkVersion mismatched tag error = %v, want match failure", err)
	}
}

func TestCheckStampingRequiresTheSameThreeFields(t *testing.T) {
	dir := t.TempDir()
	makefile := filepath.Join(dir, "Makefile")
	config := filepath.Join(dir, ".goreleaser.yml")
	make := "-X $(MODULE)/internal/config.Version=$(VERSION) -X $(MODULE)/internal/config.Commit=$(COMMIT) -X $(MODULE)/internal/config.BuildDate=$(BUILD_DATE)"
	release := "-X github.com/ScriptonBasestar/dva/internal/config.Version={{.Version}}\n-X github.com/ScriptonBasestar/dva/internal/config.Commit={{.Commit}}\n-X github.com/ScriptonBasestar/dva/internal/config.BuildDate={{.Date}}\n"
	if err := os.WriteFile(makefile, []byte(make), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte(release), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkStamping([]string{"--makefile", makefile, "--config", config}); err != nil {
		t.Fatalf("checkStamping: %v", err)
	}
	if err := os.WriteFile(config, []byte(strings.Replace(release, "BuildDate", "Build", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkStamping([]string{"--makefile", makefile, "--config", config}); err == nil {
		t.Fatal("checkStamping without BuildDate = nil, want error")
	}
}

func TestCheckVersionAllowsUntaggedSnapshot(t *testing.T) {
	if err := checkVersion([]string{"--version-file", writeVersionFile(t, "0.1.44")}); err != nil {
		t.Fatalf("checkVersion snapshot: %v", err)
	}
}

func TestCheckArtifactsVerifiesExpectedArchivesAndChecksums(t *testing.T) {
	dist := t.TempDir()
	var lines []string
	for _, target := range platforms {
		name := fmt.Sprintf("dva_%s_%s%s", target.os, target.arch, target.ext)
		data := []byte(name)
		if err := os.WriteFile(filepath.Join(dist, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		lines = append(lines, fmt.Sprintf("%x  %s", digest, name))
	}
	if err := os.WriteFile(filepath.Join(dist, "checksums.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkArtifacts([]string{"--dist", dist}); err != nil {
		t.Fatalf("checkArtifacts: %v", err)
	}
}

func TestCheckArtifactsRejectsChecksumMismatch(t *testing.T) {
	dist := t.TempDir()
	if err := os.WriteFile(filepath.Join(dist, "checksums.txt"), []byte(strings.Repeat("0", 64)+"  dva_linux_amd64.tar.gz\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := checkArtifacts([]string{"--dist", dist})
	if err == nil || !strings.Contains(err.Error(), "want exactly") {
		t.Fatalf("checkArtifacts error = %v, want exact-set failure", err)
	}
}

func TestCheckArtifactsRejectsExtraArchiveOrChecksum(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, dist string)
	}{
		{"extra archive", func(t *testing.T, dist string) {
			writeArtifactFixture(t, dist)
			if err := os.WriteFile(filepath.Join(dist, "dva_linux_386.tar.gz"), []byte("sabotage"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"extra checksum", func(t *testing.T, dist string) {
			writeArtifactFixture(t, dist)
			path := filepath.Join(dist, "checksums.txt")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			data = append(data, []byte(strings.Repeat("0", 64)+"  dva_linux_386.tar.gz\n")...)
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dist := t.TempDir()
			tc.setup(t, dist)
			if err := checkArtifacts([]string{"--dist", dist}); err == nil || !strings.Contains(err.Error(), "want exactly") {
				t.Fatalf("checkArtifacts error = %v, want exact-set failure", err)
			}
		})
	}
}

func writeArtifactFixture(t *testing.T, dist string) {
	t.Helper()
	var lines []string
	for _, target := range platforms {
		name := fmt.Sprintf("dva_%s_%s%s", target.os, target.arch, target.ext)
		data := []byte(name)
		if err := os.WriteFile(filepath.Join(dist, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		lines = append(lines, fmt.Sprintf("%x  %s", digest, name))
	}
	if err := os.WriteFile(filepath.Join(dist, "checksums.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeVersionFile(t *testing.T, version string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "version.go")
	content := "package config\nvar Version = \"" + version + "\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
