# Interaction Inheritance And Merge Semantics

## 배경

`interaction`은 DVA의 사용자-facing DSL에 가깝습니다.
현재 parent command와 subcommand 병합은 동작하지만, 필드가 늘어날수록 상속 누락이나 예외 규칙이 생기기 쉬운 구조입니다.

## 현재 상태

- command 해석: `internal/runner/interaction_tree.go`
- runner 선택: `internal/runner/runner.go`
- 실행 엔트리: `internal/cli/run.go`
- validation warnings: `internal/config/validate_warnings.go`

현재 `mergeInteraction()`은 parent를 기반으로 child 값을 덮는 방식입니다.
하지만 어떤 필드가 상속되고 어떤 필드가 replace되는지 규칙이 코드에 암묵적으로 들어 있습니다.

## 문제

- 새 필드가 추가되면 merge 로직 누락 위험이 있습니다.
- `Compose`, `Environment`, `Runner`, `Shell`, `DefaultArgs`의 merge semantics가 문서화되어 있지 않습니다.
- recursive subcommand 구조에 대한 warning/validation도 아직 얕습니다.
- 사용자는 "부모에서 무엇을 상속받는지" 예측하기 어렵습니다.

## 이번 작업의 목표

`interaction` 상속 규칙을 현재 구현 범위에서 명시적으로 정리하고, 이후 필드 추가 시 누락이 생기지 않도록 테스트와 규칙을 고정합니다.

## 범위

- parent/subcommand field inheritance 규칙 정의
- map/array/scalar별 merge 정책 정의
- nested subcommand에 대한 validation/warning 범위 검토
- `runner` 선택 로직과 interaction semantics 사이의 연결 정리

## 제외

- 새 runner 추가
- dynamic routing 자체 재설계
- command 실행 backend 변경

## 참조 정보

- `internal/runner/interaction_tree.go`
- `internal/runner/runner.go`
- `internal/cli/run.go`
- `internal/config/config.go`
- `internal/config/validate_warnings.go`
- `internal/cli/run_test.go`
- `internal/runner/runner_test.go`

## 완료 조건

- 각 interaction 필드의 상속/override 규칙이 표 또는 목록으로 정리되어 있다.
- recursive subcommand까지 포함한 validation/warning 대상이 정리되어 있다.
- runner 결정 시점과 interaction merge 시점의 책임 경계가 설명되어 있다.
- 테스트해야 할 대표 케이스가 정리되어 있다.

## 체크리스트

- [x] scalar/map/list 필드별 merge 정책이 있다.
- [x] `default_args`와 CLI 인자 결합 규칙이 적혀 있다.
- [x] `compose` 옵션 상속 규칙이 적혀 있다.
- [x] nested subcommand validation 필요 여부가 결정되어 있다.
- [x] 문서 반영 대상과 테스트 대상이 정리되어 있다.

---

# Interaction 상속/병합 규칙

## 현재 구현 분석

### mergeInteraction() 함수 (`interaction_tree.go:156-217`)

```go
func mergeInteraction(parent, child *config.InteractionCommand) *config.InteractionCommand {
    merged := &config.InteractionCommand{
        Description: parent.Description,      // ← parent 유지
        Service:     parent.Service,          // ← parent 유지
        Command:     parent.Command,          // ← parent 유지
        Workdir:     parent.Workdir,          // ← parent 유지
        User:        parent.User,             // ← parent 유지
        DefaultArgs: parent.DefaultArgs,      // ← parent 유지
        Environment: copyMap(parent.Environment), // ← parent 복사
        Shell:       parent.Shell,            // ← parent 유지
        Entrypoint:  parent.Entrypoint,       // ← parent 유지
        Runner:      parent.Runner,            // ← parent 유지
        Pod:         parent.Pod,              // ← parent 유지
        Compose:     parent.Compose,          // ← parent 복사 (struct 복사)
    }

    // Override with child values
    if child.Description != "" {
        merged.Description = child.Description  // ← child로 교체
    }
    if child.Service != "" {
        merged.Service = child.Service        // ← child로 교체
    }
    // ... (나머지 필드도 동일한 패턴)
```

