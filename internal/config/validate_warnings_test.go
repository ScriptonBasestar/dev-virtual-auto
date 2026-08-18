package config

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
)

// TestMigrationGuideURLTargetsCurrentMigrationSection locks the validate-warning
// migration link to the doc that owns §11 after the docs/40 split (TASK-090).
// The constant is what users click; a stale path is a silent dead end.
func TestMigrationGuideURLTargetsCurrentMigrationSection(t *testing.T) {
	const want = "https://github.com/ScriptonBasestar/dva/blob/master/docs/42-migration-and-compatibility.md#11-migration"
	if migrationGuideURL != want {
		t.Errorf("migrationGuideURL = %q\n  want %q", migrationGuideURL, want)
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")
	docPath := filepath.Join(repoRoot, "docs", "42-migration-and-compatibility.md")
	content, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("migration guide doc missing at %s: %v", docPath, err)
	}
	if !strings.Contains(string(content), "## 11. 마이그레이션 원칙") {
		t.Errorf("%s must retain heading ## 11. 마이그레이션 원칙 (anchor owner for #11-migration)", docPath)
	}
}

func TestNoVersionFloorRatchetWarning(t *testing.T) {
	// `version:` is the minimum DVA a config requires, so a floor below the running
	// binary is the correct, portable state and must produce no warning. Regression
	// guard for the removed warnVersionOutdated, which advised raising the floor to
	// match the binary — stranding users on older DVA and ratcheting every release.
	for _, v := range []string{"0.0.1", Version, ""} {
		c := &Config{Version: v}
		for _, w := range c.ValidateWarnings() {
			if strings.Contains(w, "older than") {
				t.Errorf("version %q must not warn about the floor: %s", v, w)
			}
		}
	}
}

func TestWarnHealthCheckRedundancy(t *testing.T) {
	// Both start and start_hint → warning
	c := &Config{
		HealthChecks: map[string]HealthCheckConfig{
			"app": {
				Type:      "http",
				URL:       "http://localhost:8080/health",
				Start:     "make run",
				StartHint: "make run",
			},
		},
		Stack: make(map[string]*LifecycleEntry),
	}
	warnings := c.warnHealthCheckRedundancy()
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "start_hint") {
		t.Errorf("unexpected warning: %s", warnings[0])
	}

	// Only start → no warning
	c = &Config{
		HealthChecks: map[string]HealthCheckConfig{
			"app": {Type: "http", Start: "make run"},
		},
		Stack: make(map[string]*LifecycleEntry),
	}
	warnings = c.warnHealthCheckRedundancy()
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(warnings))
	}

	// Only start_hint → no warning
	c = &Config{
		HealthChecks: map[string]HealthCheckConfig{
			"app": {Type: "http", StartHint: "make run"},
		},
		Stack: make(map[string]*LifecycleEntry),
	}
	warnings = c.warnHealthCheckRedundancy()
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(warnings))
	}
}

func TestWarnHealthCheckRedundancy_StackNested(t *testing.T) {
	c := &Config{
		HealthChecks: make(map[string]HealthCheckConfig),
		Stack: map[string]*LifecycleEntry{
			"infra": {
				HealthChecks: map[string]HealthCheckConfig{
					"db": {
						Type:      "tcp",
						Address:   "localhost:5432",
						Start:     "pg_ctl start",
						StartHint: "pg_ctl start",
					},
				},
			},
		},
	}
	warnings := c.warnHealthCheckRedundancy()
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for nested health check, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "stack.infra.health_checks.db") {
		t.Errorf("unexpected warning: %s", warnings[0])
	}
}

