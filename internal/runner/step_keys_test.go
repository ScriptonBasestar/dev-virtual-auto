package runner

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// composeConfig returns a config whose compose command is the given binary. Passing `echo` makes
// every compose invocation print the argv it was handed instead of contacting docker, which is
// what lets these tests assert on the exact command a step produces.
//
// Safe in-process, unlike the single-command compose path: the step keys go through
// execComposeStep (ExecSubprocess), which returns. Only execCompose replaces the process.
func composeConfig(binary string) *config.Config {
	return &config.Config{
		Stack: map[string]*config.LifecycleEntry{
			"infra": {Compose: &config.ComposePluginConfig{Command: binary}},
		},
	}
}

// stepRunners returns every runner's executeSteps, keyed by runner name. The tables that drive
// step behaviour all build their runners here so a key handled by one runner and not another
// fails somewhere. That divergence, repeated key by key, is what produced TASK-083, TASK-085,
// TASK-089, TASK-091 and TASK-094 — five defects, one shape. Adding a fourth runner should mean
// adding one entry to this map, not remembering to edit three tables.
//
// The kubectl entry is only safe because kubectlShim rebuilds PATH and refuses to run unless
// `kubectl` resolves to the fake; /bin and /usr/bin stay on it so the other two runners still
// find sh, echo and true.
func stepRunners(t *testing.T, cfg *config.Config) map[string]func(*config.Environment, []config.ProvisionItem) error {
	t.Helper()
	kubectlShim(t)
	return map[string]func(*config.Environment, []config.ProvisionItem) error{
		"local": (&LocalRunner{
			Cmd: &ResolvedCommand{}, Opts: RunOptions{Config: cfg},
		}).executeSteps,
		"docker_compose": (&DockerComposeRunner{
			Cmd: &ResolvedCommand{}, Opts: RunOptions{Config: cfg},
		}).executeSteps,
		"kubectl": (&KubectlRunner{
			Cmd: &ResolvedCommand{Pod: "steps-pod"}, Opts: RunOptions{Config: cfg},
		}).executeSteps,
	}
}

// TestComposeKeysOnInteractionPath covers TASK-085. Both runners handled exactly two of
// ProvisionItem's seven payload keys — `note:` and `run:`. An interaction step written with
// `compose_up:`, `compose_exec:`, `compose_run:`, `echo:` or `cmd:` fell through their
// `len(cmds) == 0` test and `continue`d, producing zero bytes of output and exit 0, while the
// identical item under `provision:` did the obvious thing.
//
// The five keys are runner-independent, so the table drives both runners through all of them:
// implementing a key in one runner and not the other is the failure mode that produced this
// task, TASK-086, TASK-089 and TASK-091 in the first place.
func TestComposeKeysOnInteractionPath(t *testing.T) {
	env := &config.Environment{}
	cfg := composeConfig("echo")

	runners := stepRunners(t, cfg)

	// The compose expectations below open with "\n", which anchors them to the start of the
	// echoed line. `command: echo` is one word, so the correct argv is `echo up -d …` with no
	// "compose" in it — these cases used to expect `compose up -d …` and passed, because the
	// builder kept a "compose" seed the user had replaced. The anchor is what makes the
	// absence assertable: without it, a resurrected seed still satisfies Contains. TASK-115.
	cases := []struct {
		name string
		step config.ProvisionItem
		want []string
	}{
		{
			name: "compose_up starts the named services",
			step: config.ProvisionItem{Step: "start db", ComposeUp: []string{"postgres", "minio"}},
			want: []string{"start db", "\nup -d postgres minio"},
		},
		{
			name: "compose_exec runs a command in a service",
			step: config.ProvisionItem{Step: "wait for db", ComposeExec: "pg_isready -U app"},
			want: []string{"wait for db", "\nexec pg_isready -U app"},
		},
		{
			name: "compose_run runs a one-off command",
			step: config.ProvisionItem{Step: "migrate", ComposeRun: "migrate up"},
			want: []string{"migrate", "\nrun migrate up"},
		},
		{
			name: "echo prints its message",
			step: config.ProvisionItem{Step: "say something", Echo: "VIAECHO-SHOWN"},
			want: []string{"say something", "VIAECHO-SHOWN"},
		},
		{
			name: "cmd runs a shell command",
			step: config.ProvisionItem{Step: "legacy cmd", Cmd: "echo VIACMD-RAN"},
			want: []string{"legacy cmd", "VIACMD-RAN"},
		},
	}

	for name, executeSteps := range runners {
		t.Run(name, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					var err error
					out := captureStdout(t, func() {
						err = executeSteps(env, []config.ProvisionItem{tc.step})
					})
					if err != nil {
						t.Fatalf("executeSteps: %v (output %q)", err, out)
					}
					if out == "" {
						t.Fatal("zero bytes of output — the step was silently discarded, which is the defect")
					}
					for _, w := range tc.want {
						if !strings.Contains(out, w) {
							t.Errorf("missing %q in output %q", w, out)
						}
					}
					// These items carry a payload, so IsInert is false and no notice is due.
					// Printing one would trade a silent no-op for a false accusation.
					if strings.Contains(out, config.InertStepMessage) {
						t.Errorf("a step with a payload must not be reported inert; got %q", out)
					}
				})
			}
		})
	}

	// Ordering is part of the contract, not an accident: provision.go runs `run:` and then
	// prints `echo:`, so an item carrying both must do both, in that order. Local only —
	// the compose runner would route `run:` through `compose exec` with no service set.
	t.Run("local/run: and echo: on one item both happen, run: first", func(t *testing.T) {
		r := &LocalRunner{Cmd: &ResolvedCommand{}, Opts: RunOptions{Config: cfg}}
		var err error
		out := captureStdout(t, func() {
			err = r.executeSteps(env, []config.ProvisionItem{
				{Step: "both", Run: "echo RUN-FIRST", Echo: "ECHO-SECOND"},
			})
		})
		if err != nil {
			t.Fatalf("executeSteps: %v", err)
		}
		ri, ei := strings.Index(out, "RUN-FIRST"), strings.Index(out, "ECHO-SECOND")
		if ri < 0 || ei < 0 {
			t.Fatalf("both keys must take effect; got %q", out)
		}
		if ri > ei {
			t.Errorf("run: must execute before echo: prints; got %q", out)
		}
	})

	// The mirror of the above: a compose key stands in for the whole step. provision.go
	// returns immediately after each one, so a step that says "start postgres" must not also
	// run some other key that happens to be on the same item.
	t.Run("local/a compose key short-circuits the rest of the item", func(t *testing.T) {
		r := &LocalRunner{Cmd: &ResolvedCommand{}, Opts: RunOptions{Config: cfg}}
		var err error
		out := captureStdout(t, func() {
			err = r.executeSteps(env, []config.ProvisionItem{
				{Step: "compose wins", ComposeUp: []string{"postgres"}, Cmd: "echo MUST-NOT-RUN"},
			})
		})
		if err != nil {
			t.Fatalf("executeSteps: %v", err)
		}
		if !strings.Contains(out, "\nup -d postgres") {
			t.Errorf("the compose key must run; got %q", out)
		}
		if strings.Contains(out, "MUST-NOT-RUN") {
			t.Errorf("the compose key must short-circuit the item, as provision.go does; got %q", out)
		}
	})
}
