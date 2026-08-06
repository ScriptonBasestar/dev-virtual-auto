package cli

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// captureStdout captures stdout during fn execution.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	out, _ := io.ReadAll(r)
	return string(out)
}

// stackShapedConfig covers each shape a runner declaration takes, plus the equal-order pair the
// sequence has to break deterministically. The first three shapes are what `dva.yml` files
// actually load into — entry-level `compose:` is rejected outright by load, so no fixture here
// uses it.
func stackShapedConfig() *config.Config {
	return &config.Config{
		Stack: map[string]*config.LifecycleEntry{
			// One runner, named again by default_runner: the redundant default stays out of the
			// text row.
			"infra": {
				Order:         10,
				Description:   "PostgreSQL and Redis",
				DefaultRunner: "compose",
				Runners:       map[string]any{"compose": map[string]any{}},
			},
			// Multi-runner: which one runs is a declaration in its own right.
			"api": {
				Order:         20,
				Description:   "REST API",
				DefaultRunner: "helm",
				Runners: map[string]any{
					"helm":    map[string]any{},
					"kubectl": map[string]any{},
				},
			},
			// `plugin:` plus a nested config and no runners map — RunnerNames falls back to the
			// detected plugin. Shares api's order, so the name tiebreak decides which comes first.
			"bare": {
				Order:  20,
				Plugin: "script",
				Script: &config.ScriptPluginConfig{Up: "echo up"},
			},
			// Declares nothing at all: no runner by any route, and no order. The missing order is
			// what keeps the "order:0 is not a decision" assertion honest — with every entry
			// declaring one, that assertion has nothing to catch. This shape loads and validates
			// from a real dva.yml: a stack entry has no required fields, and DetectPlugin infers
			// nothing because `void` is not a known plugin name.
			"void": {},
			// default_runner and the runners key spelled differently for the same runner, which
			// schema.json accepts. Both sides normalize to podman-compose, so this must read as one
			// runner with a redundant default, not as a default naming an undeclared runner.
			"vms": {
				Order:         30,
				DefaultRunner: "podman_compose",
				Runners:       map[string]any{"podman_compose": map[string]any{}},
			},
		},
	}
}

// TestShowNamesStackEntries is the point of the section: before it, `dva show` printed a
// `Compose:` heading — a runner's name — and no way to learn that the entry is called `infra`,
// which is the word a plan's entries[].name and the tag filters actually take.
//
// The heading used to say what command consumed the name — `dva stack up <name>` — and that
// command is gone. Nothing single replaced it: an entry name is now referenced from a plan,
// not typed at a prompt. So the heading names the two things that consume it instead.
func TestShowNamesStackEntries(t *testing.T) {
	out := captureStdout(t, func() {
		showText(stackShapedConfig())
	})

	if !strings.Contains(out, "Stack (entry names, referenced by plans and tag filters):") {
		t.Fatalf("no stack section; the entry names are unreachable from show output.\ngot:\n%s", out)
	}
	for _, name := range []string{"infra", "api", "bare", "void", "vms"} {
		if !strings.Contains(out, name) {
			t.Errorf("stack entry %q is not named anywhere in show output.\ngot:\n%s", name, out)
		}
	}

	// Naming an entry without its runner leaves the reader knowing a name they cannot act on:
	// the runner decides which lifecycle backend handles it when a plan pulls it in.
	for _, want := range []string{"runner:compose", "runners:helm,kubectl", "default:helm", "runner:script", "runner:podman-compose"} {
		if !strings.Contains(out, want) {
			t.Errorf("show output is missing %q.\ngot:\n%s", want, out)
		}
	}
	// vms spells default_runner and its runners key both as podman_compose. Canonicalizing only one
	// side made the row claim the default named an undeclared runner
	// (`[runner:podman-compose, default:podman_compose]`), so the underscore spelling must not
	// survive anywhere in the row.
	if strings.Contains(out, "podman_compose") {
		t.Errorf("the podman_compose spelling reached the output, so the two sides were compared uncanonicalized.\ngot:\n%s", out)
	}
	if !strings.Contains(out, "no runner declared") {
		t.Errorf("an entry declaring no runner rendered as an empty bracket, which reads as a formatting bug rather than a config fact.\ngot:\n%s", out)
	}
	// infra's default_runner names its only runner, so repeating it would be noise the reader has
	// to rule out; the multi-runner row above is where the field carries information.
	if strings.Contains(out, "default:compose") {
		t.Errorf("a default_runner naming the entry's only runner was printed anyway.\ngot:\n%s", out)
	}

	// Rows follow declaration order, so they have to be in it: void declares no order and so leads
	// at 0, and api precedes bare on the tiebreak because both are order 20.
	iVoid, iInfra, iAPI, iBare, iVMS := strings.Index(out, "void"), strings.Index(out, "infra"), strings.Index(out, "api"), strings.Index(out, "bare"), strings.Index(out, "vms")
	if iVoid > iInfra || iInfra > iAPI || iAPI > iBare || iBare > iVMS {
		t.Errorf("rows are not in (order, name) sequence: void@%d infra@%d api@%d bare@%d vms@%d\ngot:\n%s", iVoid, iInfra, iAPI, iBare, iVMS, out)
	}
	// order:N is a fact only when declared; printing order:0 on every undeclared entry would
	// dress the default up as a decision.
	if !strings.Contains(out, "order:10") || strings.Contains(out, "order:0") {
		t.Errorf("declared orders must show and undeclared ones must not.\ngot:\n%s", out)
	}
}

