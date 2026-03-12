# DVA Plugin for Claude Code

DVA (Docker Virtual Auto) CLI integration - Automate container operations and project tooling natively inside Claude Code.

## Installation

```bash
/plugin marketplace add ScriptonBasestar/dva
/plugin install dva@dva-marketplace
```

## Requirements

- DVA binary: `go install github.com/ScriptonBasestar/dva/cmd/dva@latest`

## Core Concept: Project Configuration First

**DVA operates based on your `dva.yml`.** It abstracts away long raw docker compose or kubernetes commands, letting the agent and developer use single-word actions.

```bash
dva up          # Starts the container environment
dva test        # Runs unit tests via docker compose
dva clean       # Teardown and prune volumes
```

## Skills

| Skill | Purpose |
|-------|---------|
| **dva** | CLI reference instruction for using dva.yml features |

## Local Test

```bash
claude --plugin-dir ./claude-plugin
```
