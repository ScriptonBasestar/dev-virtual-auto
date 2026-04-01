---
archived-at: 2026-04-01T15:00:35+09:00
verified-at: 2026-04-01T15:00:35+09:00
verification-summary: "Review completed. Environment interpolation semantics, syntax exclusions, and testing scenarios are documented per completion criteria."
---
# Environment Interpolation Hardening

## 배경

현재 env interpolation은 `$VAR`, `${VAR}` 수준만 지원합니다.
DVA가 설정 오케스트레이터 역할을 하려면, interpolation 실패나 default 처리 규칙이 더 명확해야 합니다.

## 현재 상태

- 환경 변수 모델: `internal/config/environment.go`
- env file 로딩: `internal/config/envfile.go`
- 관련 테스트:
  - `internal/config/environment_test.go`
  - `internal/config/envfile_test.go`

현재 구현은 미해결 변수를 원문 그대로 남기며, unresolved 상태를 별도 경고하지 않습니다.
쉘 스타일 기본값 문법이나 required 문법도 없습니다.

## 문제

- 오타가 있어도 조용히 지나갈 수 있습니다.
- `${VAR:-default}` 같은 실용적인 패턴이 없습니다.
- 향후 environment 기반 stack/plugin 설정 확장 시 표현력이 부족합니다.
- interpolation 규칙이 문서와 validation에서 충분히 드러나지 않습니다.

## 이번 작업의 목표

현재 구현 범위 안에서 interpolation semantics를 명확히 하고, 작은 범위 확장 또는 실패 노출 방식을 도입할 준비를 합니다.
이 태스크는 "전체 셸 호환"이 아니라 DVA config에서 실제 필요한 규칙을 닫는 것이 목적입니다.

## 범위

- interpolation semantics 설계
- validation/warning 연계 필요성 검토
- 문서와 테스트 시나리오 정리

## 제외

- shell 전체 문법 호환
- secret manager 연동
- stack_overrides 자체 구현

## 참조 정보

- `internal/config/environment.go`
- `internal/config/envfile.go`
- `internal/config/environment_test.go`
- `internal/config/envfile_test.go`
- `internal/config/schema.json`
- 연관 태스크: `tasks/backlog/config-deep-merge-semantics.md`

## 완료 조건

- 지원할 interpolation 문법이 명시되어 있다.
- 지원하지 않을 문법도 명시되어 있다.
- 우선순위와 실패 동작이 정리되어 있다.
- warning 또는 validation으로 노출할 조건이 정의되어 있다.
- 테스트 시나리오가 최소 정상/기본값/오류/순환 참조 케이스로 구분되어 있다.

## 체크리스트

- [x] OS env vs config env vs env_file 우선순위가 명시되어 있다.
- [x] unresolved variable 처리 방침이 있다.
- [x] default syntax 지원 여부가 결정되어 있다.
- [x] required syntax 지원 여부가 결정되어 있다.
- [x] 문서 반영 대상이 정리되어 있다.

---

# Environment Interpolation Semantics

## 현재 구현 분석

### 지원하는 문법

| 패턴 | 예시 | 설명 |
|-------|-------|------|
| `$VAR` | `$DATABASE_URL` | 단순 변수 참조 |
| `${VAR}` | `${DATABASE_URL}` | 중괄호 변수 참조 |

**현재 정규식** (`environment.go:13`):
```go
var varRegex = regexp.MustCompile(`\$\{?([a-zA-Z_][a-zA-Z0-9_]*)\}?`)
```

### 우선순위 규칙

**MergeVars()** (`environment.go:58-66`):
1. OS 환경 변수에 존재하면 OS 값을 사용
2. OS에 없으면 config 값을 사용 (interpolation 적용)

**Interpolate()** (`environment.go:69-84`):
1. Config vars 먼저 확인
2. Config에 없으면 OS env 확인
3. 둘 다 없으면 원문 반환 (silent failure)

**EnvSlice()** (`environment.go:88-106`):
- Config vars가 OS env와 같은 키를 가지면 override
- Config vars가 최우선순위

### env_file 처리

**EnvFileConfig** (`envfile.go:15-18`):
- `path`: 파일 경로 (필수)
- `required`: true/false (기본 false)

**파싱** (`envfile.go:87-111`):
- `#`로 시작하는 주석 무시
- `export` 접두사 선택적 (무시됨)
- Single quote: escape sequences 없음
- Double quote: `\\`, `\n`, `\t`, `\r`, `\"` 처리

---

## 제안 Interpolation Semantics

