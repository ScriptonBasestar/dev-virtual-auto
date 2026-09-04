# 55. plans alias / extends 설계 (결정 대기)

> 상태: **승인 대기** (TASK-307, needs-human). 이 문서가 승인되기 전에는 구현하지 않는다.
> 용어는 [docs/40](40-declarative-stack-and-plans.md), plan 해석은 [docs/31](31-execution-plan-resolution.md).

## 1. 문제

plan 선언이 통째로 복제되는 사례가 dogfood 5개 프로젝트(cwrapper, dripter, scripton-nd-stack,
gizzahub, matdosa)에서 반복된다. 형태는 둘이다.

- **완전 중복 쌍**: `infra` ≡ `local-infra`, `hybrid` ≡ `local-dev`. 옛 이름을 지우지 못해
  같은 본문을 두 번 둔다. validate는 `warnDuplicatePlanDeclarations`로 이미 경고한다.
- **서비스 목록 부분 복제**: nd-stack은 `[postgres, redis, minio, ...]`를 7개 plan에 걸쳐 4중으로
  적는다. plan마다 "기본 인프라 + 몇 개"인데 기본 인프라를 매번 다시 쓴다.

관련 관찰(2026-09-05): overlay 엔트리를 plan에서 조합하면 TASK-288 경고로 거부되어 overlay마다
base 서비스를 재선언하고(primeno1), `composes:` plan은 자체 `entries`를 못 가져 "인프라 + 추가
서비스"에 쓸 수 없고(dns-bridge, 8개 plan 반복), plan 경로가 `--env`를 거부해 "같은 plan, 다른
env"를 표현할 수단이 없다(matdosa). 세 가지는 모두 "차이만 적고 싶다"는 같은 요구다.

## 2. SOUL과의 대조

| SOUL 기준 | alias | extends |
|---|---|---|
| 1. 선언과 실행 계획 분리 — "환경 차이는 선언을 복제하지 않고 계획 해석 시 적용" | 중립 | 부합. 차이만 적는다 |
| 2. 예측 가능성 — "실행 전에 최종 계획을 해석·검증" | 부합. 이름 치환만 | **긴장**. 병합 규칙을 알아야 최종 계획을 읽을 수 있다 |
| 3. 하나의 소유자 | 부합. 본문은 한 곳 | 부합하나 부모가 바뀌면 자식이 조용히 바뀐다 |
| "설정 복제 vs 합성 → 합성" | 이름 복제만 제거 | 본문 복제 제거 |
| "단축 명령 vs 명시적 계획 → 명시적" | 별칭은 단축 명령에 가깝다 | — |

SOUL은 합성을 택하되 예측 가능성을 위에 둔다. 따라서 어떤 안이든 **`dva show <plan>`이 병합
결과를 평탄하게 보여 주고, validate가 순환·미정의 참조를 hard error로 막는 것**이 전제다.

## 3. 선택지

### A. alias — 이름만 추가

```yaml
plans:
  local-dev: { entries: [...] }
  hybrid: { alias: local-dev }        # 다른 필드와 공존 불가
```

- 규칙: `alias`는 단독 필드. 대상은 alias가 아닌 plan이어야 한다(체인 금지). 미정의·자기
  참조·체인은 hard error. `dva ls`는 `hybrid → local-dev`로 표시하고 `default_plan`은 어느
  이름이든 허용한다. `warnDuplicatePlanDeclarations`가 alias를 제안 문구에 쓴다.
- 해결: 완전 중복 쌍만. 서비스 목록 복제(nd-stack)는 그대로다.
- 비용: 작다. 로더에서 치환 한 번, 스키마 한 필드.

### B. extends — 단일 부모 상속

```yaml
plans:
  local-infra: { entries: [{ name: compose, services: [postgres, redis] }] }
  infra-full:
    extends: local-infra
    entries: [{ name: compose, services: [postgres, redis, minio, grafana] }]
```

- 병합 규칙 후보: plan 스칼라 필드(description, environment, site, endpoint_tags)는 자식 우선.
  `vars`는 키 병합. `entries`는 **`name`으로 매칭**해 같은 이름은 자식이 통째로 교체(services
  합집합 아님 — 합집합은 "빼기"를 표현할 수 없다), 새 이름은 추가. 단일 부모, 깊이 제한(예: 3),
  순환·미정의 hard error. `composes:` plan은 extends 불가.
- 해결: 완전 중복 쌍(본문 없는 extends)과 부분 복제 모두.
- 비용: 병합 규칙 문서, `dva show`의 해석 결과 표시, 스키마, 테스트. 그리고 사용자가 규칙을
  기억해야 한다. nd-stack의 경우 services를 자식이 통째로 다시 쓰므로 실제 절약은 크지 않다.

### C. 기능 추가 없이 — YAML anchor와 서비스 그룹

yaml.v3는 anchor/alias와 merge key(`<<:`)를 지원하므로 오늘도 다음이 동작한다.

```yaml
plans:
  local-infra:
    entries:
      - &infra { name: compose, runner: compose, order: 10, services: [postgres, redis] }
  infra-full:
    entries:
      - { <<: *infra, services: [postgres, redis, minio, grafana] }
```

- 해결: 부분 복제. 완전 중복 쌍은 `plans.hybrid: *local-dev`로 쓸 수 있으나 로더가 앵커를 풀어
  버려 `dva ls`에는 관계가 보이지 않고, `warnDuplicatePlanDeclarations`가 두 plan이 같다고
  경고한다(2026-09-05, v0.1.48로 확인). 즉 별칭 의도는 anchor로 표현할 수 없다.
- 비용: 0. 단, `dva config migrate`/`init`의 line-edit 계열은 anchor를 보존하지 않으므로
  문서로 그 한계를 명시해야 한다.

## 4. 권고

**A(alias)를 구현하고 C를 문서화한다. B는 보류한다.**

- 완전 중복 쌍은 관찰된 5개 프로젝트 모두에 있고, alias는 `dva ls`에 관계가 드러나
  SOUL 2·5(기계가 읽는 표면)에 맞는다.
- 부분 복제는 anchor로 오늘 해결되며, 그 한계(line-edit 도구가 anchor를 못 보존)가 실제로
  불편한지 확인한 뒤 B를 다시 판단한다. B를 먼저 넣으면 병합 규칙이 공개 표면이 되어 되돌리기
  어렵다.
- §1의 관련 관찰 세 건은 plan 상속이 아니라 **엔트리 재사용**(overlay `files:` 병합, TASK-288
  remedy)과 **plan 매개변수**(`--env` 대체)의 문제다. 이 문서 범위 밖으로 두고 별도 카드로
  다룬다.

## 5. 결정이 필요한 항목

1. A만 구현하는가, B까지 가는가. (권고: A)
2. alias의 `default_plan` 허용 여부. (권고: 허용, `dva show`는 대상 이름을 함께 표기)
3. `dva ls` 표기 형식: `hybrid → local-dev` vs 별도 "Aliases" 절.
4. C(anchor) 한계를 USAGE.md 어느 절에 두는가. (권고: "plans" 절 끝에 한 단락)

승인 후 작업: 스키마 `plans.<name>.alias`, 로더 치환, hard error 3종(미정의·자기·체인) 테스트,
`dva ls`/`show` 표기, USAGE.md, nd-stack 설정을 alias+anchor로 재작성한 예시의 validate 출력.
