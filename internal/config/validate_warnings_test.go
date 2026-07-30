package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestWarnDuplicateComposeApplicationOwnership(t *testing.T) {
	c := &Config{
		Stack: map[string]*LifecycleEntry{
			"devbox": {
				Runners: map[string]any{
					"compose": &ComposePluginConfig{
						Services: map[string]ServiceTagConfig{
							"django-engine": {Tags: []string{"app"}},
						},
					},
				},
			},
		},
		Applications: map[string]*ApplicationConfig{
			"django": {
				Run: AppExecPaths{Docker: AppDockerRef{Service: "django-engine"}},
			},
		},
	}

	warnings := c.warnDuplicateComposeApplicationOwnership()
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "applications.django.run.docker.service") ||
		!strings.Contains(warnings[0], "devbox") {
		t.Fatalf("unexpected warning: %s", warnings[0])
	}

	c.Applications["django"].Run.Docker.Service = "external-engine"
	if warnings := c.warnDuplicateComposeApplicationOwnership(); len(warnings) != 0 {
		t.Fatalf("expected no warning for distinct service ownership, got %v", warnings)
	}
}

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
			"reachable_without_subs": { // no subcommands -> no warning
			},
		},
	}

	warnings := c.warnUnreachableCommands()
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "interaction.unreachable:") {
		t.Errorf("unexpected warning text: %s", warnings[0])
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
