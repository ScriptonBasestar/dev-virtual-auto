---
id: TASK-247
title: "Freeze required env-file behavior for every command path"
type: chore
priority: P0
effort: M
exec-tier: strong
created-at: 2026-09-01T19:23:00+09:00
source: "PLAN-002 required-env decision gate"
scope: "all loadEnv callers, observation versus execution semantics, text/JSON/exit contract, doctor strict behavior"
status: done
needs-human: true
decision-status: decided
completed-at: 2026-09-02T16:23:04+09:00
completion-summary: "Froze an owner-aware atomic env-input report and route-specific execution, observation, validation and doctor policy, with exact text/JSON/exit fixtures and no partial environment exposure."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "exact non-test loadEnv caller census"
    result: "passed with 18 call expressions and expanded direct LoadEnvFile/config-field inventory"
  - kind: automated
    command-or-step: "ce task validate --all && ce task lint"
    result: "passed for both plans and all active/archive cards"
  - kind: automated
    command-or-step: "make doc-check && git diff --check"
    result: "passed documentation links, limits, CI labels, flow decisions and whitespace checks"
  - kind: human
    command-or-step: "2026-09-02 user approval"
    result: "approved the recorded recommended direction"
quality-review: pass
quality-reviewed-at: 2026-09-02T16:23:04+09:00
quality-review-evidence:
  - "independent strong-tier review checked precedence, direct env consumers, doctor aggregation, status/logs channels, owner isolation and the cross-plan DAG"
  - "blockers for precedence, validation I/O and doctor shape plus logs-target ambiguity were corrected; focused re-review passed with no remaining finding"
archived-at: 2026-09-02T16:23:04+09:00
verified-at: 2026-09-02T16:23:04+09:00
verification-summary: "The approved required-env contract is decision-complete and TASK-264/TASK-265 now gate implementation in TASK-248."
---

# Task 247: freeze required env behavior by command

## Summary

Classify every environment-loading command path and freeze its diagnostics, JSON, exit, and child-start
behavior before changing the shared loader.

## Decision required

`loadEnv` currently warns and continues for every error. A single hard-fail would kill doctor and
best-effort observation, but treating every caller as advisory can launch work with missing required
inputs. Optional files still error when present but unreadable or malformed, and a later file can fail
after earlier files have already merged into the environment. The proposal's lifecycle-versus-query
split does not decide `status`, `logs`, teardown, partial merge, or the duplicate JSON status path.

## Recommended direction

외부 process를 시작하거나 resource identity를 해석하는 실행·mutation 경로는 required/inaccessible/
malformed 입력에서 첫 child 전에 fail closed하는 것을 권장한다. 여기에는 teardown도 포함한다. 잘못된
환경으로 다른 resource를 정리하는 것보다 명시적으로 실패하고 진단을 제공하는 편이 안전하다.

`status`·`logs` 같은 관측 경로는 config-only 정보를 계속 보여줄 수 있지만 결과를 `partial`로 명시하고,
필요한 env가 없으면 backend child를 시작하지 않는다. Doctor는 모든 check를 끝까지 수행하고 default는
advisory, `--strict`은 non-zero로 유지한다. Partial merge는 어떤 실행 경로에도 노출하지 않고 버리며,
관측 경로에서도 변수값 자체가 아니라 실패 metadata만 보여준다.

## Decision Record

2026-09-02 사용자가 위 권장 방향으로 진행할 것을 승인했다. 이 결정은 env-file 실패를
전역 helper에서 무조건 종료시키지 않고, 먼저 effective owner와 모든 입력 상태를 계산한 뒤
route별로 실행 실패·부분 관측·진단 계속을 선택한다.

### 1. 확인된 현재 동작

- Non-test `loadEnv` 호출 표현식은 18개다(`compose.go` 8, `status.go` 3, `doctor.go` 2,
  `run.go`·`provision.go`·`kubectl.go`·`ssh.go`·`hooks.go` 각 1). 중복 cache 호출과
  hook wrapper를 사용자 route 하나로 합쳐 아래 표에서 한 번씩 판정한다.