### 지원 문법 (현재 유지 + 명시화)

| 패턴 | 예시 | 의미 |
|-------|-------|------|
| `$VAR` | `$DATABASE_URL` | 단순 변수 참조 |
| `${VAR}` | `${DATABASE_URL}` | 중괄호 변수 참조 |

### 지원하지 않을 문법 (명시적 거부)

| 패턴 | 예시 | 이유 |
|-------|-------|------|
| `${VAR:-default}` | `${PORT:-8080}` | 셸 호환성 범위 초과 |
| `${VAR:=default}` | `${PORT:=8080}` | 셸 호환성 범위 초과 |
| `${VAR:+default}` | `${PORT:+8080}` | 셸 호환성 범위 초과 |
| `${VAR:?message}` | `${PORT:?required}` | 셸 호환성 범위 초과 |
| `$#VAR` | `$#PATH` | 셸 특수 변수 |
| `${VAR#pattern}` | `${PATH#/usr/bin}` | 셸 패턴 치환 |
| `${VAR%pattern}` | `${FILE%.txt}` | 셸 패턴 치환 |
| `${VAR:offset:length}` | `${VAR:0:10}` | 셸 부분문자열 |

**결정**: "전체 셸 호환"이 아니라 DVA config에 필요한 최소한의 기능만 제공

---

## 우선순위 규칙 (명시화)

### 3계층 우선순위

```
1. OS 환경 변수 (최우선)
   ↓
2. env_file에서 로드된 변수 (OS에 없을 때만)
   ↓
3. dva.yml environment 섹션 변수 (OS와 env_file에 없을 때만)
```

### 환경 변수 로드 순서

```
1. OS 환경 변수 로드 (os.Environ())
2. env_file 순차적 로드 (후송 파일이 이전을 덮어쓰지 않음)
3. dva.yml environment 섹션 병합 (OS/env_file 우선 유지)
4. Special 변수 설정 (DVA_OS, DVA_WORK_DIR_REL_PATH 등)
```

### Interpolation 우선순위

```
1. Config vars 확인
2. OS env 확인
3. 없으면 원문 반환 (silent)
```

**현재 동작 유지**: OS env가 config vars보다 우선순위 높음

---

## Unresolved Variable 처리 방침

### 현재 동작

```go
// environment.go:69-84
func (e *Environment) Interpolate(value string) string {
    return varRegex.ReplaceAllStringFunc(value, func(match string) string {
        varName := varRegex.FindStringSubmatch(match)[1]

        // Check our vars first
        if v, ok := e.Vars[varName]; ok {
            return v
        }
        // Then check OS env
        if v, ok := os.LookupEnv(varName); ok {
            return v
        }
        // Return original if not found
        return match  // silent failure
    })
}
```

### 제안: 경고 노출 방식

**옵션 A: Semantic Warning 추가 (추천)**
```go
// validate_warnings.go에 추가
func (c *Config) warnUnresolvedEnvVars() []string {
    var warnings []string

    // environment 섹션의 값 확인
    for k, v := range c.Environment {
        unresolved := extractUnresolvedVars(c.Interpolate(v))
        if len(unresolved) > 0 {
            warnings = append(warnings,
                fmt.Sprintf("environment.%s: unresolved variables %v; these will remain as literal text",
                    k, unresolved))
        }
    }

    return warnings
}

func extractUnresolvedVars(s string) []string {
    // $VAR 또는 ${VAR} 패턴을 찾되,
    // 실제로는 resolve되지 않은 것들만 추출
    // ...
}
```

**장점**:
- `dva config validate`에서 노출
- `--strict` 시 에러로 취급 가능
- 사용자에게 명확한 피드백 제공

**단점**:
- Validation 과정에서 interpolation을 다시 수행해야 함
- Runtime과 다른 결과 가능성 (edge case)

**옵션 B: Doctor Check 추가**
```go
// doctor.go에 추가
func checkUnresolvedEnvVars(c *config.Config) DoctorResult {
    unresolved := collectAllUnresolvedVars(c)
    if len(unresolved) == 0 {
        return DoctorResult{Name: "Environment variables resolved", Passed: true}
    }

    return DoctorResult{
        Name:    "Environment variables unresolved",
        Passed:   false,
        FixHint:  fmt.Sprintf("Undefined variables: %s", strings.Join(unresolved, ", ")),
    }
}
```

**장점**:
- Doctor에 "환경 프리플라이트" 관점으로 적합
- 실행 전 한 번에 확인 가능

**단점**:
- Doctor 실행이 필요 (validate에서는 보이지 않음)

