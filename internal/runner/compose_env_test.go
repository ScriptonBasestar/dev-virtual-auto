package runner

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// Nothing tested envVars (né runVars), composeArguments or buildStepArgs before TASK-129,
// which is how the compose runner shipped for so long refusing to hand a container any
// variable the user had only written down. envVars kept `if os.Getenv(k) == "" { continue }`,
// so a key had to be exported on the host *as well as* declared in dva.yml before it crossed —
// making examples/DISCOURSE.md's `RAILS_ENV: test` (examples/DISCOURSE.md:298-302) inert for
// anyone who did not already have RAILS_ENV in their shell. Meanwhile the -e injection sat
// inside `if method == "run"` alongside --publish/--rm, so autoDetectComposeMethod's run→exec
// rewrite silently dropped the environment depending on whether the container happened to be
// up, and buildStepArgs took an *config.Environment it never read.
//
// These cases pin the three halves of that fix and the one boundary it must not cross:
//   - a config-only variable reaches argv on `run` and on `exec`;
//   - `up` still gets no -e, because docker compose up does not parse the flag and would
//     abort on it (composeProfiles forces Method="up" whenever profiles are configured);
//   - steps carry the environment too, in an argv order docker compose exec accepts;
//   - DVA_-prefixed runtime vars stay out, and the output is sorted so argv is reproducible.
//
// No test here calls os.Setenv to make a variable cross — that would restore by hand the
// precondition the fix removed, and pass against the buggy code.

// composeEnvService is the service every case below runs against. It is a constant rather
// than a parameter because the service name is only ever used as an argv landmark — the cases
// locate it to check that the -e flags land before it.
const composeEnvService = "discourse"

// composeEnvRunner builds a runner for argv inspection only. composeArguments and
// buildStepArgs are pure builders, so nothing here contacts docker.
func composeEnvRunner(method, command string) *DockerComposeRunner {
	return &DockerComposeRunner{
		Cmd: &ResolvedCommand{
			Service: composeEnvService,
			Command: command,
			Compose: ComposeOpts{Method: method},
		},
	}
}

// declaredEnv builds an Environment holding variables that exist only in config, and
// guarantees none of them is exported on the host first. config.NewEnvironment is used
// rather than a struct literal because it also seeds the DVA_ runtime vars, which is what
// the prefix filter is measured against.
func declaredEnv(t *testing.T, vars map[string]string) *config.Environment {
	t.Helper()
	for k := range vars {
		clearHostVar(t, k)
	}
	return config.NewEnvironment(vars, "/tmp/dva-work", "/tmp/dva-work")
}

// clearHostVar removes key from this process's environment for the duration of the test and
// restores it afterwards. Environment.MergeVars lets an OS value win over a declared one, so
// a developer (or CI image) that happens to export RAILS_ENV would otherwise let the
// config-only cases below pass for exactly the reason they exist to rule out.
func clearHostVar(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "") // registers the restore; the unset below is what the test needs
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("could not clear %s from the host environment: %v", key, err)
	}
}

// envPayloads returns the value of every `-e` in argv, in argv order.
func envPayloads(argv []string) []string {
	var out []string
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == "-e" {
			out = append(out, argv[i+1])
		}
	}
	return out
}

// envIndex returns the index of the `-e` introducing key, or -1 when key never crosses.
func envIndex(argv []string, key string) int {
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == "-e" && strings.HasPrefix(argv[i+1], key+"=") {
			return i
		}
	}
	return -1
}

