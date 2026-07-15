package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFile_StringPath(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("FOO=bar\nBAZ=qux\n"), 0o644); err != nil {
		t.Fatalf("creating env file: %v", err)
	}

	env := NewEnvironment(nil, dir, dir)
	if err := LoadEnvFile(envPath, dir, env); err != nil {
		t.Fatalf("LoadEnvFile: %v", err)
	}

	if env.Vars["FOO"] != "bar" {
		t.Errorf("FOO = %q, want bar", env.Vars["FOO"])
	}
	if env.Vars["BAZ"] != "qux" {
		t.Errorf("BAZ = %q, want qux", env.Vars["BAZ"])
	}
}

func TestLoadEnvFile_RelativePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "custom.env"), []byte("REL_VAR=hello\n"), 0o644); err != nil {
		t.Fatalf("creating env file: %v", err)
	}

	env := NewEnvironment(nil, dir, dir)
	if err := LoadEnvFile("custom.env", dir, env); err != nil {
		t.Fatalf("LoadEnvFile: %v", err)
	}
	if env.Vars["REL_VAR"] != "hello" {
		t.Errorf("REL_VAR = %q, want hello", env.Vars["REL_VAR"])
	}
}

func TestLoadEnvFile_SliceOfPaths(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.env"), []byte("A=1\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.env"), []byte("B=2\n"), 0o644)

	env := NewEnvironment(nil, dir, dir)
	if err := LoadEnvFile([]any{"a.env", "b.env"}, dir, env); err != nil {
		t.Fatalf("LoadEnvFile: %v", err)
	}
	if env.Vars["A"] != "1" {
		t.Errorf("A = %q, want 1", env.Vars["A"])
	}
	if env.Vars["B"] != "2" {
		t.Errorf("B = %q, want 2", env.Vars["B"])
	}
}

func TestLoadEnvFile_OptionalMissing(t *testing.T) {
	dir := t.TempDir()
	env := NewEnvironment(nil, dir, dir)
	// optional (required=false) missing file should not error
	if err := LoadEnvFile("nonexistent.env", dir, env); err != nil {
		t.Errorf("optional missing env file should not error, got: %v", err)
	}
}

func TestLoadEnvFile_RequiredMissing(t *testing.T) {
	dir := t.TempDir()
	env := NewEnvironment(nil, dir, dir)
	cfg := []any{map[string]any{"path": "nonexistent.env", "required": true}}
	if err := LoadEnvFile(cfg, dir, env); err == nil {
		t.Error("required missing env file should return error")
	}
}

func TestLoadEnvFile_PriorityOverConfig(t *testing.T) {
	// OS env should override env_file values (env_file < config environment < OS env)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "p.env"), []byte("PRIO_VAR=from_file\n"), 0o644)

	t.Setenv("PRIO_VAR", "from_os")
	// Simulate: load env_file first, then OS env should win via NewEnvironment priority logic
	env := NewEnvironment(nil, dir, dir)
	if err := LoadEnvFile("p.env", dir, env); err != nil {
		t.Fatalf("LoadEnvFile: %v", err)
	}

	// OS env should take priority (NewEnvironment already loaded OS vars first)
	// After LoadEnvFile, the OS var should still dominate per MergeVars priority
	// env_file merges directly, so we check that env.Interpolate respects OS
	got := env.Interpolate("$PRIO_VAR")
	if got != "from_os" {
		t.Errorf("PRIO_VAR = %q, want from_os (OS env must win over env_file)", got)
	}
}

func TestLoadEnvFile_QuotedValues(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "q.env"), []byte(
		`SINGLE='hello world'`+"\n"+
			`DOUBLE="tab	here"`+"\n",
	), 0o644)

	env := NewEnvironment(nil, dir, dir)
	if err := LoadEnvFile("q.env", dir, env); err != nil {
		t.Fatalf("LoadEnvFile: %v", err)
	}
	if env.Vars["SINGLE"] != "hello world" {
		t.Errorf("SINGLE = %q, want 'hello world'", env.Vars["SINGLE"])
	}
}

func TestLoadEnvFile_InlineComments(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	contents := "PORT=14011 # Temporal PostgreSQL\nQUOTED=\"value # kept\"\n"
	if err := os.WriteFile(envPath, []byte(contents), 0644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	env := NewEnvironment(nil, "", "")
	if err := LoadEnvFile(envPath, dir, env); err != nil {
		t.Fatalf("LoadEnvFile: %v", err)
	}
	if got := env.Vars["PORT"]; got != "14011" {
		t.Errorf("PORT = %q, want %q", got, "14011")
	}
	if got := env.Vars["QUOTED"]; got != "value # kept" {
		t.Errorf("QUOTED = %q, want quoted content preserved", got)
	}
}
