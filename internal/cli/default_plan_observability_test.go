package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
	"gopkg.in/yaml.v3"
)

func defaultPlanObservabilityCases() []struct {
	name        string
	config      *config.Config
	defaultPlan string
	source      string
} {
	return []struct {
		name        string
		config      *config.Config
		defaultPlan string
		source      string
	}{
		{
			name: "explicit",
			config: &config.Config{
				DefaultPlanName: "local-infra",
				Plans: map[string]*config.PlanConfig{
					"local-infra": {},
					"local-dev":   {},
				},
			},
			defaultPlan: "local-infra",
			source:      "explicit",
		},
		{
			name: "implicit single plan",
			config: &config.Config{
				Plans: map[string]*config.PlanConfig{"design": {}},
			},
			defaultPlan: "design",
			source:      "implicit-single",
		},
		{
			name: "none",
			config: &config.Config{
				Plans: map[string]*config.PlanConfig{
					"local-infra": {},
					"local-dev":   {},
				},
			},
			source: "none",
		},
	}
}

func TestShowJSONReportsEffectiveDefaultPlan(t *testing.T) {
	for _, tt := range defaultPlanObservabilityCases() {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStdout(t, func() {
				if err := showJSON(tt.config); err != nil {
					t.Fatalf("showJSON: %v", err)
				}
			})

			var got map[string]json.RawMessage
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("unmarshal show JSON: %v\ngot:\n%s", err, out)
			}
			var source string
			if err := json.Unmarshal(got["default_plan_source"], &source); err != nil {
				t.Fatalf("unmarshal default_plan_source: %v", err)
			}
			if source != tt.source {
				t.Errorf("default_plan_source = %q, want %q", source, tt.source)
			}
			assertDefaultPlanJSON(t, got, tt.defaultPlan)

			text := captureStdout(t, func() {
				if err := showText(tt.config); err != nil {
					t.Fatalf("showText: %v", err)
				}
			})
			want := "default: none"
			if tt.defaultPlan != "" {
				want = "default: " + tt.defaultPlan + " [" + tt.source + "]"
			}
			if !strings.Contains(text, want) {
				t.Errorf("show text does not report effective default %q:\n%s", want, text)
			}
		})
	}
}

func TestManifestSerializesEffectiveDefaultPlan(t *testing.T) {
	for _, tt := range defaultPlanObservabilityCases() {
		t.Run(tt.name, func(t *testing.T) {
			manifest := buildManifest(tt.config)
			if manifest.DefaultPlan != tt.defaultPlan {
				t.Errorf("manifest DefaultPlan = %q, want %q", manifest.DefaultPlan, tt.defaultPlan)
			}
			if manifest.DefaultPlanSource != tt.source {
				t.Errorf("manifest DefaultPlanSource = %q, want %q", manifest.DefaultPlanSource, tt.source)
			}

			jsonPayload, err := json.Marshal(manifest)
			if err != nil {
				t.Fatalf("marshal manifest JSON: %v", err)
			}
			var jsonValue map[string]json.RawMessage
			if err := json.Unmarshal(jsonPayload, &jsonValue); err != nil {
				t.Fatalf("unmarshal manifest JSON: %v", err)
			}
			assertDefaultPlanJSON(t, jsonValue, tt.defaultPlan)
			var jsonSource string
			if err := json.Unmarshal(jsonValue["default_plan_source"], &jsonSource); err != nil {
				t.Fatalf("unmarshal manifest default_plan_source: %v", err)
			}
			if jsonSource != tt.source {
				t.Errorf("JSON default_plan_source = %q, want %q", jsonSource, tt.source)
			}

			yamlPayload, err := yaml.Marshal(manifest)
			if err != nil {
				t.Fatalf("marshal manifest YAML: %v", err)
			}
			var yamlValue map[string]any
			if err := yaml.Unmarshal(yamlPayload, &yamlValue); err != nil {
				t.Fatalf("unmarshal manifest YAML: %v", err)
			}
			assertDefaultPlanYAML(t, yamlValue, tt.defaultPlan, tt.source)
		})
	}
}

func assertDefaultPlanJSON(t *testing.T, value map[string]json.RawMessage, want string) {
	t.Helper()
	got, ok := value["default_plan"]
	if want == "" {
		if ok {
			t.Errorf("default_plan must be omitted when none is effective, got %s", got)
		}
		return
	}
	if !ok {
		t.Fatalf("default_plan missing, want %q", want)
	}
	var plan string
	if err := json.Unmarshal(got, &plan); err != nil {
		t.Fatalf("unmarshal default_plan: %v", err)
	}
	if plan != want {
		t.Errorf("default_plan = %q, want %q", plan, want)
	}
}

func assertDefaultPlanYAML(t *testing.T, value map[string]any, wantPlan, wantSource string) {
	t.Helper()
	if source, ok := value["default_plan_source"].(string); !ok || source != wantSource {
		t.Errorf("YAML default_plan_source = %#v, want %q", value["default_plan_source"], wantSource)
	}
	plan, ok := value["default_plan"]
	if wantPlan == "" {
		if ok {
			t.Errorf("YAML default_plan must be omitted when none is effective, got %#v", plan)
		}
		return
	}
	if !ok || plan != wantPlan {
		t.Errorf("YAML default_plan = %#v, want %q", plan, wantPlan)
	}
}
