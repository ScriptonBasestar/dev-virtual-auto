---
archived-at: 2026-04-01T15:00:35+09:00
verified-at: 2026-04-01T15:00:35+09:00
verification-summary: "Review completed. Severity levels (Hard Error, Semantic Warning, Drift Warning, Suggestion Warning, Doctor Check) are strictly defined and categorizations are provided."
---
# Validation Severity Policy

## 배경

현재 schema validation, semantic warnings, reserved command 충돌, compose project name 검사 등이 분산되어 있습니다.
같은 문제라도 hard error, warning, doctor result 중 어디에 속해야 하는지 일관성이 약합니다.

## 현재 상태

- schema validation: `internal/config/validate.go`
- semantic warnings: `internal/config/validate_warnings.go`
- CLI 진입점: `internal/cli/validate.go`
- 관련 테스트:
  - `internal/config/validate_warnings_test.go`
  - `internal/cli/validate_test.go`

## 문제

- 어떤 문제는 hard error가 맞고, 어떤 문제는 warning이 맞지만 경계가 문서화되어 있지 않습니다.
- 동일한 정보를 `validate`와 `doctor`가 중복해서 가질 위험이 있습니다.
- 새 검사를 추가할 때 어느 계층에 넣어야 하는지 판단 기준이 약합니다.

## 이번 작업의 목표

config 및 실행 전 진단 항목을 `hard error`, `warning`, `doctor-only`로 나누는 기준을 고정합니다.

## 범위

- validation severity 기준 정리
- 기존 검사 항목의 재배치 후보 정리
- `doctor`와 `validate`의 책임 경계 명시

## 제외

- 개별 check 구현 전체
- plugin별 bespoke validation
- config deep merge semantics 변경

## 참조 정보

- `internal/config/validate.go`
- `internal/config/validate_warnings.go`
- `internal/cli/validate.go`
- `internal/cli/doctor.go`
- `internal/config/validate_warnings_test.go`
- `internal/cli/validate_test.go`
- 연관 태스크: `tasks/todo/doctor-preflight-expansion.md`

## 완료 조건

- 진단 항목 분류 기준이 문서화되어 있다.
- 기존 주요 검사 항목이 `hard error`, `warning`, `doctor-only` 중 하나로 배치되어 있다.
- `doctor`와 `validate`의 역할 경계가 설명되어 있다.
- 중복 검사 제거 또는 재사용 방향이 정리되어 있다.
- 테스트와 문서 반영 대상이 정리되어 있다.

## 체크리스트

- [x] severity 판단 기준이 있다.
- [x] 기존 주요 검사 항목의 분류표가 있다.
- [x] doctor와 validate 경계가 적혀 있다.
- [x] compose project name mismatch 처리 위치가 정리되어 있다.
- [x] 후속 구현 순서가 적혀 있다.

---

# Validation Severity 기준

## 세 가지 계층

### 1. Hard Error (`Validate()` - config/validate.go)
**정의**: 실행을 차단해야 하는 구조적/의미적 오류

**특징**:
- YAML 문법 오류
- JSON Schema 위반
- DVA 명령어와의 충돌로 인한 기능 상실
- 존재하지 않는 리소스 참조 (예: default_mode가 modes에 없음)
- **반드시** `dva.yml` 실행 전에 해결되어야 함

**현재 포함 항목**:
| 항목 | 설명 | 파일 |
|------|------|------|
| YAML 구문 오류 | YAML 파싱 실패 | validate.go:41-43 |
| Schema 위반 | JSON Schema 검증 실패 | validate.go:59-65 |
| Reserved command 충돌 | DVA 예약어 사용으로 인한 명령 섀도 | validate.go:67-86 |
| Hookable 아닌 명령에 hook | before/replace/after 사용 불가능 | validate.go:88-93 |
| 유효하지 않은 default_mode | modes에 없는 default_mode 참조 | validate.go:95-107 |