### 핵심 규칙

1. **Base**: Parent 모든 값으로 초기화
2. **Override**: Child 값이 비어있지 않으면 child로 교체
3. **Environment**: Parent map 복사 후 child 키로 덮어쓰기
4. **Compose**: Parent struct 복사 후 child로 교체

---

## 필드별 상속/병합 정책

### Scalar 필드 (단일 값 교체)

| 필드 | Parent 기본값 | Child Override 조건 | 최종 값 | 설명 |
|------|--------------|------------------|----------|------|
| Description | parent.Description | `child.Description != ""` | child | 자식 설명 우선 |
| Service | parent.Service | `child.Service != ""` | child | 자식 서비스 우선 |
| Command | parent.Command | `child.Command != ""` | child | 자식 명령 우선 |
| Workdir | parent.Workdir | `child.Workdir != ""` | child | 자식 작업 디렉토리 우선 |
| User | parent.User | `child.User != ""` | child | 자식 사용자 우선 |
| DefaultArgs | parent.DefaultArgs | `child.DefaultArgs != ""` | child | 자식 기본 인자 우선 |
| Entrypoint | parent.Entrypoint | `child.Entrypoint != ""` | child | 자식 엔트리포인트 우선 |
| Runner | parent.Runner | `child.Runner != ""` | child | 자식 runner 우선 |
| Pod | parent.Pod | `child.Pod != ""` | child | 자식 pod 우선 |

**병합 정책**: **Child가 빈 문자열이 아니면 child 사용**

---

### Boolean 필드 (nil 체크)

| 필드 | Parent 기본값 | Child Override 조건 | 최종 값 | 설명 |
|------|--------------|------------------|----------|------|
| Shell | parent.Shell | `child.Shell != nil` | child | 자식 shell 모드 우선 |

**병합 정책**: **Child가 nil이 아니면 child 사용**

---

### Map 필드 (merge/dedup)

| 필드 | Parent 기본값 | Child Override 조건 | 최종 값 | 설명 |
|------|--------------|------------------|----------|------|
| Environment | copyMap(parent.Environment) | child 키로 덮어쓰기 | parent + child | child 키가 parent 키를 덮어씀 |

**병합 정책**: **Parent map 복사 후 child 키로 덮어쓰기 (merge semantics)**

**예시**:
```yaml
# dva.yml
interaction:
  build:
    environment:
      NODE_ENV: production
      API_URL: http://localhost:3000
    subcommands:
      ce:
        environment:
          API_URL: http://staging.example.com
```

**결과**:
- `build`: NODE_ENV=production, API_URL=http://localhost:3000
- `build ce`: NODE_ENV=production, **API_URL=http://staging.example.com** (child 우선)

---

### Struct 필드 (교체)

| 필드 | Parent 기본값 | Child Override 조건 | 최종 값 | 설명 |
|------|--------------|------------------|----------|------|
| Compose | parent.Compose | `child.Compose != nil` | child | 자식 compose 옵션 우전 교체 |

**병합 정책**: **Child struct가 nil이 아니면 child로 전체 교체**

**주의**: Struct 전체 교체이므로 parent의 Compose 설정이 완전히 사라짐

---

## CLI 인자 결합 규칙 (`default_args`와 CLI 인자)

### commandArgs() 함수 (`runner.go:124-132`)

```go
func commandArgs(cmd *ResolvedCommand) []string {
    if len(cmd.Argv) > 0 {
        return cmd.Argv  // ← CLI 인자 우선
    }
    if cmd.DefaultArgs != "" {
        return splitCommand(cmd.DefaultArgs)  // ← default_args fallback
    }
    return nil  // ← 둘 다 없으면 nil
}
```

### 결합 규칙

```
우선순위:
1. CLI 인자 (Argv) - 사용자가 직접 전달한 인자
2. default_args - 설정된 기본 인자
```

**특징**:
- CLI 인자가 있으면 default_args 무시
- CLI 인자가 없고 default_args가 있으면 default_args 사용
- 둘 다 없으면 nil (runner 기본값 사용)