// TestComposeArgumentsForwardsConfigOnlyEnv is the regression proper. RAILS_ENV is never
// exported here — it exists only in the map a dva.yml `environment:` block would produce —
// and it must still reach the container on both methods that accept -e. Before TASK-129
// `run` dropped it for want of a host export and `exec` dropped it for being outside the
// `if method == "run"` guard, so neither column of this table crossed.
func TestComposeArgumentsForwardsConfigOnlyEnv(t *testing.T) {
	for _, method := range []string{"run", "exec"} {
		t.Run(method, func(t *testing.T) {
			env := declaredEnv(t, map[string]string{"RAILS_ENV": "test"})
			r := composeEnvRunner(method, "bundle exec rspec")

			argv := r.composeArguments(env)

			at := envIndex(argv, "RAILS_ENV")
			if at < 0 {
				t.Fatalf("`environment: {RAILS_ENV: test}` never reached argv on method %q, so the "+
					"container runs without it — this is examples/DISCOURSE.md:298-302 going inert.\nargv: %v",
					method, argv)
			}
			if got := argv[at+1]; got != "RAILS_ENV=test" {
				t.Errorf("the declared value was not forwarded verbatim: -e %q, want -e RAILS_ENV=test", got)
			}
			svc := slices.Index(argv, composeEnvService)
			if svc < 0 {
				t.Fatalf("the service name is missing from argv entirely: %v", argv)
			}
			if at > svc {
				t.Errorf("-e RAILS_ENV lands after the service name (index %d vs %d); docker compose %s "+
					"reads everything past the service as the container command, so the flag would be "+
					"passed to the program instead of to compose.\nargv: %v", at, svc, method, argv)
			}
		})
	}
}

// TestComposeArgumentsWithholdsEnvFromUp guards the boundary the fix must not cross.
// `docker compose up` parses 34 flags and none of them is env, so a single -e aborts the
// invocation on an unknown flag. Both routes to that method are covered: an explicitly
// configured `up`, and the profiles path, where composeProfiles rewrites Method itself and
// the caller never asked for `up` at all.
func TestComposeArgumentsWithholdsEnvFromUp(t *testing.T) {
	t.Run("method declared as up", func(t *testing.T) {
		env := declaredEnv(t, map[string]string{"RAILS_ENV": "test"})
		r := composeEnvRunner("up", "")

		assertNoEnvFlags(t, r.composeArguments(env), "up")
	})

	t.Run("up forced by profiles", func(t *testing.T) {
		env := declaredEnv(t, map[string]string{"RAILS_ENV": "test"})
		r := composeEnvRunner("run", "bundle exec rspec")
		r.Cmd.Compose.Profiles = []string{"dev"}

		// The real Execute order: composeProfiles rewrites Method to "up" before the
		// arguments are built, so a `run` command silently becomes an `up` one.
		profiles := r.composeProfiles()
		if got := r.Cmd.Compose.Method; got != "up" {
			t.Fatalf("composeProfiles left Method = %q; this test only means anything if it forces \"up\"", got)
		}

		assertNoEnvFlags(t, append(profiles, r.composeArguments(env)...), "up (via profiles)")
	})
}

func assertNoEnvFlags(t *testing.T, argv []string, label string) {
	t.Helper()
	if payloads := envPayloads(argv); len(payloads) > 0 {
		t.Errorf("method %s was given %d -e flag(s) (%v); `docker compose up` does not parse -e, "+
			"so the whole invocation would fail on an unknown flag rather than start anything.\nargv: %v",
			label, len(payloads), payloads, argv)
	}
}

