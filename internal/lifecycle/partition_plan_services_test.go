package lifecycle

import (
	"reflect"
	"strings"
	"testing"
)

func TestPartitionPlanServices_ScopesAndExtras(t *testing.T) {
	all := []ServiceStatus{
		{Name: "db", State: "running", Health: "healthy"},
		{Name: "redis", State: "running", Health: "healthy"},
		{Name: "web", State: "running", Health: "healthy"},
		{Name: "adminer", State: "exited"},
	}
	in, out := partitionPlanServices(all, []string{"db", "redis"})
	if got, want := namesOf(in), []string{"db", "redis"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("in-plan = %v, want %v", got, want)
	}
	if got, want := namesOf(out), []string{"web"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("out-of-plan running = %v, want %v (exited adminer excluded)", got, want)
	}
}

func TestPartitionPlanServices_MissingSelected(t *testing.T) {
	all := []ServiceStatus{
		{Name: "web", State: "running"},
	}
	in, out := partitionPlanServices(all, []string{"db", "redis"})
	if len(in) != 2 {
		t.Fatalf("in-plan len = %d, want 2 (not-found placeholders)", len(in))
	}
	for _, s := range in {
		if s.State != "not found" {
			t.Fatalf("missing selected service state = %q, want not found", s.State)
		}
	}
	if got, want := namesOf(out), []string{"web"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("out-of-plan = %v, want %v", got, want)
	}
}

func TestPrintStatus_OutOfPlanSection(t *testing.T) {
	status := &AggregatedStatus{
		Entries: []EntryStatus{
			{
				Name:   "core-compose",
				Plugin: "compose",
				Services: []ServiceStatus{
					{Name: "db", State: "running", Health: "healthy"},
				},
				OutOfPlan: []ServiceStatus{
					{Name: "web", State: "running", Health: "healthy"},
				},
			},
		},
	}
	out := captureStdout(func() {
		PrintStatus(status, "/tmp")
	})
	if !strings.Contains(out, "out of plan, still running") {
		t.Fatalf("expected out-of-plan banner, got %q", out)
	}
	if !strings.Contains(out, "web") {
		t.Fatalf("expected web in out-of-plan table, got %q", out)
	}
}

func namesOf(ss []ServiceStatus) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.Name
	}
	return out
}
