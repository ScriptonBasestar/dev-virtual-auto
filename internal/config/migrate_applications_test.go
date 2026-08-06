package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// migrateAppsAndDecode runs the migration and returns both the rewritten text and the
// config it decodes to.
//
// Decoding is the assertion that matters. A migration can emit text containing
// "build: cargo build" and still have produced nothing, if the key landed somewhere
// decodeRunnerNode does not read — which is exactly the shape the runners.native defect
// took: advertised, spelled correctly, and connected to nothing.
func migrateAppsAndDecode(t *testing.T, src string) (string, *Config) {
	t.Helper()

	out, report, err := MigrateApplications([]byte(src))
	if err != nil {
		t.Fatalf("MigrateApplications() error = %v", err)
	}
	if len(report.Changes) == 0 {
		t.Fatalf("MigrateApplications() reported no change for a config with applications:\n%s", src)
	}
	if err := VerifyMigrated(out); err != nil {
		t.Fatalf("migrated config does not load: %v\n%s", err, out)
	}
	cfg, err := decodeConfig(out)
	if err != nil {
		t.Fatalf("decode migrated config: %v\n%s", err, out)
	}
	return string(out), cfg
}

// nativeRunner returns the decoded native runner of a stack entry, failing if the
// migration produced a key nothing reads.
func nativeRunner(t *testing.T, cfg *Config, entry string) *NativeRunnerConfig {
	t.Helper()
	e, ok := cfg.Stack[entry]
	if !ok || e == nil {
		t.Fatalf("stack.%s does not exist after migration", entry)
	}
	native, ok := e.Runners["native"].(*NativeRunnerConfig)
	if !ok || native == nil {
		t.Fatalf("stack.%s.runners.native decoded to %T, not a native runner", entry, e.Runners["native"])
	}
	return native
}

const nativeAppConfig = `version: "0.1.44"
stack:
  infra:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
applications:
  api:
    description: "REST API server"
    tags: [backend]
    dir: services/api
    build:
      native: "cargo build --release -p api"
    run:
      native: "cargo run --release -p api"
    environment:
      RUST_LOG: debug
    health:
      type: http
      url: "http://localhost:11200/health"
`

// TestMigrateApplicationsProducesADecodableNativeRunner is the core case: every field
// with a target reaches it, and reaches it as a decoded value rather than as text.
func TestMigrateApplicationsProducesADecodableNativeRunner(t *testing.T) {
	got, cfg := migrateAppsAndDecode(t, nativeAppConfig)

	// Asserted on the text, not on cfg.Applications: the field was removed with the
	// section (docs/43), and decodeConfig is not strict, so a surviving applications:
	// key now decodes to nothing at all rather than to a value a test could catch.
	// The text is what the user is left holding, and it is what `dva config validate`
	// reads next.
	if strings.Contains(got, "applications:") {
		t.Errorf("the applications: key survived a migration that converted every entry:\n%s", got)
	}

	native := nativeRunner(t, cfg, "api")
	for _, tc := range []struct{ field, got, want string }{
		{"dir", native.Dir, "services/api"},
		{"build", native.Build, "cargo build --release -p api"},
		{"run", native.Run, "cargo run --release -p api"},
		{"env.RUST_LOG", native.Env["RUST_LOG"], "debug"},
	} {
		if tc.got != tc.want {
			t.Errorf("runners.native.%s = %q, want %q", tc.field, tc.got, tc.want)
		}
	}

	entry := cfg.Stack["api"]
	if entry.Description != "REST API server" {
		t.Errorf("description = %q, want it carried over", entry.Description)
	}
	if len(entry.Tags) != 1 || entry.Tags[0] != "backend" {
		t.Errorf("tags = %v, want [backend]", entry.Tags)
	}

	// health_checks is keyed by the entry, and the orchestrator reads it from the entry
	// for both --wait and status. A health check that migrated to the wrong key would
	// load cleanly and never run.
	hc, ok := entry.HealthChecks["api"]
	if !ok {
		t.Fatalf("health did not become stack.api.health_checks.api, got keys %v", entry.HealthChecks)
	}
	if hc.URL != "http://localhost:11200/health" {
		t.Errorf("health check url = %q, want it carried over", hc.URL)
	}

	// The untouched entry proves the rewrite is a splice and not a re-encode.
	if !strings.Contains(got, "        files: [compose.yml]") {
		t.Errorf("the pre-existing compose entry was reformatted:\n%s", got)
	}
	if strings.Contains(got, "applications:") {
		t.Errorf("the emptied applications: key was left behind:\n%s", got)
	}
}