// TestBuildStepArgsCarriesEnv covers the third half of TASK-129: buildStepArgs was handed an
// *config.Environment it never read, so a command written under `steps:` reached the container
// with a different environment than the same command written on its own. Steps always build
// `exec`, which accepts -e — the only question is whether the flags land where compose can
// still see them.
func TestBuildStepArgsCarriesEnv(t *testing.T) {
	env := declaredEnv(t, map[string]string{"RAILS_ENV": "test"})
	r := composeEnvRunner("run", "")

	argv := r.buildStepArgs(env, "bundle exec rake db:migrate")

	at := envIndex(argv, "RAILS_ENV")
	if at < 0 {
		t.Fatalf("a step's argv carries no environment, so `steps:` and a plain command hand the "+
			"container different environments for the same work.\nargv: %v", argv)
	}
	if got := argv[at+1]; got != "RAILS_ENV=test" {
		t.Errorf("the declared value was not forwarded verbatim: -e %q, want -e RAILS_ENV=test", got)
	}

	verb := slices.Index(argv, "exec")
	svc := slices.Index(argv, composeEnvService)
	cmd := slices.Index(argv, "bundle")
	if verb < 0 || svc < 0 || cmd < 0 {
		t.Fatalf("argv is missing the verb, the service or the command: %v", argv)
	}
	// docker compose exec [OPTIONS] SERVICE COMMAND — options only count between the two.
	if verb >= at || at >= svc || svc >= cmd {
		t.Errorf("argv order is exec=%d -e=%d service=%d command=%d, want strictly increasing; "+
			"an -e before the verb is not a flag of exec, and one after the service is an argument "+
			"to the program being run.\nargv: %v", verb, at, svc, cmd, argv)
	}
}

// TestEnvVarsExcludesDVAPrefix pins the one filter that survived. DVA_OS and friends exist for
// dva's own ${VAR} interpolation and mean nothing inside a container; DVA_HOOK_DEPTH is a
// recursion guard whose whole point is not to be inherited. The seeds come from
// config.NewEnvironment and WithHookDepth rather than from a literal map, so a future runtime
// variable added without the prefix fails here instead of leaking quietly.
func TestEnvVarsExcludesDVAPrefix(t *testing.T) {
	env := declaredEnv(t, map[string]string{"RAILS_ENV": "test"}).WithHookDepth()

	var seeded []string
	for k := range env.Vars {
		if strings.HasPrefix(k, config.EnvPrefix) {
			seeded = append(seeded, k)
		}
	}
	slices.Sort(seeded)
	// Without this the filter could be measured against nothing at all and still "pass".
	if len(seeded) < 2 {
		t.Fatalf("the Environment carries %d %s variable(s) (%v); this test cannot show that the "+
			"prefix filter works unless the constructor seeded some", len(seeded), config.EnvPrefix, seeded)
	}
	for _, want := range []string{config.EnvRuntimeOS, config.EnvHookDepthKey} {
		if _, ok := env.Vars[want]; !ok {
			t.Fatalf("%s is not in the Environment, so the case below proves nothing about it", want)
		}
	}

	r := composeEnvRunner("run", "bundle exec rspec")
	argv := r.composeArguments(env)

	payloads := envPayloads(argv)
	if len(payloads) == 0 {
		t.Fatalf("nothing crossed at all, so an empty result would satisfy the prefix check "+
			"vacuously.\nargv: %v", argv)
	}
	for _, p := range payloads {
		if strings.HasPrefix(p, config.EnvPrefix) {
			t.Errorf("%q was forwarded into the container; %s* is dva's own state — DVA_HOOK_DEPTH in "+
				"particular would let a nested dva inside the container skip its hooks.\nforwarded: %v",
				p, config.EnvPrefix, payloads)
		}
	}
	if !slices.Contains(payloads, "RAILS_ENV=test") {
		t.Errorf("the prefix filter took the declared variable with it; forwarded: %v", payloads)
	}
}

// TestEnvVarsIsSortedAndStable pins the sort added with TASK-129. env.Vars is a map, and Go
// randomises map iteration, so without it two identical `dva run` invocations produce
// different argv — which breaks --explain diffing, log comparison and any test that asserts on
// a whole command line.
func TestEnvVarsIsSortedAndStable(t *testing.T) {
	env := declaredEnv(t, map[string]string{
		"RAILS_ENV":    "test",
		"DATABASE_URL": "postgres://localhost/discourse",
		"REDIS_URL":    "redis://localhost:6379",
		"LANG":         "C.UTF-8",
		"BUNDLE_PATH":  "/bundle",
	})
	r := composeEnvRunner("run", "bundle exec rspec")

	first := envPayloads(r.composeArguments(env))
	if len(first) != 5 {
		t.Fatalf("5 declared variables produced %d -e flag(s) (%v); the ordering check below needs "+
			"all of them to cross", len(first), first)
	}
	if !slices.IsSorted(first) {
		t.Errorf("argv is not in sorted key order: %v", first)
	}

	// One run cannot distinguish "sorted" from "this seed happened to come out sorted".
	for i := range 20 {
		again := envPayloads(r.composeArguments(env))
		if !slices.Equal(first, again) {
			t.Fatalf("call %d produced a different argv than call 1, so the same dva.yml yields a "+
				"different command line run to run:\n  1: %v\n  %d: %v", i+2, first, i+2, again)
		}
	}
}

