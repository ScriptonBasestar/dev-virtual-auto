package runner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// DockerComposeRunner executes commands via Docker Compose.
type DockerComposeRunner struct {
	Cmd  *ResolvedCommand
	Opts RunOptions

	detectedProject string
}

// Execute builds and runs the docker compose command.
func (r *DockerComposeRunner) Execute(env *config.Environment) error {
	cmd := r.Cmd

	// steps: run each step as a separate docker compose exec
	if len(cmd.Steps) > 0 {
		return r.executeSteps(env, cmd.Steps)
	}

	// script/script_file in docker context: not supported natively;
	// fall back to local execution as a convenience.
	if cmd.Script != "" || cmd.ScriptFile != "" {
		local := &LocalRunner{Cmd: r.Cmd, Opts: r.Opts}
		return local.Execute(env)
	}

	// Auto-detect if container is running → switch run to exec
	r.autoDetectComposeMethod(env)

	return execCompose(env, r.Opts.Config, r.detectedProject, r.executeArgs(env))
}

// executeArgs builds the tail Execute hands to execCompose: profiles, the subcommand, and its
// arguments. The shared prefix — binary, -f files, --project-name — comes from
// dvaexec.ComposeArgv and is deliberately not repeated here (TASK-132).
//
// Split out of Execute so the argv is observable without running docker. Execute itself ends
// in syscall.Exec, so before this there was no way to assert on what it had assembled.
//
// composeProfiles must be called before Method is read: it rewrites Method to "up" when
// profiles are configured. That ordering is load-bearing, not incidental.
func (r *DockerComposeRunner) executeArgs(env *config.Environment) []string {
	var args []string
	args = append(args, r.composeProfiles()...)
	args = append(args, r.Cmd.Compose.Method)
	args = append(args, r.composeArguments(env)...)
	return args
}

// executeSteps runs each step as a separate docker compose exec command.
// Does NOT mutate r.Cmd; constructs args independently per command.
func (r *DockerComposeRunner) executeSteps(env *config.Environment, steps []config.ProvisionItem) error {
	// Ensure container state is detected once up front.
	r.autoDetectComposeMethod(env)

	return runStepLoop(env, r.Opts.Config, steps, func(cmds []string) error {
		for _, c := range cmds {
			c = strings.TrimSpace(c)
			if c == "" {
				continue
			}
			args := r.buildStepArgs(env, c)
			// execComposeStep, not execCompose: the latter replaces the process, which
			// would make this loop's second iteration unreachable (TASK-091).
			if err := execComposeStep(env, r.Opts.Config, r.detectedProject, args); err != nil {
				return err
			}
		}
		return nil
	})
}

// buildStepArgs builds docker compose exec args for a single command string.
// Does NOT mutate r.Cmd state.
//
// Emits no --project-name: the detected project reaches the argv through execComposeStep, which
// hands it to the one builder that writes the flag (TASK-132). This used to add its own copy on
// top of ComposeArgv's, so a config project_name plus a detected project produced the flag twice.
//
// Takes an Environment so a step's argv carries -e, the same as a non-step. It was created
// with that parameter, never read it, and lost it to an unparam finding; TASK-129 restored it
// with a body that uses it, so a step and the same command run outside `steps:` now hand the
// container the same environment. Steps always build `exec`, which accepts -e.
//
// env still reaches the docker CLI process separately, via execComposeStep — that is the CLI's
// own environment, used for ${VAR} substitution in compose files, and is not the container's.
// -e is the only mechanism that crosses that boundary.
func (r *DockerComposeRunner) buildStepArgs(env *config.Environment, cmd string) []string {
	var args []string
	// Always use exec for steps (container must be running)
	args = append(args, "exec")
	// Declared environment (exec accepts -e; see composeMethodAcceptsEnv)
	args = append(args, r.envVars(env)...)
	// User / workdir overrides
	if r.Cmd.User != "" {
		args = append(args, "--user", r.Cmd.User)
	}
	if r.Cmd.Workdir != "" {
		args = append(args, "--workdir", r.Cmd.Workdir)
	}
	// Service
	args = append(args, r.Cmd.Service)
	// Command
	if r.Cmd.Shell {
		args = append(args, "sh", "-c", cmd)
	} else {
		args = append(args, dvaexec.SplitCommand(cmd)...)
	}
	return args
}

func (r *DockerComposeRunner) composeProfiles() []string {
	if len(r.Cmd.Compose.Profiles) == 0 {
		return nil
	}

	// When using profiles, method must be "up" and command is cleared
	r.Cmd.Compose.Method = "up"
	r.Cmd.Command = ""
	r.Cmd.Compose.RunOptions = nil

	var args []string
	for _, p := range r.Cmd.Compose.Profiles {
		args = append(args, "--profile", p)
	}
	return args
}