**예시**:
```yaml
# dva.yml
interaction:
  test:
    command: go test
    default_args: ./... -v

# CLI
dva test                    # → go test ./... -v  (default_args)
dva test ./integration/      # → go test ./integration/  (CLI 우선)
```

---

## Compose 옵션 상속 규칙

### ComposeOptions struct (`interaction_tree.go:27-32`)

```go
type ComposeOpts struct {
    Method     string   // run, up, down, etc.
    Profiles   []string // compose profiles
    RunOptions []string // -- flags
}
```

### normalizeCompose() 함수 (`interaction_tree.go:124-138`)

```go
func normalizeCompose(entry *config.InteractionCommand) ComposeOpts {
    opts := ComposeOpts{
        Method: "run",  // ← 기본값
    }

    if entry.Compose != nil {
        if entry.Compose.Method != "" {
            opts.Method = entry.Compose.Method  // ← child로 교체
        }
        opts.Profiles = entry.Compose.Profiles  // ← child로 교체
        opts.RunOptions = normalizeRunOptions(entry.Compose.RunOptions)
    }

    return opts
}
```

### 상속 규칙

| 하위 필드 | Parent 기본값 | Child Override 조건 |
|-----------|--------------|------------------|
| Method | "run" | `child.Compose.Method != ""` |
| Profiles | nil | child.Compose.Profiles (무조건 교체) |
| RunOptions | nil | child.Compose.RunOptions (무조건 교체) |

**중요**: Compose는 **struct 전체 교체**이므로 parent의 설정이 완전히 사라짐

**예시**:
```yaml
# dva.yml
interaction:
  up:
    compose:
      method: up
      profiles: [infra]
    subcommands:
      dev:
        compose:
          method: run
          profiles: [dev, test]
```

**결과**:
- `up`: method=up, profiles=[infra]
- `up dev`: method=run, **profiles=[dev, test]** (parent profiles 완전히 사라짐)

---

## Runner 결정 로직과 Interaction Semantics 연결

### NewRunner() 함수 (`runner.go:32-53`)

```go
func NewRunner(cmd *ResolvedCommand, opts RunOptions) Runner {
    // 1. RunnerName 명시적 지정 우선
    if cmd.RunnerName != "" {
        switch strings.ToLower(cmd.RunnerName) {
        case RunnerDockerCompose:
            return &DockerComposeRunner{Cmd: cmd, Opts: opts}
        case RunnerKubectl:
            return &KubectlRunner{Cmd: cmd, Opts: opts}
        case RunnerLocal:
            return &LocalRunner{Cmd: cmd, Opts: opts}
        default:
            return &DockerComposeRunner{Cmd: cmd, Opts: opts}  // ← 기본값
        }
    }

    // 2. Service 지정 시 DockerCompose 기본
    if cmd.Service != "" {
        return &DockerComposeRunner{Cmd: cmd, Opts: opts}
    }

    // 3. Pod 지정 시 Kubectl 기본
    if cmd.Pod != "" {
        return &KubectlRunner{Cmd: cmd, Opts: opts}
    }

    // 4. 기본값: LocalRunner
    return &LocalRunner{Cmd: cmd, Opts: opts}
}
```

### Runner 결정 우선순위

```
1. RunnerName 명시적 지정 (최우선)
2. Service 지정 (DockerCompose)
3. Pod 지정 (Kubectl)
4. 기본값 (LocalRunner)
```

### Interaction Merge 시점

**InteractionTree.Find()** (`interaction_tree.go:45-68`):
```go
func (t *InteractionTree) Find(name string, argv ...string) *ResolvedCommand {
    entry, ok := t.entries[name]
    if !ok {
        return nil
    }

    // Expand into flat command map
    commands := t.expand(name, entry)  // ← 여기서 mergeInteraction() 호출

    // Try progressively shorter key combos
    keys := append([]string{name}, argv...)
    var rest []string

    for i := len(keys); i > 0; i-- {
        key := strings.Join(keys[:i], " ")
        if cmd, ok := commands[key]; ok {
            rest = keys[i:]
            cmd.Argv = rest
            return cmd  // ← 최종 ResolvedCommand 반환
        }
    }

    return nil
}
```