---

### 2. Semantic Warning (`ValidateWarnings()` - config/validate_warnings.go)
**정의**: 기능은 작동하지만, 모범 사례/설계 원칙 위배 가능성이 있는 항목

**특징**:
- 실행 차단은 아님
- 사용자 경험/유지보수성/성능에 영향 가능성
- `dva config validate` 또는 `dva up` 실행 시 `[warn]` 접두사로 표시
- `--strict` 플래그 시 에러로 취급

**현재 포함 항목**:
| 항목 | 설명 | 파일 |
|------|------|------|
| 버전 낙후 | config 버전이 binary보다 오래됨 | validate_warnings.go:48-68 |
| Health check 중복 | start + start_hint 동시 사용 | validate_warnings.go:70-93 |
| 부모/자식 명령 중복 | parent와 subcommand가 동일한 command 사용 | validate_warnings.go:95-115 |
| Stack order 중복 | 동일한 order 값 사용 | validate_warnings.go:117-154 |
| 다중 compose entry | 여러 compose plugin 사용 (모드 사용 권장) | validate_warnings.go:156-173 |
| default_mode 누락 | modes 정의되었으나 default_mode 없음 | validate_warnings.go:175-185 |
| default_mode heavy infra | 기본 모드에 무거운 인프라 포함 | validate_warnings.go:187-250 |
| 섹션 순서 위반 | 권장 섹션 순서와 다름 | validate_warnings.go:268-328 |

---

### 3. Drift Warning (cli/validate.go)
**정의**: dva.yml 설정과 실제 프로젝트 상태의 불일치

**특징**:
- 설정과 실제 파일 시스템 사이의 불일치
- `[warn] config drift:` 접두사로 표시
- 사용자가 의도적으로 무시할 수 있음 (의도된 설정일 수 있음)

**현재 포함 항목**:
| 항목 | 설명 | 파일 |
|------|------|------|
| Compose 파일 불일치 | 설정된 compose.files와 디스크 상 다름 | validate.go:112-123 |
| Interaction service 누락 | interaction.service가 compose에 없음 | validate.go:129-140 |

---

### 4. Suggestion Warning (cli/validate.go)
**정의**: 개발자 경험 향상을 위한 제안

**특징**:
- 기능 영향 없음, 순수한 DX 개선 제안
- `[warn] config suggestion:` 접두사로 표시
- `suggestion_ignore`로 필터링 가능

**현재 포함 항목**:
| 항목 | 설명 | 파일 |
|------|------|------|
| Makefile/package.json 미매핑 | DVA interaction에 없는 script/target 제안 | validate.go:151-210 |

---

### 5. Doctor-Only Checks (cli/doctor.go)
**정의**: 실행 전 환경 프리플라이트 체크

**특징**:
- dva.yml 외부 상태 확인 (Docker, 파일 존재 등)
- `dva doctor` 전용
- 사용자 정의 check (dva.yml `checks` 섹션) 지원
- `--fix` 플래그로 자동 수정 가능 항목 포함

**현재 포함 항목**:
| 항목 | 설명 | 파일 |
|------|------|------|
| Docker daemon 접근 가능 | docker info 실행 가능 확인 | doctor.go:208-222 |
| Compose 파일 존재 | compose.files에 지정된 파일 존재 확인 | doctor.go:70-82 |
| 사용자 정의 checks | dva.yml `checks` 섹션 실행 | doctor.go:84-87 |
| devcontainer.json 존재 | devcontainer 설정 시 파일 확인 | doctor.go:89-97 |
| .gitignore 상태 | .sb/dva/가 제외되었는지 확인 | doctor.go:99-156 |

---

## Doctor vs Validate 경계