// TestWarnUnreachableHealthChecks covers TASK-179: top-level health_checks.start /
// start_hint only run when a modes.*.health_checks entry names them. Nested
// stack.*.health_checks is a different field and must stay silent here.
func TestWarnUnreachableHealthChecks(t *testing.T) {
	t.Run("start without modes warns", func(t *testing.T) {
		c := &Config{
			HealthChecks: map[string]HealthCheckConfig{
				"api":  {Type: "http", URL: "http://localhost:1", Start: "make run"},
				"web":  {Type: "http", URL: "http://localhost:2", StartHint: "start web"},
				"both": {Type: "tcp", Address: "localhost:3", Start: "up", StartHint: "up by hand"},
				// readiness-only: no start/start_hint → no warning from this pass
				"probe": {Type: "http", URL: "http://localhost:4"},
			},
		}
		warnings := c.warnUnreachableHealthChecks()
		if len(warnings) != 3 {
			t.Fatalf("expected 3 warnings, got %d: %v", len(warnings), warnings)
		}
		joined := strings.Join(warnings, "\n")
		for _, name := range []string{"api", "web", "both"} {
			if !strings.Contains(joined, "health_checks."+name) {
				t.Errorf("missing warning for %s in %v", name, warnings)
			}
		}
		if strings.Contains(joined, "probe") {
			t.Errorf("readiness-only probe must not warn: %v", warnings)
		}
		if !strings.Contains(joined, "no modes.*.health_checks") {
			t.Errorf("warning should say why it will not run: %v", warnings)
		}
	})

	t.Run("referenced by modes is silent", func(t *testing.T) {
		c := &Config{
			HealthChecks: map[string]HealthCheckConfig{
				"api": {Type: "http", Start: "make run"},
				"web": {Type: "http", StartHint: "start web"},
			},
			Modes: map[string]ModeConfig{
				"dev": {HealthChecks: []string{"api", "web"}},
			},
		}
		warnings := c.warnUnreachableHealthChecks()
		if len(warnings) != 0 {
			t.Fatalf("expected 0 warnings when modes reference checks, got %v", warnings)
		}
	})

	t.Run("partial mode coverage warns only unreferenced", func(t *testing.T) {
		c := &Config{
			HealthChecks: map[string]HealthCheckConfig{
				"api":    {Start: "make api"},
				"worker": {Start: "make worker"},
			},
			Modes: map[string]ModeConfig{
				"dev": {HealthChecks: []string{"api"}},
			},
		}
		warnings := c.warnUnreachableHealthChecks()
		if len(warnings) != 1 {
			t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
		}
		if !strings.Contains(warnings[0], "health_checks.worker") {
			t.Errorf("expected worker warning, got %s", warnings[0])
		}
		if strings.Contains(warnings[0], "health_checks.api") {
			t.Errorf("api is referenced and must not warn: %s", warnings[0])
		}
	})

	t.Run("stack nested start draws no warning", func(t *testing.T) {
		c := &Config{
			Stack: map[string]*LifecycleEntry{
				"infra": {
					HealthChecks: map[string]HealthCheckConfig{
						"db": {Type: "tcp", Address: "localhost:5432", Start: "pg_ctl start"},
					},
				},
			},
		}
		warnings := c.warnUnreachableHealthChecks()
		if len(warnings) != 0 {
			t.Fatalf("nested stack health_checks must not warn, got %v", warnings)
		}
	})

	t.Run("mode references missing health_checks name", func(t *testing.T) {
		c := &Config{
			HealthChecks: map[string]HealthCheckConfig{
				"api": {Start: "make run"},
			},
			Modes: map[string]ModeConfig{
				"dev": {HealthChecks: []string{"api", "missing-svc"}},
			},
		}
		warnings := c.warnUnreachableHealthChecks()
		if len(warnings) != 1 {
			t.Fatalf("expected 1 dangling-reference warning, got %d: %v", len(warnings), warnings)
		}
		if !strings.Contains(warnings[0], "modes.dev.health_checks") || !strings.Contains(warnings[0], "missing-svc") {
			t.Errorf("unexpected warning: %s", warnings[0])
		}
	})

	t.Run("warnings are order-stable", func(t *testing.T) {
		c := &Config{
			HealthChecks: map[string]HealthCheckConfig{
				"alpha":   {Start: "a"},
				"bravo":   {Start: "b"},
				"charlie": {StartHint: "c"},
			},
			Modes: map[string]ModeConfig{
				"m1": {HealthChecks: []string{"nope-a"}},
				"m2": {HealthChecks: []string{"nope-b"}},
			},
		}
		first := c.warnUnreachableHealthChecks()
		// 3 unreferenced + 2 dangling
		if len(first) != 5 {
			t.Fatalf("expected 5 warnings, got %d: %v", len(first), first)
		}
		if !sort.StringsAreSorted(first) {
			t.Errorf("warnings are not sorted: %v", first)
		}
		for i := range 50 {
			if got := c.warnUnreachableHealthChecks(); !slices.Equal(got, first) {
				t.Fatalf("run %d differs from run 0:\n first: %v\n got:   %v", i+1, first, got)
			}
		}
	})
}

func TestWarnDuplicateParentSubcommand(t *testing.T) {
	// Same command → warning
	c := &Config{
		Interaction: map[string]*InteractionCommand{
			"build": {
				Command: "cargo build",
				Subcommands: map[string]*InteractionCommand{
					"ce": {Command: "cargo build"},
				},
			},
		},
	}
	warnings := c.warnDuplicateParentSubcommand()
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "identical to parent") {
		t.Errorf("unexpected warning: %s", warnings[0])
	}

	// Different command → no warning
	c = &Config{
		Interaction: map[string]*InteractionCommand{
			"build": {
				Command: "cargo build",
				Subcommands: map[string]*InteractionCommand{
					"all": {Command: "cargo build --workspace"},
				},
			},
		},
	}
	warnings = c.warnDuplicateParentSubcommand()
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(warnings))
	}

	// No subcommands → no warning
	c = &Config{
		Interaction: map[string]*InteractionCommand{
			"test": {Command: "cargo test"},
		},
	}
	warnings = c.warnDuplicateParentSubcommand()
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(warnings))
	}
}

func TestWarnDuplicateStackOrder(t *testing.T) {
	// Same order → warning
	c := &Config{
		Stack: map[string]*LifecycleEntry{
			"compose":      {Order: 10},
			"compose-full": {Order: 10},
		},
	}
	warnings := c.warnDuplicateStackOrder()
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "order value 10") {
		t.Errorf("unexpected warning: %s", warnings[0])
	}

	// Different order → no warning
	c = &Config{
		Stack: map[string]*LifecycleEntry{
			"compose": {Order: 10},
			"k8s":     {Order: 20},
		},
	}
	warnings = c.warnDuplicateStackOrder()
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(warnings))
	}

	// Single entry → no warning
	c = &Config{
		Stack: map[string]*LifecycleEntry{
			"compose": {Order: 10},
		},
	}
	warnings = c.warnDuplicateStackOrder()
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(warnings))
	}

	// Order 0 (default) → distinct message
	c = &Config{
		Stack: map[string]*LifecycleEntry{
			"a": {Order: 0},
			"b": {Order: 0},
		},
	}
	warnings = c.warnDuplicateStackOrder()
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for order 0, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "default") {
		t.Errorf("expected 'default' hint in warning, got: %s", warnings[0])
	}

	// A plan that names the tied entries settles them (TASK-084 half 2).
	// docs/40-declarative-stack-and-plans.md puts order in the plan layer, so warning here would
	// tell three of dva's own examples they had not chosen a sequence their plan spells out.
	c.Plans = map[string]*PlanConfig{"local": {Entries: []PlanEntry{{Name: "a", Order: 10}, {Name: "b", Order: 20}}}}
	if warnings = c.warnDuplicateStackOrder(); len(warnings) != 0 {
		t.Errorf("a plan naming every tied entry must silence the warning, got %v", warnings)
	}

	// A plan that names only some does not settle the rest — the failure mode of the original
	// `len(c.Plans) > 0` rule, which let one planned entry hide every unplanned one. Three tied at
	// the default order, one planned: the other two still have no declared position anywhere.
	c = &Config{
		Stack: map[string]*LifecycleEntry{"a": {}, "b": {}, "c": {}},
		Plans: map[string]*PlanConfig{"local": {Entries: []PlanEntry{{Name: "a", Order: 10}}}},
	}
	warnings = c.warnDuplicateStackOrder()
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for the entries no plan names, got %v", warnings)
	}
	if !strings.Contains(warnings[0], "entries b, c") {
		t.Errorf("warning must name only the entries no plan positions: %s", warnings[0])
	}
	if !strings.Contains(warnings[0], "dva up <plan>") {
		t.Errorf("with plans declared the warning must say which command plan order governs: %s", warnings[0])
	}

	// Same fixture with the plan removed: `a` is no longer covered, so it rejoins the list. That
	// the two assertions disagree about `a` is what proves the plan set does the filtering rather
	// than the message being fixed text. The plan clause must go too, or it advertises a section
	// the config has none of.
	c.Plans = nil
	warnings = c.warnDuplicateStackOrder()
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning without plans, got %v", warnings)
	}
	if !strings.Contains(warnings[0], "entries a, b, c") {
		t.Errorf("without plans every tied entry is unpositioned: %s", warnings[0])
	}
	if strings.Contains(warnings[0], "dva up <plan>") {
		t.Errorf("the plan clause reached a config declaring no plans: %s", warnings[0])
	}
}