### 책임 경계

| 단계 | 위치 | 책임 |
|------|------|------|
| **Interaction Merge** | `InteractionTree.expandInto()` | parent + child 병합 |
| **Runner 결정** | `NewRunner()` | ResolvedCommand에서 runner 선택 |
| **명령 실행** | `Runner.Execute()` | 실제 명령 실행 |

**흐름**:
```
dva.yml interaction
       ↓
InteractionTree.Find()
       ↓
expandInto() → mergeInteraction() [parent + child 병합]
       ↓
ResolvedCommand 생성
       ↓
NewRunner() [runner 선택]
       ↓
Runner.Execute() [실행]
```

---

## Nested Subcommand Validation/Warning 범위

### 현재 상태

| 검사 | 현재 구현 | 범위 |
|------|-----------|------|
| Duplicate parent/subcommand command | warnDuplicateParentSubcommand() | parent와 child가 동일한 command 사용 시 warning |
| Hookable 여부 | validate.go:88-93 | non-hookable 명령에 hook 사용 시 hard error |

### 제안: 추가 Validation/Warning

#### 1. Child overriding parent critical field (Semantic Warning)

```go
// validate_warnings.go에 추가
func (c *Config) warnChildOverridesParentCritical() []string {
    var warnings []string

    for name, cmd := range c.Interaction {
        for subName, sub := range cmd.Subcommands {
            // Runner 변경 감지
            if cmd.Runner != "" && sub.Runner != "" && cmd.Runner != sub.Runner {
                warnings = append(warnings,
                    fmt.Sprintf("interaction.%s.subcommands.%s: overrides parent runner (%s → %s); this may change execution backend unexpectedly",
                        name, subName, cmd.Runner, sub.Runner))
            }

            // Pod 변경 감지
            if cmd.Pod != "" && sub.Pod != "" && cmd.Pod != sub.Pod {
                warnings = append(warnings,
                    fmt.Sprintf("interaction.%s.subcommands.%s: overrides parent pod (%s → %s); this may change execution backend unexpectedly",
                        name, subName, cmd.Pod, sub.Pod))
            }
        }
    }

    return warnings
}
```

#### 2. Deep nesting depth limit (Semantic Warning)

```go
const MaxSubcommandDepth = 5

func (c *Config) warnDeepSubcommandNesting() []string {
    var warnings []string

    for name, cmd := range c.Interaction {
        depth := calculateSubcommandDepth(cmd, 0)
        if depth > MaxSubcommandDepth {
            warnings = append(warnings,
                fmt.Sprintf("interaction.%s: nested %d levels deep (max %d); consider flattening the command structure",
                    name, depth, MaxSubcommandDepth))
        }
    }

    return warnings
}

func calculateSubcommandDepth(cmd *config.InteractionCommand, current int) int {
    if len(cmd.Subcommands) == 0 {
        return current
    }

    maxDepth := current + 1
    for _, sub := range cmd.Subcommands {
        depth := calculateSubcommandDepth(sub, current+1)
        if depth > maxDepth {
            maxDepth = depth
        }
    }

    return maxDepth
}
```

**결정**: **경고만 추가 (Semantic Warning)**
- 이유: deeply nested 구조는 유효하지만 DX 저하 가능성
- Hard Error 아님: 잘못된 구조가 아님

#### 3. Unreachable commands (Semantic Warning)

```go
func (c *Config) warnUnreachableCommands() []string {
    var warnings []string

    tree := NewInteractionTree(c.Interaction)
    allResolved := tree.List()

    // subcommands만 있는 명령 검사
    for name, cmd := range c.Interaction {
        if len(cmd.Subcommands) > 0 {
            // parent 자체로 호출 가능한지 확인
            if _, ok := allResolved[name]; !ok {
                warnings = append(warnings,
                    fmt.Sprintf("interaction.%s: has subcommands but is not directly callable; add a command field or remove subcommands",
                        name))
            }
        }
    }

    return warnings
}
```

---

## 테스트 시나리오

