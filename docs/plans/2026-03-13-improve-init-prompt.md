# dva init -p Prompt Template Improvement Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve `dva init -p` prompt template so AI agents generate correct, functional `dva.yml` files — especially for host-build projects (Rust/Go), devbox patterns, and compose-less sub-projects.

**Architecture:** Two-layer fix: (1) enrich metadata collection in `init.go` to provide infra/ compose files, Makefile targets, and compose-readiness signals; (2) rewrite `prompt_template.txt` with runner selection logic, anti-patterns, concrete examples, and agent-team-friendly instructions.

**Tech Stack:** Go (init.go), embedded text template (prompt_template.txt)

---

### Task 1: Improve metadata collection in init.go

**Files:**
- Modify: `internal/cli/init.go:307-357` (generateAndPrintPrompt function)

**Step 1: Add infra/ subdirectory compose file detection**

In `generateAndPrintPrompt()`, after detecting root compose files, also scan `infra/` subdirectory:

```go
// After line 326 (detectedCompose assignment), add infra compose detection
infraComposeFiles := detectInfraComposeFiles()
detectedInfraCompose := "None"
if len(infraComposeFiles) > 0 {
    var infraParts []string
    for _, icf := range infraComposeFiles {
        entry := icf
        if services := extractComposeServices(icf); len(services) > 0 {
            entry += fmt.Sprintf(" → services: %s", strings.Join(services, ", "))
        }
        infraParts = append(infraParts, entry)
    }
    detectedInfraCompose = strings.Join(infraParts, "\n")
}
```

Add new function:

```go
func detectInfraComposeFiles() []string {
    dirs := []string{"infra", "docker", "deploy"}
    var found []string
    for _, dir := range dirs {
        if entries, err := os.ReadDir(dir); err == nil {
            for _, e := range entries {
                name := e.Name()
                if !e.IsDir() && (strings.HasPrefix(name, "compose") || strings.HasPrefix(name, "docker-compose")) &&
                    (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) {
                    found = append(found, filepath.Join(dir, name))
                }
            }
        }
    }
    return found
}
```

**Step 2: Add Makefile target extraction**

```go
func extractMakefileTargets() string {
    data, err := os.ReadFile("Makefile")
    if err != nil {
        return ""
    }
    lines := strings.Split(string(data), "\n")
    var targets []string
    for _, line := range lines {
        // Match: target: ## description
        if strings.Contains(line, "##") && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") {
            parts := strings.SplitN(line, ":", 2)
            if len(parts) == 2 && !strings.HasPrefix(parts[0], ".") {
                target := strings.TrimSpace(parts[0])
                desc := ""
                if idx := strings.Index(parts[1], "##"); idx >= 0 {
                    desc = strings.TrimSpace(parts[1][idx+2:])
                }
                if desc != "" {
                    targets = append(targets, fmt.Sprintf("  make %-18s # %s", target, desc))
                } else {
                    targets = append(targets, fmt.Sprintf("  make %s", target))
                }
            }
        }
    }
    if len(targets) == 0 {
        return ""
    }
    return strings.Join(targets, "\n")
}
```

**Step 3: Update format string call to include new metadata**

Change the `fmt.Sprintf` call to pass 7 arguments instead of 5:

```go
makeTargets := extractMakefileTargets()
detectedMakeTargets := "None"
if makeTargets != "" {
    detectedMakeTargets = makeTargets
}

prompt := fmt.Sprintf(promptTemplateText,
    detectedCompose,        // %s 1 - root compose
    detectedInfraCompose,   // %s 2 - infra compose
    detectedBuild,          // %s 3 - build files
    detectedMakeTargets,    // %s 4 - Makefile targets
    detectedEnv,            // %s 5 - env files
    detectedSubprojects,    // %s 6 - subprojects
    config.Version,         // %s 7 - dva version
)
```

**Step 4: Run tests**

```bash
cd /Users/archmagece/myopen/scripton/dev-virtual-auto && go test ./internal/cli/ -v -run TestInit
```

**Step 5: Commit**

```bash
git add internal/cli/init.go
git commit -m "feat(init): enrich prompt metadata with infra compose, Makefile targets"
```

---

### Task 2: Rewrite prompt_template.txt

**Files:**
- Modify: `internal/cli/prompt_template.txt` (complete rewrite)

**Step 1: Write the new prompt template**

The new template must address all identified failures:

1. **Runner selection guide** — `runner: local` vs `service:` vs `pod:`
2. **Anti-patterns** — no echo dummies, no wrong service refs, no prod images for dev
3. **Compose-less sub-projects** — use `runner: local` when no Docker service
4. **Infra compose detection** — use `infra/compose.yaml` as root compose when appropriate
5. **Makefile→interaction mapping** — extract real commands from Makefile
6. **Concrete examples** — host-build Rust/Go, devbox with infra, Node.js container
7. **Agent team instructions** — structured output format for Claude Code
8. **Compose suggestion** — when compose doesn't exist, suggest creating one

Template receives 7 `%s` parameters in order:
1. Root compose files + services
2. Infra compose files + services
3. Build files
4. Makefile targets
5. Environment files
6. Sub-projects
7. DVA version

See Task 2 implementation below for full content.

**Step 2: Verify template compiles**

```bash
cd /Users/archmagece/myopen/scripton/dev-virtual-auto && go build ./cmd/dva
```

**Step 3: Test with target project**

```bash
cd /Users/archmagece/mydevbox/scripton-gitrump-devbox && /Users/archmagece/go/bin/dva init -p 2>&1 | head -20
```

**Step 4: Commit**

```bash
git add internal/cli/prompt_template.txt
git commit -m "feat(init): rewrite LLM prompt template with runner guide, anti-patterns, examples"
```

---

### Task 3: Integration test — generate and validate on target project

**Step 1: Revert target project dva.yml files**

```bash
cd /Users/archmagece/mydevbox/scripton-gitrump-devbox
rm -f dva.yml gitrump/dva.yml gitrump-cloud/dva.yml
```

**Step 2: Run dva init -p and pipe to Claude Code**

```bash
cd /Users/archmagece/mydevbox/scripton-gitrump-devbox && dva init -p > /tmp/dva-prompt.txt
```

Then use Claude Code agent to execute the prompt and generate dva.yml files.

**Step 3: Validate generated files**

```bash
cd /Users/archmagece/mydevbox/scripton-gitrump-devbox && dva validate
cd /Users/archmagece/mydevbox/scripton-gitrump-devbox/gitrump && dva validate
cd /Users/archmagece/mydevbox/scripton-gitrump-devbox/gitrump-cloud && dva validate
```

**Step 4: Verify key properties**

- Root dva.yml uses `infra/compose.yaml` (not `docker-compose.yml`)
- Root interaction uses `runner: local` for build/test or appropriate compose method
- Sub-project gitrump uses `runner: local` for cargo commands
- No echo dummy commands
- No webhook-ok/webhook-fail as build service targets