// TestWarnDuplicateStackOrderModeIsolation covers entries that share an order value but
// are never live together. Order decides who starts first inside one invocation, so a
// group no invocation can hold twice has no sequence to control — telling the user to
// set explicit order values there is advice about an event that cannot happen.
func TestWarnDuplicateStackOrderModeIsolation(t *testing.T) {
	// The shape found in the wild: four compose entries left at the default order,
	// each selected by exactly one mode.
	c := &Config{
		Stack: map[string]*LifecycleEntry{
			"compose":               {},
			"compose-minimal":       {},
			"compose-observability": {},
			"compose-tracing":       {},
		},
		Modes: map[string]ModeConfig{
			"full":          {Stack: []string{"compose"}},
			"minimal":       {Stack: []string{"compose-minimal"}},
			"observability": {Stack: []string{"compose-observability"}},
			"tracing":       {Stack: []string{"compose-tracing"}},
		},
	}
	if warnings := c.warnDuplicateStackOrder(); len(warnings) != 0 {
		t.Fatalf("expected silence for mode-isolated entries, got %v", warnings)
	}

	// One mode pulling in two of them puts both in the same invocation.
	c.Modes["full"] = ModeConfig{Stack: []string{"compose", "compose-tracing"}}
	if warnings := c.warnDuplicateStackOrder(); len(warnings) != 1 {
		t.Fatalf("expected a warning when a mode holds two entries at the same order, got %v", warnings)
	}

	// A mode with no stack: filter selects every entry, so nothing is isolated.
	c.Modes["full"] = ModeConfig{Stack: []string{"compose"}}
	c.Modes["everything"] = ModeConfig{ComposeProfiles: []string{"all"}}
	if warnings := c.warnDuplicateStackOrder(); len(warnings) != 1 {
		t.Fatalf("expected a warning when a mode selects every entry, got %v", warnings)
	}

	// Suppression is per order group, not global: isolating one group must not silence
	// another group that genuinely races.
	c = &Config{
		Stack: map[string]*LifecycleEntry{
			"isolated-a": {Order: 0},
			"isolated-b": {Order: 0},
			"shared-c":   {Order: 10},
			"shared-d":   {Order: 10},
		},
		Modes: map[string]ModeConfig{
			"a": {Stack: []string{"isolated-a", "shared-c", "shared-d"}},
			"b": {Stack: []string{"isolated-b", "shared-c", "shared-d"}},
		},
	}
	warnings := c.warnDuplicateStackOrder()
	if len(warnings) != 1 {
		t.Fatalf("expected exactly the non-isolated group to warn, got %v", warnings)
	}
	if !strings.Contains(warnings[0], "shared-c, shared-d") || !strings.Contains(warnings[0], "order value 10") {
		t.Errorf("warning should name only the racing group: %s", warnings[0])
	}
}

func TestCanonicalOrder_Correct(t *testing.T) {
	content := `version: "0.1.29"
env_file: .env
stack:
  compose:
    order: 10
modes:
  dev:
    description: Dev
health_checks:
  app:
    type: http
interaction:
  test:
    command: make test
provision:
  default:
    - step: Setup
      run: make setup
`
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	warnings := validateCanonicalOrder(path)
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for correct order, got %d: %v", len(warnings), warnings)
	}
}

func TestCanonicalOrder_Wrong(t *testing.T) {
	// interaction before stack → out of order
	content := `version: "0.1.29"
interaction:
  test:
    command: make test
stack:
  compose:
    order: 10
`
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	warnings := validateCanonicalOrder(path)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for wrong order, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "section order") {
		t.Errorf("unexpected warning: %s", warnings[0])
	}
}

func TestCanonicalOrder_SingleSection(t *testing.T) {
	content := `version: "0.1.29"
`
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	warnings := validateCanonicalOrder(path)
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for single section, got %d", len(warnings))
	}
}