- Audit 범위는 이 함수에 한정하지 않는다. `newConfigEnvironment*`, `newOwnedConfigEnvironment`,
  runtime과 validate의 모든 직접 `LoadEnvFile`, top-level `Config.EnvFile`, interaction/subcommand의
  `InteractionCommand.EnvFile` producer·merge·consumer를 포함한다.
- `LoadEnvFile`은 선언 순서대로 즉시 merge하고 첫 오류에서 중단한다. 따라서 뒤 파일 실패 전
  값이 남고 interpolation은 실행되지 않은 상태로 caller가 계속할 수 있다.
- Optional missing만 정상 skip이다. Required missing과 open/read/scanner 오류는 반환되지만 CLI가
  warning으로 소비한다. 현재 parser는 비어 있지 않은 비-comment invalid line을 조용히 무시한다.
- Root environment는 invocation cache를 사용하고 imported plan canonical/alias는 동일한 child
  effective config와 fresh child environment를 사용한다.
- Imported interaction·provision과 direct child interaction에는 documented child owner contract를
  끝까지 전달하지 못하는 기존 결함이 있다. [TASK-264](264-restore-imported-command-ownership.md)가
  successful-path ownership을 먼저 복구하고 TASK-248이 이 정책을 적용한다.
- Interaction/subcommand `env_file`은 schema-valid이고 merge되며 tracked example에도 있지만 실행
  consumer가 없다. 이를 조용히 지원하거나 즉시 제거하지 않고 [TASK-265](265-decide-interaction-env-file-contract.md)에서
  compatibility 결정을 먼저 내린다. TASK-248은 그 결정을 구현한다.

### 2. Environment input state

모든 route는 값을 적용하기 전에 owner별 ordered report를 만든다. 실패가 하나라도 있으면 모든
env-file-derived candidate 값을 폐기하며 global cache나 caller environment를 부분 변경하지 않는다.
모든 선언을 끝까지 검사해 failure를 선언 순서대로 모으되 값, key와 file content는 출력하지 않는다.

| Entry state | Aggregate | 판정 |
| --- | --- | --- |
| 선언 없음 | `complete` | 기존 `vars`·named environment·OS precedence 유지 |
| 정상 load | `complete` | 모든 파일 성공 뒤 한 번에 merge·interpolate |
| optional missing | `complete_with_skips` | 정상 상태이며 failure diagnostic을 만들지 않음 |
| required missing | `incomplete` / `missing_required` | route policy 적용 |
| 존재하지만 stat/open/read 불가 | `incomplete` / `inaccessible` | `required`와 무관하게 route policy 적용 |
| scanner error 또는 nonblank/non-comment invalid dotenv line | `incomplete` / `malformed` | line number만 진단하고 content는 출력하지 않음 |
| 앞 파일 성공 뒤 뒤 파일 실패 | `incomplete` | 앞 파일 값까지 전부 폐기하고 모든 나머지 entry를 진단 |

뒤 env file이 앞 file을 덮고 OS가 모든 route에서 최우선인 현재 의미를 보존한다. 나머지 성공
precedence는 route별로 다르며 TASK-248은 아래 순서를 그대로 고정한다.

| Successful route | Low → high, OS 직전 |
| --- | --- |
| Whole-stack/root compose lifecycle | root `vars` < top-level `environment` < root `env_file` < selected environment preset < legacy mode < entry `vars` |
| Root interaction | root `vars` < top-level `environment` < root `env_file` < resolved interaction `environment` |
| Root plan | top-level `environment` < root `env_file` < global `vars` < named environment vars < site vars < plan vars < CLI `--var` < entry vars |
| Imported plan canonical/alias | 위 plan 순서를 owning child 값만으로 적용 |
| Direct/imported child interaction after TASK-264 | child `vars` < child top-level `environment` < child `env_file` < resolved interaction `environment` |
| Imported provision after TASK-264 | child `vars` < child top-level `environment` < child `env_file` |
| Root provision, `ktl`, `ssh up`, raw compose without entry overlay | root `vars` < top-level `environment` < root `env_file` |