// TestMigrateApplicationsAcceptsTheStringShorthand: `run: cargo run` and
// `run: {native: cargo run}` are both live spellings, so reading only the object form
// would drop every application written the short way.
func TestMigrateApplicationsAcceptsTheStringShorthand(t *testing.T) {
	_, cfg := migrateAppsAndDecode(t, `version: "0.1.44"
applications:
  web:
    run: "pnpm dev"
`)

	if got := nativeRunner(t, cfg, "web").Run; got != "pnpm dev" {
		t.Errorf("runners.native.run = %q, want %q", got, "pnpm dev")
	}
}

// TestMigrateApplicationsOpensAStackSection covers the config that declared applications
// and nothing else — with no stack: to append to, one has to be created.
func TestMigrateApplicationsOpensAStackSection(t *testing.T) {
	got, cfg := migrateAppsAndDecode(t, `version: "0.1.44"
applications:
  web:
    run: "pnpm dev"
`)

	if _, ok := cfg.Stack["web"]; !ok {
		t.Fatalf("no stack.web after migrating the only application:\n%s", got)
	}
	if strings.Count(got, "stack:") != 1 {
		t.Errorf("expected exactly one stack: section:\n%s", got)
	}
}

// TestMigrateApplicationsConvertsAlongsideWhatNeedsHands.
//
// None of these fields has a mechanical target, but none of them makes `run.native`
// any less faithful either — so the entry is written and the leftover is reported.
// Refusing the whole application over one of them converted nothing at all on the live
// corpus, where every application declares at least one.
//
// The report has to name the field and what it did, because the operator's next move is
// to write that shape by hand and the application it came from is about to be deleted.
func TestMigrateApplicationsConvertsAlongsideWhatNeedsHands(t *testing.T) {
	for _, tc := range []struct {
		name  string
		app   string
		wants []string
	}{
		{
			name:  "dev command",
			app:   "    run: \"pnpm start\"\n    dev: \"pnpm dev\"\n",
			wants: []string{"applications.web.dev", "its own entry"},
		},
		{
			name:  "variants",
			app:   "    run: \"pnpm start\"\n    variants:\n      admin:\n        run: \"pnpm admin\"\n",
			wants: []string{"applications.web.variants", "its own entry"},
		},
		{
			name: "docker service reference",
			app:  "    run:\n      native: \"pnpm start\"\n      docker: { service: web-js, profile: full }\n",
			wants: []string{"applications.web.run.docker", `compose service "web-js"`,
				`profile "full"`, "compose runner"},
		},
		{
			name:  "depends_on",
			app:   "    run: \"pnpm start\"\n    depends_on: [db]\n",
			wants: []string{"applications.web.depends_on", "entries[].depends_on"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "version: \"0.1.44\"\napplications:\n  web:\n" + tc.app

			out, report, err := MigrateApplications([]byte(src))
			if err != nil {
				t.Fatalf("MigrateApplications() error = %v", err)
			}
			if err := VerifyMigrated(out); err != nil {
				t.Fatalf("migrated config does not load: %v\n%s", err, out)
			}

			cfg, err := decodeConfig(out)
			if err != nil {
				t.Fatalf("decode migrated config: %v\n%s", err, out)
			}
			if got := nativeRunner(t, cfg, "web").Run; got != "pnpm start" {
				t.Errorf("runners.native.run = %q, want the native command converted anyway", got)
			}

			blocked := strings.Join(report.Blocked, "\n")
			for _, want := range tc.wants {
				if !strings.Contains(blocked, want) {
					t.Errorf("the leftover must mention %q, got:\n%s", want, blocked)
				}
			}
		})
	}
}

