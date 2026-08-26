package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestBuildManifest_WithResolvedPlans(t *testing.T) {
	tmpDir := t.TempDir()
	configYAML := []byte(`
version: "0.1.44"
vars:
  GLOBAL_SECRET: global-secret-value
stack:
  plan-dispatcher:
    default_runner: compose
    vars:
      STACK_SECRET: stack-secret-value
    runners:
      compose:
        files: [compose.yml, compose.monitoring.yml, compose.test.yml]
plans:
  local-infra:
    entries:
      - name: plan-dispatcher
        order: 10
        services: [postgres, nats, clickhouse]
        vars:
          ENTRY_SECRET: entry-secret-value
  review-backend:
    endpoint_tags: [api]
    vars:
      PLAN_SECRET: plan-secret-value
    entries:
      - name: plan-dispatcher
        runner: compose
        order: 10
        services: [postgres, nats, clickhouse, backend]
  local-full:
    entries:
      - name: plan-dispatcher
        runner: compose
        order: 10
        services: [postgres, nats, clickhouse, backend, frontend]
`)
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), configYAML, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	payload, err := json.Marshal(buildManifest(c))
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	var manifest struct {
		SchemaVersion string `json:"schema_version"`
		Plans         map[string]struct {
			EndpointTags []string `json:"endpoint_tags"`
			Entries      []struct {
				Name     string   `json:"name"`
				Runner   string   `json:"runner"`
				Order    int      `json:"order"`
				Services []string `json:"services"`
			} `json:"entries"`
		} `json:"plans"`
	}
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	if manifest.SchemaVersion != "1.3" {
		t.Fatalf("schema_version = %q, want 1.3", manifest.SchemaVersion)
	}
	if len(manifest.Plans) != 3 {
		t.Fatalf("plans = %v, want exactly 3 plans", manifest.Plans)
	}
	localFullIndex := bytes.Index(payload, []byte(`"local-full"`))
	localInfraIndex := bytes.Index(payload, []byte(`"local-infra"`))
	reviewBackendIndex := bytes.Index(payload, []byte(`"review-backend"`))
	if localFullIndex >= localInfraIndex || localInfraIndex >= reviewBackendIndex {
		t.Errorf("plan keys are not deterministic: local-full=%d local-infra=%d review-backend=%d", localFullIndex, localInfraIndex, reviewBackendIndex)
	}

	expectedServices := map[string][]string{
		"local-infra":    {"postgres", "nats", "clickhouse"},
		"review-backend": {"postgres", "nats", "clickhouse", "backend"},
		"local-full":     {"postgres", "nats", "clickhouse", "backend", "frontend"},
	}
	for name, services := range expectedServices {
		plan, ok := manifest.Plans[name]
		if !ok {
			t.Errorf("missing plan %q", name)
			continue
		}
		if len(plan.Entries) != 1 {
			t.Errorf("plan %q entries = %v, want exactly 1", name, plan.Entries)
			continue
		}
		if name == "review-backend" && !slices.Equal(plan.EndpointTags, []string{"api"}) {
			t.Errorf("plan %q endpoint_tags = %v, want [api]", name, plan.EndpointTags)
		}
		entry := plan.Entries[0]
		if entry.Name != "plan-dispatcher" || entry.Runner != "compose" || entry.Order != 10 {
			t.Errorf("plan %q resolved entry = %+v", name, entry)
		}
		if !slices.Equal(entry.Services, services) {
			t.Errorf("plan %q services = %v, want %v", name, entry.Services, services)
		}
	}

	for _, secret := range [][]byte{
		[]byte("global-secret-value"),
		[]byte("stack-secret-value"),
		[]byte("plan-secret-value"),
		[]byte("entry-secret-value"),
	} {
		if bytes.Contains(payload, secret) {
			t.Errorf("manifest leaked secret value %q", secret)
		}
	}
}