| 측면 | Doctor | Validate |
|------|---------|----------|
| **목적** | 환경 프리플라이트 체크 | 설정 유효성 검증 |
| **대상** | dva.yml 외부 상태 | dva.yml 내용 |
| **실행 시점** | `dva doctor` 전용 | `dva config validate`, `dva up` 등 모든 명령 |
| **자동 수정** | `--fix`로 지원 | compose name에만 `--fix` 지원 |
| **사용자 정의** | `checks` 섹션으로 가능 | 불가 (schema만) |
| **출력 형식** | `[pass]/[FAIL]/[fixed]` | `[warn]` or hard error |

**경계 원칙**:
- **dva.yml 내부 구조/값** → Validate (hard error 또는 warning)
- **dva.yml 외부 상태** (파일, Docker, 환경) → Doctor
- **예외**: Compose project name mismatch는 validate에서 `[warn]`으로 출력하지만, doctor에도 추가 가능 (환경 불일치 vs 설정 불일치)

---

## Compose Project Name Mismatch 처리 위치

현재 `ValidateComposeProjectNames()`는 config 패키지에 있으며, `validate` 명령에서 `[warn]`으로 출력됩니다.

**현재 위치**: config/validate.go (ValidateComposeProjectNames)
**출력**: cli/validate.go (printComposeNameWarnings) → `[warn]` 접두사

**재검토 필요사항**:
1. 이것이 **semantic warning** 또는 **drift warning**인지 명확히 구분
2. Doctor에도 추가할지 여부 (preflight 관점)

**제안 배치**: **Semantic Warning** 유지
- 이유: dva.yml 설정 (project_name)와 compose 파일 내용(name:)의 불일치는 "설정 불일치"이며, semantic warning 범주에 포함
- Doctor 추가 고려: 새 doctor 체크로 "compose 파일 project name 일치"를 추가하여 실행 전 확인 강화 가능

---

## 기존 검사 항목 재배치 분류표

| 현재 위치 | 항목 | 제안 배치 | 이유 |
|----------|------|-----------|------|
| validate.go | YAML 구문 오류 | Hard Error (유지) | 실행 불가 |
| validate.go | Schema 위반 | Hard Error (유지) | 구조적 오류 |
| validate.go | Reserved command 충돌 | Hard Error (유지) | 기능 상실 |
| validate.go | Hook on non-hookable | Hard Error (유지) | API 위반 |
| validate.go | Invalid default_mode | Hard Error (유지) | 참조 불가 |
| validate_warnings.go | Version 낙후 | Semantic Warning (유지) | 기능은 작동 |
| validate_warnings.go | Health check 중복 | Semantic Warning (유지) | 우선순위 혼동 가능 |
| validate_warnings.go | 부모/자식 명령 중복 | Semantic Warning (유지) | 의도 불확실 |
| validate_warnings.go | Stack order 중복 | Semantic Warning (유지) | 실행 순서 미정의 |
| validate_warnings.go | 다중 compose entry | Semantic Warning (유지) | 모드 사용 권장 |
| validate_warnings.go | default_mode 누락 | Semantic Warning (유지) | 전체 실행됨 |
| validate_warnings.go | default_mode heavy infra | Semantic Warning (유지) | 성능 영향 |
| validate_warnings.go | 섹션 순서 위반 | Semantic Warning (유지) | 가독성만 |
| validate.go | Compose project name | Semantic Warning (유지, 명시화) | 설정 불일치 |
| validate.go | Compose files drift | Drift Warning (유지, 명시화) | 설정-실제 불일치 |
| validate.go | Interaction service 누락 | Drift Warning (유지, 명시화) | 설정-실제 불일치 |
| validate.go | Makefile/package.json 제안 | Suggestion Warning (유지) | DX 개선 |
| doctor.go | Docker daemon | Doctor (유지) | 환경 상태 |
| doctor.go | Compose 파일 존재 | Doctor (유지) | 환경 상태 |
| doctor.go | 사용자 정의 checks | Doctor (유지) | 환경 상태 |
| doctor.go | devcontainer.json | Doctor (유지) | 환경 상태 |
| doctor.go | .gitignore 상태 | Doctor (유지) | 환경 상태 |

