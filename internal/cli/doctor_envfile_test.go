package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDoctorEnvFilesHonorRequired(t *testing.T) {
	tests := []struct {
		name        string
		envFileYAML string
		present     bool
		wantCount   int
		wantPassed  bool
	}{
		{name: "scalar optional missing", envFileYAML: `.env.local`, wantCount: 0},
		{name: "list string optional missing", envFileYAML: "[.env.local]", wantCount: 0},
		{name: "per-file optional missing", envFileYAML: "{files: [{path: .env.local, required: false}]}", wantCount: 0},
		{name: "per-file required missing", envFileYAML: "{files: [{path: .env.local, required: true}]}", wantCount: 1},
		{name: "outer required missing", envFileYAML: "{files: [.env.local], required: true}", wantCount: 1},
		{name: "optional existing", envFileYAML: `.env.local`, present: true, wantCount: 1, wantPassed: true},
		{name: "required existing", envFileYAML: "{files: [.env.local], required: true}", present: true, wantCount: 1, wantPassed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := loadTestConfig(t, "version: \"0.1.45\"\nenv_file: "+tt.envFileYAML+"\n")
			if tt.present {
				if err := os.WriteFile(filepath.Join(c.FileDir(), ".env.local"), []byte("A=1\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			results := checkEnvFiles(c)
			if len(results) != tt.wantCount {
				t.Fatalf("checkEnvFiles() returned %d results, want %d: %+v", len(results), tt.wantCount, results)
			}
			if tt.wantCount == 1 && results[0].Passed != tt.wantPassed {
				t.Errorf("Passed = %v, want %v: %+v", results[0].Passed, tt.wantPassed, results[0])
			}
		})
	}
}

func TestDoctorEnvFilesOmitsOnlyMissingOptionalEntries(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.45"
env_file:
  files:
    - path: optional.env
    - path: required.env
      required: true
`)
	if err := os.WriteFile(filepath.Join(c.FileDir(), "required.env"), []byte("A=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := onlyResult(t, checkEnvFiles(c))
	if !result.Passed || result.Name != "Environment file exists: required.env" {
		t.Fatalf("checkEnvFiles() = %+v, want one passing required.env result", result)
	}
}

func TestDoctorOptionalEnvFileDoesNotHideStatErrors(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.45"
env_file: parent-file/child.env
`)
	if err := os.WriteFile(filepath.Join(c.FileDir(), "parent-file"), []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := onlyResult(t, checkEnvFiles(c))
	if result.Passed {
		t.Fatalf("checkEnvFiles() = %+v, want optional path with ENOTDIR to fail", result)
	}
	if result.Finding != "Environment file is INACCESSIBLE: parent-file/child.env" {
		t.Errorf("Finding = %q, want inaccessible-path diagnostic", result.Finding)
	}
}