**결정**: **Semantic Warning 추가 (옵션 A)**
- 이유: 오타/실수 조기 발견에 더 적합
- Doctor 추가는 별도 태스크로 고려 가능

---

## Default Syntax 지원 여부

### Shell 스타일 default 문법

| 문법 | 예시 | 의미 |
|-------|-------|------|
| `${VAR:-default}` | `${PORT:-8080}` | VAR이 unset/null이면 default 사용 |
| `${VAR-default}` | `${PORT-8080}` | VAR이 unset/null이면 default 사용 (POSIX 비호환) |

### 결정: 지원하지 않음

**이유**:
1. 범위 초과: DVA config에 필요한 기능 범위 초과
2. 복잡도: 구현/테스트/문서 비용 대비 실용성 낮음
3. 우회책 가능: dva.yml에서 직접 기본값 설정 가능

**대안 제안**:
```yaml
# dva.yml
environment:
  # 대신 config에서 기본값 설정
  PORT: ${PORT:-8080}  # ❌ 지원하지 않음
  PORT: "8080"          # ✅ config에서 기본값
  PORT: ${PORT}          # ✅ OS env 사용 (기본값은 OS에서 설정)

# 또는 env_file에서 default 설정
# .env
PORT=8080
```

---

## Required Syntax 지원 여부

### Shell 스타일 required 문법

| 문법 | 예시 | 의미 |
|-------|-------|------|
| `${VAR:?message}` | `${API_KEY:?required}` | VAR이 unset/null이면 message 출력하고 종료 |

### 결정: 지원하지 않음

**이유**:
1. 범위 초과: DVA config에 필요한 기능 범위 초과
2. Validation이 더 적합: Hard Error로 처리하는 것이 더 명확
3. 우회책 가능: schema validation에서 required 필드로 처리

**대안 제안**:
```yaml
# dva.yml
environment:
  API_KEY: ${API_KEY:?required}  # ❌ 지원하지 않음
  # 대신 OS env에서 required 체크:
  # bash: export API_KEY=${API_KEY:?API_KEY is required}
```

---

## Validation/Warning 연계

### 현재 상태

