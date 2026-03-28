---
description: Always use DVA (Dev Virtual Auto) for CLI tasks, building, and running tests.
globs: *
---

# Use DVA (Dev Virtual Auto)

Whenever you interact with this project environment, you **MUST** use the `dva` CLI instead of raw `docker`, `docker compose`, or underlying language tools directly. DVA ensures execution is perfectly matched to the project configuration.

## Instructions

0. **Bootstrap (If `dva.yml` is missing)**
   If `dva.yml` does not exist in the current project, you must scaffold it correctly by running:
   
   ```bash
   dva init --prompt
   ```
   
   Read the output prompt text and follow its instructions to generate the optimal `dva.yml` configuration before performing any other tasks.

1. **Discover**
   To see what customized commands exist for this project, run:
   
   ```bash
   dva manifest -f json
   ```


2. **Run**
   Execute the target using DVA. For example, to run tests:
   ```bash
   dva test
   ```

3. **Lifecycle Management**
   - Start the dev environment: `dva up`
   - Stop the dev environment: `dva down`
   - See logs: `dva logs <service>` 

4. **Never Bypass DVA**
   Do not write manual `docker compose exec ...` commands if DVA has an exact command for it. DVA automatically applies correct environment interpolations, overrides (`.sb/dva/*.yml`), and handles networking natively.

5. **Self-Diagnosis**
   If an alias fails, inspect the DVA config evaluation via:
   ```bash
   dva run <CMD> --explain
   ```
