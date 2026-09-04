# 56. validate 억제 수단 설계 (결정 대기)

> 상태: **결정 대기** (TASK-309, needs-human). 선택지와 권고만 담고, 결정 전에는 구현하지 않는다.
> 경고 종류는 [docs/51](51-flowcheck-rules.md), 시맨틱 경고 목록은 USAGE.md "config validate" 절.

## 1. 현재 상태

`dva validate`가 내는 비(非)스키마 출력은 세 갈래이고 억제 수단은 하나만 있다.

| 출력 | 생산자 | 억제 수단 |
|---|---|---|
| suggestion: Makefile/package.json 타겟을 interaction으로 감싸라는 제안 | `internal/cli/validate.go` (`matchesSuggestionIgnore`, :642) | `suggestion_ignore: [glob...]` — 타겟 이름 glob |
| drift: (a) 루트 compose 자동탐지 파일이 `compose.files`에 없음, (b) 선언된 compose 파일이 디스크에 없음, (c) interaction `service`가 compose에 없음 | `detectConfigDriftWarnings` (:385) | 없음 |
| 시맨틱 경고 28종 | `config.ValidateWarnings()` | 없음 (설계상 의도: 설정을 고치는 것이 답) |

dogfood 관찰(2026-09-05, 12개 프로젝트):

- `suggestion_ignore`가 20줄을 넘는 프로젝트가 3개. 대부분 `docker-*`, `k8s-*`, `helm-*`처럼
  이미 dva가 대신하는 타겟과, `clean`·`fmt`처럼 개발자가 감쌀 의도가 없는 타겟이다.
- 제안 자체의 결함이 있다. 이미 interaction이 `command: make test`로 감싼 타겟을 다시 제안하고,
  `.PHONY` 보조 타겟과 pattern rule을 타겟으로 센다. 제안 소스 결함은 **TASK-320**이 따로 다룬다.
- drift (a)는 의도적으로 compose 파일을 여러 개 두고 하나만 dva에 등록한 프로젝트
  (overlay 실험 파일, `compose.ci.yaml`)에서 매 실행마다 나온다. 우회 수단이 없어 사용자가
  경고 문장을 외우고 무시한다 — 나머지 경고까지 읽지 않게 되는 것이 진짜 비용이다.
  감지 자체의 구멍(`include:` 미추적, `compose-*.yaml` 미감지, 서브디렉터리)은 **TASK-316**
  범위다. 316이 감지 폭을 넓히면 (a) 경고는 늘어나므로 억제 수단의 필요도 함께 커진다.

## 2. 원칙

SOUL 신념 2(예측 가능성)와 5(기계가 읽는 표면)에서 두 제약이 나온다.

1. **억제는 침묵이 아니라 선언이다.** 억제된 항목의 *건수*는 항상 요약에 남긴다
   (`✅ dva.yml is valid (2 suggestions, 1 drift file ignored by dva.yml)`). 어떤 파일이
   무엇을 가리는지 `dva validate --show-ignored`로 펼칠 수 있어야 한다.
2. **실행을 깨는 사실은 억제 대상이 아니다.** drift (b) 선언 파일 부재와 (c) 없는 서비스 참조는
   `dva up`/`dva run`이 실패하는 상태이므로 어떤 ignore도 적용하지 않는다. 시맨틱 경고도 같은
   이유로 억제 수단을 두지 않는다 — 경고가 틀렸다면 규칙을 고친다.

## 3. suggestion 억제 — 선택지

### A. glob 축약 유지 + 보조 명령

현 `suggestion_ignore`를 그대로 두고 `dva validate --suggest-ignore`가 현재 제안 전부를
glob 목록으로 출력해 붙여 넣게 한다. 비용은 작지만 "전부 무시"를 한 번에 만드는 손잡이라
원칙 1의 건수 표시가 없으면 침묵 남용으로 직결된다.

### B. 카테고리 opt-out

```yaml
suggestions:
  makefile: false          # Makefile 타겟 제안 전체 끔
  package_json: true
```

한 줄로 끝나 목록이 자라지 않는다. 대신 "이 프로젝트에서는 Makefile 제안이 쓸모 없다"는
결정을 명시하게 되므로 부분 억제(glob)보다 의도가 읽힌다. 단, 감싸면 좋을 타겟이 새로
생겨도 영원히 보이지 않는다.

