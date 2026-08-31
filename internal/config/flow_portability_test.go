package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPublicAgentMeshFlowsAreSelfContained(t *testing.T) {
	root := repositoryRoot(t)
	flows := []string{
		"agent-mesh-flows/dva-diagnose.yaml",
		"agent-mesh-flows/dva-discover.yaml",
		"agent-mesh-flows/dva-improve.yaml",
		"agent-mesh-flows/dva-improve-guided.yaml",
		"agent-mesh-flows/dva-improve-guided/00-analyze.yaml",
		"agent-mesh-flows/dva-improve-guided/30-configure.yaml",
	}
	for _, path := range flows {
		content := readFlowPortabilityFile(t, root, path)
		for _, forbidden := range []string{"DVA_SHARED_DIR", "shared_dir", "resolve_shared", "{{resolve_shared."} {
			if strings.Contains(content, forbidden) {
				t.Errorf("%s still depends on %q", path, forbidden)
			}
		}
	}
}

func TestPublicAgentMeshFlowsContainCurrentCorpus(t *testing.T) {
	root := repositoryRoot(t)
	checks := []struct {
		flow   string
		corpus string
	}{
		{"agent-mesh-flows/dva-improve.yaml", "agent-mesh-flows/shared/guardrails/guardrails-preserve.md"},
		{"agent-mesh-flows/dva-improve.yaml", "agent-mesh-flows/shared/guardrails/guardrails-rewrite.md"},
		{"agent-mesh-flows/dva-improve.yaml", "agent-mesh-flows/shared/library/shared-guardrails.md"},
		{"agent-mesh-flows/dva-improve.yaml", "agent-mesh-flows/shared/library/naming-presets.md"},
		{"agent-mesh-flows/dva-improve.yaml", "agent-mesh-flows/shared/library/dva-schema.md"},
		{"agent-mesh-flows/dva-improve.yaml", "agent-mesh-flows/shared/library/reference-examples.md"},
		{"agent-mesh-flows/dva-improve.yaml", "agent-mesh-flows/shared/library/shared-checklist.md"},
		{"agent-mesh-flows/dva-diagnose.yaml", "agent-mesh-flows/shared/library/shared-guardrails.md"},
		{"agent-mesh-flows/dva-improve-guided/00-analyze.yaml", "agent-mesh-flows/shared/library/shared-guardrails.md"},
		{"agent-mesh-flows/dva-improve-guided/00-analyze.yaml", "agent-mesh-flows/shared/library/naming-presets.md"},
		{"agent-mesh-flows/dva-improve-guided/30-configure.yaml", "agent-mesh-flows/shared/library/shared-guardrails.md"},
		{"agent-mesh-flows/dva-improve-guided/30-configure.yaml", "agent-mesh-flows/shared/library/dva-schema.md"},
		{"agent-mesh-flows/dva-improve-guided/30-configure.yaml", "agent-mesh-flows/shared/library/naming-presets.md"},
		{"agent-mesh-flows/dva-improve-guided/30-configure.yaml", "agent-mesh-flows/shared/library/reference-examples.md"},
		{"agent-mesh-flows/dva-improve-guided/30-configure.yaml", "agent-mesh-flows/shared/library/shared-checklist.md"},
	}
	for _, check := range checks {
		flow := readFlowPortabilityFile(t, root, check.flow)
		corpus := readFlowPortabilityFile(t, root, check.corpus)
		if !strings.Contains(flow, flowLiteralBlock(corpus)) {
			t.Errorf("%s is stale or missing corpus %s; run make generate", check.flow, check.corpus)
		}
	}
}

func flowLiteralBlock(corpus string) string {
	lines := strings.Split(strings.TrimSuffix(corpus, "\n"), "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = "      " + line
		}
	}
	return strings.Join(lines, "\n")
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func readFlowPortabilityFile(t *testing.T, root, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