---

## Severity 판단 기준 (새로운 검사 추가 시)

### Hard Error 적용 기준
다음 중 하나라면 **Hard Error**:
- YAML 문법 오류 또는 타입 불일치
- JSON Schema 위반
- DVA 예약어/명령어와의 충돌로 인한 기능 상실
- 존재하지 않는 리소스 참조 (예: modes에 없는 default_mode)
- 잘못된 플러그인/백엔드 참조
- 보안 문제 (예: 비밀키가 플레인텍스트로 로그될 가능성)

**질문**: "이 문제가 해결되지 않으면 dva가 제대로 실행될 수 있는가?" → NO면 Hard Error

---

### Semantic Warning 적용 기준
다음 중 하나라면 **Semantic Warning**:
- 기능은 작동하지만 모범 사례 위배
- 사용자 실수 가능성 (예: 중복된 설정, 우선순위 혼동)
- 성능/리소스 효율성 개선 가능 (예: heavy infra in default mode)
- 유지보수성 저하 가능성 (예: 섹션 순서, 잘못된 패턴)
- 설계 원칙 위배 가능성

**질문**: "이 경고가 없어도 기능은 작동하지만, 해결하면 더 나은가?" → YES면 Semantic Warning

---

### Drift Warning 적용 기준
다음 중 하나라면 **Drift Warning**:
- dva.yml 설정과 실제 파일 시스템/환경의 불일치
- compose.files에 없는 파일이 존재
- interaction에서 참조하는 service가 compose에 없음
- 사용자가 의도적으로 무시할 수 있는 상황

**질문**: "이것은 설정 문제인가, 아니면 설정과 실제 상태의 불일치인가?" → 실제 상태와의 불일치면 Drift Warning

---

### Suggestion Warning 적용 기준
다음 중 하나라면 **Suggestion Warning**:
- 기능 영향 전혀 없음
- 개발자 경험(DX) 개선 제안
- 자동화/중복 감소 제안
- `suggestion_ignore`로 필터링 가능해야 함

**질문**: "이것을 해결하지 않아도 아무 문제가 없는가?" → YES면 Suggestion Warning

---

### Doctor Check 적용 기준
다음 중 하나라면 **Doctor Check**:
- dva.yml 외부 상태 확인 (Docker, 파일 존재)
- 환경 프리플라이트 체크
- 사용자가 dva.yml `checks` 섹션에서 정의 가능한 항목
- `--fix`로 자동 수정 가능한 항목

**질문**: "이것은 dva.yml 내용 검증인가, 아니면 외부 환경 확인인가?" → 외부 환경이면 Doctor Check

---

## 중복 검사 제거/재사용 방향

### 현재 중복 가능성

**Compose project name**:
- 현재: validate에서 semantic warning으로 출력
- 제안: doctor에도 추가하여 실행 전 확인 강화
- 재사용: `ValidateComposeProjectNames()` 함수를 doctor에서도 호출 가능

**Compose 파일 존재**:
- Doctor: compose 파일 존재 확인 (doctor.go:70-82)
- Drift Warning: compose.files와 디스크 불일치 감지 (validate.go:112-123)
- 경계: Doctor는 "파일 존재 여부", Drift는 "설정된 목록과 실제 감지된 파일 목록 불일치"
- 재사용: 동일한 함수 사용 불가 (목적이 다름)

---

### 제안 재사용 구조

