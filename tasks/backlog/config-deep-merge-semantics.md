# Config Deep Merge Semantics

## 배경

현재 `dva.yml` 로딩은 base config, module, override를 순서대로 읽고 `mergeFrom()`으로 합칩니다.
하지만 병합 규칙이 대부분 "맵 키 단위 통째 교체"에 가깝기 때문에, 일부 필드만 덮고 싶은 경우에도 엔트리 전체를 다시 써야 합니다.

이 제약은 `stack`, `interaction`, `applications`, `modes`, `environments`가 커질수록 설정 중복과 override 파편화를 유발합니다.

## 현재 상태

- 로딩 진입점: `internal/config/config.go`의 `Load()`
- 병합 구현: `internal/config/config.go`의 `mergeFrom()`
- 관련 보조 구조:
  - `internal/config/lifecycle.go`
  - `internal/config/config.go`
  - `internal/config/subproject.go`

현재 `mergeFrom()`은 top-level map을 합치더라도 개별 엔트리 내부 필드까지는 deep merge 하지 않습니다.
예를 들어 `stack`이나 `interaction`에서 같은 이름의 키가 있으면 override 쪽 값이 사실상 전체 엔트리를 대체합니다.

## 문제

- 부분 override가 어렵습니다.
- module과 override를 많이 쓸수록 설정 중복이 커집니다.
- 향후 `stack_overrides` 같은 기능을 도입하려면 공통 merge semantics가 먼저 정리되어야 합니다.
- 현재 병합 규칙이 문서화되어 있지 않아 사용자와 구현 모두 예측 가능성이 낮습니다.

## 결정해야 할 것

- 어떤 top-level 섹션을 deep merge 대상으로 볼지
- 어떤 필드는 scalar override, append, replace, merge 중 무엇으로 처리할지
- `stack`와 `interaction` 내부 구조에서 nil/empty의 의미를 어떻게 정의할지
- backward compatibility를 유지할지, 일부 케이스에서만 새 semantics를 허용할지

## 범위

- `Load()` 시점의 config merge semantics 정의
- modules / override / subproject override에 공통 적용 가능한 merge 규칙 도출
- 문서와 테스트에서 그 semantics를 명시

## 제외

- 특정 plugin 기능 추가
- env interpolation 확장 자체
- orchestrator 실행 순서 변경

## 참조 정보

- `internal/config/config.go`
- `internal/config/subproject.go`
- `internal/config/lifecycle.go`
- `internal/config/config_test.go`
- `internal/config/examples_test.go`
- 기존 backlog: `tasks/backlog/env-stack-overrides.md`

## 완료 조건

- merge semantics가 문서로 명확히 정의되어 있다.
- top-level 섹션별 merge 정책이 구분되어 있다.
- `stack`, `interaction`, `applications`, `modes`, `environments`에 대해 "부분 override 가능 여부"와 규칙이 명시되어 있다.
- 현재 동작을 유지할 항목과 변경할 항목이 분리되어 있다.
- 테스트해야 할 대표 케이스가 체크리스트로 정리되어 있다.

## 체크리스트

- [ ] base/module/override merge 우선순위가 명시되어 있다.
- [ ] nil과 empty collection의 의미가 정리되어 있다.
- [ ] backward compatibility 위험이 적혀 있다.
- [ ] 필요한 테스트 케이스 목록이 적혀 있다.
- [ ] 문서 반영 대상 파일이 정리되어 있다.