func (r *DockerComposeRunner) composeArguments(env *config.Environment) []string {
	var argv []string

	// Run options
	argv = append(argv, r.Cmd.Compose.RunOptions...)

	method := r.Cmd.Compose.Method
	// -e goes on every method that accepts it, not just `run`. It used to share the `run`
	// guard with --publish and --rm, which do not exist on `exec` — so the environment was
	// withheld from a path that supports it, and autoDetectComposeMethod's run→exec rewrite
	// silently changed what the command saw depending on container uptime (TASK-129).
	if composeMethodAcceptsEnv(method) {
		argv = append(argv, r.envVars(env)...)
	}
	if method == "run" {
		// Publish ports
		for _, p := range r.Opts.Publish {
			argv = append(argv, "--publish="+p)
		}
		argv = append(argv, "--rm")
	}

	// User and workdir
	if r.Cmd.User != "" {
		argv = append(argv, "--user", r.Cmd.User)
	}
	if r.Cmd.Workdir != "" {
		argv = append(argv, "--workdir", r.Cmd.Workdir)
	}

	// Service name
	argv = append(argv, r.Cmd.Service)

	// Command and args
	cmd := strings.TrimSpace(r.Cmd.Command)
	if cmd != "" {
		cArgs := commandArgs(r.Cmd)
		if r.Cmd.Shell {
			// Wrap with sh -c for shell mode
			fullCmd := cmd
			if len(cArgs) > 0 {
				fullCmd = fullCmd + " " + strings.Join(cArgs, " ")
			}
			argv = append(argv, "sh", "-c", fullCmd)
		} else {
			argv = append(argv, dvaexec.SplitCommand(cmd)...)
			argv = append(argv, cArgs...)
		}
	}

	return argv
}

// composeMethodAcceptsEnv reports whether a compose subcommand takes -e/--env.
//
// Measured against Docker 29.5.3: `run` and `exec` both parse `-e, --env stringArray`;
// `up` does not (34 flags parsed, zero of them env), and composeProfiles switches the
// method to `up` whenever profiles are configured. Passing -e there would abort the
// invocation on an unknown flag, so the check is by supported flag, not by path.
func composeMethodAcceptsEnv(method string) bool {
	return method == "run" || method == "exec"
}

// envVars renders the environment as `-e KEY=VALUE` pairs for the container.
//
// The set is env.Vars minus the DVA_ prefix — the whole merged variable set, without regard
// to which layer produced a key: env_file, global `vars`, `environment:` profiles, site vars,
// plan vars, `--var` (lifecycle/resolver.go merges those five into Plan.EnvVars) and an
// interaction command's own `environment:`.
//
// That is still bounded by what was written down. MergeVars lets an OS value win for a key,
// but only for keys it was already given, so an undeclared host variable never enters
// env.Vars and never crosses. This forwards the declared environment, never os.Environ().
//
// The DVA_ skip covers every runtime var DVA injects (DVA_OS, DVA_WORK_DIR_REL_PATH,
// DVA_CURRENT_USER, DVA_CURRENT_UID, DVA_HOOK_DEPTH): they exist for dva's own
// interpolation and mean nothing inside a container.
//
// Until TASK-129 this also required os.Getenv(k) != "", so a variable declared only in
// dva.yml never crossed — which made examples/DISCOURSE.md's `RAILS_ENV: test` inert and
// disagreed with internal/lifecycle/docker.go, which forwards a declared env: unfiltered.
//
// Keys are sorted because map iteration order is not, and argv has to be reproducible.
func (r *DockerComposeRunner) envVars(env *config.Environment) []string {
	if env == nil {
		return nil
	}
	keys := make([]string, 0, len(env.Vars))
	for k := range env.Vars {
		if strings.HasPrefix(k, config.EnvPrefix) {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	args := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, env.Vars[k]))
	}
	return args
}

func (r *DockerComposeRunner) autoDetectComposeMethod(env *config.Environment) {
	if r.Cmd.Compose.Method != "run" {
		return
	}
	if r.Cmd.Service == "" {
		return
	}

	// Check if container is already running
	project := r.serviceRunningProject(env)
	if project != "" {
		r.Cmd.Compose.Method = "exec"
		r.detectedProject = project
		// Remove --rm from run_options
		var filtered []string
		for _, o := range r.Cmd.Compose.RunOptions {
			if !strings.Contains(o, "--rm") {
				filtered = append(filtered, o)
			}
		}
		r.Cmd.Compose.RunOptions = filtered
	}
}

// serviceRunningProject checks if a service has a running container and returns
// its Docker Compose project name. Empty string = not running.
//
// Goes through composeArgv, the same builder every other compose call uses. It used to shell
// out to a bare `docker compose ps` — no -f, no --project-name, and a hardcoded `docker` — so
// it asked about whatever project the CWD happened to imply and the answer was used on the
// configured one. With the compose file reached through `files:` rather than sitting in the
// working directory, the bare command exits 1 with "no configuration file provided", detection
// reports nothing, and `dva run` starts a throwaway `run --rm` container instead of exec'ing
// into the one that is up — succeeding, in the wrong container, with --rm deleting the evidence
// (TASK-133).
//
// An error still means "not running". That is right for a service that is merely down, and it
// is also what happens when the configured compose binary does not accept these flags; falling
// back to `run` is the safe answer either way.
func (r *DockerComposeRunner) serviceRunningProject(env *config.Environment) string {
	cmd, args, err := r.detectArgv(env)
	if err != nil {
		return ""
	}
	out, err := dvaexec.ExecSubprocessOutput(cmd, args...)
	if err != nil || out == "" {
		return ""
	}
	// Return first line (project name)
	lines := strings.Split(out, "\n")
	if len(lines) > 0 && lines[0] != "" {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

// detectArgv builds the `ps` invocation serviceRunningProject runs. Split out so the argv can
// be asserted on without a docker daemon — the defect it exists to prevent is entirely in which
// project the command names, which is visible in the argv and nowhere else.
//
// No project override is passed: detection is what produces one, so there is nothing to
// override yet. The config's own project_name is what the query has to use, because that is the
// project `dva up` would have created.
func (r *DockerComposeRunner) detectArgv(env *config.Environment) (string, []string, error) {
	return composeArgv(env, r.Opts.Config, "", []string{
		"ps", "--filter", "status=running", "--format", "{{.Project}}", r.Cmd.Service,
	})
}
