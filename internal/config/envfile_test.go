package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

// TASK-246 acceptance suite for the TASK-245 declaration contract.
//
// The property under test throughout is a negative one: adding sops_source must
// change nothing that already worked. So these tests do not assert that the new
// field loads — it deliberately does not load — they assert that the five legacy
// shapes still produce identical entries, identical precedence, and an identical
// `config show` round trip with the field present and absent.

const (
	sourceSentinelKey   = "DVA_TEST_SOURCE_SENTINEL"
	sourceSentinelValue = "dva-test-source-sentinel"
)

func writeEnvSourceFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func loadEnvSourceConfig(t *testing.T, files map[string]string) *Config {
	t.Helper()
	dir := writeEnvSourceFixture(t, files)
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return c
}

// TestConfigEnvLegacyShapeRoundTrip pins TASK-245 §2-1: every existing env_file
// shape keeps its load order, its required semantics, its merge result and its
// `config show` round trip, whether or not an entry declares sops_source.
func TestConfigEnvLegacyShapeRoundTrip(t *testing.T) {
	for _, tt := range []struct {
		name    string
		decl    string
		want    []EnvFileConfig
		wantVal string
	}{
		{
			name: "bare string",
			decl: "env_file: .env\n",
			want: []EnvFileConfig{{Path: ".env"}},
		},
		{
			name: "list of strings keeps declaration order",
			decl: "env_file: [.env.defaults, .env]\n",
			want: []EnvFileConfig{{Path: ".env.defaults"}, {Path: ".env"}},
		},
		{
			name: "list of objects",
			decl: "env_file:\n  - {path: .env.defaults}\n  - {path: .env, required: true}\n",
			want: []EnvFileConfig{{Path: ".env.defaults"}, {Path: ".env", Required: true}},
		},
		{
			name: "wrapper with string files",
			decl: "env_file:\n  files: .env\n  required: true\n",
			want: []EnvFileConfig{{Path: ".env", Required: true}},
		},
		{
			// The wrapper's required ORs into each entry rather than replacing it,
			// which is the one merge rule a reader is most likely to get wrong.
			name: "wrapper required ORs into entries",
			decl: "env_file:\n  files:\n    - {path: .env.defaults}\n    - {path: .env, required: true}\n  required: true\n",
			want: []EnvFileConfig{{Path: ".env.defaults", Required: true}, {Path: ".env", Required: true}},
		},
		{
			name: "entry with sops_source normalizes like any other entry",
			decl: "env_file:\n  - {path: .env.defaults}\n  - {path: .env, required: true, sops_source: secrets.env.enc}\n",
			want: []EnvFileConfig{{Path: ".env.defaults"}, {Path: ".env", Required: true, SopsSource: "secrets.env.enc"}},
		},
		{
			name: "sops_source inside a wrapper entry",
			decl: "env_file:\n  files:\n    - {path: .env, sops_source: secrets.env.enc}\n  required: true\n",
			want: []EnvFileConfig{{Path: ".env", Required: true, SopsSource: "secrets.env.enc"}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string]string{
				FileName:          "version: \"0.1.45\"\n" + tt.decl,
				".env":            sourceSentinelKey + "=" + sourceSentinelValue + "\nSHARED=from-env\n",
				".env.defaults":   "SHARED=from-defaults\nONLY_DEFAULT=yes\n",
				"secrets.env.enc": "placeholder\n",
			}
			c := loadEnvSourceConfig(t, files)

			got := c.AllEnvFileConfigs()
			if len(got) != len(tt.want) {
				t.Fatalf("entries = %+v, want %+v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("entry %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}

			// Load order: a later file's value wins. This is the observable that
			// would break first if sops_source ever reached the load path.
			env := NewEnvironment(nil, c.FileDir(), c.FileDir())
			if err := LoadEnvFile(c.EnvFile, c.FileDir(), env); err != nil {
				t.Fatalf("load env: %v", err)
			}
			if env.Vars[sourceSentinelKey] != sourceSentinelValue {
				t.Errorf("sentinel = %q, want %q", env.Vars[sourceSentinelKey], sourceSentinelValue)
			}
			if len(tt.want) > 1 && env.Vars["SHARED"] != "from-env" {
				t.Errorf("SHARED = %q, want the later file to win", env.Vars["SHARED"])
			}

			// `config show` marshals the struct and reads it back, so a declaration
			// that survives that trip is one the user sees unchanged.
			data, err := yaml.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			var view struct {
				EnvFile any `yaml:"env_file"`
			}
			if err := yaml.Unmarshal(data, &view); err != nil {
				t.Fatal(err)
			}
			var declared struct {
				EnvFile any `yaml:"env_file"`
			}
			if err := yaml.Unmarshal([]byte(tt.decl), &declared); err != nil {
				t.Fatal(err)
			}
			gotYAML, _ := yaml.Marshal(view.EnvFile)
			wantYAML, _ := yaml.Marshal(declared.EnvFile)
			if string(gotYAML) != string(wantYAML) {
				t.Errorf("config show round trip:\n got %s\nwant %s", gotYAML, wantYAML)
			}
		})
	}
}

