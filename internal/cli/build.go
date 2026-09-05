package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

// planBuildTarget is one entry of a plan that has something to build.
//
// Read from the plan's resolved runner rather than the stack declaration, for the reason
// planLogTarget is: an entry may declare several runners and the plan started one of them.
type planBuildTarget struct {
	name     string
	runner   string
	compose  *config.ComposePluginConfig // nil unless the plan runs this entry under compose
	services []string                    // compose service subset the plan selected, if any
	command  string                      // native: runners.native.build, run by a shell
	dir      string                      // native: runners.native.dir, before resolution
	vars     map[string]string           // the entry's resolved vars, runners.native.env included
}

// planBuildTargets lists the plan's entries that have something to build, in plan order.
//
// Two runners qualify and no others. compose has `docker compose build`; native has
// runners.native.build. Every other runner config in config/lifecycle.go carries no build
// field at all — runners.docker decodes to DockerPluginConfig, which names an image to pull
// and has nothing to build from — so there is no third case to write.
//
// A native entry with no build command is absent rather than an error: it is an entry that
// does not need building, so `dva build <plan>` on a plan of one compose entry and three
// plain `run` entries builds the one thing and says nothing about the other three.
func planBuildTargets(plan *lifecycle.ExecutionPlan) []planBuildTarget {
	var targets []planBuildTarget
	for _, entry := range plan.Entries {
		switch cfg := entry.RunnerConfig.(type) {
		case *config.ComposePluginConfig:
			targets = append(targets, planBuildTarget{
				name: entry.Name, runner: entry.Runner, compose: cfg, services: entry.Services,
			})
		case *config.NativeRunnerConfig:
			if cfg.Build == "" {
				continue
			}
			targets = append(targets, planBuildTarget{
				name: entry.Name, runner: entry.Runner,
				command: cfg.Build, dir: cfg.Dir, vars: entry.Vars,
			})
		}
	}
	return targets
}

// planComposeBuildArgs builds the compose argv for one target.
//
// Same precedence as planComposeLogArgs and for the same reason: compose reads positionals
// as the service list, so the plan's subset and an explicit one cannot both be appended.
// Building more services than the plan runs is the wider mistake of the two, so an explicit
// selection replaces the subset rather than adding to it.
//
// As for logs, a flag alone (`--no-cache`) keeps the subset: it says how to build, not
// what. `buildable` narrows the subset to the services the compose files give a `build:`;
// nil means the files could not be read and the subset goes through as declared (TASK-314).
func planComposeBuildArgs(target planBuildTarget, passthrough []string, buildable map[string]bool) []string {
	args := append([]string{"build"}, passthrough...)
	if !passthroughNamesServices(passthrough, composeBuildValueFlags) {
		for _, svc := range target.services {
			if known, ok := buildable[svc]; ok && !known {
				continue // declared in the compose file, and image-only: `compose build` would refuse it
			}
			args = append(args, svc)
		}
	}
	return args
}

// composeBuildValueFlags are the `compose build` flags that consume the next token.
var composeBuildValueFlags = map[string]bool{
	"--build-arg": true, "--builder": true, "--memory": true, "-m": true,
	"--progress": true, "--ssh": true,
}

// composeBuildableServices maps every service the entry's compose files declare to whether
// it carries a `build:`. Files are resolved the way ComposeArgv resolves them, so the map
// describes the same project the build command will address. Returns nil when no file was
// readable, so the caller cannot mistake "unknown" for "nothing builds".
func composeBuildableServices(e *config.Environment, c *config.Config, cc *config.ComposePluginConfig) map[string]bool {
	if cc == nil {
		return nil
	}
	var out map[string]bool
	for _, f := range cc.Files {
		f = e.Interpolate(f)
		if !filepath.IsAbs(f) {
			f = filepath.Join(c.FileDir(), f)
		}
		services, ok := extractComposeBuildable(f)
		if !ok {
			continue
		}
		if out == nil {
			out = map[string]bool{}
		}
		for name, hasBuild := range services {
			out[name] = out[name] || hasBuild
		}
	}
	return out
}

// buildComposeTarget runs `docker compose build` for one entry of a plan.
//
// Not execComposePassthroughForEntry, which is the obvious reuse and the wrong one: it ends
// in ExecReplace, so the DVA process becomes docker and never comes back. `logs` can afford
// that because it hands over once and streams until the user stops it. A plan build is a
// sequence, and the first compose entry would be the last thing that ran — the remaining
// entries would be silently skipped with rc 0 from docker.
func buildComposeTarget(e *config.Environment, c *config.Config, target planBuildTarget, passthrough []string) error {
	entry := &config.LifecycleEntry{Name: target.name, Compose: target.compose}
	args := planComposeBuildArgs(target, passthrough, composeBuildableServices(e, c, target.compose))
	if len(target.services) > 0 && len(args) == 1 && !passthroughNamesServices(passthrough, composeBuildValueFlags) {
		// Every selected service is image-only. A bare `compose build` here would build the
		// whole file — the services the plan does not run — so say so and do nothing.
		fmt.Fprintf(os.Stderr, "[build: %s] nothing to build — none of the plan's services (%s) declares build: in its compose file\n",
			target.name, strings.Join(target.services, ", "))
		return nil
	}
	composeCmd, composeArgs, err := buildComposeArgsForEntry(e, c, entry, args)
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] %s: %s %s\n", target.name, composeCmd, strings.Join(composeArgs, " "))
		return nil
	}
	if dvaexec.Debug {
		fmt.Fprintf(os.Stderr, "[debug] compose build [%s]: %s %v\n", target.name, composeCmd, composeArgs)
	}
	return dvaexec.ExecSubprocess(e, composeCmd, composeArgs, false)
}

