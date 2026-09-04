package config

import (
	"slices"
	"strings"
	"testing"
)

func refsFixture(t *testing.T, yaml string) *Config {
	t.Helper()
	return loadConfigForSchemaTest(t, t.TempDir(), yaml)
}

func requireContaining(t *testing.T, got []string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !slices.ContainsFunc(got, func(s string) bool { return strings.Contains(s, w) }) {
			t.Errorf("missing warning containing %q\n got: %s", w, strings.Join(got, "\n      "))
		}
	}
}

func requireNoneContaining(t *testing.T, got []string, forbid ...string) {
	t.Helper()
	for _, f := range forbid {
		if slices.ContainsFunc(got, func(s string) bool { return strings.Contains(s, f) }) {
			t.Errorf("unexpected warning containing %q\n got: %s", f, strings.Join(got, "\n      "))
		}
	}
}

func TestWarnPlanServicesNotDeclared(t *testing.T) {
	c := refsFixture(t, `
version: "0.1.0"
stack:
  infra:
    default_runner: compose
    runners:
      compose:
        files: [docker-compose.yml]
        services:
          postgres: {}
          redis: {}
  nomap:
    default_runner: compose
    runners:
      compose:
        files: [other.yml]
plans:
  dev:
    entries:
      - name: infra
        services: [postgres, mailhog]
      - name: nomap
        services: [anything]
default_plan: dev
`)
	got := c.warnPlanServicesNotDeclared()
	requireContaining(t, got, "plans.dev.entries[0].services: mailhog not declared under stack.infra.runners.compose.services (declared: postgres, redis); either the plan names a service the compose file lacks, or the services map is stale")
	requireNoneContaining(t, got, "nomap", "postgres not declared")
	if len(got) != 1 {
		t.Fatalf("want exactly 1 warning, got %d: %v", len(got), got)
	}
}

func TestWarnUnreferencedEnvironmentsAndSites(t *testing.T) {
	c := refsFixture(t, `
version: "0.1.0"
stack:
  app:
    default_runner: native
    runners:
      native:
        run: "go run ."
environments:
  dev: {description: used}
  prod: {description: unused}
sites:
  local: {}
  remote: {}
plans:
  dev:
    environment: dev
    site: local
    entries: [{name: app}]
default_plan: dev
`)
	got := c.warnUnreferencedEnvironmentsAndSites()
	want := []string{
		"environments.prod: no plan selects it via environment:, so its values never apply; reference it from a plan or remove it",
		"sites.remote: no plan selects it via site:, so its vars and entry_overrides never apply; reference it from a plan or remove it",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}

	// `environment: ${ENV:-dev}` is decided at run time: no environment warning,
	// but the site axis is still static and still reported.
	dyn := &Config{
		Environments: map[string]EnvironmentProfile{"dev": {}, "prod": {}},
		Sites:        map[string]*SiteConfig{"local": {}, "remote": {}},
		Plans:        map[string]*PlanConfig{"p": {Environment: "${ENV:-dev}", Site: "local"}},
	}
	if w := dyn.warnUnreferencedEnvironmentsAndSites(); len(w) != 1 || !strings.HasPrefix(w[0], "sites.remote:") {
		t.Fatalf("dynamic environment selection: want only sites.remote, got %v", w)
	}

	// modes-only configs reach environments through -E; stay quiet there.
	legacy := &Config{Environments: map[string]EnvironmentProfile{"dev": {}}}
	if w := legacy.warnUnreferencedEnvironmentsAndSites(); len(w) != 0 {
		t.Fatalf("modes-only config must not warn, got %v", w)
	}
}

