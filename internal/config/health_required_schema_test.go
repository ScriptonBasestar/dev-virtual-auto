package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestValidateApplicationHealthRequiredContract locks TASK-118 option C:
// applications.*.health.required and variants.*.health.required are Boolean,
// default false, and top-level health_checks must not accept the field.
//
// Required is read via reflection so this file compiles before HealthCheckConfig
// gains the field; missing field or wrong value still fails the contract.
func TestValidateApplicationHealthRequiredContract(t *testing.T) {
	t.Run("parent_required_true_loads_and_parses", func(t *testing.T) {
		tmpDir := t.TempDir()
		content := `version: "0.1.44"
applications:
  api:
    run:
      native: "sleep 1"
    health:
      type: http
      url: "http://localhost:11200/healthz"
      required: true
`
		cfg := loadConfigForSchemaTest(t, tmpDir, content)
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() error = %v, want accept applications.api.health.required: true", err)
		}
		app := cfg.Applications["api"]
		if app == nil || app.Health == nil {
			t.Fatal("expected applications.api.health to be parsed")
		}
		if got := healthRequired(t, app.Health); !got {
			t.Fatalf("applications.api.health.Required = %v, want true", got)
		}
	})

	t.Run("variant_required_true_survives_resolve_app", func(t *testing.T) {
		tmpDir := t.TempDir()
		content := `version: "0.1.44"
applications:
  api:
    run:
      native: "sleep 1"
    variants:
      worker:
        run:
          native: "sleep 2"
        health:
          type: tcp
          address: "localhost:5432"
          required: true
`
		cfg := loadConfigForSchemaTest(t, tmpDir, content)
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() error = %v, want accept variant health.required: true", err)
		}
		variant := cfg.Applications["api"].Variants["worker"]
		if variant == nil || variant.Health == nil {
			t.Fatal("expected applications.api.variants.worker.health to be parsed")
		}
		if got := healthRequired(t, variant.Health); !got {
			t.Fatalf("variant.Health.Required = %v, want true", got)
		}

		_, resolved, err := cfg.ResolveApp("api.worker")
		if err != nil {
			t.Fatalf("ResolveApp(api.worker) error = %v", err)
		}
		if resolved.Health == nil {
			t.Fatal("ResolveApp result missing health")
		}
		if got := healthRequired(t, resolved.Health); !got {
			t.Fatalf("ResolveApp health.Required = %v, want true", got)
		}
	})

	t.Run("omitted_defaults_false", func(t *testing.T) {
		tmpDir := t.TempDir()
		content := `version: "0.1.44"
applications:
  api:
    run:
      native: "sleep 1"
    health:
      type: http
      url: "http://localhost:11200/healthz"
`
		cfg := loadConfigForSchemaTest(t, tmpDir, content)
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() error = %v, want accept health without required", err)
		}
		app := cfg.Applications["api"]
		if app == nil || app.Health == nil {
			t.Fatal("expected applications.api.health to be parsed")
		}
		if got := healthRequired(t, app.Health); got {
			t.Fatalf("applications.api.health.Required = %v, want false when omitted", got)
		}
	})

	t.Run("non_boolean_application_value_rejected", func(t *testing.T) {
		tmpDir := t.TempDir()
		content := `version: "0.1.44"
applications:
  api:
    run:
      native: "sleep 1"
    health:
      type: http
      url: "http://localhost:11200/healthz"
      required: "yes"
`
		// Load may succeed (YAML decode); schema Validate must reject non-Boolean.
		if err := os.WriteFile(filepath.Join(tmpDir, FileName), []byte(content), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		cfg, err := Load(tmpDir)
		if err != nil {
			// Load-time rejection is acceptable if it names the bad value/type.
			if !strings.Contains(err.Error(), "required") &&
				!strings.Contains(err.Error(), "Invalid type") &&
				!strings.Contains(err.Error(), "expected bool") {
				t.Fatalf("Load() error = %v, want type/required rejection", err)
			}
			return
		}
		err = cfg.Validate()
		if err == nil {
			t.Fatal("Validate() expected error for applications.api.health.required: \"yes\"")
		}
		msg := err.Error()
		// After the field exists, schema must reject by type (not coerce "yes" → true).
		// Before the field exists, additional-property rejection also proves non-acceptance.
		if !strings.Contains(msg, "Invalid type") &&
			!strings.Contains(msg, "expected bool") &&
			!strings.Contains(msg, "required") {
			t.Fatalf("Validate() error = %v, want type rejection for non-Boolean required", err)
		}
	})

	t.Run("top_level_health_checks_required_rejected", func(t *testing.T) {
		tmpDir := t.TempDir()
		content := `version: "0.1.44"
health_checks:
  api:
    type: http
    url: "http://localhost:11200/healthz"
    required: true
`
		cfg := loadConfigForSchemaTest(t, tmpDir, content)
		err := cfg.Validate()
		if err == nil {
			t.Fatal("Validate() expected error for health_checks.api.required")
		}
		if !strings.Contains(err.Error(), "Additional property required is not allowed") {
			t.Fatalf("Validate() error = %v, want additional property rejection for top-level health_checks.required", err)
		}
	})
}

// healthRequired reads HealthCheckConfig.Required without a compile-time field
// dependency so the red-phase test package builds before the struct is extended.
func healthRequired(t *testing.T, hc *HealthCheckConfig) bool {
	t.Helper()
	if hc == nil {
		t.Fatal("health config is nil")
	}
	v := reflect.ValueOf(hc).Elem().FieldByName("Required")
	if !v.IsValid() {
		t.Fatal("HealthCheckConfig.Required field is missing")
	}
	if v.Kind() != reflect.Bool {
		t.Fatalf("HealthCheckConfig.Required kind = %s, want bool", v.Kind())
	}
	return v.Bool()
}