// buildNativeTarget runs runners.native.build for one entry of a plan.
//
// This is the first code to execute that field. schema.json has advertised
// native_runner_config.build as "Build command" and decodeRunnerNode has decoded it since
// runners were introduced, but both native→process desugar points drop it, so until now it
// was a setting that parsed, validated, and did nothing (TASK-050 D0).
//
// `sh -c` in EntryDir with the entry's vars, which is exactly how the process plugin runs
// runners.native.run. Build and run are two commands about one piece of software; if they
// disagreed about the directory or the environment, the build would succeed against a
// different tree than the one that starts.
func buildNativeTarget(e *config.Environment, c *config.Config, target planBuildTarget, passthrough []string) error {
	if len(passthrough) > 0 {
		// The command is a string dva hands to a shell, not an argv it can extend: appending
		// `--no-cache` to `go build ./cmd/api` produces a command the user never wrote and
		// probably a build failure with dva's fingerprints on it.
		return fmt.Errorf("entry %q builds with the shell command %q, which dva runs as written; it takes no extra arguments, got %s\n"+
			"       → put them in runners.native.build",
			target.name, target.command, strings.Join(passthrough, " "))
	}

	dir := lifecycle.EntryDir(c.FileDir(), target.dir)

	// A copy per entry: MergeVars writes into the Environment it is given, so building two
	// native entries in one run would carry the first one's runners.native.env into the second.
	env := e.Clone()
	env.MergeVars(target.vars)

	if dryRun {
		// Interpolated, not as declared. ExecSubprocessInDir substitutes before handing the
		// string to sh, so printing the raw form would preview a command that differs from the
		// one that runs — on the single invocation whose entire purpose is to be that preview.
		fmt.Fprintf(os.Stderr, "[dry-run] %s: sh -c %q in %s\n", target.name, env.Interpolate(target.command), dir)
		return nil
	}

	// Interpolated here too, for the same reason: the echo is a record of what ran.
	fmt.Fprintf(os.Stderr, "  $ %s\n", env.Interpolate(target.command))
	return dvaexec.ExecSubprocessInDir(env, dir, target.command, nil, true)
}

// buildPlanEntry routes one target to whatever owns its build.
func buildPlanEntry(e *config.Environment, c *config.Config, target planBuildTarget, passthrough []string) error {
	if target.compose != nil {
		return buildComposeTarget(e, c, target, passthrough)
	}
	return buildNativeTarget(e, c, target, passthrough)
}

// runPlanBuild builds a plan, or one named entry of it.
//
// Like runPlanLogs this does not go through parsePlanFlags: everything after the optional
// entry name belongs to the tool doing the building — `--no-cache`, `--pull`, a service
// name — and parsePlanFlags exists to reject exactly that. The DVA flags were already taken
// by parseDvaFlags before the plan was chosen, which is how --dry-run reaches here.
func runPlanBuild(c *config.Config, el *envLoad, planName string, extraArgs []string) error {
	runtime, err := resolvePlanRuntime(c, el, planName, nil)
	if err != nil {
		return err
	}
	// Fail closed before the first child. The plan's own owner decides this — a root
	// env_file failure never reaches an imported plan, and vice versa (TASK-247 §3, §4).
	if err := runtime.report.Err(); err != nil {
		return err
	}
	plan, c, e := runtime.plan, runtime.config, runtime.env

	targets := planBuildTargets(plan)
	if len(targets) == 0 {
		return fmt.Errorf("plan %q has nothing to build: it runs no compose entry, and none of its "+
			"native entries declares runners.native.build", planName)
	}

	// Only the first token is ever the entry-name slot, and only when it is not a flag — the
	// same reading detectPlanRoute gives the plan-name slot one position earlier. That parity
	// is the point of the line, so it moved to isFlagToken when the plan-name slot did: a bare
	// strings.HasPrefix here would call a lone "-" a flag one position after the plan-name slot
	// had called it a name. Before TASK-218 this refused an entry named "-" while advertising
	// it in its own suggestion — `dva build multi <-|s2>`, measured 2026-08-21.
	if len(extraArgs) > 0 && !isFlagToken(extraArgs[0]) {
		for _, target := range targets {
			if target.name == extraArgs[0] {
				return buildPlanEntry(e, c, target, extraArgs[1:])
			}
		}
		// Not an error yet: on a single-target compose plan the token is far more likely a
		// service name, which the path below forwards as one.
	}

	if len(targets) == 1 {
		return buildPlanEntry(e, c, targets[0], extraArgs)
	}

	if len(extraArgs) > 0 {
		return fmt.Errorf("plan %q builds %d entries, so %s cannot be routed to one of them; name it: dva build %s <%s>",
			planName, len(targets), strings.Join(extraArgs, " "), planName, strings.Join(planBuildTargetNames(targets), "|"))
	}

	// Unqualified, this builds everything — the opposite of `dva logs <plan>`, which refuses.
	// Logs is a stream you watch, so answering with one arbitrary entry would be a guess at
	// which one you meant. A build is a batch that finishes, so building all of them is not a
	// guess: it is the whole of what was asked for.
	//
	// The first failure stops the rest. Entries are built in plan order, which is start order,
	// so a later entry may well consume what an earlier one produced; carrying on past a
	// failure would build against a tree that is known to be wrong.
	for _, target := range targets {
		fmt.Fprintf(os.Stderr, "[build: %s (%s)]\n", target.name, target.runner)
		if err := buildPlanEntry(e, c, target, nil); err != nil {
			return fmt.Errorf("entry %q: %w", target.name, err)
		}
	}
	return nil
}

// planBuildTargetNames lists the selectable names in a stable order, for the message above.
func planBuildTargetNames(targets []planBuildTarget) []string {
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		names = append(names, target.name)
	}
	sort.Strings(names)
	return names
}
