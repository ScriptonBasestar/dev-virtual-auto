package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestStrictStillWarnsOnMissingComposeOutsideCorpus proves the compose-absence exemption
// TestExamplesStrictCleanExceptComposeAbsence grants the examples corpus lives in that
// corpus test alone, not in `dva config validate --strict` itself. TASK-276's ruling is
// explicit about why: a dva.yml outside the corpus that names a compose file which does not
// exist is a real defect, and weakening the validator to pass the fragment corpus would turn
// off that warning for every real project too. A dva.yml built here, in a scratch directory
// that is not part of examples/, must still fail strict validation with the same warning.
func TestStrictStillWarnsOnMissingComposeOutsideCorpus(t *testing.T) {
	bin := strictValidateBinary(t)

	dir := t.TempDir()
	cfg := `version: "0.1.44"
stack:
  app:
    default_runner: compose
    runners:
      compose:
        files:
          - docker-compose.yml
plans:
  dev:
    entries:
      - name: app
        runner: compose
        order: 10
`
	if err := os.WriteFile(filepath.Join(dir, "dva.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write scratch dva.yml: %v", err)
	}

	cmd := exec.Command(bin, "config", "validate", "--strict")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit from strict validate on a missing compose file, got success:\n%s", out)
	}

	if !strings.Contains(string(out), `is configured by dva.yml but does not exist`) {
		t.Fatalf("expected missing-compose-file warning, got:\n%s", out)
	}
}