```go
// config/validate.go - 공통 함수
func (c *Config) ValidateComposeProjectNames() []ComposeNameWarning

// cli/validate.go - semantic warning으로 출력
printComposeNameWarnings(warnings)

// cli/doctor.go - doctor check로 추가 (신규)
func checkComposeProjectNames(c *config.Config) DoctorResult {
    warnings := c.ValidateComposeProjectNames()
    if len(warnings) == 0 {
        return DoctorResult{Name: "Compose project name alignment", Passed: true}
    }
    // 첫 번째 경고만 표시 (doctor는 단일 결과)
    w := warnings[0]
    msg := fmt.Sprintf("Compose file %s has %s", w.File,
        map[bool]string{true: "missing project name", false: fmt.Sprintf("name '%s'", w.ComposeName)}[w.ComposeName == ""])
    return DoctorResult{
        Name:    msg,
        Passed:   false,
        FixHint:  fmt.Sprintf("Set 'name: %s' in %s", w.DvaName, w.File),
        Fixable:  true,
        fixFunc: func() error { return c.FixComposeProjectName(w) },
    }
}
```

---

## 후속 구현 순서

### 단계 1: 문서화 완료 (본 태스크)
- [x] Severity 기준 문서화
- [x] 기존 항목 분류표 작성
- [x] Doctor vs Validate 경계 명시
- [x] Compose project name 처리 위치 결정

### 단계 2: 코드 정리 (신규 태스크)
- [ ] 각 warning 함수에 주석으로 severity 명시
- [ ] 새로운 항목 추가 가이드라인 주석 추가
- [ ] 코드에서 Drift Warning과 Semantic Warning 구분 강화 (접두사 일치 확인)

### 단계 3: Doctor 확장 (연관 태스크: doctor-preflight-expansion.md)
- [ ] Doctor에 compose project name 체크 추가
- [ ] Doctor에 env/stack/mode 관련 preflight 체크 후보 정리
- [ ] Doctor 체크 우선순위 정의

### 단계 4: 테스트 보강
- [ ] 각 severity 유형별 테스트 커버리지 확인
- [ ] 경계 케이스 (예: hard error vs warning 경계) 테스트 추가
- [ ] doctor와 validate 중복 로직 공유 테스트

### 단계 5: 문서 반영
- [ ] 사용자 문서 (README, docs/)에 severity 계층 설명 추가
- [ ] 사용자 가이드: "dva doctor vs dva config validate" 차이점
- [ ] 새 검사 추가 시 어떤 계층에 넣어야 하는지 가이드 추가

---

## 테스트와 문서 반영 대상

### 테스트 보강 필요 항목
| 테스트 파일 | 현재 상태 | 필요 작업 |
|------------|-----------|---------|
| validate_warnings_test.go | 각 warning 함수별 테스트 있음 | 새 severity 기준에 맞게 테스트 설명 주석 추가 |
| validate_test.go | compose name warning 테스트 있음 | Drift Warning/Semantic Warning 구분 테스트 추가 |
| doctor_test.go | 개별 doctor check 테스트 필요 | doctor vs validate 중복 로직 테스트 추가 |

### 문서 반영 대상
| 문서 | 내용 |
|------|------|
| README.md | `dva doctor`, `dva config validate` 명령 차이점 설명 |
| docs/validation.md (신규) | Severity 기준, 각 계층 설명, 예시 |
| CONTRIBUTING.md (또는 DEVELOPMENT.md) | 새 검사 추가 시 어떤 계층에 넣어야 하는지 가이드 |
| CHANGELOG.md | (향후) severity 정책 도입 관련 변경 사항 |

---

## 결론

이 문서는 DVA의 validation severity 정책을 명시합니다. 새로운 검사를 추가할 때 다음 질문을 참고하여 적절한 계층에 배치하세요:

1. **Hard Error?** 실행 불가능하거나 기능 상실인가?
2. **Semantic Warning?** 기능은 작동하지만 개선/모범 사례 위배인가?
3. **Drift Warning?** 설정과 실제 상태가 불일치인가?
4. **Suggestion Warning?** DX 개선 제안인가?
5. **Doctor Check?** dva.yml 외부 환경 확인인가?

이 기준을 따르면 validation 경계가 명확해지고, 새로운 검사를 추가할 때 일관성을 유지할 수 있습니다.
