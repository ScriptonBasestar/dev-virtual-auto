package cli

import (
	"fmt"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// commandRuntime binds an interaction route or a provision profile to the single
// effective config that owns it, together with the environment built from that config.
//
// It is the interaction/provision twin of planRuntime (TASK-262), added by TASK-264 for
// the two surfaces plan ownership never covered. The invariant is the one ARCHITECTURE.md
// states for imports: a parent that imports `child/deploy` exposes a *route*, and the
// route resolves against the child's declarations — its vars, top-level environment,
// env_file, config directory and default working directory — not the parent's.
type commandRuntime struct {
	config *config.Config
	env    *config.Environment
}

// ownedRuntime builds the runtime for a config that owns itself: a dynamically loaded
// subproject (`run --project`, `project:command`) or an imported item's child config.
// The environment is rooted at the child's config directory, so relative script, compose
// and provision assets resolve from the child rather than from the caller's cwd.
func ownedRuntime(owner *config.Config) *commandRuntime {
	return &commandRuntime{config: owner, env: newOwnedConfigEnvironment(owner)}
}

// resolveInteractionRuntime selects the owner of a top-level interaction entry before any
// environment is loaded.
//
// Owner selection deliberately precedes env loading rather than following it. loadEnv
// reads the root's env_file, and TASK-248 will turn a failure there into an exit code
// instead of today's warning; resolving first is what keeps a broken root env_file from
// deciding the fate of a route that never reads it. Until TASK-248 lands, the
// warning-and-continue policy inside newConfigEnvironmentAt is unchanged.
//
// Subcommand nodes need no separate lookup: cloneImportedInteraction stamps the same
// owner onto the whole subtree, so the top-level key answers for every route beneath it.
func resolveInteractionRuntime(root *config.Config, entryName string) (*commandRuntime, error) {
	owner := root.Interaction[entryName].OwnerConfig(root)
	if owner == nil {
		return nil, fmt.Errorf("interaction %q has no execution owner", entryName)
	}
	if owner == root {
		return &commandRuntime{config: root, env: loadEnv(root)}, nil
	}
	return ownedRuntime(owner), nil
}

// resolveProvisionRuntime is resolveInteractionRuntime for a resolved provision profile
// name. The name must be the one resolveProvisionProfile settled on — canonical, alias or
// default_profile fallback — because ownership is recorded per registered name.
func resolveProvisionRuntime(root *config.Config, profileName string) (*commandRuntime, error) {
	owner := root.Provision.ProfileOwner(profileName, root)
	if owner == nil {
		return nil, fmt.Errorf("provision profile %q has no execution owner", profileName)
	}
	if owner == root {
		return &commandRuntime{config: root, env: loadEnv(root)}, nil
	}
	return ownedRuntime(owner), nil
}