TASK-265가 interaction-level `env_file` 지원을 선택하기 전에는 해당 field가 이 표에 새 layer를
추가하지 않는다. Display path는 설정에 적힌 path를 사용한다. 상대 선언을 local absolute path로
확장해 출력하지 않으며 사용자가 절대 path를 선언한 경우만 그대로 보인다.

### 3. Owner contract

| Route identity | Env owner |
| --- | --- |
| Root/whole-stack/root plan | root effective config |
| Imported plan canonical 또는 explicit alias | 동일한 owning child effective config |
| `run --project`, `project:interaction` | 선택한 child effective config |
| Imported interaction/provision canonical 또는 explicit alias | 등록 이름은 parent-visible이지만 env와 working directory는 owning child |

다른 owner의 env를 먼저 load하지 않는다. Root env failure는 child route를 막지 않고 child failure는
root route에 누출되지 않는다. Child interaction은 child `vars`, top-level `environment`, `env_file`,
resolved item `environment`와 child working directory를 사용한다. Imported provision은 child `vars`,
top-level `environment`, `env_file`과 child working directory를 사용한다. 두 route 모두 parent 값은
merge하지 않는다.

### 4. Route policy

| Class | User-visible routes | Incomplete behavior |
| --- | --- | --- |
| Execution/mutation | `up`, `restart`, `build`, `run`, `provision`, `compose`, `ktl`, `ssh up` | exit 1; dry-run/explain 포함; hook·health·runner·backend 전에 중단 |
| Teardown | `down`, `stop` | exit 1; hook, marker 삭제와 backend 전에 중단 |
| Observation | named/whole `status`, named/stack `logs` | config-only partial document, exit 1; hook·backend·HTTP health를 시작하지 않음 |
| Diagnostic | `doctor`, `doctor --strict` | 모든 독립 check를 계속하고 env-dependent probe는 blocked finding으로 기록; default 0, strict 1 |
| Structural validation | `validate`, `config validate` | env file I/O 없이 schema·semantic 검사를 계속; availability는 doctor/runtime 소유 |
| Config-only/no env input | `show`, `ls`, `manifest`와 다른 `config` query, `provision --list`, `console`, meta commands | 현재 동작과 exit 유지; env file을 load하지 않음 |
| Env-independent SSH | `ssh down`, `ssh status` | 현재 Docker 동작 유지; env failure policy를 적용하지 않음 |

Hooks는 target route의 owner와 report가 결정된 뒤에만 평가한다. Imported route를 처리하기 위해 root
env를 선행 load하지 않는다. Whole-stack과 root hooks는 root owner를 사용한다. `status`·`logs`의
partial은 resource 상태나 log content를 추측하지 않으며 declared target 정보만 보여준다.

`Config.ValidateWarnings`와 CLI `configuredComposeFiles`의 직접 `LoadEnvFile`은 제거한다. Validation은
literal `vars`·top-level `environment`만으로 결정할 수 없는 unresolved-variable와 compose-path 검사를
defer하고 성공으로 주장하지 않으며, env availability error나 새 warning category도 만들지 않는다.
Default/strict의 기존 schema·semantic exit 의미는 그대로다. `doctor --strict`이 env availability gate다.

Interaction/subcommand `env_file`은 TASK-265 결정 전까지 schema acceptance와 inert 동작을 보존한다.
TASK-248은 field를 조용히 읽기 시작하거나 제거하지 않으며 TASK-265가 고른 support 또는 versioned
rejection contract를 따른다.

### 5. Text, JSON and exit contract

Execution·mutation·teardown 실패의 human mode는 stdout을 비우고 stderr의 기존 root error path에 다음
stable message를 전달한다. File별 reason은 `missing required file`, `cannot read file`, `invalid dotenv
syntax at line N` 중 하나이며 raw OS error, key와 value는 내보내지 않는다.

```text
environment inputs are incomplete
  - <configured-path>: <stable-reason>
```

JSON mode는 stdout에 기존 envelope 하나만 출력하고 stderr에는 같은 human error를 유지한다.

```json
{"error":{"message":"environment inputs are incomplete\n  - <configured-path>: <stable-reason>","exit_code":1}}
```

Whole `status` human mode는 기존 DVA/config/subproject metadata 뒤에 아래 block을 stdout으로 출력하고
endpoint table을 생략한다. Stderr에는 file failure list를 한 번 출력한다.

