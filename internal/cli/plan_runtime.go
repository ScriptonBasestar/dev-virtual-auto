package cli

import (
	"fmt"
	"path/filepath"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

// planRuntime binds a resolved plan to the single effective config that owns
// every resource used by that plan. Root plans retain the caller's environment;
// imported plans receive a fresh child-rooted environment.
type planRuntime struct {
	plan   *lifecycle.ExecutionPlan
	config *config.Config
	env    *config.Environment

	// report is the env-input verdict of the config that owns this plan, which is
	// the only verdict this route may act on. A root env_file failure must not
	// stop an imported plan, and a child failure must not leak to a root plan.
	report *config.EnvInputReport
}

func resolvePlanRuntime(root *config.Config, rootLoad *envLoad, planName string, cliVars map[string]string) (*planRuntime, error) {
	plan, err := lifecycle.ResolvePlan(root, planName, cliVars)
	if err != nil {
		return nil, err
	}
	owner := plan.OwnerConfig(root)
	if owner == nil {
		return nil, fmt.Errorf("plan %q has no execution owner", planName)
	}

	load := rootLoad
	if load == nil || filepath.Clean(load.env.CfgDir()) != filepath.Clean(owner.FileDir()) {
		if owner != root {
			load = newOwnedConfigEnvironment(owner)
		} else {
			load = newConfigEnvironment(owner)
		}
	}

	// Plan vars are merged onto whichever owner's environment was selected. If that
	// owner's inputs are incomplete the caller refuses before anything runs, so the
	// merge here never reaches a backend on a failed report.
	load.env.MergeVars(plan.EnvVars)

	return &planRuntime{plan: plan, config: owner, env: load.env, report: load.report}, nil
}
