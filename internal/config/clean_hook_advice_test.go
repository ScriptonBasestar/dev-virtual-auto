package config

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// adviceProperty matches the relocation targets the clean-hook message proposes, e.g.
// "interaction.clean.command/steps" -> "command/steps". It reads the message rather than
// restating it so the assertions below cannot drift away from the string users actually see.
var adviceProperty = regexp.MustCompile(`interaction\.clean\.([a-z_]+(?:/[a-z_]+)*)`)

// cleanAdviceBodies gives each proposed property a config body that exercises it. A property
// with no entry here fails the test rather than being skipped: an unexercised suggestion is
// exactly the state this card found the message in.
var cleanAdviceBodies = map[string]string{
	"command": "    command: echo CLEANING\n",
	"steps":   "    steps:\n      - {step: prune, run: \"echo PRUNE-RAN\"}\n",
	"script":  "    script: |\n      echo CLEANING\n",
}

// TestCleanHookAdviceNamesSchemaValidProperties checks the advice the way an author would
// follow it: take the properties the message names, write them, and see whether DVA accepts
// the result.
//
// The message used to propose a property interaction_command does not have, so an author who
// did exactly this traded dead hooks for a config the schema rejects — a worse position than
// the one the message was helping them out of, and one no test could have caught, because the
// only assertion on this message matched its first clause. TASK-273.
func TestCleanHookAdviceNamesSchemaValidProperties(t *testing.T) {
	// The legacy shape: hooks on `clean` and nothing else, which is what the removed built-in
	// left behind in real configs.
	legacy := `version: "0.1.44"
interaction:
  clean:
    before:
      - {step: prune, run: "echo PRUNE-RAN"}
`
	cfg := loadConfigForSchemaTest(t, t.TempDir(), legacy)
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() = nil; the clean hooks here run on nothing and must be reported")
	}

	var proposed []string
	for _, m := range adviceProperty.FindAllStringSubmatch(err.Error(), -1) {
		proposed = append(proposed, strings.Split(m[1], "/")...)
	}
	if len(proposed) == 0 {
		t.Fatalf("no interaction.clean.<property> advice found in %q; either the message stopped "+
			"suggesting a relocation or it changed shape, and this test is checking nothing", err)
	}

	declared := interactionCommandProperties(t)
	for _, prop := range proposed {
		t.Run(prop, func(t *testing.T) {
			if !declared[prop] {
				t.Fatalf("the message tells the author to write interaction.clean.%s, but "+
					"interaction_command has no such property; valid ones are %s",
					prop, strings.Join(sortedKeys(declared), ", "))
			}
			body, ok := cleanAdviceBodies[prop]
			if !ok {
				t.Fatalf("the message proposes interaction.clean.%s and this test has no body "+
					"for it, so the suggestion round-trips only in theory; add one to "+
					"cleanAdviceBodies", prop)
			}
			relocated := "version: \"0.1.44\"\ninteraction:\n  clean:\n" + body
			// Schema first, because that is the gate the retired advice failed, and a struct
			// decode would have silently dropped an unknown key instead of reporting it.
			if err := validateYAMLSchema([]byte(relocated)); err != nil {
				t.Fatalf("following the advice produces a config the schema rejects: %v", err)
			}
			moved := loadConfigForSchemaTest(t, t.TempDir(), relocated)
			if err := moved.Validate(); err != nil {
				t.Fatalf("following the advice produces a config Validate() rejects: %v", err)
			}
		})
	}
}

// interactionCommandProperties reads the property names schema.json declares for
// interaction_command. Read from the schema rather than listed here: a hand-copied list is
// the same kind of claim the message made, and it would go stale the same way.
func interactionCommandProperties(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := embeddedSchema.ReadFile("schema.json")
	if err != nil {
		t.Fatalf("read embedded schema: %v", err)
	}
	var doc struct {
		Definitions map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	def, ok := doc.Definitions["interaction_command"]
	if !ok || len(def.Properties) == 0 {
		t.Fatal("schema.json declares no interaction_command properties; the check below would " +
			"pass vacuously")
	}
	props := make(map[string]bool, len(def.Properties))
	for name := range def.Properties {
		props[name] = true
	}
	return props
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