// TestConfigEnvSourceMetadataScope pins where encrypted-source metadata is
// accepted and, more importantly, that everywhere else rejects it rather than
// dropping it. A silently ignored sops_source is the dangerous outcome: the
// config reads as if a secret is wired up and nothing ever unseals it.
func TestConfigEnvSourceMetadataScope(t *testing.T) {
	base := "version: \"0.1.45\"\n"

	t.Run("accepted at top level", func(t *testing.T) {
		c := loadEnvSourceConfig(t, map[string]string{
			FileName: base + "env_file:\n  - {path: .env, sops_source: secrets.env.enc}\n",
		})
		if got := c.EncryptedEnvEntries(); len(got) != 1 || got[0].SopsSource != "secrets.env.enc" {
			t.Fatalf("encrypted entries = %+v", got)
		}
		if origin := c.EnvFileOrigin(); origin.Kind != EnvOriginRoot || !origin.Writable() {
			t.Fatalf("origin = %+v, want a writable root origin", origin)
		}
	})

	for _, tt := range []struct {
		name  string
		yaml  string
		token string
	}{
		{
			// §2-3 R1: the wrapper level is where one source would claim several
			// targets, which the selector could never resolve.
			name:  "R1 wrapper level",
			yaml:  base + "env_file:\n  files: [.env, .env.local]\n  sops_source: secrets.env.enc\n",
			token: "source_not_on_entry",
		},
		{
			name:  "R2 duplicate target",
			yaml:  base + "env_file:\n  - {path: .env, sops_source: a.enc}\n  - {path: .env, sops_source: b.enc}\n",
			token: "duplicate_env_target",
		},
		{
			name:  "R3 duplicate source",
			yaml:  base + "env_file:\n  - {path: .env, sops_source: s.enc}\n  - {path: .env.local, sops_source: s.enc}\n",
			token: "duplicate_env_source",
		},
		{
			name:  "R4 source is another entry's target",
			yaml:  base + "env_file:\n  - {path: .env, sops_source: secrets.env.enc}\n  - {path: .env.local, sops_source: .env}\n",
			token: "env_source_is_target",
		},
		{
			name:  "R5 source equals its own target",
			yaml:  base + "env_file:\n  - {path: .env, sops_source: .env}\n",
			token: "source_is_target",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeEnvSourceFixture(t, map[string]string{FileName: tt.yaml})
			_, err := Load(dir)
			if err == nil {
				t.Fatalf("load succeeded; the declaration must be rejected, not ignored")
			}
			if !strings.Contains(err.Error(), tt.token) {
				t.Fatalf("error = %q, want it to name %q", err, tt.token)
			}
		})
	}

	// §2-3 R2 must not tighten configs that never opted in: a repeated path with
	// no sops_source anywhere keeps loading exactly as it does today.
	t.Run("duplicate plaintext targets stay permissive", func(t *testing.T) {
		c := loadEnvSourceConfig(t, map[string]string{
			FileName: base + "env_file:\n  - {path: .env}\n  - {path: .env}\n",
			".env":   "A=1\n",
		})
		if got := len(c.AllEnvFileConfigs()); got != 2 {
			t.Fatalf("entries = %d, want 2", got)
		}
	})

	// §5-1: a subproject-owned declaration loads, and is refused as a write target
	// because the parent session cannot prove the child's anchor or git repository.
	t.Run("subproject origin is not writable", func(t *testing.T) {
		dir := writeEnvSourceFixture(t, map[string]string{
			FileName:                base + "subprojects:\n  child:\n    path: ./child\n",
			"child/" + FileName:     base + "env_file:\n  - {path: .env, sops_source: secrets.env.enc}\n",
			"child/.env":            "A=1\n",
			"child/secrets.env.enc": "placeholder\n",
		})
		parent, err := Load(dir)
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		children, err := LoadSubprojects(parent.FileDir(), parent.Subprojects)
		if err != nil {
			t.Fatalf("load subprojects: %v", err)
		}
		child := children["child"]
		if child == nil {
			t.Fatal("child config missing")
		}
		if got := child.EncryptedEnvEntries(); len(got) != 1 {
			t.Fatalf("child encrypted entries = %+v, want the declaration to still load", got)
		}
		origin := child.EnvFileOrigin()
		if origin.Kind != EnvOriginSubproject {
			t.Fatalf("origin kind = %q, want %q", origin.Kind, EnvOriginSubproject)
		}
		if origin.Writable() {
			t.Fatal("subproject origin must not be writable from the parent session")
		}
	})
}