### 시나리오 1: 일반적인 상속 (scalar fields)

```yaml
# dva.yml
interaction:
  build:
    command: make build
    workdir: /src
    subcommands:
      ce:
        command: make build-ce
```

**기대 동작**:
- `build`: command=make build, workdir=/src
- `build ce`: command=make build-ce, workdir=/src (parent workdir 상속)

**테스트 코드**:
```go
func TestMergeInteraction_ScalarFields(t *testing.T) {
    parent := &config.InteractionCommand{
        Command: "make build",
        Workdir: "/src",
    }
    child := &config.InteractionCommand{
        Command: "make build-ce",
    }

    merged := mergeInteraction(parent, child)

    if merged.Command != "make build-ce" {
        t.Errorf("Command = %q, want 'make build-ce'", merged.Command)
    }
    if merged.Workdir != "/src" {
        t.Errorf("Workdir = %q, want '/src'", merged.Workdir)
    }
}
```

---

### 시나리오 2: Environment map merge

```yaml
# dva.yml
interaction:
  deploy:
    environment:
      NODE_ENV: production
      API_URL: http://api.example.com
    subcommands:
      staging:
        environment:
          API_URL: http://staging.example.com
```

**기대 동작**:
- `deploy`: NODE_ENV=production, API_URL=http://api.example.com
- `deploy staging`: NODE_ENV=production (parent), API_URL=http://staging.example.com (child override)

**테스트 코드**:
```go
func TestMergeInteraction_EnvironmentMap(t *testing.T) {
    parentEnv := map[string]string{
        "NODE_ENV": "production",
        "API_URL":  "http://api.example.com",
    }
    childEnv := map[string]string{
        "API_URL": "http://staging.example.com",
    }

    parent := &config.InteractionCommand{Environment: parentEnv}
    child := &config.InteractionCommand{Environment: childEnv}

    merged := mergeInteraction(parent, child)

    if merged.Environment["NODE_ENV"] != "production" {
        t.Errorf("NODE_ENV = %q, want 'production'", merged.Environment["NODE_ENV"])
    }
    if merged.Environment["API_URL"] != "http://staging.example.com" {
        t.Errorf("API_URL = %q, want 'http://staging.example.com'", merged.Environment["API_URL"])
    }
}
```

---

### 시나리오 3: Compose struct 완전 교체

```yaml
# dva.yml
interaction:
  up:
    compose:
      method: up
      profiles: [infra]
    subcommands:
      dev:
        compose:
          method: run
          profiles: [dev]
```

**기대 동작**:
- `up`: method=up, profiles=[infra]
- `up dev`: method=run, profiles=[dev] (parent profiles 완전히 사라짐)

**테스트 코드**:
```go
func TestMergeInteraction_ComposeReplace(t *testing.T) {
    parentCompose := &config.ComposeOptions{
        Method:   "up",
        Profiles:  []string{"infra"},
    }
    childCompose := &config.ComposeOptions{
        Method:   "run",
        Profiles:  []string{"dev"},
    }

    parent := &config.InteractionCommand{Compose: parentCompose}
    child := &config.InteractionCommand{Compose: childCompose}

    merged := mergeInteraction(parent, child)

    if merged.Compose.Method != "run" {
        t.Errorf("Method = %q, want 'run'", merged.Compose.Method)
    }
    if !slices.Equal(merged.Compose.Profiles, []string{"dev"}) {
        t.Errorf("Profiles = %v, want [dev]", merged.Compose.Profiles)
    }
}
```

---

### 시나리오 4: CLI 인자 vs default_args

```yaml
# dva.yml
interaction:
  test:
    command: go test
    default_args: ./...
```

**기대 동작**:
- `dva test`: go test ./... (default_args)
- `dva test ./integration`: go test ./integration (CLI 우선)

**테스트 코드**:
```go
func TestCommandArgs_CLIOverride(t *testing.T) {
    cmd := &ResolvedCommand{
        Command:     "go test",
        DefaultArgs: "./...",
        Argv:        nil,
    }

    if got := commandArgs(cmd); !slices.Equal(got, []string{"./..."}) {
        t.Errorf("Args = %v, want [./...]", got)
    }

    // CLI 인자 우선
    cmd.Argv = []string{"./integration"}
    if got := commandArgs(cmd); !slices.Equal(got, []string{"./integration"}) {
        t.Errorf("Args = %v, want [./integration]", got)
    }
}
```

