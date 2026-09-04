package lifecycle

import (
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TestCompositionPlanCalculatesWavesFromCalculateWaves proves composed children are
// wave-numbered by the same CalculateWaves algorithm (resolver.go:291) a leaf plan's
// stack entries use — not a second DAG/ordering implementation (TASK-260 §3.8).
func TestCompositionPlanCalculatesWavesFromCalculateWaves(t *testing.T) {
	dir := t.TempDir()
	writeImportedPlanConfig(t, dir, `
version: "0.1.0"
stack:
  a:
    default_runner: process
    runners:
      process:
        command: echo A
  b:
    default_runner: process
    runners:
      process:
        command: echo B
  c:
    default_runner: process
    runners:
      process:
        command: echo C
plans:
  a-plan:
    entries:
      - name: a
  b-plan:
    entries:
      - name: b
  c-plan:
    entries:
      - name: c
  release:
    composes:
      - plan: a-plan
        order: 0
      - plan: b-plan
        order: 1
        depends_on: ["a-plan"]
      - plan: c-plan
        order: 2
        depends_on: ["b-plan"]
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	resolved, err := ResolveCompositionPlan(cfg, "release")
	if err != nil {
		t.Fatalf("ResolveCompositionPlan: %v", err)
	}
	if len(resolved.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(resolved.Entries))
	}

	// Cross-check against a direct CalculateWaves call over the same Order/DependsOn
	// shape: composed children must land on identical wave numbers to stack entries
	// given the same graph, since it is the same algorithm.
	direct := []ResolvedEntry{
		{Name: "a-plan", Order: 0},
		{Name: "b-plan", Order: 1, DependsOn: []string{"a-plan"}},
		{Name: "c-plan", Order: 2, DependsOn: []string{"b-plan"}},
	}
	if err := CalculateWaves(direct); err != nil {
		t.Fatalf("CalculateWaves: %v", err)
	}
	wantWave := make(map[string]int, len(direct))
	for _, e := range direct {
		wantWave[e.Name] = e.Wave
	}

	for _, entry := range resolved.Entries {
		want, ok := wantWave[entry.ChildPlan.Name]
		if !ok {
			t.Fatalf("unexpected composed child %q", entry.ChildPlan.Name)
		}
		if entry.Wave != want {
			t.Errorf("wave for %q = %d, want %d (from direct CalculateWaves)", entry.ChildPlan.Name, entry.Wave, want)
		}
	}

	// Entries come back sorted by wave then order: a-plan(0) before b-plan(1) before c-plan(2).
	for i, wantName := range []string{"a-plan", "b-plan", "c-plan"} {
		if got := resolved.Entries[i].ChildPlan.Name; got != wantName {
			t.Errorf("entries[%d] = %q, want %q", i, got, wantName)
		}
	}
}

// TestCompositionChildResolvesAgainstOwnConfig proves each composed child resolves with
// its own owning project's effective config (environment, site, env_file, vars, hooks,
// endpoints, readiness) exactly as TASK-262 already guarantees for a direct or
// imported-plan invocation — the root's environment/site/vars never reach the child, and
// only CompositionEntry.Vars may override a specific key (TASK-260 §3.6: override, not
// merge — other child-owned keys survive untouched).
func TestCompositionChildResolvesAgainstOwnConfig(t *testing.T) {
	rootDir := t.TempDir()
	childDir := rootDir + "/backend"
	writeImportedPlanConfig(t, childDir, `
version: "0.1.0"
vars:
  SHARED: child-global
  CHILD_ONLY: child
environments:
  dev:
    environment:
      CHILD_ENV: child
stack:
  api:
    default_runner: process
    runners:
      process:
        command: echo CHILD
plans:
  deploy:
    environment: dev
    vars:
      SHARED: child-plan
    entries:
      - name: api
`)
	writeImportedPlanConfig(t, rootDir, `
version: "0.1.0"
vars:
  SHARED: root-global
  ROOT_ONLY: root
stack:
  local:
    default_runner: process
    runners:
      process:
        command: echo ROOT
subprojects:
  backend:
    path: backend
    import:
      plans:
        - name: deploy
plans:
  local-plan:
    entries:
      - name: local
  release:
    composes:
      - plan: backend/deploy
        order: 0
        vars:
          SHARED: composed-override
      - plan: local-plan
        order: 1
`)

	cfg, err := config.Load(rootDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	resolved, err := ResolveCompositionPlan(cfg, "release")
	if err != nil {
		t.Fatalf("ResolveCompositionPlan: %v", err)
	}
	if len(resolved.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(resolved.Entries))
	}

	var backendChild, localChild *ExecutionPlan
	for _, entry := range resolved.Entries {
		switch entry.ChildPlan.Name {
		case "backend/deploy":
			backendChild = entry.ChildPlan
		case "local-plan":
			localChild = entry.ChildPlan
		}
	}
	if backendChild == nil || localChild == nil {
		t.Fatalf("expected both backend/deploy and local-plan children, got %+v", resolved.Entries)
	}

	// The composed child owns the config it resolved against: backend/deploy resolves
	// against the child project, not root.
	if got := backendChild.OwnerConfig(cfg).FileDir(); got != childDir {
		t.Fatalf("backend/deploy owner dir = %q, want child %q", got, childDir)
	}
	if got := localChild.OwnerConfig(cfg).FileDir(); got != rootDir {
		t.Fatalf("local-plan owner dir = %q, want root %q", got, rootDir)
	}

	// CompositionEntry.Vars overrides the one key it names...
	if got := backendChild.EnvVars["SHARED"]; got != "composed-override" {
		t.Fatalf("backend/deploy SHARED = %q, want composed-override", got)
	}
	// ...without merging root vars in, and without erasing the child's own untouched keys.
	if got := backendChild.EnvVars["CHILD_ONLY"]; got != "child" {
		t.Fatalf("backend/deploy CHILD_ONLY = %q, want child-owned value untouched", got)
	}
	if got := backendChild.EnvVars["CHILD_ENV"]; got != "child" {
		t.Fatalf("backend/deploy CHILD_ENV = %q, want child environment value", got)
	}
	if _, leaked := backendChild.EnvVars["ROOT_ONLY"]; leaked {
		t.Fatal("backend/deploy EnvVars leaked ROOT_ONLY from root config")
	}

	// The uncomposed-override child keeps its own value for the shared key — root's
	// SHARED never reaches it either.
	if got := localChild.EnvVars["SHARED"]; got != "root-global" {
		t.Fatalf("local-plan SHARED = %q, want its own owner's root-global value", got)
	}
}

// TestCompositionPlanParticipatesInDefaultPlanSelection proves default_plan and the
// "exactly one declared plan" auto-selection rule treat a composition plan identically
// to a leaf plan — config.DefaultPlan() applies no special-casing for composes: plans
// (TASK-260 §3.5), and the selected name resolves end to end.
func TestCompositionPlanParticipatesInDefaultPlanSelection(t *testing.T) {
	// A composition plan always composes at least one other declared/imported plan
	// (TASK-260 §3.2), so that other plan is itself always present in cfg.Plans too —
	// a composition plan can therefore never be the sole entry that triggers implicit
	// single-plan selection. What §3.5 actually requires is checked below instead: with
	// several declared plans (leaf and composition mixed) and no explicit default_plan,
	// selection is refused exactly as it would be for several leaf plans alone — no
	// special-casing lets the composition plan win by being a composition plan.
	t.Run("multiple plans without explicit default_plan reject auto-selection", func(t *testing.T) {
		dir := t.TempDir()
		writeImportedPlanConfig(t, dir, `
version: "0.1.0"
stack:
  a:
    default_runner: process
    runners:
      process:
        command: echo A
plans:
  a-plan:
    entries:
      - name: a
  release:
    composes:
      - plan: a-plan
`)
		cfg, err := config.Load(dir)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if len(cfg.Plans) != 2 {
			t.Fatalf("expected both a-plan and release in cfg.Plans, got %d", len(cfg.Plans))
		}
		if got := cfg.DefaultPlan(); got != "" {
			t.Fatalf("DefaultPlan() = %q, want \"\" (ambiguous, same as leaf-only case)", got)
		}
		if got := cfg.DefaultPlanSource(); got != "none" {
			t.Fatalf("DefaultPlanSource() = %q, want none", got)
		}
	})

	t.Run("explicit default_plan naming a composition plan", func(t *testing.T) {
		dir := t.TempDir()
		writeImportedPlanConfig(t, dir, `
version: "0.1.0"
default_plan: release
stack:
  a:
    default_runner: process
    runners:
      process:
        command: echo A
plans:
  a-plan:
    entries:
      - name: a
  other-leaf:
    entries:
      - name: a
  release:
    composes:
      - plan: a-plan
`)
		cfg, err := config.Load(dir)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got := cfg.DefaultPlan(); got != "release" {
			t.Fatalf("DefaultPlan() = %q, want explicit default_plan %q", got, "release")
		}
		if got := cfg.DefaultPlanSource(); got != "explicit" {
			t.Fatalf("DefaultPlanSource() = %q, want explicit", got)
		}
		if _, err := ResolveCompositionPlan(cfg, cfg.DefaultPlan()); err != nil {
			t.Fatalf("ResolveCompositionPlan(DefaultPlan()): %v", err)
		}
	})
}

// TestCompositionPlanResolutionIsImmutablePerInvocation proves a resolved CompositionPlan
// (and its resolved children) does not change if the source config is mutated afterward —
// mirroring ExecutionPlan/ResolvedEntry immutability (TASK-260 §3.9). A config reload
// mid-run must not reach back into an already-resolved wave assignment or child owner.
func TestCompositionPlanResolutionIsImmutablePerInvocation(t *testing.T) {
	dir := t.TempDir()
	writeImportedPlanConfig(t, dir, `
version: "0.1.0"
stack:
  a:
    default_runner: process
    runners:
      process:
        command: echo A
  b:
    default_runner: process
    runners:
      process:
        command: echo B
  c:
    default_runner: process
    runners:
      process:
        command: echo C
plans:
  a-plan:
    entries:
      - name: a
  b-plan:
    entries:
      - name: b
  c-plan:
    entries:
      - name: c
  release:
    composes:
      - plan: a-plan
        order: 0
      - plan: b-plan
        order: 1
        depends_on: ["a-plan"]
`)
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	resolved, err := ResolveCompositionPlan(cfg, "release")
	if err != nil {
		t.Fatalf("ResolveCompositionPlan: %v", err)
	}
	if len(resolved.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(resolved.Entries))
	}

	origWave := make(map[string]int, len(resolved.Entries))
	origDependsOn := make(map[string][]string, len(resolved.Entries))
	for _, e := range resolved.Entries {
		origWave[e.ChildPlan.Name] = e.Wave
		origDependsOn[e.ChildPlan.Name] = append([]string(nil), e.DependsOn...)
	}
	origOwnerDir := resolved.OwnerConfig(cfg).FileDir()

	// Simulate a mid-run config mutation: reorder, drop the dependency, and add a third
	// composed entry directly on the already-loaded config's plan.
	planCfg := cfg.Plans["release"]
	planCfg.Composes[0].Order = 99
	planCfg.Composes[1].Order = -99
	planCfg.Composes[1].DependsOn = nil
	planCfg.Composes = append(planCfg.Composes, config.CompositionEntry{Plan: "c-plan", Order: 2})

	// The already-resolved CompositionPlan must be untouched by any of that.
	if len(resolved.Entries) != 2 {
		t.Fatalf("resolved.Entries length changed after config mutation: got %d, want 2", len(resolved.Entries))
	}
	for _, e := range resolved.Entries {
		if got, want := e.Wave, origWave[e.ChildPlan.Name]; got != want {
			t.Errorf("wave for %q changed after config mutation: got %d, want %d", e.ChildPlan.Name, got, want)
		}
		want := origDependsOn[e.ChildPlan.Name]
		if len(e.DependsOn) != len(want) {
			t.Errorf("depends_on for %q changed after config mutation: got %v, want %v", e.ChildPlan.Name, e.DependsOn, want)
			continue
		}
		for i := range want {
			if e.DependsOn[i] != want[i] {
				t.Errorf("depends_on for %q changed after config mutation: got %v, want %v", e.ChildPlan.Name, e.DependsOn, want)
			}
		}
	}
	if got := resolved.OwnerConfig(cfg).FileDir(); got != origOwnerDir {
		t.Errorf("owner dir changed after config mutation: got %q, want %q", got, origOwnerDir)
	}

	// Re-resolving from the mutated config produces the new shape — proving the
	// mutation was real and the first resolve simply did not observe it.
	reResolved, err := ResolveCompositionPlan(cfg, "release")
	if err != nil {
		t.Fatalf("ResolveCompositionPlan after mutation: %v", err)
	}
	if len(reResolved.Entries) != 3 {
		t.Fatalf("re-resolved entries = %d, want 3 (mutation should be observed on a fresh resolve)", len(reResolved.Entries))
	}
}
