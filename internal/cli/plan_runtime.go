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
}

func resolvePlanRuntime(root *config.Config, rootEnv *config.Environment, planName string, cliVars map[string]string) (*planRuntime, error) {
	plan, err := lifecycle.ResolvePlan(root, planName, cliVars)
	if err != nil {
		return nil, err
	}
	owner := plan.OwnerConfig(root)
	if owner == nil {
		return nil, fmt.Errorf("plan %q has no execution owner", planName)
	}

	runtimeEnv := rootEnv
	if runtimeEnv == nil || filepath.Clean(runtimeEnv.CfgDir()) != filepath.Clean(owner.FileDir()) {
		if owner != root {
			runtimeEnv = newOwnedConfigEnvironment(owner)
		} else {
			runtimeEnv = newConfigEnvironment(owner)
		}
	}
	runtimeEnv.MergeVars(plan.EnvVars)

	return &planRuntime{plan: plan, config: owner, env: runtimeEnv}, nil
}