func TestWarnDefaultModeHeavyInfra(t *testing.T) {
	svcList := func(svcs ...string) *[]string { return &svcs }

	// Default mode with kafka + monitoring → warning
	c := &Config{
		DefaultMode: "infra-only",
		Modes: map[string]ModeConfig{
			"infra-only": {
				ComposeServices: svcList("postgres", "redis", "kafka", "prometheus", "grafana", "jaeger"),
			},
		},
		Stack: map[string]*LifecycleEntry{
			"compose": {
				Compose: &ComposePluginConfig{
					Services: map[string]ServiceTagConfig{
						"postgres":   {Tags: []string{"infra", "data"}},
						"redis":      {Tags: []string{"infra", "data"}},
						"kafka":      {Tags: []string{"infra", "kafka"}},
						"prometheus": {Tags: []string{"infra", "monitoring"}},
						"grafana":    {Tags: []string{"infra", "monitoring"}},
						"jaeger":     {Tags: []string{"infra", "monitoring"}},
					},
				},
			},
		},
	}
	warnings := c.warnDefaultModeHeavyInfra()
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "non-core infrastructure") {
		t.Errorf("unexpected warning: %s", warnings[0])
	}
	// Should list the heavy services
	for _, svc := range []string{"kafka", "prometheus", "grafana", "jaeger"} {
		if !strings.Contains(warnings[0], svc) {
			t.Errorf("warning should mention %q: %s", svc, warnings[0])
		}
	}

	// Default mode with only core services → no warning
	c = &Config{
		DefaultMode: "infra",
		Modes: map[string]ModeConfig{
			"infra": {
				ComposeServices: svcList("postgres", "redis"),
			},
		},
		Stack: map[string]*LifecycleEntry{
			"compose": {
				Compose: &ComposePluginConfig{
					Services: map[string]ServiceTagConfig{
						"postgres": {Tags: []string{"infra", "data"}},
						"redis":    {Tags: []string{"infra", "data"}},
					},
				},
			},
		},
	}
	warnings = c.warnDefaultModeHeavyInfra()
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for core-only, got %d: %v", len(warnings), warnings)
	}

	// Name-based heuristic: no tags but heavy service names → warning
	c = &Config{
		DefaultMode: "infra",
		Modes: map[string]ModeConfig{
			"infra": {
				ComposeServices: svcList("postgres", "redis", "kafka", "minio"),
			},
		},
		Stack: map[string]*LifecycleEntry{
			"compose": {Compose: &ComposePluginConfig{}},
		},
	}
	warnings = c.warnDefaultModeHeavyInfra()
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for name heuristic, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "kafka") || !strings.Contains(warnings[0], "minio") {
		t.Errorf("warning should mention kafka and minio: %s", warnings[0])
	}

	// No default mode → no warning
	c = &Config{
		Modes: map[string]ModeConfig{
			"infra": {ComposeServices: svcList("postgres", "kafka")},
		},
	}
	warnings = c.warnDefaultModeHeavyInfra()
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings when no default_mode, got %d", len(warnings))
	}

	// compose_services nil (all services) → no warning (can't enumerate)
	c = &Config{
		DefaultMode: "infra",
		Modes: map[string]ModeConfig{
			"infra": {ComposeServices: nil},
		},
	}
	warnings = c.warnDefaultModeHeavyInfra()
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings when compose_services is nil, got %d", len(warnings))
	}
}

func TestWarnMultiStackComposeSplit(t *testing.T) {
	// Multiple compose entries → warning
	c := &Config{
		Stack: map[string]*LifecycleEntry{
			"compose":      {Compose: &ComposePluginConfig{}},
			"compose-full": {Compose: &ComposePluginConfig{}},
		},
	}
	warnings := c.warnMultiStackComposeSplit()
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "compose entries [compose, compose-full]") {
		t.Errorf("unexpected warning: %s", warnings[0])
	}

	// Single compose entry → no warning
	c = &Config{
		Stack: map[string]*LifecycleEntry{
			"compose": {Compose: &ComposePluginConfig{}},
		},
	}
	warnings = c.warnMultiStackComposeSplit()
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(warnings))
	}

	// Compose + kubectl → no warning (different backends)
	c = &Config{
		Stack: map[string]*LifecycleEntry{
			"compose": {Compose: &ComposePluginConfig{}},
			"k8s":     {Kubectl: &KubectlPluginConfig{}},
		},
	}
	warnings = c.warnMultiStackComposeSplit()
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for different backends, got %d", len(warnings))
	}
}

// TestWarnMultiStackComposeSplitModeIsolation covers the shape that replaced the
// removed modes.<name>.compose: one compose entry per mode, picked by
// modes.<name>.stack. The warning used to fire on it and tell users to consolidate,
// which modes.compose_services cannot express when the entries load different files.
func TestWarnMultiStackComposeSplitModeIsolation(t *testing.T) {
	composeStack := func() map[string]*LifecycleEntry {
		return map[string]*LifecycleEntry{
			"compose-base": {Compose: &ComposePluginConfig{Files: []string{"docker-compose.yml"}}},
			"compose-obs":  {Compose: &ComposePluginConfig{Files: []string{"docker-compose.yml", "docker-compose.obs.yml"}}},
		}
	}

	tests := []struct {
		name  string
		modes map[string]ModeConfig
		warn  bool
	}{
		{
			name: "each entry owned by one mode",
			modes: map[string]ModeConfig{
				"infra":         {Stack: []string{"compose-base"}},
				"observability": {Stack: []string{"compose-obs"}},
			},
			warn: false,
		},
		{
			name: "one mode pulls in both entries",
			modes: map[string]ModeConfig{
				"infra": {Stack: []string{"compose-base"}},
				"full":  {Stack: []string{"compose-base", "compose-obs"}},
			},
			warn: true,
		},
		{
			name: "a mode without stack: selects every entry",
			modes: map[string]ModeConfig{
				"infra": {Stack: []string{"compose-base"}},
				"full":  {ComposeProfiles: []string{"all"}},
			},
			warn: true,
		},
		{
			name: "an entry no mode claims always runs",
			modes: map[string]ModeConfig{
				"infra": {Stack: []string{"compose-base"}},
			},
			warn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{Stack: composeStack(), Modes: tt.modes}
			warnings := c.warnMultiStackComposeSplit()
			if tt.warn && len(warnings) != 1 {
				t.Fatalf("expected a warning, got %v", warnings)
			}
			if !tt.warn && len(warnings) != 0 {
				t.Fatalf("expected silence, got %v", warnings)
			}
		})
	}
}

// TestWarnDuplicateComposeApplicationOwnership was here. It covered the shape where
// applications.<app>.run.docker.service named a service a compose stack entry already
// owned, so `dva up` started it once through the orchestrator and again through the
// application manager. Both halves of that condition are gone (docs/43): there is no
// second lifecycle owner left to collide with, so the warning had nothing to warn about
// and went with the section it read.

