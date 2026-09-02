package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestImportedPlanLifecycleParity(t *testing.T) {
	root, child := loadImportedPlanCLIFixture(t)
	rootEnv := config.NewEnvironment(nil, root.FileDir(), root.FileDir())

	if err := runPlanUp(root, planEnv(rootEnv), "child/dev", []string{"--var", "CLI_VALUE=cli"}); err != nil {
		t.Fatalf("imported up: %v", err)
	}
	for _, name := range []string{"first-up", "second-up"} {
		data, err := os.ReadFile(filepath.Join(child, name))
		if err != nil {
			t.Fatalf("read child %s: %v", name, err)
		}
		for _, want := range []string{"child", "child-env", "child-site", "child-file", "cli"} {
			if !strings.Contains(string(data), want) {
				t.Errorf("%s = %q, missing child-owned value %q", name, data, want)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(root.FileDir(), "parent-up")); !os.IsNotExist(err) {
		t.Fatalf("parent stack executed for imported up: %v", err)
	}

	if err := runPlanDown(root, planEnv(rootEnv), "child/dev", nil); err != nil {
		t.Fatalf("imported down: %v", err)
	}
	downOrder, err := os.ReadFile(filepath.Join(child, "down-order"))
	if err != nil {
		t.Fatalf("read down order: %v", err)
	}
	if got := strings.Fields(string(downOrder)); strings.Join(got, ",") != "second,first" {
		t.Fatalf("down order = %v, want reverse dependency order [second first]", got)
	}

	if err := runPlanUp(root, planEnv(rootEnv), "child/dev", nil); err != nil {
		t.Fatalf("second imported up: %v", err)
	}
	if err := runPlanStop(root, planEnv(rootEnv), "child/dev", nil); err != nil {
		t.Fatalf("imported stop: %v", err)
	}
	if _, err := os.Stat(filepath.Join(child, "second-stop")); err != nil {
		t.Fatalf("child stop did not run: %v", err)
	}
	if err := runPlanRestart(root, planEnv(rootEnv), "child-dev", nil); err != nil {
		t.Fatalf("imported alias restart: %v", err)
	}
	if err := runPlanStatus(root, planEnv(rootEnv), "child/dev"); err != nil {
		t.Fatalf("imported status: %v", err)
	}

	writeEntryLog(t, root, "first", "parent-log-only")
	owner := root.Plans["child/dev"].OwnerConfig(root)
	writeEntryLog(t, owner, "first", "child-log-only")
	var logErr error
	logOutput := captureStdout(t, func() {
		logErr = runPlanLogs(root, planEnv(rootEnv), "child/dev", []string{"first"})
	})
	if logErr != nil {
		t.Fatalf("imported logs: %v", logErr)
	}
	if !strings.Contains(logOutput, "child-log-only") || strings.Contains(logOutput, "parent-log-only") {
		t.Fatalf("imported logs used wrong owner:\n%s", logOutput)
	}

	if err := runPlanBuild(root, planEnv(rootEnv), "child/build", nil); err != nil {
		t.Fatalf("imported build: %v", err)
	}
	builtFrom, err := os.ReadFile(filepath.Join(child, "app", "build-dir"))
	if err != nil {
		t.Fatalf("read build working directory: %v", err)
	}
	gotDir, err := filepath.EvalSymlinks(strings.TrimSpace(string(builtFrom)))
	if err != nil {
		t.Fatalf("resolve build directory: %v", err)
	}
	wantDir, err := filepath.EvalSymlinks(filepath.Join(child, "app"))
	if err != nil {
		t.Fatalf("resolve child app root: %v", err)
	}
	if gotDir != wantDir {
		t.Fatalf("build directory = %q, want child app root %q", gotDir, wantDir)
	}
}

func TestImportedPlanUsesOwnerHooksAndEndpoints(t *testing.T) {
	root, child := loadImportedPlanCLIFixture(t)
	oldCfg, oldEnv, oldDryRun, oldJSON := cfg, env, dryRun, jsonOutput
	cfg, env, dryRun, jsonOutput = root, nil, false, false
	t.Cleanup(func() {
		cfg, env, dryRun, jsonOutput = oldCfg, oldEnv, oldDryRun, oldJSON
	})

	up := &cobra.Command{RunE: func(_ *cobra.Command, args []string) error {
		planName, extra, ok := detectPlanRoute(root, args)
		if !ok {
			t.Fatalf("up did not detect imported plan in %v", args)
		}
		return runPlanUp(root, rootEnvLoad(root), planName, extra)
	}}
	wrapWithHooks("up", up)

	var upErr error
	stdout, _ := captureStreams(t, func() {
		upErr = up.RunE(up, []string{"child/dev"})
	})
	if upErr != nil {
		t.Fatalf("wrapped imported up: %v", upErr)
	}
	for _, marker := range []string{"child-before", "child-after"} {
		if _, err := os.Stat(filepath.Join(child, marker)); err != nil {
			t.Errorf("child hook marker %s missing: %v", marker, err)
		}
	}
	for _, marker := range []string{"parent-before", "parent-after"} {
		if _, err := os.Stat(filepath.Join(root.FileDir(), marker)); !os.IsNotExist(err) {
			t.Errorf("parent hook leaked into imported plan (%s): %v", marker, err)
		}
	}
	if !strings.Contains(stdout, "Child API") || strings.Contains(stdout, "Parent API") {
		t.Fatalf("endpoint output did not use child owner:\n%s", stdout)
	}
	firstUp, err := os.ReadFile(filepath.Join(child, "first-up"))
	if err != nil {
		t.Fatalf("read child output after before hook: %v", err)
	}
	if !strings.Contains(string(firstUp), "child-file") || strings.Contains(string(firstUp), "hook-changed") {
		t.Fatalf("built-in reloaded env_file after before hook: %q", firstUp)
	}

	jsonOutput = true
	var jsonErr error
	jsonText := captureStdout(t, func() {
		jsonErr = runPlanUp(root, rootEnvLoad(root), "child-dev", nil)
	})
	jsonOutput = false
	if jsonErr != nil {
		t.Fatalf("JSON imported alias up: %v", jsonErr)
	}
	var document planUpOutput
	if err := json.Unmarshal([]byte(jsonText), &document); err != nil {
		t.Fatalf("decode imported plan JSON: %v\n%s", err, jsonText)
	}
	if document.Plan != "child-dev" || len(document.Endpoints) != 1 || document.Endpoints[0].Label != "Child API" {
		t.Fatalf("imported plan JSON used wrong route or owner: %+v", document)
	}

	originalDownRan := false
	down := &cobra.Command{RunE: func(_ *cobra.Command, _ []string) error {
		originalDownRan = true
		return nil
	}}
	wrapWithHooks("down", down)
	if err := down.RunE(down, []string{"child/dev"}); err != nil {
		t.Fatalf("wrapped imported down replace: %v", err)
	}
	if originalDownRan {
		t.Fatal("child replace hook did not replace the built-in")
	}
	if _, err := os.Stat(filepath.Join(child, "child-replace")); err != nil {
		t.Fatalf("child replace hook did not run from child root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root.FileDir(), "parent-replace")); !os.IsNotExist(err) {
		t.Fatalf("parent replace hook leaked into imported plan: %v", err)
	}

	logs := &cobra.Command{RunE: func(_ *cobra.Command, args []string) error {
		filtered, err := consumeRootPersistentFlags(args)
		if err != nil {
			return err
		}
		planName, extra, ok := detectPlanRoute(root, filtered)
		if !ok {
			t.Fatalf("logs did not detect imported plan in %v", filtered)
		}
		return runPlanLogs(root, rootEnvLoad(root), planName, extra)
	}}
	wrapWithHooks(config.LogsDirName, logs)
	writeEntryLog(t, root.Plans["child/dev"].OwnerConfig(root), "first", "child-log")
	var logsErr error
	captureStdout(t, func() {
		logsErr = logs.RunE(logs, []string{"--json=true", "child-dev", "first"})
	})
	if logsErr != nil {
		t.Fatalf("wrapped imported logs with inline root flag: %v", logsErr)
	}
	if _, err := os.Stat(filepath.Join(child, "child-logs-before")); err != nil {
		t.Fatalf("child logs hook did not run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root.FileDir(), "parent-logs-before")); !os.IsNotExist(err) {
		t.Fatalf("parent logs hook leaked before imported alias route: %v", err)
	}
}

func TestImportedPlanInvalidOwnerFailsBeforeHook(t *testing.T) {
	root, child := loadImportedPlanCLIFixture(t)
	owner := root.Plans["child/dev"].OwnerConfig(root)
	delete(owner.Stack, "first")

	oldCfg, oldEnv := cfg, env
	cfg, env = root, nil
	t.Cleanup(func() { cfg, env = oldCfg, oldEnv })

	originalRan := false
	up := &cobra.Command{RunE: func(_ *cobra.Command, _ []string) error {
		originalRan = true
		return nil
	}}
	wrapWithHooks("up", up)
	err := up.RunE(up, []string{"child/dev"})
	if err == nil || !strings.Contains(err.Error(), `stack entry "first" not found`) {
		t.Fatalf("invalid imported plan error = %v, want missing child stack entry", err)
	}
	if originalRan {
		t.Fatal("invalid imported plan reached built-in command")
	}
	if _, statErr := os.Stat(filepath.Join(child, "child-before")); !os.IsNotExist(statErr) {
		t.Fatalf("invalid imported plan ran child before-hook: %v", statErr)
	}
}

func TestImportedPlanManifestPreservesSchema(t *testing.T) {
	root, child := loadImportedPlanCLIFixture(t)
	plans := buildManifestPlans(root)
	for _, name := range []string{"child/dev", "child-dev", "child/build"} {
		plan, ok := plans[name]
		if !ok {
			t.Errorf("manifest plan %q missing", name)
			continue
		}
		if plan.ResolutionError != "" {
			t.Errorf("manifest plan %q resolution error = %q", name, plan.ResolutionError)
		}
	}

	encoded, err := json.Marshal(plans)
	if err != nil {
		t.Fatalf("marshal manifest plans: %v", err)
	}
	text := string(encoded)
	for _, forbidden := range []string{root.FileDir(), child, `"owner"`, `"owner_config"`} {
		if strings.Contains(text, forbidden) {
			t.Errorf("manifest leaked internal owner data %q: %s", forbidden, text)
		}
	}
}

func loadImportedPlanCLIFixture(t *testing.T) (*config.Config, string) {
	t.Helper()
	parent := t.TempDir()
	child := filepath.Join(parent, "child")
	if err := os.MkdirAll(filepath.Join(child, "app"), 0o755); err != nil {
		t.Fatal(err)
	}

	parentYAML := `version: "0.1.0"
stack:
  first:
    default_runner: script
    runners:
      script:
        up: touch parent-up
plans:
  parent:
    entries: [{name: first}]
interaction:
  up:
    before: [{run: touch parent-before}]
    after: [{run: touch parent-after}]
  down:
    replace: [{run: touch parent-replace}]
  logs:
    before: [{run: touch parent-logs-before}]
endpoints:
  parent:
    label: Parent API
    url: http://parent.invalid
subprojects:
  child:
    path: child
    import:
      plans:
        - name: dev
          as: child-dev
        - name: build
`
	childYAML := `version: "0.1.0"
vars:
  OWNER: child
env_file: .env
environments:
  dev:
    environment:
      ENV_VALUE: child-env
sites:
  local:
    vars:
      SITE_VALUE: child-site
stack:
  first:
    default_runner: script
    health_checks:
      ready:
        type: command
        command: 'test "$OWNER" = child && grep -q "Child API" dva.yml'
    runners:
      script:
        up: 'printf "%s %s %s %s %s" "$OWNER" "$ENV_VALUE" "$SITE_VALUE" "$ENV_FILE_VALUE" "$CLI_VALUE" > first-up'
        down: 'echo first >> down-order'
        stop: 'touch first-stop'
  second:
    default_runner: script
    runners:
      script:
        up: 'printf "%s %s %s %s %s" "$OWNER" "$ENV_VALUE" "$SITE_VALUE" "$ENV_FILE_VALUE" "$CLI_VALUE" > second-up'
        down: 'echo second >> down-order'
        stop: 'touch second-stop'
  builder:
    default_runner: native
    runners:
      native:
        dir: app
        run: "true"
        build: 'pwd > build-dir'
plans:
  dev:
    environment: dev
    site: local
    endpoint_tags: [child]
    entries:
      - name: first
      - name: second
        depends_on: [first]
  build:
    entries: [{name: builder}]
interaction:
  up:
    before:
      - run:
          - touch child-before
          - printf "ENV_FILE_VALUE=hook-changed\\n" > .env
    after: [{run: touch child-after}]
  down:
    replace: [{run: touch child-replace}]
  logs:
    before: [{run: touch child-logs-before}]
endpoints:
  child:
    label: Child API
    url: http://child.invalid
    tags: [child]
`
	if err := os.WriteFile(filepath.Join(parent, config.FileName), []byte(parentYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, config.FileName), []byte(childYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, ".env"), []byte("ENV_FILE_VALUE=child-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	root, err := config.Load(parent)
	if err != nil {
		t.Fatalf("load imported-plan fixture: %v", err)
	}
	return root, child
}