func TestWarnNoOpEntryOverrides(t *testing.T) {
	c := refsFixture(t, `
version: "0.1.0"
stack:
  app:
    default_runner: native
    runners:
      native:
        run: "go run ."
      compose:
        files: [docker-compose.yml]
sites:
  local:
    entry_overrides:
      ghost: {runner: compose}
      app: {}
  ci:
    entry_overrides:
      app: {runner: native}
  ok:
    entry_overrides:
      app: {runner: compose}
plans:
  dev:
    site: local
    entries: [{name: app}]
default_plan: dev
`)
	got := c.warnNoOpEntryOverrides()
	requireContaining(t, got,
		`sites.local.entry_overrides.ghost: "ghost" is not a stack entry`,
		"sites.local.entry_overrides.app: sets neither runner nor vars",
		`sites.ci.entry_overrides.app: runner "native" is already stack.app.default_runner`)
	requireNoneContaining(t, got, "sites.ok.")
	if len(got) != 3 {
		t.Fatalf("want 3 warnings, got %d: %v", len(got), got)
	}
}

func TestWarnEmptyInteractionCommands(t *testing.T) {
	c := &Config{Interaction: map[string]*InteractionCommand{
		"noop":   {Description: "left behind", Command: ""},
		"parent": {Subcommands: map[string]*InteractionCommand{"child": {Command: ""}, "ok": {Command: "echo hi"}}},
		"real":   {Command: "make test"},
	}}
	got := c.warnEmptyInteractionCommands()
	want := []string{
		"interaction.noop: declares no command, script, steps, or subcommands, so 'dva run noop' has nothing to execute; add a command or remove the entry",
		"interaction.parent.subcommands.child: declares no command, script, steps, or subcommands, so 'dva run parent child' has nothing to execute; add a command or remove the entry",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestWarnRemovedCLIReferences(t *testing.T) {
	c := &Config{
		Stack:        map[string]*LifecycleEntry{"infra": {Description: "start with dva stack up"}},
		Plans:        map[string]*PlanConfig{"dev": {Description: "was: dva up -M full"}},
		HealthChecks: map[string]HealthCheckConfig{"pg": {StartHint: "run dva infra first"}},
		Interaction: map[string]*InteractionCommand{
			"clean": {Command: "rm -rf tmp", Description: "dva clean replacement"},
			"setup": {Steps: []ProvisionItem{{Step: "prep", Run: "echo x", Note: "then dva app up api"}}},
			"deploy": {Subcommands: map[string]*InteractionCommand{
				"web": {Command: "dva up --mode=prod web"},
			}},
			"fine": {Command: "dva run dev", Description: "dva up dev keeps working"},
		},
	}
	got := c.warnRemovedCLIReferences()
	requireContaining(t, got,
		"stack.infra.description: mentions 'dva stack', which was removed (docs/43); use dva <up|down|...> <plan> instead",
		"plans.dev.description: mentions '-M', which was removed (docs/43); select a plan by name instead: dva up <plan>",
		"health_checks.pg.start_hint: mentions 'dva infra'",
		"interaction.setup.steps[0].note: mentions 'dva app'",
		"interaction.deploy.subcommands.web.command: mentions '--mode'",
	)
	// The user owns interaction.clean, so `dva clean` is a live command here.
	requireNoneContaining(t, got, "dva clean", "interaction.fine")
	if len(got) != 5 {
		t.Fatalf("want 5 warnings, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
}

func TestWarnOrphanHealthChecks(t *testing.T) {
	c := &Config{
		HealthChecks: map[string]HealthCheckConfig{
			"pg":     {URL: "http://localhost:5432"},
			"redis":  {URL: "http://localhost:6379"},
			"hinted": {Start: "make up"}, // warnUnreachableHealthChecks owns this one
		},
		Modes: map[string]ModeConfig{"dev": {HealthChecks: []string{"pg"}}},
	}
	got := c.warnOrphanHealthChecks()
	want := []string{"health_checks.redis: no modes.*.health_checks entry references it and plans do not read top-level health_checks, so it never runs; move it under stack.<entry>.health_checks or remove it"}
	if !slices.Equal(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestRemovedCommandsCoversDocs43Surface(t *testing.T) {
	rc := RemovedCommands()
	for _, verb := range []string{"stack", "app", "infra", "clean", "dev"} {
		if rc[verb] == "" {
			t.Errorf("RemovedCommands() lacks %q", verb)
		}
		if IsReservedCommand(verb) {
			t.Errorf("%q is both removed and reserved", verb)
		}
	}
}