### C. 소스 개선

제안 규칙 자체를 좁힌다. (1) 어떤 interaction이든 `command`/`steps`에 `make <target>` 또는
`pnpm <script>`를 포함하면 그 타겟은 제안하지 않는다. (2) dva가 대체하는 타겟군
(`docker-*`, `compose-*`, `k8s-*`, `helm-*`, `up`, `down`, `logs`, `ps`)은 기본 제외한다.
(3) `.PHONY` 전용·pattern rule·`_` 접두 타겟은 세지 않는다. dogfood 프로젝트의 ignore 목록은
대부분 (1)·(2)에 해당하는 이름이라 C만으로 크게 줄 것으로 보이나, 정확한 수치는 결정 후
TASK-320 구현 시 전후 validate 출력으로 확인한다.

**권고: C를 먼저 하고, 남는 목록에 대해 B를 추가한다. A는 하지 않는다.**
C 이후에도 목록이 남는 프로젝트는 "제안 자체가 필요 없다"는 쪽이므로 카테고리 opt-out이
맞고, glob 축약은 그 결정을 미루는 도구다. 기존 `suggestion_ignore`는 호환을 위해 유지한다.

## 4. drift 억제 — 선택지

drift (a)만 대상이다(§2 원칙 2).

### D. `compose.files`에 등록하되 plan에서 안 쓰기

기능 추가 없이 오늘 가능하다. 그러나 `files:`는 compose에 넘겨지는 실행 입력이라 실험용
파일을 등록하면 `dva up`의 동작이 바뀐다. 억제 목적으로 실행 표면을 건드리는 것이라
권하지 않는다.

### E. `drift_ignore` glob 목록

```yaml
drift_ignore:
  - "compose.ci.yaml"
  - "compose.*.experimental.yaml"
```

- 적용 범위: 루트 자동탐지 규칙 (a)에서 탐지된 파일 이름과만 대조한다. (b)·(c)에는
  절대 적용하지 않고, 스키마 설명에 그 사실을 적는다. TASK-316이 서브디렉터리 감지를 넣으면
  대조 대상은 루트 기준 상대 경로가 된다.
- glob은 `suggestion_ignore`와 같은 `path.Match` 의미론을 쓴다. 디렉터리 구분자는 넘지 않는다.
- 요약에 `N drift file(s) ignored` 표시(원칙 1). `--show-ignored`로 파일명 표시.
- 매치되지 않는 패턴(가리는 대상이 사라짐)은 `drift_ignore[i]: matches no file` 경고를 낸다.
  stale ignore가 쌓이는 것을 막는다 — `suggestion_ignore`에도 같은 규칙을 추가한다.

### F. 자동탐지를 끄는 스위치 (`compose_autodiscover: false`)

(a) 자체를 끈다. 가장 간단하지만 새 compose 파일이 생겨도 알리지 않아 drift 검사의 목적을
잃는다. E가 있으면 필요 없다.

**권고: E. D는 문서에 "하지 말 것"으로, F는 채택하지 않는다.**

## 5. 결정이 필요한 항목

1. suggestion: C→B 순서로 가는가, A(glob 보조 명령)도 두는가. (권고: C→B, A 없음)
2. B의 카테고리 단위: 소스 종류(makefile/package_json)만인가, 타겟군(`docker-*`)도 카테고리로
   두는가. (권고: 소스 종류만. 타겟군은 C의 기본 제외로 흡수)
3. drift: E를 채택하는가. 채택 시 키 이름 `drift_ignore` vs `compose_ignore`. (권고: E,
   `drift_ignore` — 규칙 (a) 외로 확장할 여지를 이름에 남김)
4. 억제 건수 요약 표시를 끌 수 있게 하는가. (권고: 불가. 원칙 1의 핵심)
5. stale ignore 경고를 `suggestion_ignore`에도 소급 적용하는가. (권고: 적용. dogfood 목록의
   일부는 이미 사라진 타겟을 가리킨다)

결정 후 작업: C 규칙 3종 + 테스트(TASK-320과 합침), `suggestions:` 스키마·B 로직, `drift_ignore`
스키마·(a) 필터·stale 경고·요약 표시·`--show-ignored`, USAGE.md "config validate" 절과
스키마 설명, dogfood 3개 프로젝트의 ignore 목록을 새 수단으로 옮긴 전후 validate 출력.