```text
Lifecycle: (not queried: environment inputs incomplete)
```

Whole `status --json`은 기존 `dva_version`, `config_found`, config metadata를 유지하고 runtime-derived
`stack`을 생략한 채 아래 fields를 같은 단일 document에 넣는다.

```json
{
  "target": "stack",
  "environment": {
    "state": "partial",
    "failures": [
      {"file":"<configured-path>","required":true,"kind":"missing_required"}
    ]
  },
  "runtime": {"queried":false,"reason":"environment_incomplete"},
  "error": {"message":"environment inputs are incomplete","exit_code":1}
}
```

Named root/imported plan `status`는 owner load 전에 기존 `[plan: ...]` stderr header를 쓰지 않는다.
Human stdout에는 lifecycle table 대신 다음 한 줄, stderr에는 failure list를 출력한다. JSON은 기존
generic envelope 대신 아래 단일 partial document를 출력한다. `plan`은 canonical/alias 중 사용자가
입력한 spelling을 보존하고 성공 status/health fields는 생략한다.

```text
Plan: <invoked-name> (not queried: environment inputs incomplete)
```

```json
{
  "action": "status",
  "plan": "<invoked-name>",
  "environment": {
    "state": "partial",
    "failures": [
      {"file":"<configured-path>","required":true,"kind":"missing_required"}
    ]
  },
  "runtime": {"queried":false,"reason":"environment_incomplete"},
  "error": {"message":"environment inputs are incomplete","exit_code":1}
}
```

모든 `logs` route는 partial일 때 log content나 control record를 stdout에 쓰지 않는다. Human stderr의
target은 named plan route에서 trailing backend argv와 무관하게 `plan <invoked-name>`, non-plan compose
route에서 모든 trailing argv와 무관하게 `stack`으로 고정한다. DVA는 raw argv에서 flag value와 service를
추측하지 않는다. JSON은 이 전체 text를 message로 갖는 기존 root failure envelope 하나만 stdout에 쓴다.
Hook, compose child와 process log-file read는 모두 0회다.

```text
Logs not queried for plan <invoked-name>: environment inputs are incomplete
Logs not queried for stack: environment inputs are incomplete
```

Whole/named `status`와 모든 `logs` partial은 exit 1이고 기존 env warning을 중복 출력하지 않는다.

Doctor는 새 JSON field를 만들지 않고 기존 `DoctorResult`만 사용한다. Env report의 각 선언은 기존
env-file existence check를 대체하는 아래 row 하나로 표현한다. Optional missing은 row를 생략한다.

```json
{
  "name": "Environment input loads: <configured-path>",
  "finding": "Environment input is UNAVAILABLE: <stable-reason>",
  "passed": false,
  "fix_hint": "Fix env_file entry: <configured-path>"
}
```

성공 entry는 같은 `name`, `passed:true`이며 `finding`/`fix_hint`를 생략한다. Doctor check 실행/skip은
다음과 같이 고정한다.

| Doctor check | Env incomplete behavior |
| --- | --- |
| Docker daemon/socket, compose project name, subproject collision, non-compose stack file, devcontainer, gitignore | 계속 실행 |
| Environment input loads | 모든 entry를 보고하며 failure row는 built-in `passed:false` |
| Compose file existence | 실행하지 않고 `name:"Compose file existence (skipped: environment input unavailable)"`, `passed:true` |
| Compose config resolves | child를 시작하지 않고 `name:"Compose config resolves (skipped: environment input unavailable)"`, `passed:true` |
| User-defined `file_exists`, `command`, `docker_socket` | DVA env-file 비의존이므로 기존대로 실행·집계 |

Text renderer는 같은 row를 기존 `[PASS]`/`[FAIL]`과 `->` hint 형식으로 출력한다. Built-in env failure만
있으면 default doctor exit 0, strict exit 1이다. User-defined check failure가 있으면 default도 기존대로
non-zero다. JSON은 기존 `{"checks":[...]}` 하나이며 generic failure envelope를 덧붙이지 않는다.
어떤 partial/doctor output도 env key, value 또는 선행 merge count를 포함하지 않는다.

