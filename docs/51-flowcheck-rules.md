# flowcheck 규칙 — 조용히 틀리거나 늦게 막히는 16가지 결정 경로

`tools/flowcheck`는 `agent-mesh-flows/`의 모든 플로우를 읽고 16개 규칙 중 하나라도 위반하면
빌드를 세운다. 대부분은 **am이 exit 0으로 끝내면서 틀린 답을 내는** 경우다. 셸 허용목록
규칙은 batch run을 실패시키고, 대화형 실행에서는 계속할지 물은 뒤 기본값으로 진행한다.

```bash
go run ./tools/flowcheck
```

## 검사 대상과 비목표

- **본다**: 런타임이 `/bin/sh`에 넘기는 필드, `when:` 피연산자, `parameters:` 타입
  (`param-type`), `exit_if_empty:` 구조(`exit-if-empty`).
- **보지 않는다**: 위 두 구조 검사를 제외한 YAML 스키마(그건 `am validate`), 프롬프트 본문
  (dva 설명 산문이라 노이즈만 나온다).

규칙 id의 정본은 소스의 문자열 리터럴이다. 아래 목록이 소스와 어긋나면 소스가 맞다:

```bash
grep -rhoE '(s\.add\(|rule :?= |rule: *)"[a-z-]+"' tools/flowcheck/*.go | sed 's/.*"\(.*\)"/\1/' | sort -u
```

## 게이트 — `when:` 계약

am의 `when:`은 `{{ref}} OP '따옴표'` 하나만 평가한다. 이 형태를 벗어나면 게이트가 열린 채로
굳거나 닫힌 채로 굳고, 어느 쪽이든 실행은 성공한다.

| 규칙 | am이 조용한 이유 | 잘못 → 올바름 |
|---|---|---|
| `gate-operand` | 렌더된 문자열과 YAML 불리언을 비교하므로 영원히 같지 않다. `!=`는 항상 실행, `==`는 항상 건너뜀 | `{{c.ok}} == true` → `{{c.ok}} == 'true'` |
| `gate-filter` | 참조 안의 필터가 비교 자체를 무력화한다. 값과 연산자에 무관하게 스텝이 돈다 | `{{c.ok \| trim}} == 'true'` → `{{c.ok}} == 'true'` |
| `gate-producer-newline` | `echo`는 개행을 붙이고 am은 트림하지 않는다. `"true\n"`은 `'true'`와 같지 않다 | `echo true` → `printf true` |
| `gate-skip-leak` | 건너뛴 스텝의 키는 빈 값이 아니라 리터럴 `{{step.key}}`로 렌더된다. 읽는 쪽은 그걸 쥐고 실행된다 | `depends_on` 없이 `{{G.k}}` 읽기 → 소비자에 `depends_on: [G]` **와** 자신의 `when:` |
| `gate-skip-prompt` | 같은 리터럴이 `instruction`·`prompt`·`file.path`·`file.content`·`file.from`·`file.to`·`src`로 가면 모델이 그럴듯한 답을 만들거나 디스크에 남긴다 | `instruction: "... {{G.k}}"` (G는 게이트됨) → 위와 같은 보호. 리터럴이 런보다 오래 살아서 id를 나눴다 |

안전 조건은 "소비자에게 `when:`이 있으면 된다"가 **아니다**. 측정된 규칙은 이렇다:
`{{G.key}}`를 읽는 소비자는 `G → … → 소비자` `depends_on` 경로가 있고 그 경로의 G 이후 모든
스텝이 게이트를 가질 때에만 안전하다. `depends_on` 없이 키만 읽는 스텝은 자기 게이트가 참이어도
리터럴을 손에 쥔 채 실행된다.

## 셸 텍스트 — am의 명령 허용목록

am은 셸 필드를 정적 분석해 명령 이름을 뽑고 허용목록에 없으면 스텝을 막는다. `am validate`는 이
단계를 보지 않으므로 **플로우는 valid인데 런에서 막힌다**.

| 규칙 | am이 조용한 이유 | 잘못 → 올바름 |
|---|---|---|
| `bare-word-arg` | `printf`·`test`·`[`·`[[`의 따옴표 없는 인자를 명령 이름으로 읽는다 (`command "dva.yml" not in allowlist`) | `[ -f dva.yml ]` → `[ -f 'dva.yml' ]` |
| `local-function` | 필드가 정의한 함수도 호출 지점에서 막힌다. 허용목록은 명령을 알지 함수를 모른다 | `f() { ...; }` … `f` → 함수를 지우고 본문을 호출 자리에 편다 |
| `heredoc-delimiter` | 따옴표 없는 구분자는 본문을 셸로 읽어 제목 줄 첫 단어에서 막는다 | `<<EOF` → `<<'EOF'` |

`echo hello`, `ls hello`, `grep hello file`, `cp a b`는 막히지 않는다. 위 네 명령은 이 규칙이
검사하는 오판 트리거다. `eval`·`exec`도 막히지만 일부러 뺐다 — 거기서는 첫 인자가 실제로 명령
이름이라 허용목록이 제 일을 하는 것이다.

`true`/`false`가 통과하는 것은 **허용목록에 있는 명령이기 때문**이지 이 자리가 안전해서가 아니다.
게이트 생산자 형태인 `printf true || printf false`가 합법인 것도 그 우연 덕이고, 다음 사람이
`printf yes`라고 쓰면 막힌다.

## 주석 — `/bin/sh`처럼 무시되지 않는다

