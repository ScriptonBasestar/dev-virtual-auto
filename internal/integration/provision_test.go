//go:build integration

package integration

import (
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestProvisionProfileResolution_FullStack(t *testing.T) {
	c := loadFixtureConfig(t, "full-stack")

	// Default profile is "setup"
	if c.Provision.DefaultProfile != "setup" {
		t.Errorf("DefaultProfile = %q, want %q", c.Provision.DefaultProfile, "setup")
	}

	// Direct lookup
	steps, ok := c.Provision.Profiles["setup"]
	if !ok {
		t.Fatal("setup profile not found")
	}
	if len(steps) != 2 {
		t.Errorf("setup steps = %d, want 2", len(steps))
	}
	if steps[0].Step != "Install deps" {
		t.Errorf("step[0].Step = %q, want %q", steps[0].Step, "Install deps")
	}

	// RunCommands for multi-run step
	cmds := steps[1].RunCommands()
	if len(cmds) != 2 {
		t.Errorf("step[1] RunCommands = %d, want 2", len(cmds))
	}
}

func TestProvisionParallelSteps(t *testing.T) {
	c := loadFixtureConfig(t, "provision-profiles")

	steps := c.Provision.Profiles["parallel-demo"]
	if steps == nil {
		t.Fatal("parallel-demo profile not found")
	}

	// Verify parallel steps have compose_up
	if len(steps[0].ComposeUp) == 0 {
		t.Error("step[0] should have ComposeUp")
	}
	if len(steps[1].ComposeUp) == 0 {
		t.Error("step[1] should have ComposeUp")
	}

	// Verify barrier step
	if steps[2].Parallel {
		t.Error("step[2] should be sequential (barrier)")
	}
}

func TestProvisionRunCommands(t *testing.T) {
	c := loadFixtureConfig(t, "provision-profiles")

	setup := c.Provision.Profiles["setup"]
	// Step 1: single run
	cmds := setup[0].RunCommands()
	if len(cmds) != 1 {
		t.Errorf("setup step[0] RunCommands = %d, want 1", len(cmds))
	}
	if cmds[0] != `echo "bundle install"` {
		t.Errorf("setup step[0] cmd = %q", cmds[0])
	}

	// Step 2: multi run
	cmds = setup[1].RunCommands()
	if len(cmds) != 3 {
		t.Errorf("setup step[1] RunCommands = %d, want 3", len(cmds))
	}
}

func TestProvisionAllProfilesLoadable(t *testing.T) {
	c := loadFixtureConfig(t, "provision-profiles")

	expectedProfiles := []string{"setup", "reset", "parallel-demo"}
	for _, name := range expectedProfiles {
		steps, ok := c.Provision.Profiles[name]
		if !ok {
			t.Errorf("profile %q not found", name)
			continue
		}
		if len(steps) == 0 {
			t.Errorf("profile %q has 0 steps", name)
		}
		// Verify each step has a step name
		for i, step := range steps {
			if step.Step == "" && step.Raw == "" {
				t.Errorf("profile %q step[%d] has no name", name, i)
			}
		}
	}
}

// RunCommands is defined on ProvisionItem — verify it handles all formats
func TestProvisionItemRunCommands(t *testing.T) {
	tests := []struct {
		name string
		item config.ProvisionItem
		want int
	}{
		{"string run", config.ProvisionItem{Run: "echo hello"}, 1},
		{"array run", config.ProvisionItem{Run: []any{"echo a", "echo b"}}, 2},
		{"nil run", config.ProvisionItem{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmds := tt.item.RunCommands()
			if len(cmds) != tt.want {
				t.Errorf("RunCommands() = %d, want %d", len(cmds), tt.want)
			}
		})
	}
}