func TestValidateWarnings_Integration(t *testing.T) {
	c := &Config{
		Version: "0.0.1",
		HealthChecks: map[string]HealthCheckConfig{
			"app": {
				Type:      "http",
				Start:     "make run",
				StartHint: "make run",
			},
		},
		Interaction: map[string]*InteractionCommand{
			"test": {
				Command: "make test",
				Subcommands: map[string]*InteractionCommand{
					"unit": {Command: "make test"},
				},
			},
		},
		Stack: map[string]*LifecycleEntry{
			"a": {Order: 10},
			"b": {Order: 10},
		},
	}

	warnings := c.ValidateWarnings()
	// Expect at least: version outdated + health check redundancy + duplicate command + duplicate order
	if len(warnings) < 4 {
		t.Errorf("expected at least 4 warnings, got %d: %v", len(warnings), warnings)
	}
}

func TestWarnChildOverridesParentCritical(t *testing.T) {
	c := &Config{
		Interaction: map[string]*InteractionCommand{
			"app": {
				Runner: "local",
				Pod:    "app-pod",
				Subcommands: map[string]*InteractionCommand{
					"dev": {
						Runner: "docker-compose",
						Pod:    "dev-pod", // both overridden
					},
					"test": {
						Runner: "local",
						Pod:    "app-pod", // same as parent, no warning
					},
				},
			},
		},
	}

	warnings := c.warnChildOverridesParentCritical()
	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d", len(warnings))
	}

	hasRunnerWarn := false
	hasPodWarn := false
	for _, w := range warnings {
		if strings.Contains(w, "overrides parent runner") {
			hasRunnerWarn = true
		}
		if strings.Contains(w, "overrides parent pod") {
			hasPodWarn = true
		}
	}
	if !hasRunnerWarn || !hasPodWarn {
		t.Errorf("missing expected warnings: %v", warnings)
	}
}

