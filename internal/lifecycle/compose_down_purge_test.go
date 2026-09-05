package lifecycle

import (
	"reflect"
	"strings"
	"testing"
)

// TASK-311: a plan that selects services tears down with `rm`, which cannot reach named
// volumes or the project network. --purge asks for a clean slate, so it must widen to the
// project-wide `down` even when services are selected; plain down and --volumes keep the
// service scope and say what they leave behind.
func TestComposeDownArgsPurgeWidensToProject(t *testing.T) {
	services := []string{"postgres", "redis"}
	cases := []struct {
		name string
		pctx *PluginContext
		want []string
		note string
	}{
		{"services plain", &PluginContext{ComposeServices: &services},
			[]string{"rm", "--force", "--stop", "postgres", "redis"}, "the project network stay"},
		{"services volumes", &PluginContext{ComposeServices: &services, Volumes: true},
			[]string{"rm", "--force", "--stop", "--volumes", "postgres", "redis"}, "named volumes and the project network stay"},
		{"services purge", &PluginContext{ComposeServices: &services, Volumes: true, RemoveImages: true, Purge: true},
			[]string{"down", "--remove-orphans", "--volumes", "--rmi", "local"}, ""},
		{"no services purge", &PluginContext{Purge: true},
			[]string{"down", "--remove-orphans", "--volumes", "--rmi", "local"}, ""},
		{"no services volumes", &PluginContext{Volumes: true},
			[]string{"down", "--remove-orphans", "--volumes"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := composeDownArgs(tc.pctx)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("args = %v, want %v", got, tc.want)
			}
			note := composeDownLeftovers(tc.pctx, got)
			if tc.note == "" && note != "" {
				t.Fatalf("unexpected leftovers note for a project-wide down: %q", note)
			}
			if tc.note != "" && (!strings.Contains(note, tc.note) || !strings.Contains(note, "--purge")) {
				t.Fatalf("note = %q, want it to name %q and point at --purge", note, tc.note)
			}
		})
	}
}