// TestEnvVarsNilEnvironment: Execute's callers are not all guaranteed to hand over an
// Environment, and an argv builder crashing is a worse failure than an empty environment.
func TestEnvVarsNilEnvironment(t *testing.T) {
	r := composeEnvRunner("run", "bundle exec rspec")

	if got := r.envVars(nil); got != nil {
		t.Errorf("envVars(nil) = %v, want nil", got)
	}
	// The same through the caller, since that is where a nil would actually arrive.
	argv := r.composeArguments(nil)
	if payloads := envPayloads(argv); len(payloads) > 0 {
		t.Errorf("a nil Environment produced -e flags from nowhere: %v", payloads)
	}
}

// TestEnvVarsExcludesUndeclaredHostVars pins the boundary that makes dropping the host-export
// filter safe to document as "declared values only". MergeVars can overwrite a key it was
// given; it never adds one. So a variable exported in the developer's shell and mentioned
// nowhere in dva.yml must not reach the container — otherwise "-e forwards the declared
// environment" would in practice mean "-e forwards the developer's shell", and secrets like
// AWS_SECRET_ACCESS_KEY would cross into every container dva touches.
func TestEnvVarsExcludesUndeclaredHostVars(t *testing.T) {
	t.Setenv("AWS_SECRET_ACCESS_KEY", "leaked-if-this-crosses")
	env := declaredEnv(t, map[string]string{"RAILS_ENV": "test"})
	r := composeEnvRunner("exec", "bundle exec rspec")

	payloads := envPayloads(r.composeArguments(env))
	if !slices.Contains(payloads, "RAILS_ENV=test") {
		t.Fatalf("the declared variable did not cross, so this case cannot show that the "+
			"undeclared one was filtered rather than everything being dropped: %v", payloads)
	}
	for _, p := range payloads {
		if strings.HasPrefix(p, "AWS_SECRET_ACCESS_KEY=") {
			t.Errorf("a host variable that appears nowhere in dva.yml was forwarded into the "+
				"container (%q); forwarding is meant to be bounded by what was declared.\nforwarded: %v",
				p, payloads)
		}
	}
}

// TestEnvVarsHostStillWinsForDeclaredKeys pins what TASK-129 did *not* change. Dropping the
// host-export requirement made a declared-only variable cross; it did not make config outrank
// the shell for a key the user declared and also exported. That precedence lives in
// Environment.MergeVars, and the runner forwarding whatever it settled on is the contract the
// doc comment on envVars claims.
func TestEnvVarsHostStillWinsForDeclaredKeys(t *testing.T) {
	t.Setenv("RAILS_ENV", "development")
	env := config.NewEnvironment(map[string]string{"RAILS_ENV": "test"}, "/tmp/dva-work", "/tmp/dva-work")
	r := composeEnvRunner("run", "bundle exec rspec")

	argv := r.composeArguments(env)

	at := envIndex(argv, "RAILS_ENV")
	if at < 0 {
		t.Fatalf("RAILS_ENV did not cross even with a host export present: %v", argv)
	}
	if got := argv[at+1]; got != "RAILS_ENV=development" {
		t.Errorf("-e %q, want -e RAILS_ENV=development — an exported shell value must still beat the "+
			"declared one for the same key", got)
	}
}