// TestMigrateApplicationsRefusesWhenThereIsNothingToStart: with no native command there
// is no entry to write, which is the one case where the application stays put. It also
// has to stay put byte for byte — a refusal that still edited the file would leave the
// operator writing the replacement against text the tool had already changed underneath
// them.
func TestMigrateApplicationsRefusesWhenThereIsNothingToStart(t *testing.T) {
	src := `version: "0.1.44"
applications:
  web:
    description: "docs only"
    run:
      docker: { service: web-js }
`
	out, report, err := MigrateApplications([]byte(src))
	if err != nil {
		t.Fatalf("MigrateApplications() error = %v", err)
	}

	if string(out) != src {
		t.Errorf("migration wrote an entry with nothing to start:\n%s", out)
	}
	if len(report.Changes) != 0 {
		t.Errorf("a refused migration still reported changes: %v", report.Changes)
	}
	if !strings.Contains(strings.Join(report.Blocked, "\n"), "nothing for a native runner to start") {
		t.Errorf("the refusal must say why, got: %v", report.Blocked)
	}
}

// TestMigrateApplicationsReportsTheUnreachableDockerBuild: resolveCommand sent the docker
// build path to resolveDockerCommand, which reads run.docker — so build.docker was
// declared, schema-checked, and never executed. Repeating the generic docker guidance for
// it would send someone to reproduce a command that never ran.
func TestMigrateApplicationsReportsTheUnreachableDockerBuild(t *testing.T) {
	_, report, err := MigrateApplications([]byte(`version: "0.1.44"
applications:
  api:
    run: "cargo run"
    build:
      native: "cargo build"
      docker: { service: api-builder }
`))
	if err != nil {
		t.Fatalf("MigrateApplications() error = %v", err)
	}

	joined := strings.Join(report.Changes, "\n")
	if !strings.Contains(joined, "never executed") {
		t.Errorf("build.docker must be reported as unreachable, got:\n%s", joined)
	}
	if strings.Contains(strings.Join(report.Blocked, "\n"), "build.docker") {
		t.Errorf("build.docker needs no hand-work, so it must not be listed as blocked: %v", report.Blocked)
	}
}

// TestMigrateApplicationsMigratesWhatItCanAndLeavesTheRest.
//
// All-or-nothing would let one untranslatable application block every other one in the
// file, and the operator would have to hand-migrate the lot to get any of it.
//
// What the leftover means changed with docs/43. It used to be that both sections loaded
// together, so a half-migrated file still ran. Now `applications:` is not in the schema
// at all, so the leftover is a file `dva config validate` rejects — pointing at exactly
// the entries the report said it could not convert, with the removedSchemaKeys guidance
// attached. Partial migration is still the right behaviour; it is a worklist now rather
// than a working file, and the report is what says so.
func TestMigrateApplicationsMigratesWhatItCanAndLeavesTheRest(t *testing.T) {
	got, cfg := migrateAppsAndDecode(t, `version: "0.1.44"
applications:
  api:
    run: "cargo run"
  web:
    description: "container only"
    run:
      docker: { service: web-js }
`)

	if _, ok := cfg.Stack["api"]; !ok {
		t.Errorf("the translatable application did not migrate:\n%s", got)
	}
	if _, ok := cfg.Stack["web"]; ok {
		t.Errorf("the untranslatable application was migrated anyway:\n%s", got)
	}

	// Read back through YAML rather than cfg: Config has no Applications field to decode
	// into any more, and a substring check on "web:" cannot tell the leftover section
	// from a stack entry of the same name.
	var raw map[string]any
	if err := yaml.Unmarshal([]byte(got), &raw); err != nil {
		t.Fatalf("migrated output is not YAML: %v\n%s", err, got)
	}
	leftover, ok := raw["applications"].(map[string]any)
	if !ok {
		t.Fatalf("the untranslatable application was deleted rather than left in place:\n%s", got)
	}
	if _, ok := leftover["web"]; !ok {
		t.Errorf("applications: survived without the entry that needed it:\n%s", got)
	}
	if _, ok := leftover["api"]; ok {
		t.Errorf("the migrated application is now declared twice:\n%s", got)
	}
}