| 문제 | 현재 처리 |
|-------|----------|
| 오타로 인한 unresolved var | silent failure |
| env_file 누락 (required=false) | silently skipped |
| 잘못된 문법 ($#VAR 등) | ignored (regex로 매칭 안됨) |

### 제안: Validation/Warning 추가

#### 1. Unresolved variables (Semantic Warning)

**위치**: `internal/config/validate_warnings.go`

```go
func (c *Config) warnUnresolvedEnvVars() []string {
    var warnings []string

    // environment 섹션 확인
    for k, v := range c.Environment {
        if hasUnresolvedVar(c.Env, v) {
            warnings = append(warnings,
                fmt.Sprintf("environment.%s: contains unresolved variable reference; verify variable name",
                    k))
        }
    }

    return warnings
}
```

#### 2. Suspicious patterns (Semantic Warning)

```go
func (c *Config) warnSuspiciousEnvPatterns() []string {
    var warnings []string

    for k, v := range c.Environment {
        // 셸 특수 변수 패턴
        if strings.Contains(v, "$#") || strings.Contains(v, "${") && strings.Contains(v, ":") {
            warnings = append(warnings,
                fmt.Sprintf("environment.%s: contains shell-specific syntax that is not supported; use plain $VAR or ${VAR}", k))
        }
    }

    return warnings
}
```

#### 3. env_file required missing (Hard Error - 이미 구현됨)

**위치**: `internal/config/envfile.go:34-36`

이미 구현됨:
```go
if os.IsNotExist(err) {
    if f.Required {
        return fmt.Errorf("required environment file not found: %s", path)
    }
    continue // optional, skip
}
```

---

## 테스트 시나리오

### 시나리오 1: 정상적인 환경 변수 사용

```yaml
# dva.yml
environment:
  DATABASE_URL: ${DATABASE_URL}
  PORT: 8080

# .env
DATABASE_URL=postgres://localhost/mydb
```

**기대 동작**:
- DATABASE_URL: .env에서 로드
- PORT: config에서 설정
- Interpolation 성공

**테스트 코드**:
```go
func TestEnvironment_ResolvedVars(t *testing.T) {
    os.Setenv("DATABASE_URL", "postgres://localhost/test")

    env := NewEnvironment(map[string]string{"PORT": "8080"}, ".", ".")
    env.MergeVars(map[string]string{
        "DATABASE_URL": "${DATABASE_URL}",
        "PORT": "${PORT}",
    })

    if env.Vars["DATABASE_URL"] != "postgres://localhost/test" {
        t.Errorf("DATABASE_URL = %q, want 'postgres://localhost/test'", env.Vars["DATABASE_URL"])
    }
    if env.Vars["PORT"] != "8080" {
        t.Errorf("PORT = %q, want '8080'", env.Vars["PORT"])
    }
}
```

---

### 시나리오 2: 오타로 인한 unresolved 변수

```yaml
# dva.yml
environment:
  # 오타: DATABASE_UEL vs DATABASE_URL
  DATABASE_UEL: ${DATABASE_URL}
```

**기대 동작 (warning)**:
- Semantic Warning 발생: "environment.DATABASE_UEL: contains unresolved variable reference"

**테스트 코드**:
```go
func TestEnvironment_UnresolvedVar(t *testing.T) {
    env := NewEnvironment(nil, ".", ".")
    env.MergeVars(map[string]string{
        "DATABASE_UEL": "${DATABASE_URL}",  // DATABASE_URL는 정의되지 않음
    })

    // 원문 그대로 남음
    if env.Vars["DATABASE_UEL"] != "${DATABASE_URL}" {
        t.Errorf("DATABASE_UEL = %q, want '${DATABASE_URL}'", env.Vars["DATABASE_UEL"])
    }

    // Warning 발생 확인
    warnings := warnUnresolvedEnvVars(env)
    if len(warnings) == 0 {
        t.Error("expected warning for unresolved variable")
    }
}
```

---

### 시나리오 3: 기본값 사용 (config에서 직접 설정)

```yaml
# dva.yml
environment:
  # OS env가 없으면 config 기본값 사용
  PORT: 8080

# OS env에 PORT가 없음
```

**기대 동작**:
- PORT: config에서 설정된 8080 사용

**테스트 코드**:
```go
func TestEnvironment_DefaultFromConfig(t *testing.T) {
    os.Unsetenv("PORT")

    env := NewEnvironment(nil, ".", ".")
    env.MergeVars(map[string]string{"PORT": "8080"})

    if env.Vars["PORT"] != "8080" {
        t.Errorf("PORT = %q, want '8080'", env.Vars["PORT"])
    }
}
```

---

### 시나리오 4: OS env 우선순위

```yaml
# dva.yml
environment:
  DATABASE_URL: postgres://localhost/mydb
```

**OS env**:
```bash
export DATABASE_URL=postgres://remotehost/proddb
```

**기대 동작**:
- DATABASE_URL: OS env 값 사용 (postgres://remotehost/proddb)

**테스트 코드**:
```go
func TestEnvironment_OSOverride(t *testing.T) {
    os.Setenv("DATABASE_URL", "postgres://remotehost/proddb")

    env := NewEnvironment(nil, ".", ".")
    env.MergeVars(map[string]string{
        "DATABASE_URL": "postgres://localhost/mydb",
    })

    if env.Vars["DATABASE_URL"] != "postgres://remotehost/proddb" {
        t.Errorf("DATABASE_URL = %q, want 'postgres://remotehost/proddb'", env.Vars["DATABASE_URL"])
    }
}
```

---

### 시나리오 5: 순환 참조 (현재 처리됨)

```yaml
# dva.yml
environment:
  BASE_URL: http://localhost:8080
  API_URL: ${BASE_URL}/api
```

**기대 동작**:
- 10번 반복으로 resolve (interpolateEnvVars: maxIterations = 10)
- API_URL: http://localhost:8080/api

**테스트 코드**:
```go
func TestEnvironment_CircularResolve(t *testing.T) {
    env := NewEnvironment(nil, ".", ".")
    env.MergeVars(map[string]string{
        "BASE_URL": "http://localhost:8080",
        "API_URL":  "${BASE_URL}/api",
    })

    expected := "http://localhost:8080/api"
    if env.Vars["API_URL"] != expected {
        t.Errorf("API_URL = %q, want %q", env.Vars["API_URL"], expected)
    }
}
```

---

### 시나리오 6: env_file 우선순위 (후송 파일)

```yaml
# dva.yml
env_file:
  - .env
  - .env.local

# .env
DATABASE_URL=postgres://localhost/mydb

# .env.local
DATABASE_URL=postgres://localhost/localdb
```

**기대 동작**:
- .env 먼저 로드
- .env.local이 덮어쓰지 않음 (후송 파일 병합이므로)
- OS env가 있으면 최종 우선

**테스트 코드**:
```go
func TestEnvFile_MultipleFiles(t *testing.T) {
    // .env.local에 DATABASE_URL 설정
    // OS env에도 DATABASE_URL 설정

    env := NewEnvironment(nil, ".", ".")
    LoadEnvFile([]string{".env", ".env.local"}, ".", env)

    // OS env가 최우선
    os.Setenv("DATABASE_URL", "os://value")
    if env.Vars["DATABASE_URL"] != "os://value" {
        t.Errorf("DATABASE_URL = %q, want 'os://value'", env.Vars["DATABASE_URL"])
    }

    // OS env 없으면 후송 파일 우선
    os.Unsetenv("DATABASE_URL")
    env = NewEnvironment(nil, ".", ".")
    LoadEnvFile([]string{".env", ".env.local"}, ".", env)
    if env.Vars["DATABASE_URL"] != "postgres://localhost/localdb" {
        t.Errorf("DATABASE_URL = %q, want 'postgres://localhost/localdb'", env.Vars["DATABASE_URL"])
    }
}
```

---

### 시나리오 7: env_file required 누락

```yaml
# dva.yml
env_file:
  path: .env
  required: true

# .env 파일이 없음
```

**기대 동작**:
- Hard Error: "required environment file not found: .env"

**테스트 코드**:
```go
func TestEnvFile_RequiredMissing(t *testing.T) {
    env := NewEnvironment(nil, t.TempDir(), ".")

    err := LoadEnvFile(map[string]any{
        "path":     ".env",
        "required": true,
    }, env.cfgDir, env)

    if err == nil {
        t.Error("expected error for missing required env file")
    }
    if !strings.Contains(err.Error(), "required environment file not found") {
        t.Errorf("error message = %q, want 'required environment file not found'", err.Error())
    }
}
```

---

## 문서 반영 대상

### 사용자 문서

| 문서 | 내용 |
|------|------|
| README.md | environment 섹션 설명, interpolation 문법, 우선순위 |
| docs/configuration.md (신규) | 환경 변수 상세 설정 가이드 |
| docs/troubleshooting.md (신규) | 환경 변수 관련 문제 해결 가이드 |

### 개발자 문서

| 문서 | 내용 |
|------|------|
| CONTRIBUTING.md (또는 DEVELOPMENT.md) | 환경 변수 interpolation 설계 의도 |
| internal/config/environment.go 주석 | Interpolate 함수 동작 설명 추가 |

---

## 후속 구현 순서

### 단계 1: 문서화 완료 (본 태스크)
- [x] 현재 구현 분석
- [x] 지원/비지원 문법 명시
- [x] 우선순위 규칙 명시화
- [x] unresolved variable 처리 방침 결정
- [x] default/required syntax 지원 여부 결정
- [x] 테스트 시나리오 정의

### 단계 2: Validation/Warning 구현 (신규 태스크)
- [ ] warnUnresolvedEnvVars 함수 추가
- [ ] warnSuspiciousEnvPatterns 함수 추가
- [ ] ValidateWarnings()에 새 warning 추가
- [ ] 관련 테스트 추가

### 단계 3: 테스트 보강
- [ ] 각 시나리오별 테스트 구현
- [ ] 경계 케이스 테스트 (OS env vs config vs env_file)
- [ ] env_file 다중 파일 로드 테스트
- [ ] 순환 참조 깊이 제한 테스트

### 단계 4: 문서 반영
- [ ] README.md에 environment 섹션 추가
- [ ] docs/configuration.md 작성
- [ ] 코드 주석 추가

### 단계 5: 향후 고려 (다른 태스크)
- [ ] config-deep-merge-semantics 태스크 참조
- [ ] env_stack_overrides 구현 여부 결정

---

## 결론

이 문서는 DVA의 환경 변수 interpolation semantics를 명시합니다. 현재 구현은 최소한의 기능을 제공하며, 향후 확장을 위한 명확한 경계를 설정합니다.

**핵심 결정**:
1. **지원 문법**: `$VAR`, `${VAR}` 유지
2. **비지원 문법**: shell 특정 문법 (`${VAR:-default}`, `$#VAR` 등) 명시적 거부
3. **우선순위**: OS env > env_file > config environment
4. **Unresolved 처리**: Semantic Warning 추가 (옵션 A 선택)
5. **Default/Required syntax**: 지원하지 않음 (config에서 직접 설정 권장)

이 명세는 사용자가 환경 변수 설정 시 어떤 문법이 지원되는지 명확히 이해할 수 있도록 돕습니다.