// TestEnvFileOriginTracksTheMergeWinner covers §5-2. env_file is replaced whole,
// so exactly one file owns the effective declaration — and a merge source that
// says nothing about env_file must not erase the previous winner's provenance.
func TestEnvFileOriginTracksTheMergeWinner(t *testing.T) {
	base := "version: \"0.1.45\"\n"

	t.Run("override wins", func(t *testing.T) {
		c := loadEnvSourceConfig(t, map[string]string{
			FileName:           base + "env_file: .env\n",
			"dva.override.yml": base + "env_file:\n  - {path: .env, sops_source: secrets.env.enc}\n",
			".env":             "A=1\n",
		})
		origin := c.EnvFileOrigin()
		if origin.Kind != EnvOriginOverride {
			t.Fatalf("origin = %+v, want override", origin)
		}
		if filepath.Base(origin.Path) != "dva.override.yml" {
			t.Fatalf("origin path = %q", origin.Path)
		}
	})

	t.Run("module wins over root", func(t *testing.T) {
		c := loadEnvSourceConfig(t, map[string]string{
			FileName:           base + "modules: [prod]\nenv_file: .env\n",
			".sb/dva/prod.yml": base + "env_file:\n  - {path: .env, sops_source: secrets.env.enc}\n",
			".env":             "A=1\n",
		})
		if origin := c.EnvFileOrigin(); origin.Kind != EnvOriginModule {
			t.Fatalf("origin = %+v, want module", origin)
		}
	})

	t.Run("silent module leaves the root as owner", func(t *testing.T) {
		c := loadEnvSourceConfig(t, map[string]string{
			FileName:           base + "modules: [prod]\nenv_file: .env\n",
			".sb/dva/prod.yml": base + "vars:\n  A: b\n",
			".env":             "A=1\n",
		})
		if origin := c.EnvFileOrigin(); origin.Kind != EnvOriginRoot {
			t.Fatalf("origin = %+v, want root", origin)
		}
	})

	t.Run("no declaration means no origin", func(t *testing.T) {
		c := loadEnvSourceConfig(t, map[string]string{FileName: base})
		if origin := c.EnvFileOrigin(); origin.Kind != EnvOriginUnknown || origin.Writable() {
			t.Fatalf("origin = %+v, want an unwritable unknown origin", origin)
		}
	})
}

// TestValidateDotenvStreamOversizedLine pins TASK-284 §4.
//
// A line the reader cannot hold used to surface as bufio's own scanner error,
// which carries no line number, so the caller reported "not valid dotenv" at
// line 0 — a corrupt-file diagnosis for a file whose only problem is one long
// line. The env bridge shows this message for decrypted output nobody can look
// at, which is exactly where a wrong diagnosis costs the most.
func TestValidateDotenvStreamOversizedLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.env")
	content := "SMALL=1\n# a comment\nBIG=" + strings.Repeat("x", MaxDotenvLineBytes+1) + "\nTAIL=2\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	count, line, err := ValidateDotenvStream(f)
	if err == nil {
		t.Fatal("an oversized line was accepted")
	}
	if !errors.Is(err, ErrDotenvLineTooLong) {
		t.Errorf("error = %v, want it to wrap ErrDotenvLineTooLong", err)
	}
	if line != 3 {
		t.Errorf("line = %d, want 3 — the oversized line's own number", line)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 on failure", count)
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Errorf("message %q does not name the line", err.Error())
	}
}

// A line at the limit is still readable, so the boundary is not reported as an
// error one byte early.
func TestValidateDotenvStreamLineAtTheLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edge.env")
	// Exactly MaxDotenvLineBytes, terminator excluded: the longest line the
	// reader accepts, not one byte less.
	if err := os.WriteFile(path, []byte("BIG="+strings.Repeat("x", MaxDotenvLineBytes-4)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	count, line, err := ValidateDotenvStream(f)
	if err != nil {
		t.Fatalf("a line at the limit was refused: %v (line %d)", err, line)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}