// TestMigrateApplicationsReportsTheDroppedPort: port drove the port-reclaim check in
// 'dva app up'. That check is being removed with the application manager, so the field
// has nowhere to go — but it had behaviour, and dropping it without a word is the
// difference between a migration and a quiet deletion.
func TestMigrateApplicationsReportsTheDroppedPort(t *testing.T) {
	out, report, err := MigrateApplications([]byte(`version: "0.1.44"
applications:
  api:
    port: 11200
    run: "cargo run"
`))
	if err != nil {
		t.Fatalf("MigrateApplications() error = %v", err)
	}
	if strings.Contains(string(out), "port: 11200") {
		t.Errorf("port was carried into a config that has nothing to read it:\n%s", out)
	}

	joined := strings.Join(report.Changes, "\n")
	for _, want := range []string{"applications.api.port", "11200"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the report must name the dropped port; missing %q in:\n%s", want, joined)
		}
	}
}

// TestMigrateApplicationsReportsTheDroppedHealthRequired: `required` is the one health
// key with no target, and it is a worse quiet deletion than port. port is visibly absent
// from the migrated file, so an operator reading the output can see it went. Copying
// `required: true` through would leave it visibly *present* — and inert, because
// HealthCheckConfig has no such field and the entry-scoped health_checks schema declares
// no additionalProperties bound, so the dead key validates clean.
func TestMigrateApplicationsReportsTheDroppedHealthRequired(t *testing.T) {
	got, cfg := migrateAppsAndDecode(t, `version: "0.1.44"
applications:
  web:
    run: "./bin/web"
    health:
      type: http
      url: "http://localhost:8080/health"
      ready_timeout: 30
      required: true
`)
	if strings.Contains(got, "required:") {
		t.Errorf("required was carried into a config that has nothing to read it:\n%s", got)
	}

	// The rest of the check must survive intact — the drop is one key, not the section.
	check, ok := cfg.Stack["web"].HealthChecks["web"]
	if !ok {
		t.Fatalf("stack.web.health_checks.web did not decode:\n%s", got)
	}
	if check.Type != "http" || check.URL != "http://localhost:8080/health" || check.ReadyTimeout != 30 {
		t.Errorf("the surviving health keys did not reach the decoded check: %+v", check)
	}

	_, report, err := MigrateApplications([]byte(`version: "0.1.44"
applications:
  web:
    run: "./bin/web"
    health:
      type: http
      required: true
`))
	if err != nil {
		t.Fatalf("MigrateApplications() error = %v", err)
	}
	joined := strings.Join(report.Changes, "\n")
	for _, want := range []string{"applications.web.health.required", "strict readiness"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the report must name the dropped strictness; missing %q in:\n%s", want, joined)
		}
	}
}

// TestMigrateApplicationsRefusesToOverwriteAnExistingEntry: with stack.api already
// declared there is no way to tell which of the two is authoritative, and picking one
// silently discards the other.
func TestMigrateApplicationsRefusesToOverwriteAnExistingEntry(t *testing.T) {
	_, report, err := MigrateApplications([]byte(`version: "0.1.44"
stack:
  api:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
applications:
  api:
    run: "cargo run"
`))
	if err != nil {
		t.Fatalf("MigrateApplications() error = %v", err)
	}

	if len(report.Changes) != 0 {
		t.Fatalf("migration overwrote an existing stack entry: %v", report.Changes)
	}
	if !strings.Contains(strings.Join(report.Blocked, "\n"), "stack.api already exists") {
		t.Errorf("the refusal must say what it collided with, got: %v", report.Blocked)
	}
}

// TestMigrateApplicationsIgnoresAConfigWithoutThem: the command runs on every config, so
// the no-op path has to be byte-exact rather than a reformat.
func TestMigrateApplicationsIgnoresAConfigWithoutThem(t *testing.T) {
	src := `version: "0.1.44"

stack:
  infra:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
`
	out, report, err := MigrateApplications([]byte(src))
	if err != nil {
		t.Fatalf("MigrateApplications() error = %v", err)
	}
	if !report.Empty() {
		t.Errorf("reported changes for a config with no applications: %+v", report)
	}
	if string(out) != src {
		t.Errorf("a config with nothing to migrate was rewritten:\n%s", out)
	}
}
