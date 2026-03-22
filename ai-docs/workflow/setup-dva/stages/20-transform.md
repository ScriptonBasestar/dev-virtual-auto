<!-- v:2026-03-23 -->

<constants>
SELF = ai-docs/workflow/setup-dva/stages/20-transform.md
WORKFLOW_ROOT = ai-docs/workflow/setup-dva
TARGET = {resolved from entry.md or auto.md — target project path}
</constants>

[EXECUTE IMMEDIATELY - NO QUESTIONS]

<role>DVA structure transformation agent — execute approved migration plan</role>

<input>
| Source | Description |
|--------|-------------|
| Proposal | `tmp/setup-dva/10-proposal-{project-name}.md` from stage 10 |
| User approval | state.yaml `stage_10.approval: approved` |
</input>

<objective>
Execute the approved change plan from stage 10.
Migrate target project's file/directory structure to DVA-compatible layout.
</objective>

<steps>
## Phase 0: Pre-flight Check

- Verify `stage_10.approval: approved` in state.yaml
- If not approved → HALT immediately (gate dependency)
- Read proposal document for change plan table

## Phase 1: Backup

```bash
# Create timestamped backup of files that will change
BACKUP_DIR="tmp/setup-dva/backup-$(date +%Y%m%d%H%M%S)"
mkdir -p $BACKUP_DIR
# Copy each source file from the change plan
cp -r $TARGET/docker-compose*.yml $BACKUP_DIR/ 2>/dev/null
cp -r $TARGET/compose*.yml $BACKUP_DIR/ 2>/dev/null
cp -r $TARGET/.env* $BACKUP_DIR/ 2>/dev/null
cp -r $TARGET/dva.yml $BACKUP_DIR/ 2>/dev/null
```

Record backup path in state.yaml for rollback.

## Phase 2: Execute Change Plan

Process change plan table from proposal, in order:

### RENAME operations
```bash
# e.g., docker-compose.yml → compose.yml
# Use git mv if inside a git repo, otherwise mv
git mv $TARGET/{old} $TARGET/{new} 2>/dev/null || mv $TARGET/{old} $TARGET/{new}
```

### MOVE operations
```bash
git mv $TARGET/{source} $TARGET/{destination} 2>/dev/null || mv $TARGET/{source} $TARGET/{destination}
```

### DELETE operations
```bash
# Only obsolete files explicitly marked in proposal
git rm $TARGET/{file} 2>/dev/null || rm $TARGET/{file}
```

### CREATE operations
- Defer to stage 30 (Configuration) for compose.yml and dva.yml content
- Create placeholder directories if needed

## Phase 3: Structure Validation

After all operations:
```bash
# Verify expected structure
ls -la $TARGET/compose*.yml 2>/dev/null
ls -d $TARGET/*/ 2>/dev/null | head -20
# Verify no broken symlinks
find $TARGET -maxdepth 2 -type l ! -exec test -e {} \; -print
```

## Phase 4: Record Changes

Generate transform log at `tmp/setup-dva/20-transform-log-{project-name}.md`:

```markdown
# Transform Log: {project-name}

## Executed Actions
| # | Action | Detail | Status |
|---|--------|--------|--------|

## Backup Location
{backup path}

## Rollback Command
cp -r {backup}/* {target}/
```
</steps>

<constraints>
- HALT if stage 10 approval not found
- Use `git mv` for renames/moves if target is a git repo (preserve history)
- Always create backup before mutations
- Do not generate compose.yml/dva.yml content here — that is stage 30
- Do not start infrastructure — that is stage 40
- If any operation fails, record error and continue with remaining operations
</constraints>

<gate>
- [ ] Backup created at tmp/setup-dva/backup-*/
- [ ] All RENAME operations completed
- [ ] All MOVE operations completed
- [ ] Directory structure matches proposal's "After" layout
- [ ] Transform log generated
- [ ] No broken symlinks
</gate>

<output>
| Artifact | Path |
|----------|------|
| Transform Log | `tmp/setup-dva/20-transform-log-{project-name}.md` |
| Backup | `tmp/setup-dva/backup-{timestamp}/` |
</output>

<trigger>Verify approval → backup → execute change plan → validate structure.</trigger>