// TestShowStackOrderIsStableAcrossRenders guards what a single assertion cannot: `api` and `bare`
// share order 20, and SortedStack collects entries from a map, so without the name tiebreak
// (TASK-084) the two rows swap at Go's map-iteration whim. One render would pass about half the
// time — a flaky test, which is worse than none.
//
// This asserts through showText rather than SortedStack even though the tiebreak now lives in
// config: what a reader compares against a plan's entry list is the rendered listing, so the property
// belongs to this surface regardless of which layer supplies it.
func TestShowStackOrderIsStableAcrossRenders(t *testing.T) {
	c := stackShapedConfig()
	first := captureStdout(t, func() { showText(c) })
	for i := range 20 {
		if got := captureStdout(t, func() { showText(c) }); got != first {
			t.Fatalf("render %d differs from the first; the listing is not reproducible.\nfirst:\n%s\ngot:\n%s", i, first, got)
		}
	}
}

// TestShowJSONNamesStackEntries: a consumer reading `compose` with no `stack` has the same gap as
// the text reader — settings with no owner. Both surfaces come from stackViews for that reason.
func TestShowJSONNamesStackEntries(t *testing.T) {
	out := captureStdout(t, func() {
		showJSON(stackShapedConfig())
	})

	var got struct {
		Stack map[string]struct {
			Description   string   `json:"description"`
			Runners       []string `json:"runners"`
			DefaultRunner string   `json:"default_runner"`
			Order         int      `json:"order"`
		} `json:"stack"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("show --json is not valid JSON: %v\ngot:\n%s", err, out)
	}
	if len(got.Stack) != 5 {
		t.Fatalf("stack has %d entries, want 5: %+v", len(got.Stack), got.Stack)
	}
	// schema.json says default_runner "Must reference a key in the runners map", so a consumer is
	// entitled to match it against an element of runners. That only holds if both are canonical.
	if vms := got.Stack["vms"]; vms.DefaultRunner != "podman-compose" ||
		len(vms.Runners) != 1 || vms.Runners[0] != "podman-compose" {
		t.Errorf("vms default_runner=%q runners=%v; default_runner must match a runners element, canonicalized",
			vms.DefaultRunner, vms.Runners)
	}
	if r := got.Stack["infra"].Runners; len(r) != 1 || r[0] != "compose" {
		t.Errorf("infra runners = %v, want [compose]", r)
	}
	if r := got.Stack["api"].Runners; len(r) != 2 || r[0] != "helm" || r[1] != "kubectl" {
		t.Errorf("api runners = %v, want [helm kubectl]", r)
	}
	if d := got.Stack["api"].DefaultRunner; d != "helm" {
		t.Errorf("api default_runner = %q, want %q", d, "helm")
	}
	// The order field is what lets a consumer reconstruct the sequence a JSON object loses.
	if o := got.Stack["api"].Order; o != 20 {
		t.Errorf("api order = %d, want 20", o)
	}
	if _, ok := got.Stack["void"]; !ok {
		t.Errorf("an entry declaring no runner was dropped from JSON: %+v", got.Stack)
	}
	// The text row suppresses a default that repeats the only runner; JSON does not, because a
	// consumer reconstructing the file needs to know the key was written.
	if d := got.Stack["infra"].DefaultRunner; d != "compose" {
		t.Errorf("infra default_runner = %q, want %q", d, "compose")
	}
}

func TestShowText_MinimalConfig(t *testing.T) {
	c := &config.Config{}
	output := captureStdout(t, func() {
		showText(c)
	})
	if !strings.Contains(output, "DVA v") {
		t.Errorf("showText should start with DVA version header, got: %s", output)
	}
}

func TestShowText_WithCompose(t *testing.T) {
	c := &config.Config{
		Stack: map[string]*config.LifecycleEntry{
			"compose": {
				Order: 10,
				Compose: &config.ComposePluginConfig{
					ProjectName: "myproject",
					Files:       []string{"compose.yml", "compose.dev.yml"},
				},
			},
		},
	}
	output := captureStdout(t, func() {
		showText(c)
	})
	if !strings.Contains(output, "Compose:") {
		t.Error("showText should display Compose section")
	}
	if !strings.Contains(output, "myproject") {
		t.Error("showText should display project name")
	}
}

func TestShowText_WithModes(t *testing.T) {
	c := &config.Config{
		Modes: map[string]config.ModeConfig{
			"docker": {Description: "Full Docker mode"},
			"native": {Description: "Native services"},
		},
	}
	output := captureStdout(t, func() {
		showText(c)
	})
	if !strings.Contains(output, "Modes (--mode/-M):") {
		t.Error("showText should display Modes section")
	}
	if !strings.Contains(output, "docker") || !strings.Contains(output, "native") {
		t.Error("showText should list all modes")
	}
}

func TestShowText_WithEnvironments(t *testing.T) {
	c := &config.Config{
		Environments: map[string]config.EnvironmentProfile{
			"dev": {Description: "Development"},
			"stg": {Description: "Staging"},
		},
	}
	output := captureStdout(t, func() {
		showText(c)
	})
	if !strings.Contains(output, "Environments (--env/-E):") {
		t.Error("showText should display Environments section")
	}
}

func TestShowText_WithInteraction(t *testing.T) {
	c := &config.Config{
		Interaction: map[string]*config.InteractionCommand{
			"test": {Description: "Run tests"},
			"lint": {
				Description: "Run linter",
				Subcommands: map[string]*config.InteractionCommand{
					"fix": {Description: "Auto-fix"},
				},
			},
		},
	}
	output := captureStdout(t, func() {
		showText(c)
	})
	if !strings.Contains(output, "Interaction Commands: 2 defined") {
		t.Error("showText should display interaction command count")
	}
	if !strings.Contains(output, "+1 sub") {
		t.Error("showText should display subcommand count")
	}
}

func TestShowText_WithProvision(t *testing.T) {
	c := &config.Config{
		Provision: config.ProvisionConfig{
			DefaultProfile: "setup",
			Profiles: map[string][]config.ProvisionItem{
				"setup": {{Step: "Install deps"}},
				"reset": {{Step: "Reset DB"}},
			},
		},
	}
	output := captureStdout(t, func() {
		showText(c)
	})
	if !strings.Contains(output, "Provision Profiles:") {
		t.Error("showText should display provision profiles")
	}
	if !strings.Contains(output, "(default)") {
		t.Error("showText should mark default profile")
	}
}

func TestShowText_WithHealthChecks(t *testing.T) {
	c := &config.Config{
		HealthChecks: map[string]config.HealthCheckConfig{
			"db":  {Type: "tcp"},
			"api": {Type: "http"},
		},
	}
	output := captureStdout(t, func() {
		showText(c)
	})
	if !strings.Contains(output, "Health Checks:") {
		t.Error("showText should display health checks")
	}
}

func TestShowText_WithEnvironmentVars(t *testing.T) {
	c := &config.Config{
		Environment: map[string]string{
			"DB_HOST": "localhost",
			"APP_ENV": "dev",
		},
	}
	output := captureStdout(t, func() {
		showText(c)
	})
	if !strings.Contains(output, "Environment Variables: 2 defined") {
		t.Error("showText should display environment variable count")
	}
}

func TestShowJSON_MinimalConfig(t *testing.T) {
	c := &config.Config{}
	output := captureStdout(t, func() {
		showJSON(c)
	})
	if !strings.Contains(output, "dva_version") {
		t.Error("showJSON should contain dva_version field")
	}
	if !strings.Contains(output, "environment_variables_count") {
		t.Error("showJSON should contain environment_variables_count")
	}
}

func TestShowJSON_FullConfig(t *testing.T) {
	c := &config.Config{
		Stack: map[string]*config.LifecycleEntry{
			"compose": {
				Order: 10,
				Compose: &config.ComposePluginConfig{
					ProjectName: "test",
					Files:       []string{"compose.yml"},
				},
			},
		},
		Modes: map[string]config.ModeConfig{
			"docker": {Description: "Docker"},
		},
		Environments: map[string]config.EnvironmentProfile{
			"dev": {Description: "Development"},
		},
		Interaction: map[string]*config.InteractionCommand{
			"test": {Description: "Run tests", Command: "make test"},
		},
		Provision: config.ProvisionConfig{
			DefaultProfile: "setup",
			Profiles: map[string][]config.ProvisionItem{
				"setup": {{Step: "Install"}},
			},
		},
		HealthChecks: map[string]config.HealthCheckConfig{
			"db": {Type: "tcp"},
		},
		Infra: map[string]config.InfraConfig{
			"shared": {},
		},
		Environment: map[string]string{
			"APP_ENV": "dev",
		},
	}
	output := captureStdout(t, func() {
		showJSON(c)
	})
	// Top-level keys, not substrings: this fixture's stack entry is *keyed* "compose", so a
	// strings.Contains check for "compose" is satisfied by the stack section alone and would pass
	// with the compose block gone entirely.
	var decoded map[string]any
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("showJSON did not emit valid JSON: %v", err)
	}
	for _, key := range []string{"compose", "modes", "environments", "interaction_commands", "provision", "health_checks", "infra"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("showJSON output missing %q section", key)
		}
	}
}

func TestShowJSON_WithModes(t *testing.T) {
	c := &config.Config{
		Modes: map[string]config.ModeConfig{
			"docker": {Description: "Docker mode"},
		},
	}
	output := captureStdout(t, func() {
		showJSON(c)
	})
	if !strings.Contains(output, `"modes"`) {
		t.Error("showJSON should contain modes key")
	}
	if !strings.Contains(output, "Docker mode") {
		t.Error("showJSON should contain mode description")
	}
}

func TestSortedKeys(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]string
		want []string
	}{
		{"empty", map[string]string{}, nil},
		{"single", map[string]string{"a": "1"}, []string{"a"}},
		{"sorted", map[string]string{"c": "3", "a": "1", "b": "2"}, []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortedKeys(tt.m)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("sortedKeys()[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestMaxKeyLen(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want int
	}{
		{"empty", nil, 0},
		{"single", []string{"abc"}, 3},
		{"multiple", []string{"a", "abcd", "ab"}, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxKeyLen(tt.keys)
			if got != tt.want {
				t.Errorf("maxKeyLen() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountSubcommands(t *testing.T) {
	tests := []struct {
		name string
		ic   *config.InteractionCommand
		want int
	}{
		{
			"nil subcommands",
			&config.InteractionCommand{},
			0,
		},
		{
			"flat subcommands",
			&config.InteractionCommand{
				Subcommands: map[string]*config.InteractionCommand{
					"sub1": {},
					"sub2": {},
				},
			},
			2,
		},
		{
			"nested subcommands",
			&config.InteractionCommand{
				Subcommands: map[string]*config.InteractionCommand{
					"sub1": {
						Subcommands: map[string]*config.InteractionCommand{
							"nested1": {},
						},
					},
					"sub2": {},
				},
			},
			3, // sub1 + sub2 + nested1
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countSubcommands(tt.ic)
			if got != tt.want {
				t.Errorf("countSubcommands() = %d, want %d", got, tt.want)
			}
		})
	}
}
