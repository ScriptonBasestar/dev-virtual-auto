package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

// entryLogFile is where the process and script plugins write an entry's output.
func entryLogFile(c *config.Config, name string) string {
	return filepath.Join(c.FileDir(), config.DotDirName, config.LogsDirName, name+".log")
}

// showEntryLogFile prints the tail of a non-compose entry's log file.
//
// Lifted out of stack.go, where it was showStackEntryLog: `dva logs <plan> <entry>` needs
// exactly this and stack.go is being removed, so the reader moves ahead of its old caller
// rather than being copied and then deleted twice.
func showEntryLogFile(c *config.Config, name string) error {
	logFile := entryLogFile(c, name)
	data, err := os.ReadFile(logFile)
	if err != nil {
		return fmt.Errorf("no log file for entry %q: %w", name, err)
	}

	lines := strings.Split(string(data), "\n")
	start := 0
	if len(lines) > 100 {
		start = len(lines) - 100
	}
	for _, line := range lines[start:] {
		fmt.Println(line)
	}
	return nil
}

// planLogTarget is one entry of a plan that produces logs, with what is needed to reach them.
//
// Built from the plan's resolved runner rather than from the stack declaration: an entry may
// declare several runners and the plan picked one, so reading the declaration could route to
// a runner this plan never started.
type planLogTarget struct {
	name     string
	runner   string
	compose  *config.ComposePluginConfig // nil unless the plan runs this entry under compose
	services []string                    // compose service subset the plan selected, if any
}

// planLogTargets lists the plan's entries whose logs DVA knows how to reach, in plan order.
//
// Runners that are absent from the switch are absent from the result: helm, kubectl and the
// rest hand their logs to their own tool, and there is no file or compose project for DVA to
// read. Naming them in an error beats routing them to the primary compose project, which is
// what the stack path did — for a kubectl entry that answered with some other entry's logs
// and said nothing about the substitution.
func planLogTargets(plan *lifecycle.ExecutionPlan) []planLogTarget {
	var targets []planLogTarget
	for _, entry := range plan.Entries {
		switch cfg := entry.RunnerConfig.(type) {
		case *config.ComposePluginConfig:
			targets = append(targets, planLogTarget{
				name: entry.Name, runner: entry.Runner, compose: cfg, services: entry.Services,
			})
		case *config.NativeRunnerConfig, *config.ProcessPluginConfig, *config.ScriptPluginConfig:
			targets = append(targets, planLogTarget{name: entry.Name, runner: entry.Runner})
		}
	}
	return targets
}

// showPlanEntryLogs routes one target to whatever owns its output.
func showPlanEntryLogs(e *config.Environment, c *config.Config, target planLogTarget, passthrough []string) error {
	if target.compose == nil {
		// The file reader prints a fixed tail and cannot follow, so anything left over here
		// would be accepted and ignored — `dva logs p api -f` would print 100 lines and stop,
		// looking like a stream that ended. Name the file instead so `tail -f` is one step away.
		if len(passthrough) > 0 {
			return fmt.Errorf("entry %q runs under %q, whose logs are the file %s; it takes no extra arguments, got %s\n"+
				"       → tail -f %s",
				target.name, target.runner, entryLogFile(c, target.name),
				strings.Join(passthrough, " "), entryLogFile(c, target.name))
		}
		return showEntryLogFile(c, target.name)
	}

	entry := &config.LifecycleEntry{Name: target.name, Compose: target.compose}
	return execComposePassthroughForEntry(e, c, entry, planComposeLogArgs(target, passthrough))
}

// planComposeLogArgs builds the compose argv for one target.
//
// The plan's service subset goes on as compose's own positional filter, which is what makes
// `dva logs <plan>` show the services the plan started rather than every service in the
// compose file. An explicit passthrough replaces it rather than adding to it: compose reads
// positionals as the service list, so appending the subset to a caller's own service name
// would widen the selection past what was asked for, and appending it to `-f` would name
// services the caller deliberately left out.
func planComposeLogArgs(target planLogTarget, passthrough []string) []string {
	args := append([]string{config.LogsDirName}, passthrough...)
	if len(passthrough) == 0 {
		args = append(args, target.services...)
	}
	return args
}

// runPlanLogs shows logs for a plan, or for one named entry of it.
//
// Unlike the lifecycle verbs this does not go through parsePlanFlags. Everything after the
// optional entry name belongs to the tool that owns the logs — `-f`, `--tail 50`, a service
// name — and parsePlanFlags exists to reject exactly that.
func runPlanLogs(c *config.Config, e *config.Environment, planName string, extraArgs []string) error {
	plan, err := lifecycle.ResolvePlan(c, planName, nil)
	if err != nil {
		return err
	}
	e.MergeVars(plan.EnvVars)

	targets := planLogTargets(plan)
	if len(targets) == 0 {
		return fmt.Errorf("plan %q has no entry whose logs dva can reach; its runners hand logging to their own tools", planName)
	}

	// Only the first token is ever the entry-name slot, and only when it is not a flag —
	// the same reading detectPlanRoute gives the plan-name slot one position earlier, so a
	// flag value is never mistaken for a name. isFlagToken is what makes that one reading
	// rather than two spellings of it; see the same line in runPlanBuild. TASK-218.
	if len(extraArgs) > 0 && !isFlagToken(extraArgs[0]) {
		for _, target := range targets {
			if target.name == extraArgs[0] {
				return showPlanEntryLogs(e, c, target, extraArgs[1:])
			}
		}
		// Not an error yet: on a compose plan the token is far more likely a service name,
		// which is what the single-target path below forwards it as. Only an ambiguous plan
		// turns it into a dead end, and that error is raised below where the ambiguity is.
	}

	if len(targets) == 1 {
		return showPlanEntryLogs(e, c, targets[0], extraArgs)
	}

	return fmt.Errorf("plan %q runs %d entries with logs; name one: dva logs %s <%s>",
		planName, len(targets), planName, strings.Join(planLogTargetNames(targets), "|"))
}

// planLogTargetNames lists the selectable names in a stable order, for the message above.
func planLogTargetNames(targets []planLogTarget) []string {
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		names = append(names, target.name)
	}
	sort.Strings(names)
	return names
}