---

### 시나리오 5: Deep nesting 경고

```yaml
# dva.yml
interaction:
  app:
    subcommands:
      backend:
        subcommands:
          api:
            subcommands:
              graphql:
                command: echo "too deep"
```

**기대 동작**:
- 경고: "interaction.app: nested 5 levels deep (max 5)"

**테스트 코드**:
```go
func TestWarnDeepSubcommandNesting(t *testing.T) {
    c := &Config{
        Interaction: map[string]*InteractionCommand{
            "app": {
                Subcommands: map[string]*InteractionCommand{
                    "backend": {
                        Subcommands: map[string]*InteractionCommand{
                            "api": {
                                Subcommands: map[string]*InteractionCommand{
                                    "graphql": {},
                                },
                            },
                        },
                    },
                },
            },
        },
    }

    warnings := c.warnDeepSubcommandNesting()
    if len(warnings) == 0 {
        t.Error("expected warning for deep nesting")
    }
    if !strings.Contains(warnings[0], "nested 5 levels deep") {
        t.Errorf("unexpected warning: %s", warnings[0])
    }
}
```

---

## 문서 반영 대상

### 사용자 문서

| 문서 | 내용 |
|------|------|
| README.md | interaction 섹션 설명, 상속 규칙, CLI 인자 결합 |
| docs/interaction-syntax.md (신규) | interaction DSL 상세 가이드 |

### 개발자 문서

| 문서 | 내용 |
|------|------|
| CONTRIBUTING.md (또는 DEVELOPMENT.md) | interaction merge 설계 의도, 필드 추가 시 주의사항 |
| internal/runner/interaction_tree.go 주석 | mergeInteraction() 동작 설명 추가 |

---

## 후속 구현 순서

### 단계 1: 문서화 완료 (본 태스크)
- [x] 현재 구현 분석
- [x] 필드별 상속/병합 규칙 정리
- [x] CLI 인자 결합 규칙 정리
- [x] Compose 옵션 상속 규칙 정리
- [x] Runner 결정 로직 연결 설명
- [x] 테스트 시나리오 정의

### 단계 2: Validation/Warning 구현 (신규 태스크)
- [ ] warnChildOverridesParentCritical() 추가
- [ ] warnDeepSubcommandNesting() 추가
- [ ] warnUnreachableCommands() 추가
- [ ] ValidateWarnings()에 새 warning 추가
- [ ] 관련 테스트 추가

### 단계 3: 테스트 보강
- [ ] 각 시나리오별 테스트 구현
- [ ] 경계 케이스 테스트 (scalar vs map vs struct)
- [ ] 깊은 nesting 테스트
- [ ] CLI 인자 vs default_args 테스트

### 단계 4: 문서 반영
- [ ] README.md에 interaction 섹션 추가
- [ ] docs/interaction-syntax.md 작성
- [ ] 코드 주석 추가

---

## 결론

이 문서는 DVA의 interaction 상속/병합 규칙을 명시합니다. 현재 구현은 명시적이지 않지만 일관성 있는 패턴을 따르며, 향후 필드 추가 시 누락이 생기지 않도록 명확한 가이드를 제공합니다.

**핵심 규칙 요약**:
1. **Scalar fields**: Child가 비어있지 않으면 child 사용
2. **Boolean fields**: Child가 nil이 아니면 child 사용
3. **Map fields (Environment)**: Parent 복사 후 child 키로 덮어쓰기
4. **Struct fields (Compose)**: Child가 nil이 아니면 child로 전체 교체
5. **CLI 인자**: CLI > default_args
6. **Runner 결정**: RunnerName > Service > Pod > Local

이 명세는 사용자가 interaction DSL을 사용할 때 "어떤 값이 상속되고 어떤 값이 override되는지" 명확히 이해할 수 있도록 돕습니다.