### 6. Acceptance fixtures for TASK-248

- Root와 child에 같은 변수·file 이름과 상반된 상태를 두고 root, imported canonical, alias,
  `run --project`, `project:interaction`, imported interaction/provision이 오직 자기 owner report를 사용한다.
- `loaded → required missing → malformed`, optional missing 사이 성공, optional unreadable, scanner error와
  invalid line을 table test로 만들고 failure 뒤 caller/cache에 file-derived 값이 0개임을 확인한다.
- 모든 execution/teardown route에 hook marker, health counter, fake backend와 purge marker를 두고 실패 시
  hook/backend/health count 0과 marker byte-identical을 확인한다.
- Whole/root-plan/imported canonical/alias `status`와 plan/stack/service-filtered invocation `logs`의
  route별 text/JSON snapshot, channel, exit 1, failure order와 secret sentinel 부재를 검증한다.
  Service-filtered non-plan invocation의 diagnostic target도 항상 `stack`이어야 한다.
- Doctor default/strict는 같은 complete check list를 출력하며 built-in-only env failure에서 각각
  exit 0/1이다. User-defined failure 집계는 보존하고 env-dependent compose probe count는 0이다.
- Validate는 env file을 open하지 않고 두 기존 direct `LoadEnvFile` 경로가 partial map으로 warning이나
  compose path를 판정하지 않음을 검증한다.

### 7. Rejected alternatives and migration

- Helper-level unconditional error/exit는 doctor와 partial observation의 진단 도달성을 끊어 기각한다.
- Observation backend를 partial env로 계속 호출하면 다른 resource identity를 조회할 수 있어 기각한다.
- `down`·`stop` advisory 예외는 잘못된 resource를 정리할 수 있어 기각한다.
- 앞 file 값을 실패 뒤 유지하거나 imported alias에 parent env를 쓰는 방식은 atomicity와 owner 계약을
  위반해 기각한다.

TASK-248은 warning-and-continue를 위 계약으로 바꾼다. Invalid dotenv line을 과거처럼 무시하던 file은
명시적 오류가 되므로 line을 수정해야 한다. Optional missing, 성공 precedence와 config-only query는
호환된다. 새 encrypted-source metadata나 unseal hint는 TASK-245·246 전에는 도입하지 않는다.

## Completion Criteria

- [x] Inventory every `loadEnv` caller and classify it by observable command and purpose rather than source filename | verify: `test "$(/usr/bin/grep -R --include='*.go' --exclude='*_test.go' -n 'loadEnv(' internal/cli | /usr/bin/grep -v 'func loadEnv' | /usr/bin/wc -l | /usr/bin/tr -d ' ')" -eq 18`
- [x] Freeze a complete matrix for owner (root/imported canonical/imported alias) × required true/false × missing/inaccessible/malformed × single/multi-file partial merge across text, JSON, exit code, child-process start, and diagnostic completeness | verify: human — all inventoried call sites and every state class must appear exactly once
- [x] Decide `up`, `restart`, `build`, `run`, hooks, provision, kubectl, ssh, compose passthrough, `down`, `stop`, `status`, `logs`, doctor default/strict, and query surfaces explicitly | verify: human — no row may inherit a generic “lifecycle” label without rationale
- [x] Ensure any command classified fail-closed stops before hooks, health checks, backend runners, or any other external child; imported canonical and alias routes must use the owning child effective config, `env_file`, and working directory | verify: human — expected text/JSON/exit fixtures must be written before implementation
- [x] Preserve doctor diagnostic reachability and define how built-in advisory default differs from `doctor --strict`; do not use default doctor exit 0 as a promotion gate | verify: human — the decision must cite both default and strict outcomes
- [x] Define whether any caller may observe variables merged before a later file failure; execution paths must never continue on an accidental partial environment, and observation commands must either return an explicitly marked partial result or discard it | verify: human — exact text and single-document JSON shapes are required
- [x] Record the decision, rejected alternatives, and migration effect in this card | verify: `make doc-check`

## Constraint

Do not implement the policy as an unconditional error inside `loadEnv`. The helper may return richer
status, but the caller contract remains explicit and testable.