func TestWarnDeepSubcommandNesting(t *testing.T) {
	c := &Config{
		Interaction: map[string]*InteractionCommand{
			"level0": {
				Subcommands: map[string]*InteractionCommand{
					"level1": {
						Subcommands: map[string]*InteractionCommand{
							"level2": {
								Subcommands: map[string]*InteractionCommand{
									"level3": {
										Subcommands: map[string]*InteractionCommand{
											"level4": {
												Subcommands: map[string]*InteractionCommand{
													"level5": {
														Subcommands: map[string]*InteractionCommand{
															"level6": {
																Command: "echo too deep",
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"shallow": {
				Subcommands: map[string]*InteractionCommand{
					"sub": {Command: "echo ok"},
				},
			},
		},
	}

	warnings := c.warnDeepSubcommandNesting()
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "nested 6 levels deep") {
		t.Errorf("unexpected warning text: %s", warnings[0])
	}
}

func TestWarnUnreachableCommands(t *testing.T) {
	svc := "my-service"
	c := &Config{
		Interaction: map[string]*InteractionCommand{
			"unreachable": {
				Subcommands: map[string]*InteractionCommand{
					"sub": {Command: "echo ok"},
				},
			},
			"reachable_with_cmd": {
				Command: "echo hi",
				Subcommands: map[string]*InteractionCommand{
					"sub": {Command: "echo ok"},
				},
			},
			"reachable_with_svc": {
				Service: svc,
				Subcommands: map[string]*InteractionCommand{
					"sub": {Command: "echo ok"},
				},
			},
			"reachable_with_hooks": {
				Replace: []ProvisionItem{{Run: "echo replaced"}},
				Subcommands: map[string]*InteractionCommand{
					"sub": {Command: "echo ok"},
				},
			},
			// Was "reachable_without_subs", asserted here as producing no warning because the
			// check returned early on leaves. The name was the tell: nothing about this node is
			// reachable. `dva run dead_leaf` resolves, runs `sh -c ""` and exits 0. TASK-165
			// made it the second shape this check reports, so the row is renamed to what it is
			// and now asserts the warning rather than its absence.
			"dead_leaf": {},
			// The false positive that shape could produce, and the reason the check consults
			// inherited state rather than the raw node: this leaf sets nothing either, but
			// `dva run reachable_with_cmd inherits_target` runs `echo hi`. Warning about it
			// would make every correctly-factored config noisy — the mistake TASK-128 fixed for
			// the group shape, which this one must not reintroduce.
			"leaf_parent": {
				Command: "echo hi",
				Subcommands: map[string]*InteractionCommand{
					"inherits_target": {},
				},
			},
			// default_args alone executes: exec.buildCommandLine joins command and args and,
			// in shell mode, `sh -c` gets the args as the whole line. Measured — a node with
			// only `default_args: "echo reached"` prints `reached`. So this is a target, and a
			// warning here would be false.
			"args_only": {DefaultArgs: "echo reached"},
		},
	}

	warnings := c.warnUnreachableCommands()
	want := []string{
		"interaction.dead_leaf: has no execution target and no subcommands",
		"interaction.unreachable: has subcommands but is not directly callable",
	}
	if len(warnings) != len(want) {
		t.Fatalf("expected %d warnings, got %d: %v", len(want), len(warnings), warnings)
	}
	for _, w := range want {
		if !slices.ContainsFunc(warnings, func(got string) bool { return strings.Contains(got, w) }) {
			t.Errorf("missing warning %q, got: %v", w, warnings)
		}
	}
	// Named separately from the count: a count alone would not say which node went missing if
	// one of these ever started warning.
	for _, w := range warnings {
		for _, mustNotWarn := range []string{"inherits_target", "args_only"} {
			if strings.Contains(w, mustNotWarn) {
				t.Errorf("%s has an execution target (own or inherited) and must not warn: %s", mustNotWarn, w)
			}
		}
	}
}

func TestWarnUnresolvedEnvVars(t *testing.T) {
	c := &Config{
		Environment: map[string]string{
			"OK":   "value",
			"BAD":  "${MISSING_VAR}",
			"NOPE": "$MISSING_VAR_TOO",
			"GOOD": "${EXISTING_VAR}",
		},
	}
	env := NewEnvironment(c.Environment, ".", ".")
	env.Vars["EXISTING_VAR"] = "exists"

	warnings := c.warnUnresolvedEnvVars(env)
	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d", len(warnings))
	}
	// Warnings are sorted
	if !strings.Contains(warnings[0], "environment.BAD:") {
		t.Errorf("unexpected warning text: %s", warnings[0])
	}
	if !strings.Contains(warnings[1], "environment.NOPE:") {
		t.Errorf("unexpected warning text: %s", warnings[1])
	}
}

func TestWarnSuspiciousEnvPatterns(t *testing.T) {
	c := &Config{
		Environment: map[string]string{
			"DEFAULT": "${VAR:-default}",
			"OP":      "${VAR:=ok}",
			"SPECIAL": "count is $#",
			"GOOD":    "${VAR} and $VAR2",
		},
	}

	warnings := c.warnSuspiciousEnvPatterns()
	// Should warn for DEFAULT, OP, SPECIAL
	if len(warnings) != 3 {
		t.Fatalf("expected 3 warnings, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "environment.DEFAULT:") {
		t.Errorf("unexpected warning text: %s", warnings[0])
	}
	if !strings.Contains(warnings[1], "environment.OP:") {
		t.Errorf("unexpected warning text: %s", warnings[1])
	}
	if !strings.Contains(warnings[2], "environment.SPECIAL:") {
		t.Errorf("unexpected warning text: %s", warnings[2])
	}
}

// nestedInteractionConfig places the three mistakes the interaction-tree warnings look for
// one level below where the depth-1 versions of those checks could see them: `db` duplicates
// its own child's command and disagrees with its runner, and `grp.mid` is a group node with
// no execution target anywhere above it. A check that only walks the top level finds none of
// them.
//
// `rails.tools.nested` is the opposite case and is why it is here: it also sets no execution
// target, but inherits `tools-group` from its parent, so `dva run rails tools nested` runs.
// Warning about it is a false positive — the one this fixture originally asserted as correct
// (TASK-125), fixed in TASK-128.
func nestedInteractionConfig() *Config {
	return &Config{
		Interaction: map[string]*InteractionCommand{
			"rails": {
				Command: "bundle exec rails",
				Subcommands: map[string]*InteractionCommand{
					"db": {
						Command: "db-group",
						Runner:  "local",
						Subcommands: map[string]*InteractionCommand{
							"migrate": {Command: "db-group", Runner: "docker"},
						},
					},
					"tools": {
						Command: "tools-group",
						Subcommands: map[string]*InteractionCommand{
							"nested": {
								Subcommands: map[string]*InteractionCommand{
									"leaf": {Command: "echo leaf"},
								},
							},
						},
					},
				},
			},
			// Nothing in this chain supplies anything to execute, so `grp` and `grp.mid`
			// are both genuinely uncallable. `mid` is the depth-2 one.
			"grp": {
				Subcommands: map[string]*InteractionCommand{
					"mid": {
						Subcommands: map[string]*InteractionCommand{
							"leaf": {Command: "echo leaf"},
						},
					},
				},
			},
		},
	}
}

// TestInteractionWarningsRecurseIntoNestedSubcommands is the contract these three checks were
// missing. `subcommands` is recursive in the schema and the runner executes it to unbounded
// depth, so a warning that stops at depth 1 reports the shallow mistake and stays silent on
// the identical deep one. Measured before the fix: 3 warnings at depth 1, 0 at depth 2.
func TestInteractionWarningsRecurseIntoNestedSubcommands(t *testing.T) {
	c := nestedInteractionConfig()

	cases := []struct {
		name      string
		got       []string
		wantCount int
		wantPath  string
		wantText  string
	}{
		{
			name:      "warnDuplicateParentSubcommand",
			got:       c.warnDuplicateParentSubcommand(),
			wantCount: 1,
			wantPath:  "interaction.rails.subcommands.db.subcommands.migrate",
			wantText:  "identical to parent",
		},
		{
			name:      "warnChildOverridesParentCritical",
			got:       c.warnChildOverridesParentCritical(),
			wantCount: 1,
			wantPath:  "interaction.rails.subcommands.db.subcommands.migrate",
			wantText:  "overrides parent runner",
		},
		{
			// Two: `grp` at depth 0 and `grp.mid` at depth 2. The depth-2 one is the
			// contract; `grp` is included so the count is exact rather than a floor.
			name:      "warnUnreachableCommands",
			got:       c.warnUnreachableCommands(),
			wantCount: 2,
			wantPath:  "interaction.grp.subcommands.mid",
			wantText:  "not directly callable",
		},
	}

	for _, tc := range cases {
		if len(tc.got) != tc.wantCount {
			t.Errorf("%s: expected %d warning(s), got %d: %v", tc.name, tc.wantCount, len(tc.got), tc.got)
			continue
		}
		// The path must be the full YAML location, not just the top-level entry name —
		// a user cannot act on a warning that does not say which node it means.
		var found string
		for _, w := range tc.got {
			if strings.Contains(w, tc.wantPath) {
				found = w
				break
			}
		}
		if found == "" {
			t.Errorf("%s: want path %q in warnings, got: %v", tc.name, tc.wantPath, tc.got)
			continue
		}
		if !strings.Contains(found, tc.wantText) {
			t.Errorf("%s: want text %q in warning, got: %s", tc.name, tc.wantText, found)
		}
	}

	// The false positive this fixture used to assert as correct: `rails.tools.nested` sets no
	// execution target but inherits one, so it must not be reported. Asserted separately from
	// the count above because a count alone would not say which node went missing.
	for _, w := range c.warnUnreachableCommands() {
		if strings.Contains(w, "tools.subcommands.nested") {
			t.Errorf("inherited execution target must not warn as unreachable, got: %s", w)
		}
	}
}

// TestChildOverrideComparesInheritedRunner pins the fix for a false negative that only shows
// up below depth 1: the check read the parent's *raw* runner, but a middle node that sets no
// runner still passes its own parent's down.
//
// The two configs below are runtime-identical — `db` resolves to `local` either way, `migrate`
// to `docker` — and differ only in whether the author redundantly restated `runner: local` on
// the middle node. Before TASK-128 only the redundant one warned, making the trigger condition
// "did the author happen to type the value twice" rather than "does the backend change".
func TestChildOverrideComparesInheritedRunner(t *testing.T) {
	build := func(midRunner string) *Config {
		mid := &InteractionCommand{
			Subcommands: map[string]*InteractionCommand{
				"migrate": {Command: "db:migrate", Runner: "docker"},
			},
		}
		mid.Runner = midRunner
		return &Config{
			Interaction: map[string]*InteractionCommand{
				"rails": {
					Command:     "bundle exec rails",
					Runner:      "local",
					Subcommands: map[string]*InteractionCommand{"db": mid},
				},
			},
		}
	}

	for _, tc := range []struct{ name, midRunner string }{
		{"runner inherited implicitly", ""},
		{"runner restated redundantly", "local"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := build(tc.midRunner).warnChildOverridesParentCritical()
			if len(got) != 1 {
				t.Fatalf("expected 1 warning, got %d: %v", len(got), got)
			}
			const wantPath = "interaction.rails.subcommands.db.subcommands.migrate"
			if !strings.Contains(got[0], wantPath) {
				t.Errorf("want path %q, got: %s", wantPath, got[0])
			}
			// The effective parent runner, so both spellings report the same transition.
			if !strings.Contains(got[0], "(local → docker)") {
				t.Errorf("want the effective transition (local → docker), got: %s", got[0])
			}
		})
	}
}

// TestInteractionWarningsDepth1WordingIsUnchanged pins the exact depth-1 strings.
//
// The three pre-existing tests for these checks assert a phrase and a count
// (`strings.Contains(w, "identical to parent")`), never the config path, so every one of them
// would still pass if the recursion rewrite had mangled the `interaction.x.subcommands.y`
// prefix into something wrong. They are kept as-is, and this test supplies the byte-identity
// they do not cover: the fixture is the same shape used to measure the before/after output of
// the real binary, so a diff here is a diff a user would have seen.
func TestInteractionWarningsDepth1WordingIsUnchanged(t *testing.T) {
	c := &Config{
		Interaction: map[string]*InteractionCommand{
			"rails": {
				Command: "bundle exec rails",
				Runner:  "local",
				Subcommands: map[string]*InteractionCommand{
					"console": {Command: "bundle exec rails", Runner: "docker"},
				},
			},
			"grp": {
				Subcommands: map[string]*InteractionCommand{
					"leaf": {Command: "echo leaf"},
				},
			},
		},
	}

	cases := []struct {
		name string
		got  []string
		want []string
	}{
		{
			name: "warnDuplicateParentSubcommand",
			got:  c.warnDuplicateParentSubcommand(),
			want: []string{`interaction.rails.subcommands.console: command "bundle exec rails" is identical to parent; subcommand is redundant`},
		},
		{
			name: "warnChildOverridesParentCritical",
			got:  c.warnChildOverridesParentCritical(),
			want: []string{"interaction.rails.subcommands.console: overrides parent runner (local → docker); this may change execution backend unexpectedly"},
		},
		{
			name: "warnUnreachableCommands",
			got:  c.warnUnreachableCommands(),
			want: []string{"interaction.grp: has subcommands but is not directly callable; add an execution target or remove subcommands"},
		},
	}

	for _, tc := range cases {
		if !slices.Equal(tc.got, tc.want) {
			t.Errorf("%s: depth-1 output changed\n want: %q\n got:  %q", tc.name, tc.want, tc.got)
		}
	}
}

// TestInteractionWarningsAreOrderStable pins the sort. Both the interaction tree and each
// node's subcommands are maps, so without sorting the same dva.yml prints its warnings in a
// different order on consecutive runs — measured at 3 distinct orderings across 20 runs of
// `dva config validate` on a 3-warning fixture. That is the defect TASK-107 closed for
// command suggestions, and recursion makes it more likely by raising the per-check count.
func TestInteractionWarningsAreOrderStable(t *testing.T) {
	c := &Config{
		Interaction: map[string]*InteractionCommand{
			"rails": {
				Command: "shared-cmd",
				Subcommands: map[string]*InteractionCommand{
					"aaa": {Command: "shared-cmd"},
					"bbb": {Command: "shared-cmd"},
					"ccc": {Command: "shared-cmd"},
				},
			},
		},
	}

	first := c.warnDuplicateParentSubcommand()
	if len(first) != 3 {
		t.Fatalf("expected 3 warnings, got %d: %v", len(first), first)
	}
	if !sort.StringsAreSorted(first) {
		t.Errorf("warnings are not sorted: %v", first)
	}
	// Repeat: Go randomizes map iteration per range, so an unsorted implementation diverges
	// within a handful of calls rather than needing a separate process.
	for i := range 50 {
		got := c.warnDuplicateParentSubcommand()
		if !slices.Equal(got, first) {
			t.Fatalf("run %d differs from run 0:\n first: %v\n got:   %v", i+1, first, got)
		}
	}
}

// TestFlatMapWarningsAreOrderStable covers the two checks TASK-125 sorted nothing for while
// sorting its three siblings. Both range Go maps directly, so both printed a different order on
// consecutive runs of the same file.
//
// It is a separate test rather than more cases in TestInteractionWarningsAreOrderStable because
// neither check goes through eachInteractionNode: with either sort removed that test stays green
// (measured — the whole package still passed both times), so without this one the two sorts would
// ship with no automated guard and only a hand-run binary probe behind them. TASK-128.
func TestFlatMapWarningsAreOrderStable(t *testing.T) {
	redundant := HealthCheckConfig{Type: "tcp", Address: "localhost:1", Start: "up", StartHint: "up by hand"}

	// depth links of subcommands; calculateSubcommandDepth counts links, so > MaxSubcommandDepth
	// is what makes warnDeepSubcommandNesting fire.
	chain := func(depth int) *InteractionCommand {
		node := &InteractionCommand{Command: "echo leaf"}
		for range depth {
			node = &InteractionCommand{Subcommands: map[string]*InteractionCommand{"n": node}}
		}
		return node
	}
	deep := MaxSubcommandDepth + 1

	cases := []struct {
		name string
		run  func() []string
		want int
	}{
		{
			name: "warnHealthCheckRedundancy",
			run: (&Config{
				HealthChecks: map[string]HealthCheckConfig{"alpha": redundant, "bravo": redundant},
				Stack: map[string]*LifecycleEntry{
					"infra": {HealthChecks: map[string]HealthCheckConfig{"charlie": redundant, "delta": redundant}},
				},
			}).warnHealthCheckRedundancy,
			// Two per source, so an unsorted implementation can interleave the two maps as well
			// as shuffle within each.
			want: 4,
		},
		{
			name: "warnDeepSubcommandNesting",
			run: (&Config{
				Interaction: map[string]*InteractionCommand{
					"aaa": chain(deep), "bbb": chain(deep), "ccc": chain(deep),
				},
			}).warnDeepSubcommandNesting,
			want: 3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			first := tc.run()
			if len(first) != tc.want {
				t.Fatalf("expected %d warnings, got %d: %v", tc.want, len(first), first)
			}
			if !sort.StringsAreSorted(first) {
				t.Errorf("warnings are not sorted: %v", first)
			}
			for i := range 50 {
				if got := tc.run(); !slices.Equal(got, first) {
					t.Fatalf("run %d differs from run 0:\n first: %v\n got:   %v", i+1, first, got)
				}
			}
		})
	}
}

// TestWarnLiteralKeyShadowsSubproject covers the one ambiguity TASK-167's routing change
// introduces, and — just as importantly — the three shapes it must stay quiet about.
//
// A warning that fired on every colon key would be worse than none: the ordinary case this
// task exists to fix (`mytool:fast`, prefix naming no subproject) is now simply a working
// command, and warning about it would tell authors their correct config is suspect.
func TestWarnLiteralKeyShadowsSubproject(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  *Config
		want string // "" means: no warning at all
	}{
		{
			// The shape that could not exist before TASK-167: the parent declares the
			// literal key AND a subproject of the same prefix, so the child's `test`
			// loses the `engine:test` spelling to it.
			name: "prefix names a subproject: the child's command is shadowed",
			cfg: &Config{
				Subprojects: map[string]SubprojectConfig{"engine": {Path: "./engine"}},
				Interaction: map[string]*InteractionCommand{"engine:test": {Command: "echo parent"}},
			},
			want: "interaction.engine:test: `dva engine:test` runs this key, not subproject " +
				"`engine`'s `test` — the literal key takes precedence; use " +
				"`dva run --project engine test` to reach the subproject",
		},
		{
			// The headline TASK-167 case. Nothing is shadowed, because no subproject
			// named `mytool` exists to lose anything.
			name: "prefix names no subproject: nothing to shadow",
			cfg: &Config{
				Interaction: map[string]*InteractionCommand{"mytool:fast": {Command: "echo ok"}},
			},
		},
		{
			// ValidateReservedCommands already makes this a hard error, and
			// LiteralKeyWins excepts it so the key stays unroutable. A second opinion
			// here would describe a precedence that does not happen.
			name: "reserved prefix: unroutable, so it shadows nothing",
			cfg: &Config{
				Subprojects: map[string]SubprojectConfig{"compose": {Path: "./compose"}},
				Interaction: map[string]*InteractionCommand{"compose:ps": {Command: "echo x"}},
			},
		},
		{
			// The row above read `app:build` against a subproject named `app` while
			// `dva app` existed, and it belonged in the silent group for the reason
			// stated there. Removing the built-in moved it: `app` is no longer reserved,
			// so LiteralKeyWins stops excepting the key, it routes, and it does now take
			// precedence over the subproject — which is exactly the condition this
			// warning exists to name.
			//
			// Kept as its own row rather than folded into the first, because the pair is
			// the evidence: the same key and the same subproject answer differently on
			// either side of the removal, and neither row alone shows that.
			name: "removed built-in's prefix names a subproject: now it does shadow",
			cfg: &Config{
				Subprojects: map[string]SubprojectConfig{"app": {Path: "./app"}},
				Interaction: map[string]*InteractionCommand{"app:build": {Command: "echo x"}},
			},
			want: "interaction.app:build: `dva app:build` runs this key, not subproject " +
				"`app`'s `build` — the literal key takes precedence; use " +
				"`dva run --project app build` to reach the subproject",
		},
		{
			name: "no colon: not a candidate for splitting in the first place",
			cfg: &Config{
				Subprojects: map[string]SubprojectConfig{"engine": {Path: "./engine"}},
				Interaction: map[string]*InteractionCommand{"test": {Command: "echo ok"}},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			warnings := tc.cfg.warnLiteralKeyShadowsSubproject()
			if tc.want == "" {
				if len(warnings) != 0 {
					t.Fatalf("expected no warning, got %v", warnings)
				}
				return
			}
			if len(warnings) != 1 {
				t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
			}
			// Compared whole, not by substring. The escape hatch is the half of this
			// message the reader acts on, and `dva --project engine test` — the shorter
			// spelling, without the verb — exits 1 with `unknown command "test"`. A
			// Contains check on the first clause would pass while the advice sent the
			// reader to a command that refuses.
			if warnings[0] != tc.want {
				t.Errorf("warning text drifted:\n got:  %s\n want: %s", warnings[0], tc.want)
			}
		})
	}
}
