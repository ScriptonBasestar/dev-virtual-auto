---
title: "Kubernetes (kubectl/helm) Stack Coverage Scanning"
priority: P1
effort: S
created: 2026-04-02
status: todo
---

# Kubernetes (kubectl/helm) Stack Coverage Scanning

## Description
Currently, `00-analyze.yaml` and `dva-improve.yaml` rely heavily on Docker Compose files (`compose.yml`) to detect infrastructure and suggest modes. However, DVA supports `kubectl` and `helm` plugins. 

To improve DVA configurations for Kubernetes-based environments, we need to add a context shell script step to scan for Kubernetes manifests (e.g., `k8s/`, `manifests/`) and Helm charts (`Chart.yaml`). 
By explicitly providing this to the LLM (e.g., `scan_k8s`), the agent can automatically suggest `stack.kubectl` or `stack.helm` configurations and appropriate `modes:` (e.g., local = compose, staging = kubectl).

## Acceptance Criteria
- [ ] Add `k8s_manifests` and `helm_charts` scanning to `00-analyze.yaml` (within `scan_project` or similar).
- [ ] Add the same scanning logic to `dva-improve.yaml` (Phase 1).
- [ ] Ensure the prompt explicitly instructs the LLM to use `stack.kubectl` or `stack.helm` when these files are detected instead of forcing compose only.
