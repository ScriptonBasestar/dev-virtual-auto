# DVA Configuration Migration Guide

This document provides practical examples and scenarios for using `dva migrate` to upgrade your DVA configurations.

## Table of Contents

- [Overview](#overview)
- [Common Migration Scenarios](#common-migration-scenarios)
- [Step-by-Step Migration Walkthrough](#step-by-step-migration-walkthrough)
- [AI-Assisted Migration](#ai-assisted-migration)
- [Troubleshooting](#troubleshooting)

---

## Overview

The `dva migrate` command helps you upgrade your `dva.yml` configuration by:
- Detecting deprecated features
- Identifying legacy patterns
- Recommending new features
- Providing migration examples
- Generating AI-friendly prompts

**When to use:**
- Upgrading DVA from older versions (7.x → 8.x → 9.x)
- Adopting new features (env_file, step/run/note syntax)
- Fixing deprecated warnings
- Preparing for breaking changes

---

## Common Migration Scenarios

### Scenario 1: Upgrading from v8.x to v9.2.0

**Your current `dva.yml` (v8.1.0):**
```yaml
version: '8.1.0'

compose:
  files:
    - docker-compose.yml

interaction:
  rails:
    description: Run Rails commands
    service: web
    command: bundle exec rails
    compose_run_options:  # ❌ DEPRECATED
      - service-ports
      - rm

provision:
  default:
    - echo '📦 Installing gems...'        # ❌ LEGACY
    - bundle install
    - echo '📦 Setting up database...'    # ❌ LEGACY
    - rails db:create
    - rails db:migrate
```

**Run migration analysis:**
```bash
$ dva migrate

# DVA Configuration Migration Guide

## Current Configuration
- File: /path/to/dva.yml
- Current Version: 8.1.0
- Latest Version: 9.2.0
- Migration Required: YES

## Breaking Changes & Deprecations

### 1. compose_run_options (Deprecated)
**Location**: `interaction.rails`
**Migrate to**:
```yaml
compose:
  run_options: ["service-ports", "rm"]
```

### 2. Provision Legacy Format
**Location**: `provision.default`
**Migrate to step/run/note syntax**:
```yaml
provision:
  default:
    - step: Installing gems
      run: bundle install
    - step: Setting up database
      run:
        - rails db:create
        - rails db:migrate
```
```

**Migrated `dva.yml` (v9.2.0):**
```yaml
version: '8.1.0'

compose:
  files:
    - docker-compose.yml

interaction:
  rails:
    description: Run Rails commands
    service: web
    command: bundle exec rails
    compose:
      run_options:  # ✅ UPDATED
        - service-ports
        - rm

provision:
  default:
    - step: Installing gems  # ✅ MODERNIZED
      run: bundle install

    - step: Setting up database
      run:
        - rails db:create
        - rails db:migrate

    - step: Setup complete
      note: |
        ✅ Development environment ready!
        Run: dva rails server
```

---

### Scenario 2: Adding env_file Support

**Before (manually managing environment):**
```yaml
version: '9.0.0'

environment:
  RAILS_ENV: development
  DATABASE_URL: postgres://user:pass@db:5432/myapp_dev
  REDIS_URL: redis://redis:6379
```

**After migration analysis:**
```bash
$ dva migrate

## New Features Available

### env_file Support (v9.1.3+)
Load environment variables from .env files

**Usage**:
```yaml
# Simple
env_file: .env

# Multiple files (later overrides earlier)
env_file:
  - .env.defaults
  - .env
  - .env.local
```
```

**Migrated configuration:**
```yaml
version: '8.1.0'

# Move secrets to .env files (git-ignored)
env_file:
  - .env.defaults  # Team defaults (committed)
  - .env           # Secrets (git-ignored)
  - .env.local     # Local overrides (git-ignored)

# Keep only non-secret defaults
environment:
  RAILS_ENV: development
```

**.env.defaults** (committed):
```bash
# Team-wide defaults
RAILS_MAX_THREADS=5
WEB_CONCURRENCY=2
```

**.env** (git-ignored):
```bash
# Secrets - DO NOT COMMIT
DATABASE_URL=postgres://user:pass@db:5432/myapp_dev
REDIS_URL=redis://redis:6379
SECRET_KEY_BASE=your_secret_key_here
```

---

### Scenario 3: Complex Multi-Service Migration

**Before (v7.x with old patterns):**
```yaml
version: '7.0.0'

compose:
  files:
    - docker-compose.yml

interaction:
  rails:
    service: backend
    command: bundle exec rails
    compose_run_options: [service-ports]  # ❌ DEPRECATED

  npm:
    service: frontend
    command: npm
    compose_run_options: [rm]  # ❌ DEPRECATED

provision:
  default:
    - echo 'Starting backend...'  # ❌ LEGACY
    - docker-compose up -d backend
    - sleep 5
    - echo 'Installing backend gems...'
    - dva rails bundle install
    - echo 'Starting frontend...'
    - docker-compose up -d frontend
    - echo 'Installing frontend packages...'
    - dva npm install
```

**Fully migrated (v9.2.0):**
```yaml
version: '8.1.0'

compose:
  files:
    - docker-compose.yml

interaction:
  rails:
    description: Run Rails commands
    service: backend
    command: bundle exec rails
    compose:
      run_options: [service-ports]  # ✅ UPDATED

  npm:
    description: Run npm commands
    service: frontend
    command: npm
    compose:
      run_options: [rm]  # ✅ UPDATED

provision:
  # Note: Containers auto-start since v9.1.3!
  default:
    - step: Installing backend dependencies
      run: dva rails bundle install

    - step: Installing frontend dependencies
      run: dva npm install

    - step: Setting up database
      run:
        - dva rails db:create
        - dva rails db:migrate

    - step: Setup complete
      note: |
        ✅ Full-stack environment ready!

        Backend:  dva rails server
        Frontend: dva npm start
```

---

## Step-by-Step Migration Walkthrough

### Step 1: Run Migration Analysis

```bash
$ dva migrate > migration-guide.md
```

This generates a comprehensive guide saved to `migration-guide.md`.

### Step 2: Review Breaking Changes

Open `migration-guide.md` and read the "Breaking Changes & Deprecations" section carefully.

**Common issues:**
- `compose_run_options` → `compose.run_options`
- Legacy provision echo commands → step/run/note syntax
- Old environment patterns → env_file

### Step 3: Backup Current Config

```bash
$ cp dva.yml dva.yml.backup
```

### Step 4: Apply Changes

**Option A: Manual (Recommended for learning)**
- Edit `dva.yml` following the migration guide
- Apply one change at a time
- Validate after each change

**Option B: AI-Assisted (Faster for complex migrations)**
- Copy migration guide + current `dva.yml` to Claude/ChatGPT
- Ask: "Please migrate this dva.yml following the guide"
- Review and apply suggested changes

### Step 5: Validate

```bash
$ dva validate
dva.yml is valid
```

If validation fails, read the error message and fix accordingly.

### Step 6: Test Provision

```bash
$ dva provision
📦 [1/3] Installing dependencies
   → dva bundle install
   ✓ Complete

📦 [2/3] Setting up database
   → dva rails db:create
   → dva rails db:migrate
   ✓ Complete

📦 [3/3] Setup complete
   ✅ Development environment ready!
```

### Step 7: Clean Up

```bash
$ rm migration-guide.md dva.yml.backup
$ git add dva.yml
$ git commit -m "chore(config): upgrade dva.yml to v9.2.0"
```

---

## AI-Assisted Migration

### Using Claude Code

1. **Generate migration guide:**
   ```bash
   dva migrate | pbcopy  # macOS
   dva migrate | xclip -selection clipboard  # Linux
   ```

2. **Prompt Claude:**
   ```
   I need to migrate my dva.yml configuration. Here's the migration guide:

   [Paste migration guide]

   And here's my current dva.yml:

   [Paste your dva.yml]

   Please migrate the configuration following the guide. Explain each change.
   ```

3. **Review and apply:**
   - Claude will provide the migrated configuration
   - Review each change carefully
   - Apply to your `dva.yml`
   - Run `dva validate` to verify

### Using ChatGPT/Other LLMs

1. **Prepare context:**
   ```bash
   cat dva.yml > context.txt
   dva migrate >> context.txt
   ```

2. **Prompt:**
   ```
   I have a DVA configuration file that needs migration.
   The file and migration guide are below.

   Please:
   1. Analyze deprecated features
   2. Apply recommended changes
   3. Explain what changed and why

   [Paste context.txt contents]
   ```

3. **Validate output:**
   - Save migrated config
   - Run `dva validate`
   - Test with `dva provision`

---

## Troubleshooting

### Migration Guide Shows No Issues

**Problem:** `dva migrate` says your config is up-to-date but you want to adopt new features.

**Solution:** The migration guide only shows *required* changes (deprecations). New features are in the "New Features Available" section.

**Example:**
```bash
$ dva migrate

✅ Your dva.yml is already at version 9.2.0
   Target version: 9.2.0

# Your config is valid but you can still improve it by adopting:
# - env_file for better secret management
# - step/run/note syntax for cleaner provision scripts
```

### Validation Fails After Migration

**Problem:** `dva validate` reports errors after applying migration.

**Common causes:**
1. **Syntax error** - Check YAML indentation (use 2 spaces)
2. **Missing required fields** - Each interaction needs `command:`
3. **Invalid version format** - Use quotes: `version: '8.1.0'`

**Debug steps:**
```bash
# Check YAML syntax
ruby -ryaml -e "YAML.load_file('dva.yml')"

# Validate with verbose errors
DVA_DEBUG=1 dva validate

# Compare with working example
diff dva.yml examples/basic.yml
```

### Provision Behavior Changed

**Problem:** Provision scripts work differently after migration.

**Since v9.1.3:** Containers auto-start before provision runs!

**Before:**
```bash
$ dva up        # Start containers first
$ dva provision # Then run provision
```

**After v9.1.3:**
```bash
$ dva provision # Auto-starts containers if needed, then provisions
```

**If you need old behavior:**
```yaml
# Explicitly manage containers in provision
provision:
  manual:
    - step: Starting containers
      run: dva compose up -d

    - step: Installing dependencies
      run: bundle install
```

### Lost Functionality After Migration

**Problem:** Some commands don't work after upgrading.

**Checklist:**
1. Did you remove `compose_run_options`? Make sure you added `compose.run_options`
2. Did you change provision syntax? Test with `dva provision --help`
3. Did version change break compatibility? Check `CHANGELOG.md`

**Rollback if needed:**
```bash
$ cp dva.yml.backup dva.yml
$ dva validate
```

---

## Best Practices

### Before Migration
- ✅ Backup current config (`cp dva.yml dva.yml.backup`)
- ✅ Commit working config to git
- ✅ Read the migration guide thoroughly
- ✅ Test current setup works (`dva provision`)

### During Migration
- ✅ Apply one change at a time
- ✅ Validate after each change
- ✅ Test provision after major changes
- ✅ Keep migration guide for reference

### After Migration
- ✅ Run full validation (`dva validate`)
- ✅ Test all provision profiles
- ✅ Update team documentation
- ✅ Commit with descriptive message
- ✅ Delete backup if all works well

---

## Additional Resources

- **Schema Reference:** [`schema.json`](../schema.json)
- **Provision Examples:** [`provision-step-syntax.yml`](provision-step-syntax.yml)
- **Env File Examples:** [`env-file-basic.yml`](env-file-basic.yml)
- **Full Changelog:** [`CHANGELOG.md`](../CHANGELOG.md)
- **Getting Help:** [GitHub Issues](https://github.com/ScriptonBasestar/dev-virtual-auto/issues)

---

**Last updated:** 2025-12-02
**Applies to:** DVA v9.2.0+
