# internal/lifecycle — Lifecycle Plugin Backends

DVA의 실행 엔진. 각 플러그인 타입별로 up/down/stop/status를 구현.

## Plugin Interface (plugin.go)

```go
type Plugin interface {
    Up(ctx context.Context, entry config.LifecycleEntry, opts UpOptions) error
    Down(ctx context.Context, entry config.LifecycleEntry, opts DownOptions) error
    Stop(ctx context.Context, entry config.LifecycleEntry, opts StopOptions) error
    Status(ctx context.Context, entry config.LifecycleEntry) ([]StatusEntry, error)
}
```

`registry.go`에서 PluginType → Plugin 구현체 매핑.

## Plugin Tiers

| Tier | Plugins | 파일 |
|---|---|---|
| 1 (Core) | compose, process, script, docker, kubectl, helm | compose.go, process.go, ... |
| 2 (Extended) | kustomize, tilt, skaffold, podman-compose, vagrant | kustomize.go, ... |
| 3 (Niche) | sam, serverless, multipass | sam.go, ... |

## Orchestrator (orchestrator.go)

`Orchestrator`가 `stack:` 항목들을 `order` 오름차순으로 실행.
- `Up`: 낮은 order부터 순차 실행
- `Down`: 높은 order부터 역순 실행
- 병렬 실행은 동일 `order` 값 항목들에만 적용

## Plan Orchestrator (plan_orchestrator.go)

`plans.<name>`을 실행합니다. `ResolvePlan`(resolver.go)이 environment/site/vars를 병합해
`ExecutionPlan`을 만들고, `materializeResolvedEntry`가 러너 설정을 플러그인 설정으로
변환합니다 — `runners.native`는 여기서 `ProcessPluginConfig`로 강등되어 기존
`ProcessPlugin`이 실행합니다.

앱 프로세스 전용 런타임(`app_manager.go`)은 `applications:` 섹션과 함께 제거됐습니다
(docs/43). 앱은 `native` 러너를 쓰는 stack 엔트리이고, 다른 엔트리와 같은 경로로 돕니다.

## Health Checks (health.go)

`HealthChecker` — up 완료 후 서비스 준비 확인.
`UpOptions.Wait=true` 시 health check 통과까지 대기.

## Status (status.go)

`StatusEntry` — 각 스택 항목의 실행 상태 표현.
`dva status` 명령어가 이를 수집해 출력.

## Adding a New Plugin

1. `plugin_type.go`에 `PluginXxx PluginType = "xxx"` 추가
2. `config/lifecycle.go`의 `knownPluginNames` 맵에 추가
3. `xxx.go` 파일에 `Plugin` 인터페이스 구현
4. `registry.go`에 등록
