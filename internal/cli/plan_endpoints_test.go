package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestRunPlanUpPrintsAllConfiguredEndpointsAfterSuccessfulStartup(t *testing.T) {
	c := loadTestConfig(t, planEndpointTestConfig(""))
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	var err error
	out := captureStdout(t, func() {
		err = runPlanUp(c, e, "demo", nil)
	})

	if err != nil {
		t.Fatalf("runPlanUp failed: %v", err)
	}
	for _, want := range []string{"Endpoints:", "API", "http://127.0.0.1:18080", "Web", "http://127.0.0.1:13000"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "API") > strings.Index(out, "Web") {
		t.Errorf("endpoint order is not deterministic:\n%s", out)
	}
}

func TestRunPlanUpPrintsOnlyEndpointsMatchingPlanTags(t *testing.T) {
	c := loadTestConfig(t, planEndpointTestConfig("    endpoint_tags: [app]\n"))
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	var err error
	out := captureStdout(t, func() {
		err = runPlanUp(c, e, "demo", nil)
	})

	if err != nil {
		t.Fatalf("runPlanUp failed: %v", err)
	}
	if !strings.Contains(out, "API") {
		t.Errorf("matching endpoint missing:\n%s", out)
	}
	if strings.Contains(out, "Web") {
		t.Errorf("unrelated endpoint was printed:\n%s", out)
	}
}

func TestRunPlanUpDoesNotProbeEndpointsExcludedByPlanTags(t *testing.T) {
	var requests atomic.Int32
	excludedEndpoint := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer excludedEndpoint.Close()

	configText := strings.Replace(
		planEndpointTestConfig("    endpoint_tags: [app]\n"),
		"http://127.0.0.1:13000",
		excludedEndpoint.URL,
		1,
	)
	c := loadTestConfig(t, configText)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	if err := runPlanUp(c, e, "demo", nil); err != nil {
		t.Fatalf("runPlanUp failed: %v", err)
	}
	if got := requests.Load(); got != 0 {
		t.Errorf("excluded endpoint received %d health probes, want 0", got)
	}
}

func TestRunPlanUpDoesNotProbeConfiguredEndpoints(t *testing.T) {
	var requests atomic.Int32
	endpoint := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer endpoint.Close()

	configText := strings.Replace(
		planEndpointTestConfig("    endpoint_tags: [app]\n"),
		"http://127.0.0.1:18080",
		endpoint.URL,
		1,
	)
	c := loadTestConfig(t, configText)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	if err := runPlanUp(c, e, "demo", nil); err != nil {
		t.Fatalf("runPlanUp failed: %v", err)
	}
	if got := requests.Load(); got != 0 {
		t.Errorf("configured endpoint received %d health probes, want 0", got)
	}
}

func TestRunPlanUpOmitsEndpointsWhenStartupFails(t *testing.T) {
	c := loadTestConfig(t, planEndpointTestConfigWithUp("false", ""))
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	var err error
	out := captureStdout(t, func() {
		err = runPlanUp(c, e, "demo", nil)
	})

	if err == nil {
		t.Fatal("runPlanUp should fail")
	}
	if strings.Contains(out, "Endpoints:") {
		t.Errorf("failure output implied successful connection information:\n%s", out)
	}
}

func TestRunPlanUpDryRunOmitsEndpoints(t *testing.T) {
	c := loadTestConfig(t, planEndpointTestConfig(""))
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	var err error
	out := captureStdout(t, func() {
		err = runPlanUp(c, e, "demo", []string{"--dry-run"})
	})

	if err != nil {
		t.Fatalf("runPlanUp dry-run failed: %v", err)
	}
	if strings.Contains(out, "Endpoints:") {
		t.Errorf("dry-run output implied live endpoints:\n%s", out)
	}
}

func TestRunPlanUpJSONOutputReturnsStructuredResult(t *testing.T) {
	c := loadTestConfig(t, planEndpointTestConfig(""))
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())
	previousJSONOutput := jsonOutput
	jsonOutput = true
	t.Cleanup(func() {
		jsonOutput = previousJSONOutput
	})

	var err error
	out := captureStdout(t, func() {
		err = runPlanUp(c, e, "demo", nil)
	})

	if err != nil {
		t.Fatalf("runPlanUp failed: %v", err)
	}
	if strings.Contains(out, "Endpoints:") {
		t.Errorf("JSON output included a human endpoint table:\n%s", out)
	}

	var got struct {
		Action    string `json:"action"`
		Plan      string `json:"plan"`
		DryRun    bool   `json:"dry_run"`
		Endpoints []struct {
			Label string `json:"label"`
			URL   string `json:"url"`
		} `json:"endpoints"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("JSON output is invalid: %v\n%s", err, out)
	}
	if got.Action != "up" || got.Plan != "demo" || got.DryRun {
		t.Errorf("unexpected plan result: %+v", got)
	}
	if len(got.Endpoints) != 2 || got.Endpoints[0].Label != "API" || got.Endpoints[1].Label != "Web" {
		t.Errorf("unexpected endpoints: %+v", got.Endpoints)
	}
}

func TestRunPlanUpOmitsEmptyEndpointHeadingWhenNoEndpointsConfigured(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.44"
stack:
  demo:
    default_runner: script
    runners:
      script:
        up: "true"
plans:
  demo:
    entries:
      - name: demo
`)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	var err error
	out := captureStdout(t, func() {
		err = runPlanUp(c, e, "demo", nil)
	})

	if err != nil {
		t.Fatalf("runPlanUp failed: %v", err)
	}
	if strings.Contains(out, "Endpoints:") {
		t.Errorf("empty endpoint heading was printed:\n%s", out)
	}
}

func planEndpointTestConfig(planFields string) string {
	return planEndpointTestConfigWithUp("true", planFields)
}

func planEndpointTestConfigWithUp(up, planFields string) string {
	return `version: "0.1.44"
stack:
  demo:
    default_runner: script
    runners:
      script:
        up: "` + up + `"
plans:
  demo:
` + planFields + `    entries:
      - name: demo
endpoints:
  api:
    label: API
    url: http://127.0.0.1:18080
    tags: [app]
  web:
    label: Web
    url: http://127.0.0.1:13000
    tags: [ui]
`
}
