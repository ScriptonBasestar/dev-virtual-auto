<!-- AI_SKIP: Human documentation -->

# dva-config (dva:dva-config)

Reusable workflow for creating, preserving, migrating, and diagnosing DVA configuration in devbox
projects. Ported from the `claude-ce-plugin` `tool-dva-config` skill; the DVA repository is now the
canonical owner.

## Use it for

- `dva.yml` creation or repair;
- DVA schema migration warnings;
- `dva config validate`, `dva config show`, `dva manifest`, and `dva doctor` diagnosis;
- named-plan authoring and legacy modes/applications migration;
- root/subproject DVA ownership;
- separating config, CLI, environment, and project defects.

## Relationship to the `dva` skill

- **`dva`** — CLI execution: build, test, run, logs, lifecycle. Enforces DVA over raw
  docker/compose/kubectl.
- **`dva-config`** (this skill) — configuration authoring, migration, and defect attribution.
  Auto-triggered (`user-invocable: false`) when DVA configuration work is detected.

See [SKILL.md](SKILL.md) for the executable workflow.