| 규칙 | am이 조용한 이유 | 잘못 → 올바름 |
|---|---|---|
| `comment-substitution` | 주석의 평범한 단어는 버리지만 백틱과 `$(...)` 스팬은 그대로 뽑아 첫 명령에서 막는다 | 주석에서 백틱과 `$( )`를 지운다 |
| `comment-quote` | 인용 상태가 `#`와 줄을 넘어 이어진다. 주석의 홑따옴표 하나가 뒤쪽 패리티를 뒤집어 세 줄 아래 `awk '...'`가 `command "BEGIN" not in allowlist`로 막힌다 | 주석에 `'`와 `"`를 쓰지 않는다 (`don't` → `does not`) |

스팬이 실제로 뽑히는지는 필드 전체의 따옴표 패리티에 달려 있다. 그래서 규칙은 오늘 막히는 것만이
아니라 **모든 스팬**을 보고한다. 주석에서 단어 하나를 지우면 세 줄 아래가 무장된다.

## 판단 경로 — 결과를 읽는 쪽

| 규칙 | am이 조용한 이유 | 잘못 → 올바름 |
|---|---|---|
| `dead-gate` | jq `//`는 `null`뿐 아니라 `false`도 대체한다. 명시적 `false`가 `true`로 읽혀 정지 분기가 도달 불가가 된다 | `.dva_needed // true` → `has("dva_needed")`로 부재와 거짓을 분리 |
| `unguarded-report` | `tmp/` 경로를 읽는 `jq` 필드가 `jq -e -s` guard를 쓰지 않으면 발동한다. `jq -e .`는 **스트림**을 받아 `[1][2]{...}`도 exit 0이고 뒤쪽 객체에서 그럴듯한 값이 나온다 | `jq -e -s`로 객체가 정확히 하나인지 확인한다 |
| `phantom-command` | 없는 `dva` 하위 명령의 오류 텍스트가 리포트에 **발견 사항처럼** 렌더된다 | `dva app ls` → `dva ls` (`ReservedCommands()` 중 하나) |
| `exit-if-empty` | `exit_if_empty`는 파이프라인을 **성공으로** 끝낸다. 전제 조건 누락과 "할 일 없음"이 구분되지 않는다 | `exit_if_empty: true` → `[ -n "$x" ] \|\| exit 1` |

`phantom-command`가 쓰는 예약 명령 목록은 `internal/config`를 컴파일 시점에 import해서 얻는다
(`tools/flowcheck/main.go`의 `config.ReservedCommands()`). 손으로 관리하는 목록이 아니므로 명령이
추가되면 다음 `go run ./tools/flowcheck` 실행부터 곧바로 반영된다 — 생성 단계(`make generate`)도,
생성된 산출물도 거치지 않는다.

## 스키마

| 규칙 | am이 조용한 이유 | 잘못 → 올바름 |
|---|---|---|
| `param-type` | `flow.schema.json`은 파라미터 기본값과 enum 값을 문자열로 요구한다. 불리언·숫자로 쓰면 스키마 타입 실패 | `default: true` → `default: 'true'` |

## 코퍼스 전역 — 규칙 하나

`config-probe-drift`만 필드 하나가 아니라 **코퍼스 전체**를 읽는다. 네 플로우가 "이 타깃에 이미
설정이 있는가"를 각자 계산하는데, 그럴 수밖에 없다. 서로 다른 플로우이고 guided 스테이지는 각각
단독 실행되므로, 파이프라인 파라미터로 내려준 플래그는 단독 실행에서 정확히 없다. 없는 참조는
빈 값이 아니라 리터럴로 렌더되므로 게이트가 닫히고 백업이 조용히 건너뛰어진다.

네 벌인 것은 정직한 모양이고, 네 벌이 갈라지는 것은 아니다. 그래서 각 복사본은 정렬된 경로 순서상
**첫 번째와 같은 철자**를 요구받는다. 갈라지는 방식이 곧 위의 결함들이 한 번에 한 복사본씩 도착하는
것이기 때문이다. `printf`가 `echo`로 바뀌면 그 플로우의 게이트가 죽고, `dva.yml`만 검사하도록
좁혀지면 다른 철자를 쓰는 모든 프로젝트에서 조용히 동작을 멈춘다.

잘못 → 올바름: 한 벌만 `printf true`에서 `echo true`로 바뀌거나 `dva.yml`만 검사하도록
좁혀지는 것 → 네 벌이 같은 철자를 유지한다.

다수결이 아니라 첫 번째와 비교하는 것은 의도적이다. 다수결은 복사본이 셋이 되는 순간 드리프트를
승인한다. 대신 기준 복사본을 고치면 나머지 셋이 보고된다 — 편집 하나에 발견 셋, 각각 비교 대상의
위치를 가리킨다.

## 판정 옆의 개수

flowcheck는 판정과 함께 스캔한 개수를 찍는다:

```
flowcheck: 10 flow file(s), 103 shell field(s), 14 when-gate(s), 40 dva invocation(s), 3 report-reading field(s), 3 skippable reference(s), 4 config-presence probe(s), 23 built-in command(s)
flowcheck: OK — no decision-path defects
```

개수 없는 "발견 없음"은 **검사 대상을 잃은 규칙**과 **통과한 규칙**을 구별하지 못한다. 어떤 수가
0이 되면 그 규칙은 더 이상 아무것도 보고 있지 않은 것이다.
