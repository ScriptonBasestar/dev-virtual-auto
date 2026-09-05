package config

import (
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// modeFlagPattern matches a `dva … -M x` / `--mode x` / `--mode=x` invocation inside a
// shell line. The mode flag was removed with modes themselves; a command that still passes
// it fails at run time, which is exactly the kind of remnant a migration report exists to
// name before the author discovers it by running the interaction (TASK-317).
var modeFlagPattern = regexp.MustCompile(`\bdva\b[^\n|&;]*\s(-M|--mode)(=|\s)`)

// ReportLegacyFields lists fields that still parse but no longer do anything, each with
// the place its intent moved to.
//
// Every entry here is a field the schema rejects or a runtime ignores; none of them has a
// mechanical conversion, because each one moved to a section the author has to choose
// (which entry, which endpoint, which profile). Reporting them from the raw document,
// like the other steps, means the dotted path names the line the author will edit.
func ReportLegacyFields(src []byte) []string {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	root := doc.Content[0]
	var out []string
	out = append(out, legacyEnvironmentComposeFiles(root)...)
	out = append(out, legacyEnvFileKeys(root)...)
	out = append(out, legacyTopLevelHealthStart(root)...)
	out = append(out, legacyModeFlagRemnants(root)...)
	return out
}

// environments.<name>.compose_files: the environment schema has no such key. Compose
// files live on the entry, so the environment's stack_overrides is where a per-environment
// file list goes.
func legacyEnvironmentComposeFiles(root *yaml.Node) []string {
	envs := mapValue(root, "environments")
	if envs == nil || envs.Kind != yaml.MappingNode {
		return nil
	}
	var out []string
	for i := 0; i+1 < len(envs.Content); i += 2 {
		name, env := envs.Content[i].Value, envs.Content[i+1]
		if mapValue(env, "compose_files") == nil {
			continue
		}
		out = append(out, fmt.Sprintf(
			"environments.%s.compose_files: not read — compose files belong to the entry. Put the "+
				"list in stack.<entry>.runners.compose.files, or in environments.%s.stack_overrides.<entry>."+
				"runners.compose.files if only this environment uses it", name, name))
	}
	return out
}

// env_file: {priority: …, interpolate: …}: the object form takes `files` and the
// selection keys only; priority and interpolation are fixed by the load order.
func legacyEnvFileKeys(root *yaml.Node) []string {
	ef := mapValue(root, "env_file")
	if ef == nil || ef.Kind != yaml.MappingNode {
		return nil
	}
	var out []string
	for _, key := range []string{"priority", "interpolate"} {
		if mapValue(ef, key) == nil {
			continue
		}
		switch key {
		case "priority":
			out = append(out, "env_file.priority: not read — the order is fixed as environment: < env_file < OS "+
				"environment; delete the key and order the 'files' list from lowest to highest priority instead")
		case "interpolate":
			out = append(out, "env_file.interpolate: not read — ${VAR} expansion inside env files is always on; "+
				"delete the key")
		}
	}
	return out
}

// health_checks.<name>.start / start_hint at the top level: the plan path never runs a
// start command from a health check, so the process it was meant to launch never starts.
func legacyTopLevelHealthStart(root *yaml.Node) []string {
	hcs := mapValue(root, "health_checks")
	if hcs == nil || hcs.Kind != yaml.MappingNode {
		return nil
	}
	var out []string
	for i := 0; i+1 < len(hcs.Content); i += 2 {
		name, hc := hcs.Content[i].Value, hcs.Content[i+1]
		for _, key := range []string{"start", "start_hint"} {
			if mapValue(hc, key) == nil {
				continue
			}
			out = append(out, fmt.Sprintf(
				"health_checks.%s.%s: not run by 'dva up <plan>' — a top-level health check only waits. "+
					"Move the process into stack.%s.runners.native.command and the check into "+
					"stack.%s.health_checks so the plan starts it and then waits", name, key, name, name))
		}
	}
	return out
}

// legacyModeFlagRemnants finds `dva … --mode X` inside interaction commands and provision
// steps. `dva up X` is the replacement once modes.X became plans.X.
func legacyModeFlagRemnants(root *yaml.Node) []string {
	var out []string
	visit := func(path string, n *yaml.Node) {
		if n == nil || n.Kind != yaml.ScalarNode || !modeFlagPattern.MatchString(n.Value) {
			return
		}
		out = append(out, fmt.Sprintf(
			"%s: still passes --mode/-M, which 'dva' no longer accepts — run 'dva up <plan>' "+
				"(the plan the mode became) in its place", path))
	}

	var walkCmd func(path string, cmd *yaml.Node)
	walkCmd = func(path string, cmd *yaml.Node) {
		if cmd == nil || cmd.Kind != yaml.MappingNode {
			return
		}
		for _, key := range []string{"command", "script"} {
			visitScalarOrList(path+"."+key, mapValue(cmd, key), visit)
		}
		if steps := mapValue(cmd, "steps"); steps != nil && steps.Kind == yaml.SequenceNode {
			for i, step := range steps.Content {
				for _, key := range []string{"run", "cmd"} {
					visit(fmt.Sprintf("%s.steps[%d].%s", path, i, key), mapValue(step, key))
				}
			}
		}
		if subs := mapValue(cmd, "subcommands"); subs != nil && subs.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(subs.Content); i += 2 {
				walkCmd(path+".subcommands."+subs.Content[i].Value, subs.Content[i+1])
			}
		}
	}
	if interaction := mapValue(root, "interaction"); interaction != nil && interaction.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(interaction.Content); i += 2 {
			walkCmd("interaction."+interaction.Content[i].Value, interaction.Content[i+1])
		}
	}

	if provision := mapValue(root, "provision"); provision != nil && provision.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(provision.Content); i += 2 {
			name, items := provision.Content[i].Value, provision.Content[i+1]
			if items.Kind != yaml.SequenceNode {
				continue
			}
			for j, item := range items.Content {
				for _, key := range []string{"run", "cmd"} {
					visit(fmt.Sprintf("provision.%s[%d].%s", name, j, key), mapValue(item, key))
				}
			}
		}
	}
	return out
}

func visitScalarOrList(path string, n *yaml.Node, visit func(string, *yaml.Node)) {
	if n == nil {
		return
	}
	if n.Kind == yaml.SequenceNode {
		for i, item := range n.Content {
			visit(fmt.Sprintf("%s[%d]", path, i), item)
		}
		return
	}
	visit(path, n)
}
