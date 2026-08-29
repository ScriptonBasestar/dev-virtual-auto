# DVA 사용 가이드

> DVA CLI 전체 커맨드 레퍼런스 및 설정 가이드.
> 현재 권장 모델은 `stack`을 선언 저장소로 두고, 실제 실행은 `plans`의 이름을 대상으로 수행하는 구조입니다.
> 빠른 시작은 [README.md](README.md), 설계 배경은 [docs/40-declarative-stack-and-plans.md](docs/40-declarative-stack-and-plans.md) 참조.

## Global Flags

| Flag | Description |
|------|-------------|
| `--debug` | 디버그 로깅 활성화 |
| `--dry-run` | 실행 계획만 표시 (실제 실행하지 않음) |
| `--json` | JSON 출력 (LLM 최적화) |

## Commands

### Core Commands

| Command | Description |
|---------|-------------|
| `dva config init` | 현재 디렉토리에 `dva.yml` 생성 (`dva init` alias 지원) |
| `dva config docs` | 프로젝트 AI 파트너용 CLAUDE.md/AGENTS.md 생성/갱신 |
| `dva config migrate` | legacy compose 선언을 `runners` 형태로 재작성 |
| `dva run CMD [ARGS]` | `dva.yml`에 정의된 interaction 커맨드 실행 |
| `dva ls` | 실행 가능한 이름과 interaction 목록 표시 |
| `dva show <NAME>` | 특정 실행 이름 또는 설정 개요 표시 |
| `dva up <NAME>` | named execution entry 실행 |
| `dva down <NAME>` | named execution entry teardown |
| `dva stop <NAME>` | named execution entry 중지 |
| `dva status [NAME]` | 실행 상태 표시 |
| `dva version` | 버전 표시 |
| `dva skill install/status/uninstall` | 내장 AI 스킬 설치 상태 관리 |

`dva run`은 생략 가능합니다. `dva shell`은 `dva run shell`과 동일합니다.
`namespace:command` 문법도 지원합니다 (예: `dva engine:test`).

### AI 스킬 설치

`make install`은 바이너리만 설치합니다. 바이너리에 포함된 정본 스킬 `dva`와
`dva-config`는 AI 에이전트 없이 다음 명령으로 복사 설치합니다. 기본 scope는 `user`,
기본 runtime은 지원 대상 전체입니다.

바이너리 설치는 각 목적지에서 atomic rename을 사용하며, 두 번째 목적지 교체가 실패하면
먼저 교체된 목적지를 가능한 한 이전 파일로 복원합니다. 이는 서로 다른 filesystem을
하나의 atomic commit으로 묶는 보장은 아닙니다. 프로세스 crash·전원 손실·rollback 자체
실패 사이에는 두 경로가 서로 다른 버전을 가질 수 있습니다. rollback이 실패하면 유일한
복구 backup은 지우지 않고 실패 출력과 ledger에 정확한 경로를 남깁니다. 그 파일을 보존한
채 명령의 실패 출력을 확인한 뒤 재실행하거나 수동 복구해야 합니다.

```bash
dva skill install
dva skill install --runtime claude-code,codex,opencode,grok,antigravity,agent-mesh
dva skill install --scope project --runtime codex,opencode
dva skill status --json
dva skill uninstall --runtime grok

# receipt 없는 기존 dva 이름만 백업 후 명시적으로 인수
dva skill install --runtime codex --takeover
# 일반 uninstall은 백업을 복원하지 않음; 명시할 때만 검증 후 복원
dva skill uninstall --runtime codex --restore-takeover-backup
# 보존된 backup ID와 검증 상태 조회; state를 바꾸지 않음
dva skill backup list --runtime codex
dva --json skill backup list --scope project --runtime codex,antigravity

# 실제 파일과 receipt를 바꾸지 않고 충돌까지 미리 검사
dva skill install --dry-run
dva skill uninstall --dry-run --runtime claude-code
```

| Runtime | User scope | Project scope |
|---------|------------|---------------|
| Claude Code | `~/.claude/skills` | `.claude/skills` |
| Codex | `~/.agents/skills` | `.agents/skills` |
| OpenCode | `~/.config/opencode/skills` | `.opencode/skills` |
| Grok | `~/.grok/skills` | `.grok/skills` |
| Antigravity IDE | `~/.gemini/config/skills` | `.agents/skills` |
| Agent Mesh | `~/.config/agent-mesh/skills/dva` | `.agent-mesh/skills/dva` |

프로젝트 scope에서 Codex와 Antigravity IDE는 같은 `.agents/skills`를 공유하므로 한 번만
복사하고 receipt에서 두 runtime의 소유 관계를 함께 기록합니다. 설치 상태는
`$XDG_STATE_HOME/dva/skill-installs/` 아래 receipt로 관리하며, `XDG_STATE_HOME`이 없으면
`~/.local/state/dva/skill-installs/`를 사용합니다.

공유 runtime root 자체의 소유자는 없습니다. 설치기는 각 최상위 스킬마다
`$XDG_STATE_HOME/agent-skills/claims/v1/`에 producer-neutral claim을 기록하며, DVA는
`dva` producer로 자신의 두 이름만 claim합니다. 다른 producer claim, symlink, 특수 파일은
`--takeover`로도 거부합니다. 인수 백업은 DVA state에 남아 있으며 status에서 available/corrupt를
확인할 수 있습니다. `dva skill backup list`는 scope/runtime으로 필터한 receipt-backed backup ID,
대상, 스킬과 무결성 상태를 조회하며 state를 바꾸지 않습니다. 프로젝트 scope에서 runtime들이
공유하는 같은 `(destination, backup_id)`는 중복하지 않고, 서로 다른 backup ID는 별도 행으로
유지합니다. 일반 uninstall은 백업을 보존하고 자동 복원하지 않습니다.
두 위험 옵션은 지원 runtime 전체를 암묵적으로 선택하지 않으며 `--runtime`을 반드시 지정해야
합니다. 공유 목적지에 다른 consumer가 남아 있으면 복원을 거부합니다.

takeover 백업은 `$XDG_STATE_HOME/dva/skill-takeovers/` 아래의 검증된 ID로 계산하며 receipt의
임의 절대경로를 신뢰하지 않습니다. 일반 uninstall로 마지막 consumer를 제거하면 DVA 파일과
claim만 없애고 `backup-only` receipt tombstone을 남깁니다. 이후 명시적 restore가 원본과
manifest를 검증해 성공한 뒤에만 backup과 tombstone을 제거합니다. 보존 범위는 regular
file/directory의 bytes, 상대 경로, 빈 directory, permission bits입니다. symlink와 특수 파일은
거부하며 uid/gid, ACL, xattr, birthtime, sparse/hardlink identity는 보존 계약 밖입니다.
인수 시에는 live 항목을 같은 runtime root의 capture stage로 먼저 atomic rename한 뒤 그 고정된
snapshot에서 durable backup을 만듭니다. 따라서 복사 도중 바뀌는 live tree를 나중에 덮어쓰지
않습니다. 실패 복원 여부가 불확실하면 capture stage와 durable backup을 삭제하지 않고 오류에
recovery artifact 경로를 남깁니다.

Claim 생성·갱신·삭제는 두 DVA 이름을 정렬해 잠근 뒤 reservation/generation CAS로 수행합니다.
중간에 남은 non-active claim, claim/receipt 불일치, malformed claim은 자동 추론하지 않고
`recovery-required`로 중단합니다. 포맷과 digest vector는
[Agent Skills claim protocol](internal/skillclaim/PROTOCOL.md)에 고정되어 다른 producer도 같은
계약을 독립 구현할 수 있습니다. 프로세스 crash나 전원 손실을 여러 filesystem에 걸친 하나의
atomic commit으로 보장하지는 않으며, 실패 시 보존된 stage/claim을 오류에 표시해 복구 근거로
남깁니다. 여러 runtime destination은 모두 mutation 전에 preflight하지만, 첫 destination 완료 뒤
두 번째 destination에서 예측 불가능한 I/O 오류가 발생한 경우 전체 destination을 하나의 transaction으로
되돌리지는 않습니다. 원인을 해결한 뒤 같은 명령을 재실행해 수렴시키는 것이 운영 복구 절차입니다.

안전 규칙:

- receipt 없는 동명 스킬이나 symlink는 덮어쓰지 않습니다.
- 설치 뒤 수정된 파일은 update/uninstall하지 않고 `drifted`로 보고합니다.
- `uninstall`은 receipt와 현재 SHA-256이 모두 일치하는 DVA 소유 파일만 제거합니다.
- 과거 이름 `config`는 이름만 보고 삭제하지 않습니다.

Agent Mesh는 DVA namespace 아래에 `dva.md`, `dva-config.md`만 설치합니다. 이 파일은
frontmatter를 제거하고 canonical body, `references/*`, text `assets/*`를 경로순으로 inline한 생성물입니다.
Agent Mesh가 local bundle을 해석한다고 가정하지 않으므로 local path는 해당 inline anchor로 바꿉니다.
DVA의 책임은 파일 설치와 receipt 검증까지이며,
`am skill sync`의 필요성·결과·다른 native target과의 상호작용은 이 설치 계약의 범위 밖입니다.
Antigravity CLI(`agy`)의 flat skill 형식도 Antigravity IDE와 다르므로 이 runtime 이름은 IDE만 뜻합니다.

#### init (config init)

```bash
dva config init                  # 자동 감지 기반 dva.yml 생성
dva init                         # 위와 동일 (backward compat alias)
dva config init -t node          # --template: 템플릿 지정 (minimal, rails, node, python, go)
dva config init --recursive      # 서브프로젝트에도 dva.yml 생성
dva config init --devcontainer   # .devcontainer/devcontainer.json 포함 생성
dva config init --all            # 가능한 모든 기능 통합 활성화 (devcontainer 등)
```

생성 후 `am run dva-improve`로 AI 기반 최적화를 실행할 수 있습니다.

#### docs (config docs)

```bash
dva config docs                  # CLAUDE.md/AGENTS.md 가이드 생성/갱신
```

`docs`는 AI 에이전트가 DVA 환경을 인식하게 만드는 기본 문서를 생성합니다.
(과거 `dva config improve --docs-only`와 동일)

#### migrate (config migrate)

```bash
dva config migrate               # 변경 결과만 출력 (파일은 그대로)
dva config migrate --write       # 실제 적용
dva config migrate ../other-repo # 다른 프로젝트 미리보기
```

compose를 stack 항목에 직접 선언하던 세 가지 legacy 형태 — 이름이 `compose`인
항목에 compose 키를 그대로 둔 형태, `plugin: compose`, 중첩 `compose:` 하위 키 —
를 현재 스키마가 요구하는 `default_runner` + `runners.compose` 형태로 옮깁니다.

```yaml
stack:                          stack:
  compose:                        compose:
    files: [compose.yml]   ->       default_runner: compose
                                    runners:
                                      compose:
                                        files: [compose.yml]
```

바뀌는 항목만 재작성하므로 나머지 줄은 주석·빈 줄까지 원본 바이트 그대로
유지됩니다. `--write` 전에 결과를 메모리에서 먼저 로드해 검증하므로 DVA가 읽을 수
없는 상태로 파일이 남지 않습니다.

`tags`는 옮기지 않고 **양쪽에 복사**합니다. `LifecycleEntry.Tags`는 stack 항목
필터링에, `ComposePluginConfig.Tags`는 compose 서비스 필터 기본값에 쓰이는데
legacy 형태에서는 한 키가 두 역할을 겸했기 때문입니다.

#### run

```bash
dva run shell             # interaction 커맨드 실행
dva shell                 # 위와 동일 (run 생략)
dva run -e test           # --explain: --dry-run 별칭으로 실행 계획 표시
dva run -p 8080:80 web    # --publish: 포트 퍼블리시
dva run --project api test  # 서브프로젝트 커맨드 실행
dva api:test              # 위와 동일 (namespace 문법)
```

#### ls

```bash
dva ls                    # 테이블 형식
dva ls -f json            # JSON 출력
dva ls -f yaml            # YAML 출력
dva ls -d                 # 상세 정보 (runner type, service, command)
```

### Project Management

| Command | Description |
|---------|-------------|
| `dva show` | 설정 요약 또는 특정 실행 이름 상세 표시 |
| `dva status` | effective default plan 상태; 없으면 워크스페이스 상태 |
| `dva config show` | 최종 병합된 설정 출력 (modules + override 적용 후) |

```bash
dva show                  # 등록된 설정 전체 요약
dva show local-dev        # 특정 named execution entry 상세
dva show --json           # JSON 출력
dva status                # effective default plan 상태; 없으면 전체 상태
dva status local-dev      # 특정 named execution entry 상태
dva status --json         # JSON 출력
dva config show           # JSON 형식 (기본)
dva config show -f yaml   # YAML 형식
```

`dva show --json`과 `dva manifest -f json`/`yaml`은 bare lifecycle이 실제로 선택하는
플랜을 `default_plan`으로, 선택 근거를 `default_plan_source` (`explicit`, `implicit-single`,
`none`)으로 출력합니다. 선택할 플랜이 없으면 `default_plan`은 생략되고 source는 항상
출력됩니다.

### Lifecycle

#### 권장 실행 모델

권장 구조:

- `stack` = 재사용 가능한 실행 대상 선언
- `plans` = 실제 실행 가능한 이름
- `environments` = dev/stg/prd 같은 환경 차이
- `sites` = local/office/remote/cloud 같은 실행 host 차이
- `interactions` = 단발성 편의 명령
- `provision` = 준비/초기화 절차

> DVA는 로컬 개발·유지보수 도구입니다. `environments`의 `stg`/`prd`는 변수 묶음의 이름일
> 뿐 해당 환경을 조작할 권한이 아니며, 원격 대상은 로컬에서 실행할 수 없는 리소스에만
> 선언합니다. 제품 경계는 [PRODUCT.md](PRODUCT.md#product-boundaries)가 소유합니다.

```bash
dva ls
dva show local-dev
dva up local-dev
dva status local-dev
dva stop local-dev
dva down local-dev
```

#### named execution entry

실행 명령의 직접 대상은 `stack`이 아니라 `plans.<name>` 입니다.

예:

```bash
dva up local-dev
dva up backend/local-dev
```

`stack`은 선언 저장소일 뿐이며, 실행 표면은 아래 동사 하나뿐입니다. `dva stack`,
`dva app`, `dva infra`, `dva clean`은 제거됐습니다 (docs/43).

| Command | Description |
|---------|-------------|
| `dva up <NAME>` | named execution entry 실행 |
| `dva down <NAME>` | named execution entry teardown |
| `dva down <NAME> --purge` | 볼륨·로컬 이미지·provision 마커까지 제거 (구 `dva clean`) |
| `dva stop <NAME>` | 중지 (제거하지 않음) |
| `dva restart <NAME>` | 재시작 |
| `dva logs [NAME]` | 로그 보기 |
| `dva build [NAME]` | 빌드 수행 |

```bash
dva up local-dev
dva down local-dev
dva stop local-dev
```

위치 인자로 stack entry 이름을 받는 라이프사이클 동사는 `restart` 하나뿐입니다.
`up`/`down`/`stop`은 위치 인자를 plan 이름으로만 읽으므로, entry 이름을 주면 그것이
선언된 이름이든 아니든 거부합니다 — plan이 없는 설정에서 `dva up s1`은 `s1`이 실제
entry여도 rc=1입니다. `restart`는 선언된 entry 이름을 받고, 선언되지 않은 이름은 오류로
끝냅니다 (rc=1, 아무것도 실행하지 않음).

entry 이름을 받는 것이 `restart`뿐이라, 거기서만 플래그와 이름을 `--`로 끊을 수
있습니다. `dva restart -- <name>`에서 `--` 뒤의 인자는 `-`로 시작하더라도 전부 이름으로
읽히므로, 거기 쓴 플래그는 조용히 버려지지 않고 "모르는 이름"으로 보고됩니다. 뒤에
아무것도 없는 `dva restart --`는 "이름 없음"이며 맨 `dva restart`와 같게 동작합니다 —
**양방향으로** 같습니다. plan이 여러 개고 기본 plan이 없으면 맨 `dva restart`처럼
"plan을 지정하라"며 거부하고, plan이 없는 설정에서는 맨 `dva restart`처럼 선언된 entry를
**전부** stop+start 합니다. 후자가 래퍼 스크립트에 중요합니다: `dva restart -- "$@"`에서
`"$@"`가 비면 master에서는 아무 일도 없었지만 이제는 전체 재시작이거나 rc=1이며, 둘 다
이전 동작이 아닙니다.

기본 plan이 **해석되는** 설정도 같습니다 — `dva restart --`는 맨 `dva restart`처럼 기본
plan을 실행합니다. 이것이 마지막 예외였고 TASK-210에서 없앴습니다. `--`는 구분자이므로
분류되는 것은 그 **뒤**이지 구분자 자신이 아닙니다. 같은 이유로 `dva restart -- s1`은
`dva restart s1`과, `dva restart -- <plan>`은 `dva restart <plan>`과 동일합니다.

이 규칙은 `restart`만이 아니라 `up`/`down`/`stop`에도 그대로 적용됩니다. `dva up --`/
`down --`/`stop --`는 설정 모양과 무관하게 각각 맨 형태와 동일하며, **양방향으로**
같습니다: 기본 plan이 해석되면 그 plan을 실행하고, plan이 없으면 선언된 entry를 전부
다루며 (`dva down --`은 전부 내리고 rc=0), plan이 여럿이고 기본 plan이 없으면 맨 형태와
**같은 문장**으로 거부합니다. `"$@"`가 빌 수 있는 `dva down -- "$@"` 래퍼가 이 동일성이
필요한 자리입니다.

TASK-207은 이 동일성을 `restart` 전용으로 판정했었습니다 — 위치 인자를 받지 않는 동사는
끊을 이름이 없으니 거기 쓴 `--`는 보고할 만한 실수라는 논리였습니다. TASK-216이 그것을
뒤집었습니다: 18쌍(여섯 fixture x 세 동사) 중 12쌍이 맨 형태와 갈렸고 그중 9쌍은 표현이
아니라 **결과**가 달랐습니다. 끊을 이름이 없다는 관찰은 맞지만, `--`는 자기 인자 목록이
빌 수도 있는 래퍼가 쓰는 토큰이고 그 빈 경우가 여기서 깨졌습니다. 지금은 `restart` 6쌍을
더한 24쌍 전부가 맨 형태와 일치합니다.

바뀐 것이 빈 경우뿐이라고 읽으면 안 됩니다. `dva up -- X`가 `dva up X`와 같은 경로를
타는 것은 **보존된** 성질이 아니라 이번에 생긴 성질이고, 그래서 `dva up X`가 이미
허용하던 X는 `-- X` 형태에서도 새로 허용됩니다. 측정값으로: `dva up -- -`와
`dva up -- --debug`, `dva down -- --debug`, `dva stop -- --debug`가 모두 rc=1에서
rc=0으로 바뀌었습니다 — `dva up -`와 `dva up --debug`가 master에서 **이미** rc=0이었기
때문입니다 (`rejectUnknownFlags`는 `parseDvaFlags`가 토큰을 소비한 뒤에야 도달합니다).
동일성이 그대로 적용된 결과이며, `dva up -`가 맨 대시를 받는 것은 TASK-218이 걸어둔
버그입니다. `dva down -- -`는 rc=1로 남습니다 — `teardownCommon`이 `-`를 직접 거부하기
때문입니다.

구분자가 되는 것은 **맨 앞** 하나뿐이며, 이것은 "몇 번째냐"가 아니라 "어디 있느냐"의
문제입니다. `dva up -- --`는 rc=1(`unknown flag "--"`)이고, `dva up --debug --`도
rc=1입니다 — 후자의 `--`는 그 목록의 첫 `--`이지만 맨 앞이 아니기 때문입니다. `restart`는
이름 목록에서 첫 `--`를 위치와 무관하게 끊으므로 `dva restart s1 -- s2`는 rc=0이고,
`dva restart -- -- s1`은 `unknown stack entry "--"`입니다. `build`는 반대 방향으로
갈리며 (`--` 형태가 맨 형태보다 **더** 합니다) 그것은 TASK-217이 다룹니다.

#### 무인자 lifecycle 선택

완전히 인자 없는 `dva up`/`down`/`stop`/`restart`/`build`/`logs`는 명시된
`default_plan`을 선택하고, `plans`가 정확히 하나이면 그 plan을 자동 선택합니다. 여러 plan이
있는데 `default_plan`이 없으면 DVA는 범위를 추측하지 않고 plan 이름을 요구합니다.

- plan이 없을 때 `up`/`down`/`stop`/`restart`는 기존 whole-stack lifecycle 경로를
  사용합니다.
- plan이 없을 때 `build`/`logs`는 primary Compose에 대한 legacy passthrough를 사용합니다.
  다른 stack 엔트리의 native/process runner까지 포함한다는 뜻은 아닙니다.
- `status`는 effective default가 있으면 해당 plan을 조회하고, 없으면 plan이 없는 구성뿐 아니라
  기본값 없는 다중 plan 구성에서도 워크스페이스 전체를 조회합니다.

`build`/`logs`에 plan 대신 Compose option이나 service를 쓰면 legacy primary-Compose
passthrough를 명시적으로 선택한 것입니다. `up`/`down`/`stop`/`restart`에 stack-path flag를
쓰는 경우는 아래 규칙을 따릅니다.

#### 라이프사이클 플래그

플래그 집합은 **이름 없이 실행할 때**와 **named plan을 지정해 실행할 때**가 서로 다릅니다.

**plan 이름 없이 stack-path flag 사용 시** (`dva up`, `dva down`, `dva stop`, `dva restart`)

effective default가 있는데 plan-name 위치에 flag만 남으면 기본 plan 경로가 막히므로,
`dva up <plan> --force`처럼 plan 이름을 명시해야 합니다. 반대로 기본값 없는 다중 plan 구성은
지원되는 stack-path flag가 raw 인자를 남기면 legacy stack 경로를 허용합니다. selector
(`--tag`, `--exclude-tag`)는 범위를 좁힐 수 있지만 `--force`, `--no-wait`, `--dry-run`은
범위를 좁히지 않습니다. 따라서 `dva up --force`는 다중 plan 구성에서도 whole-stack
force-recreate가 될 수 있습니다. 이것은 완전한 무인자 호출이 모호성으로 거부되는 것과 다른
호환 경로입니다. `--`는 여기서 말하는 flag가 아닙니다. 맨 앞의 `--`는 구분자로 소비되므로
기본 plan 경로를 막지 않고 (TASK-210), plan이 없는 설정에서 whole-stack 경로도 막지 않습니다
(TASK-216). 이 검사는 플래그 유효성보다 먼저 돌 수 있으므로, 오타 난 플래그가 먼저 "plan
이름을 쓰라"는 메시지를 받는 경우가 있습니다.

| Flag | Description |
|---|---|
| `--mode`, `-M MODE` | `modes` 섹션의 named mode 적용 |
| `--env`, `-E ENV` | `environments` 섹션의 named environment 적용 |
| `--tag`, `--tags`, `-T TAG[,TAG]` | 해당 태그를 가진 lifecycle 엔트리만 포함 |
| `--exclude-tag`, `--exclude-tags TAG[,TAG]` | 해당 태그를 가진 lifecycle 엔트리 제외 |

`dva up`은 위에 더해 다음을 인식합니다.

| Flag | Description |
|---|---|
| `--force` | 이미 실행 중이어도 강제로 재시작 |
| `--no-wait` | 서비스 시작 후 준비 상태를 기다리지 않고 즉시 반환 |

> `--dev`/`--docker`는 `applications:`와 함께 제거됐습니다. 엔트리는 `run` 명령 하나를
> 선언하므로 hot-reload 변형은 별도 엔트리로 선언하고 plan으로 선택합니다.

**named plan 지정 시** (`dva up <NAME>`, `dva down <NAME>`, `dva stop <NAME>`)

| Flag | Description |
|---|---|
| `--force` | 이미 실행 중이어도 강제로 재시작 |
| `--no-wait` | 준비 상태를 기다리지 않고 즉시 반환 |
| `--var KEY=VAL` | 실행 시점 변수 override |
| `-v`, `--volumes` | **`down` 전용** — teardown 시 named 볼륨까지 제거 |
| `--purge` | **`down` 전용** — 볼륨 + 로컬 빌드 이미지 + provision 마커까지 제거 |
| `--dry-run` | 실행 계획만 표시 |

`--volumes`/`--purge`는 파싱은 모든 plan 동사에서 되지만 `down`이 아니면 에러입니다
(`--purge is only supported by down`). 데이터를 지우는 플래그가 조용히 무시되는 경우를
없애기 위한 것으로, `dva up p --purge`는 성공하지 않습니다.

환경/모드/태그는 plan 정의(`plans.<name>`)가 결정하므로, named plan 실행에는 `--mode`/`--env`/`--tag`를 쓸 수 없습니다.

```bash
dva up --tag db,cache          # db/cache 태그 엔트리만 시작
dva up --exclude-tag heavy     # heavy 태그 엔트리 제외하고 시작
dva up --force --no-wait       # 강제 재시작 후 대기 없이 반환
dva down -E staging            # staging environment 설정으로 teardown
dva up local-dev --force       # named plan을 강제 재시작
```

`--tag`/`--exclude-tag`은 `--exclude-tag=heavy,slow` 형태의 `=` 문법도 지원합니다.

#### 엔트리 부분 실행

개별 엔트리만 돌리려면 그 조합을 plan으로 선언하고 이름으로 실행합니다 —
`dva stack up <entry>` 같은 엔트리 직접 지정 표면은 없습니다.

```yaml
plans:
  db-only:
    entries:
      - name: infra
        runner: compose
        services: [postgres, redis]
```

```bash
dva up db-only
dva logs db-only
dva down db-only
```

> **완전히 인자 없는 `dva up`은 명시된 `default_plan` 또는 유일한 plan을 선택합니다.** 여러
> plan에 기본값이 없으면 plan 이름을 요구하며, plan이 전혀 없을 때만 선언된 stack 전체를
> 대상으로 합니다. 이 whole-stack 경로의 Compose 러너는 `--profile` 없는
> `docker compose up`이므로 profile 없는 서비스만 뜹니다. 이 경로의 기본을 최소로 유지하려면
> **Docker Compose 네이티브 `profiles:`**로 계층을 나누세요 — 코어 데이터
> (postgres/redis)는 profile 없이 항상 시작하고, 무거운 계층은
> `profiles: [workflow|monitoring|dev-tools|apps]`로 opt-in 합니다. 명시적 서비스 서브셋
> 실행은 `dva up <plan>`(`plans.entries[].services`)을 쓰며, plan이 profile 걸린 서비스를
> 이름으로 지정하면 profile과 무관하게 시작됩니다.

#### 앱 프로세스

`applications:` 섹션과 `dva app` 명령은 제거됐습니다 (docs/43). 앱은 `native` 러너를 쓰는
stack 엔트리로 선언하고 plan으로 조합합니다. 기존 파일은 `dva config migrate`가
`stack.<name>.default_runner: native` + `runners.native`로 변환합니다 —
`dev`/`variants`/`depends_on`/`port`는 변환되지 않으므로 리포트가 이름을 대는 항목만
손으로 옮기면 됩니다.

```yaml
stack:
  api:
    default_runner: native
    runners:
      native:
        dir: ./api
        build: go build -o bin/api ./cmd/api
        run: ./bin/api
        env:
          PORT: "8080"
```

> **기능 손실**: `applications.<name>.health.required: true`(엄격 준비 상태 — 미달 시
> non-zero exit + `[FAIL]`)에 해당하는 스위치가 plan 경로에는 없습니다. 최상위
> `health_checks`는 `required`를 지원하지 않으며 항상 advisory(exit 0)입니다. 준비 상태
> 실패를 CI에서 실패로 만들려면 지금은 `checks:` 또는 interaction 명령으로 직접 게이트를
> 세워야 합니다.

#### 볼륨·이미지·마커까지 제거 (`--purge`)

```bash
dva down <PLAN> -v            # + named 볼륨 제거 (데이터 손실 주의)
dva down <PLAN> --purge       # + 로컬 빌드 이미지 + provision 마커 제거
dva down <PLAN> --purge --force   # 확인 프롬프트 스킵
```

`--purge`는 확인 프롬프트를 띄웁니다. 프롬프트에 답할 수 없는 환경(파이프, CI 러너,
`</dev/null`)에서는 아무것도 지우지 않고 **실패**합니다 — 조용히 exit 0으로 끝나
스크립트가 삭제된 것으로 오해하는 일을 막기 위해서입니다. 비대화식 실행에는 `--force`를
명시하세요. 터미널에서 `n` 또는 Enter로 거절한 경우는 그대로 exit 0입니다.

provision 마커는 plan이 아니라 provision 프로파일 단위라서, `--purge`는 설정 디렉토리의
마커를 **전부** 지웁니다 (구 `dva clean`과 같은 사정거리). `--dry-run`을 붙이면 지우지 않고
`would delete provision marker ...`만 출력합니다.

`--purge`에는 hook이 없습니다 — hook을 걸 수 있는 명령은 `down` 자체이므로
`interaction.down.before`/`after`가 `--purge` 실행에도 그대로 돕니다.

#### 환경 분기 (`environment` / `site` / `vars`)

새 구조에서는 실행 이름이 기본 컨텍스트를 담고 있으므로, 환경 분기는 주로 설정의 `plans`에서 결정합니다.
아래 세 항목은 CLI 플래그가 아니라 `plans.<name>` 안의 YAML 필드입니다.

- `environment`: `environments.<name>` 선택
- `site`: `sites.<name>` 선택
- `vars`: 실행 시점 변수 override

권장 방식:

- 기본은 `plans.<name>` 안에 `environment`, `site`를 정의
- 추가 일회성 조정이 필요하면 `--var KEY=VALUE` 같은 명시적 override 사용

### Integration Tools

| Command | Description |
|---------|-------------|
| `dva compose ARGS` | raw Docker Compose 패스스루 (escape hatch — 내가 소유한 compose를 직접 실행) |
| `dva ktl ARGS` | kubectl 패스스루 |
| `dva ssh up/down/status` | SSH agent 컨테이너 관리 |

> `dva infra`는 제거됐습니다 (docs/43). 외부 스택은 stack `source:`가 흡수했으므로
> ([stack.source](#stacksource-외부-스택-소싱) 참조), 해당 엔트리를 plan에 넣고
> `dva up <plan>`으로 실행합니다.

#### ssh up

```bash
dva ssh up                        # 기본값으로 SSH agent 컨테이너 시작
dva ssh up -k ~/.ssh/id_ed25519   # --key: SSH 키 경로 (기본값 $HOME/.ssh/id_rsa)
dva ssh up -u devuser             # --user: ssh-agent 컨테이너에서 사용할 사용자
dva ssh up -v /workspace          # --volume: 마운트할 볼륨 (기본값 $HOME)
```

### Advanced Utilities

| Command | Description |
|---------|-------------|
| `dva manifest` | LLM용 커맨드 매니페스트 출력 |
| `dva console start/inject` | 셸 통합 |
| `dva provision [PROFILE]` | 프로비저닝 스크립트 실행 |
| `dva validate` | dva.yml 스키마 + 시맨틱 검증 (`dva config validate`도 지원) |
| `dva doctor` | 환경 사전조건 및 설정 문제 진단 (`--fix` 자동 수정) |

#### provision

```bash
dva provision             # default 프로필 실행
dva provision setup       # 특정 프로필 실행
dva provision --list      # 사용 가능한 프로필 목록
```

#### doctor

```bash
dva doctor                # 환경 사전조건 체크 (Docker, compose 파일, .env 등)
dva doctor --fix          # 수정 가능한 문제 자동 해결
dva doctor --json         # JSON 출력
dva doctor --strict       # 빌트인 체크 실패도 exit≠0 (기본은 advisory — 사용자 checks:만 게이트)
```

빌트인 체크는 기본이 **advisory**입니다: 실패해도 전체 exit 0일 수 있고, 사용자 정의
`checks:` 실패만 비-0을 만듭니다. CI에서 “doctor가 통과 = 전부 통과”가 필요하면
`--strict`를 사용하세요 (`dva config validate --strict`와 같은 관례).

빌트인 체크 항목:
- Docker 소켓 권한 및 데몬 접근 가능 여부
- Compose 파일 존재 여부 및 project name 정합성
- `required: true`로 선언된 `env_file` 존재 여부 (누락된 선택 파일은 진단에서 제외)
- Stack 엔트리 참조 파일 존재 여부
- `.sb/dva/`가 `.gitignore`에 포함되어 있는지
- devcontainer 설정 시 `devcontainer.json` 존재 여부
- `dva.yml`의 `checks` 섹션에 정의된 사용자 커스텀 체크

#### config validate

```bash
dva config validate          # 스키마 + 시맨틱 검증
dva config validate --fix    # compose 파일 project name 불일치 자동 수정
dva config validate --strict # drift 경고 시에도 검증 실패 처리
```

스키마 검증 외에 13개 시맨틱 경고를 검사합니다:
- 중복 stack order, 다중 compose 엔트리 분할 권고
- 실행 계획 누락 또는 과도하게 무거운 기본 실행 구성 경고
- 미해결 환경변수 (`${MISSING_VAR}`), 비지원 셸 문법 감지
- 깊은 서브커맨드 중첩 (5단계 초과), 도달 불가능 커맨드
- 정규 섹션 순서 검증

## Configuration (`dva.yml`)

### 기본 구조

```yaml
version: "0.1.44"         # 최소 DVA 버전

env_file:
  - .env

stack:
  core-compose:
    default_runner: compose
    runners:
      compose:
        files: [docker-compose.yml]
        project_name: myproject

plans:
  local-dev:
    environment: dev
    site: local
    entries:
      - name: core-compose
        runner: compose
        services: [postgres, redis]

interaction:
  shell:
    description: "Open shell"
    service: app
    command: /bin/bash
```

### 설정 섹션 레퍼런스

정규 섹션 순서 (validate에서 검증):

| Section | Description |
|---------|-------------|
| `version` | 최소 DVA 버전 |
| `vars` | 글로벌 환경변수 |
| `env_file` | .env 파일 로딩 |
| `stack` | 재사용 가능한 실행 대상 선언 |
| `plans` | 실제 실행 가능한 이름 |
| `default_plan` | 플랜 이름 미지정 시 적용할 기본 `plans` 엔트리 (여러 plan 중 기본 선택) |
| `checks` | `dva doctor` 환경 사전조건 체크 |
| `default_mode` | `--mode` 미지정 시 적용할 기본 `modes` 엔트리 |
| `modes` | 런타임 전략 프리셋 (`--mode`로 선택) |
| `environments` | 환경 프리셋 (`dev/stg/prd`) |
| `sites` | 실행 host 프리셋 (`local/remote/cloud`) |
| `health_checks` | 비-compose 서비스 헬스체크 |
| `interaction` | 커맨드 정의 (command, command list, script, script_file, steps, subcommands 등) — 예약어/훅 규칙은 아래 [interaction](#interaction-예약어와-훅) 참조 |
| `provision` | 프로비저닝 프로필 및 스텝 정의 |
| `modules` | `.sb/dva/*.yml` 모듈 분리 |
| `subprojects` | 서브프로젝트 참조 (모노레포) |
| `endpoints` | 사용자 노출 URL 정의 |
| `infra` | 공유 인프라 서비스 (git 기반) |
| `ssh` | SSH agent 설정 |
| `devcontainer` | devcontainer 통합 (실험적) |

### stack (선언 저장소)

`stack:`은 logical unit 선언 모음입니다.
직접 실행 대상이 아니라 `plans.entries[].name`에서 참조됩니다.

```yaml
stack:
  core-compose:
    description: infra bundle
    default_runner: compose
    runners:
      compose:
        files: [docker-compose.yml, docker-compose.dev.yml]

  api:
    description: backend api
    default_runner: native
    runners:
      native:
        dir: apps/api
        run: go run ./cmd/api
      docker:
        image: myorg/api:dev
        run: docker run --rm myorg/api:dev
      helm:
        chart: ./charts/api
        release: api
        namespace: default
```

핵심 규칙:

- 하나의 `stack` 엔트리는 multi-runner logical unit이 될 수 있음
- `default_runner`는 기본 실행 백엔드
- 실제 실행 runner는 plan/site에서 override 가능
- 정의되지 않은 runner 선택은 validation error

지원 가능한 runner 예:

| Tier | Plugins |
|------|---------|
| Core | `compose`, `kubectl`, `helm`, `process`, `script`, `docker` |
| Extended | `kustomize`, `tilt`, `skaffold`, `podman-compose`, `vagrant` |
| Niche | `sam`, `serverless`, `multipass` |

### stack.source (외부 스택 소싱)

stack 엔트리는 `source:`로 **외부 소유 스택**(다른 repo나 로컬 디렉토리에 정의된
compose 스택)을 가져와 실행합니다. 소싱과 실행을 분리해, 정의는 외부 도구가
소유하고 DVA는 fetch와 수명 주기만 조정합니다.

```yaml
stack:
  postgres:
    default_runner: compose
    source:
      git: https://example.com/shared-infra.git
      ref: v1.2.0                    # 재현성 위해 SHA/tag 권장
    runners:
      compose:
        files: [docker-compose.yml]  # source 디렉토리 기준 (생략 시 자동 탐색)

  local-infra:
    default_runner: compose
    source:
      path: ../shared-infra          # 로컬 디렉토리 참조 (fetch 없음)
```

핵심 규칙:

- `git`과 `path`는 상호 배타 — 정확히 하나만 지정.
- git 소스는 `dva up` 시 **없을 때만 clone**하며 자동 pull하지 않습니다(재현성).
  갱신하려면 캐시 디렉토리(`.sb/dva/sources/<name>/`)를 지우고 다시 실행합니다 —
  전용 갱신 명령(`dva infra update`)은 제거됐습니다.
- 소싱된 엔트리의 `runners.compose.files`와 실행 작업 디렉토리(`.env`,
  build context, 볼륨)는 **source 디렉토리 기준**으로 해석됩니다.
- git 캐시 위치: `.sb/dva/sources/<name>/`.

**`infra:` 마이그레이션** — 구 top-level `infra:` 맵은 deprecated입니다. 로드 시
`source:` 기반 stack 엔트리(태그 `infra`)로 자동 변환되며 경고를 출력합니다. 변환된
엔트리는 다른 stack 엔트리와 동일하게 plan에서 선택합니다 (`--tag infra`로도 필터 가능).
새 설정은 `stack.<name>.source`를 직접 사용하세요.

### plans

`plans`는 실제 실행 가능한 이름입니다.

```yaml
plans:
  local-dev:
    environment: dev
    site: local
    endpoint_tags: [app]
    vars:
      LOG_LEVEL: debug
    entries:
      - name: core-compose
        runner: compose
        order: 10
        services: [postgres, redis]
      - name: api
        runner: native
        order: 20
        depends_on: [core-compose]
```

`dva up local-dev`처럼 직접 실행합니다.

성공한 `up`은 설정된 endpoint를 출력합니다. `endpoint_tags`를 생략하거나 빈 배열로
두면 모든 endpoint를 표시하고, 값을 지정하면 tag가 하나라도 일치하는 endpoint만
표시합니다. `--dry-run`과 실패한 startup은 endpoint 연결 정보를 출력하지 않습니다.

### default_mode

`default_mode`는 `--mode`(`-M`)를 지정하지 않았을 때 적용할 `modes` 엔트리를 선택합니다.

```yaml
default_mode: infra

modes:
  infra:
    ...
  full:
    ...
```

- 기본값이 없습니다. 설정하지 않으면 어떤 mode도 적용되지 않습니다. 실행 범위는 plan
  선택 규칙을 따르며, plan이 없는 whole-stack 경로에서는 모든 stack 엔트리와 Compose의
  profile 없는 서비스를 대상으로 합니다.
- `modes`가 정의되어 있는데 `default_mode`가 비어 있으면 `dva validate`가 경고합니다. 최소 인프라 mode(예: `infra`)를 지정하는 것을 권장합니다.
- `modes`에 없는 이름을 지정하면 경고가 아니라 검증 에러입니다.

### default_plan

`default_plan`은 플랜 이름 없이 `dva up/down/stop/restart/build/logs/status`를 실행할 때
적용할 `plans` 엔트리를 선택합니다.

```yaml
default_plan: dev

plans:
  dev:     { entries: [ { name: frontend-dev,     runner: process } ] }
  preview: { entries: [ { name: frontend-preview, runner: process } ] }
```

- `plans`가 정확히 1개면 그 플랜이 자동으로 기본값입니다. `default_plan`은 **여러 plan 중** 기본을 고를 때 씁니다.
- 완전히 인자 없는 action 명령은 다중 plan에 기본값이 없으면 plan 이름을 요구하지만,
  `status`는 이 경우 워크스페이스 전체를 조회합니다.
- `plans`에 없는 이름을 지정하면 검증 에러입니다 (`dva config validate`).
- 무엇을 기본으로 둘지는 프로젝트 정책입니다 (예: devbox 로컬은 `dev`). DVA는 선택지를 표현할 뿐 기본을 강제하지 않습니다.

### environments / sites

```yaml
environments:
  dev:
    environment:
      APP_ENV: dev
      LOG_LEVEL: debug

sites:
  local:
    vars:
      DVA_SITE: local
    entry_overrides:
      api:
        runner: native
```

`vars` 우선순위 (낮음 → 높음) — plan 실행 경로(`dva up <plan>`) 기준:

```text
env_file < global vars < environment vars < site vars < plan vars < CLI vars < OS 환경 변수
```

여기서 `environment vars`는 `environments.<name>.environment`를 뜻하며, 최상위
`environment:` 블록과는 다릅니다. 최상위 `environment:`는 `dva run` 경로에서
`env_file`보다 **먼저** 적용되어 덮어써집니다 (`environment:` < `env_file` < OS).

OS 환경 변수가 가장 높은 우선순위입니다. 같은 키가 OS에 설정되어 있으면
`dva.yml`의 어떤 레이어(`--var` 포함)도 그 값을 덮어쓰지 못합니다.

#### 컨테이너로 전달되는 환경변수

위 우선순위는 변수의 **값**을 정합니다. 그 값이 **어디까지 가는지**는 실행 경로가
정합니다. `dva run`의 compose 경로에서 DVA는 병합된 선언 환경변수를 `-e KEY=VALUE`로
argv에 주입합니다.

| compose 경로 | `-e` 전달 |
|---|---|
| `method: run` (컨테이너 미실행) | O |
| `method: exec` — 설정값, 또는 실행 중 컨테이너에서 `run`이 자동 전환된 경우 | O |
| `steps:` 항목 (항상 `exec`) | O |
| `profiles:` 설정 시의 `up` | X — `docker compose up`에 `-e` 플래그가 없음 |

- 전달 대상은 **위 우선순위 체인이 만들어낸 병합 결과 전체**입니다 — `env_file`, global
  `vars`, `environment:`, site vars, plan vars, `--var`, 그리고 커맨드 자신의
  `environment:`. 어느 레이어에서 왔는지는 구분하지 않습니다.
- 반대로 호스트 환경 전체가 넘어가는 것은 **아닙니다**. `dva.yml`(또는 `--var`)에 선언된
  키만 대상이며, OS 값은 그 키를 덮어쓸 뿐 목록을 늘리지 않습니다. 선언하지 않은 호스트
  변수는 컨테이너에 전달되지 않습니다.
- `DVA_*`는 제외됩니다 (DVA 자체 런타임 변수 — [특수 변수](#특수-변수) 참조).
- kubectl 경로는 전달하지 않습니다. `kubectl exec`에 env 플래그가 없으며, pod의
  환경은 pod spec이 결정합니다.

> **주의**: 선언한 변수는 이미지에 내장된 값을 덮어씁니다. `dva.yml`에 `PATH`를
> 선언하면 exec 시 컨테이너의 `PATH`가 그 값으로 교체됩니다.

#### 실제 적용 결과 확인

`--dry-run`은 실행 대신 **해석 결과**를 출력합니다. 위 순서의 각 레이어가 실제로 몇 개
키를 얹었는지, 어떤 레이어가 비어 있는지를 그대로 보여주므로 "이 변수가 왜 이 값인가"를
추측 없이 확인할 수 있습니다.

```bash
dva up <plan> --dry-run
```

```text
Resolution:
  plan: resolved "local-dev"
  vars: env_file — declared [.env], applied at config load below every layer here
  vars: environment: — not declared
  vars: global vars — merged (2 keys)
  vars: environments."dev" — merged (1 key)
  vars: sites."local".vars — merged (1 key)
  vars: plans."local-dev".vars — merged (1 key)
  vars: cli --var — none passed
  vars: OS environment overrides every layer above
```

`down`, `stop`, `restart`도 동일합니다. 출력은 stderr로 나가므로 `--json`을 함께 써도
stdout의 JSON은 그대로 파싱됩니다. 각 레이어의 의미는
[docs/31-execution-plan-resolution.md](docs/31-execution-plan-resolution.md#4-3-vars-병합)을
참조하세요.

### health_checks

비-compose 서비스(로컬 프로세스 등)의 상태를 확인합니다. `start` 필드가 있으면 자동 시작도 합니다.

```yaml
health_checks:
  local-api:
    type: http           # http, tcp, command
    url: http://localhost:3000/health
    start: "npm run dev"
    start_hint: "Run 'npm run dev' in another terminal"
    timeout: 2           # 헬스체크 타임아웃 (초)
    ready_timeout: 30    # 시작 후 대기 (초)
```

### interaction (예약어와 훅)

`interaction:` 키는 `dva run <name>`으로 실행할 커맨드를 정의합니다. 이름이 내장
커맨드와 겹치면 `dva validate`가 exit 1로 실패하고, 설정을 읽을 때마다 경고가 출력됩니다.
선언이 버려지는 것은 아니며 짧은 형식만 내장 커맨드에게 넘어갑니다 — 아래 규칙을 따릅니다.

**예약어 23개** — 내장 커맨드 이름입니다:

```text
help  version   ls       compose  up      stop    down   build
run   provision validate manifest ktl     ssh     console
completion init  status   config   logs    restart show   doctor
```

**훅 가능 6개** — 예약어 중 `before`/`replace`/`after` 훅을 받는 것:

```text
build  down  logs  restart  stop  up
```

> `stack`/`app`/`infra`/`clean`은 예약어에서 빠졌습니다 (docs/43). 이제 그 이름의
> interaction은 정상 등록되어 `dva app`으로 바로 실행됩니다.
>
> 단 `interaction.clean.before`/`replace`/`after`를 쓰던 설정은 **`validate`가 exit 1로
> 거부**합니다. 훅을 실행하는 경로(`wrapWithHooks`)는 훅 가능 내장 커맨드에만 연결돼
> 있어서, 내장 `clean`이 사라진 지금 그 훅은 아무 데서도 돌지 않기 때문입니다. 조용히
> 넘기지 않는 이유는 훅이 죽어도 출력이 그대로라 사용자가 알아챌 신호가 없어서입니다
> (`before: [backup]`이 안 돌아도 결과는 정상으로 보입니다). teardown을 계속 확장하려면
> `interaction.down.before`/`after`로, `dva clean`을 독립 명령으로 남기려면
> `interaction.clean.exec`/`steps`로 옮기라는 안내가 에러 메시지에 실립니다.

판정 규칙:

| `interaction:` 키 | 훅 필드 | 결과 | 도달하는 호출 |
| --- | --- | --- | --- |
| 예약어 아님 | — | 정상 등록 | `dva <name>` |
| 훅 가능 예약어 | `before`/`replace`/`after` 중 하나 이상 | 내장 커맨드를 감싸는 훅으로 동작 | `dva <name>` (내장이 훅을 실행) |
| 훅 가능 예약어 | 없음 (`command:`만) | **충돌** — `validate` 실패 | `dva run <name>` |
| 훅 불가 예약어 | 무관 | **충돌** — `validate` 실패 | `dva run <name>` |
| `compose:ps`처럼 `:` 앞이 예약어 | 무관 | **충돌** — `validate` 실패 | **없음** (아래 참조) |
| `mytool:fast`처럼 `:` 앞이 예약어가 아님 | — | 정상 등록 | `dva mytool:fast` |

즉 `build`처럼 **예약어이면서 훅 가능한** 이름은 `command:`로 재정의할 수 없고
`replace:`로만 대체할 수 있습니다.

충돌은 **경고가 아니라 에러**입니다 — `dva validate`(= `dva config validate`)가 exit 1로
실패합니다. 다만 `ls`·`manifest`·`run`은 같은 설정을 읽고도 종료 코드 0으로 동작하므로,
설정이 "무효인 상태로 실행 중"일 수 있습니다. 충돌 여부는 `dva validate`로만 확정됩니다.

선언이 버려지는 것은 아닙니다. 짧은 형식(`dva build`)만 내장 커맨드에게 넘어가고, 선언한 커맨드
자체는 `dva run build`로 그대로 실행됩니다. `dva ls`와 `dva manifest`는 충돌한 키를
계속 보여주되 도달 가능한 호출을 함께 표시합니다 — `manifest`의 경우
`usage_example: "dva run build"`와 `shadowed_by_builtin: "build"` 필드입니다.

`compose:ps`처럼 `:` 앞이 **예약어인** 경우만 예외로 **어떤 호출로도 도달할 수
없습니다**: 짧은 형식은 내장 커맨드가 아니고, `run` 형식은 `compose:`를 서브프로젝트
참조로 읽어 `subproject 'compose' not found`로 실패합니다. 구분자를 바꾸는 것
(`compose-ps`)이 유일한 해결책입니다.

이 경우 `manifest`는 위의 `shadowed_by_builtin`과 다른 필드를 씁니다 — 도달 가능한 호출이
아예 없으므로 `usage_example`은 **생략되고**, 대신 `unroutable: "compose"`(문제의 접두사)와
`unroutable_reason`(전체 설명)이 실립니다. `dva ls --json`도 같은 값을 노출하고,
사람이 읽는 `dva ls`는 `(unreachable: ...)` 표시를 붙입니다. `usage_example`이 없다는 것
자체가 신호입니다: 실행하면 반드시 실패하는 문자열을 제안하지 않기 위한 것입니다.

`subcommands:`를 가진 키도 마찬가지입니다 — `compose:ps fast`처럼 파생된 항목 역시
접두사가 죽어 있으므로 동일하게 표시됩니다.

**예약어가 아닌 접두사는 도달합니다.** `mytool:fast`는 선언된 키 그대로 조회되어
`dva mytool:fast`로 실행됩니다(선언이 추론을 이깁니다). 접두사와 같은 이름의 서브프로젝트가
실제로 있는 경우는 영향이 없습니다 — 부모가 `subprojects: {engine: ...}`를 선언했다면
`engine:test` 리터럴 키는 부모에 없고 자식 `dva.yml`에 있기 때문입니다.

이 규칙은 **살아 있는 예약어 집합**을 기준으로 판정합니다. 그래서 `app`이 내장 커맨드에서
빠진 지금 `app:build`는 unroutable이 아니라 평범한 interaction입니다 — 접두사가 더 이상
DVA가 소유한 이름이 아니기 때문입니다.

이름을 바꿀 때 `dva`가 제안하는 형태는 **콜론을 모두** 없앤 것입니다
(`compose:sub:cmd` → `compose-sub-cmd`). 콜론이 남아도 라우팅 자체는 되지만, 예약어 접두사가
사라지면서 `validate`가 붙여 주던 표시도 함께 사라지므로 제안대로 전부 없애는 편이
읽기에 명확합니다.

```yaml
interaction:
  build:                    # 예약어 + 훅 가능
    replace:                # command: 를 쓰면 충돌 → dva build 는 내장이 실행
      - step: "빌드"          # step: 은 라벨 — 실행할 명령은 run: 에 씁니다
        run: "make build"
    after:
      - step: "완료 알림"
        run: "echo built"

  my-build:                 # 예약어 아님 → 자유롭게 정의
    command: "make build"
```

실행 순서는 `before` → (`replace` 또는 내장 커맨드) → `after`입니다. 훅 스텝
안에서 `dva`를 다시 호출해도 재귀 가드가 걸려 안쪽 호출은 훅 없이 내장 커맨드만
실행합니다.

### interaction 실행 대상 (`service:` / `pod:`)

interaction이 **무엇을 대상으로** 실행되는지는 다음 필드로 고릅니다 (서로 배타적 권장).

| 필드 | 선택되는 러너 | 비고 |
|------|----------------|------|
| `service:` | Docker Compose | `docker compose exec/run <service> …` |
| `pod:` | kubectl | `kubectl exec` — 값 `name` 또는 `name:container` (`parsePod`) |
| (둘 다 없음) | local | 호스트에서 직접 실행 |

#### `pod:` 와 kubectl 실행 형태

`pod:`가 있으면 kubectl 러너가 쓰입니다. 그 안에서 실제 작업 선언은 다음 중 하나입니다
(우선순위: `steps` > `script_file` > `script` > `command` 리스트 > `command` 스칼라).

| 형태 | kubectl에서 하는 일 |
|------|---------------------|
| `command:` (스칼라) | `kubectl exec … -- <cmd>` (한 번, TTY 가능) |
| `command:` (리스트) | 줄마다 `kubectl exec` (실패 시 중단; 리스트는 TTY 없음) |
| `steps:` | 스텝마다 exec (라벨 있는 시퀀스) |
| `script:` / `script_file:` | **파드 안에서** `sh -c <body>` 로 실행. shebang은 따르지 **않음** — schema가 shell 스크립트로 문서화하고, 파드에 호스트 인터프리터 경로가 없을 수 있음. 호스트에서 shebang으로 돌리려면 local 러너를 쓰세요. |

**compose와의 차이:** `service:` + `script:`/`script_file:` 은 compose 러너가 네이티브로
지원하지 않아 **호스트 local 실행으로 폴백**합니다. 같은 YAML을 `pod:`로 바꾸면 스크립트는
클러스터 안에서 돌아가므로, 대상 파일시스템·DB가 달라집니다.

```yaml
interaction:
  rails-console:
    pod: web                    # 또는 web:app (컨테이너 지정)
    command: bundle exec rails console
  seed-in-pod:
    pod: web
    script: |
      set -e
      bundle exec rails db:seed
```

### interaction.subcommands (`default_args` 상속)

`subcommands:`의 자식은 부모의 필드를 물려받습니다. 대부분은 "자식이 선언하면 자식 값,
아니면 부모 값"이라는 단순한 규칙이지만 **`default_args`만 다릅니다** — 자식이 아무것도
선언하지 않아도 버려질 수 있습니다.

`default_args`는 *그것이 쓰여진 커맨드에 속한 인자*이기 때문입니다. 자식이 실행할 것을
다시 선언하면 부모의 인자는 그 커맨드가 요구한 적 없는 인자가 됩니다.

| 자식이 선언한 것 | `default_args` 결과 |
| --- | --- |
| `default_args:` | **자식 값**이 이깁니다 (부모 값은 버려짐) |
| `command:` (스칼라 또는 리스트) | **비워집니다** — 인자 없이 시작 |
| `script:` / `script_file:` / `steps:` | **비워집니다** — 위와 같은 이유 |
| 아무것도 없음 (`description:`만 있는 컨테이너) | **부모 값을 물려받습니다** (커맨드도 함께) |

마지막 줄이 그룹 하나가 인자 목록 하나를 공유하게 해 주는 규칙입니다.

```yaml
interaction:
  rails:
    command: "bundle exec rails"
    default_args: "-e development"
    subcommands:
      db:                              # 컨테이너 → `bundle exec rails -e development`
        subcommands:
          migrate:
            command: "db:migrate"      # 재선언 → `db:migrate` (인자 없음)
      console:
        command: "console"             # 재선언 → `console` (인자 없음)
      test:
        default_args: "-e test"        # 자식 값 → `bundle exec rails -e test`
      lint:
        script: "bundle exec rubocop"  # 재선언 → 인자 없음
```

확인은 `dva run <name> --explain`으로 합니다. `--json`을 붙이면 `arguments` 키에 같은
값이 실립니다. 이 키는 *실제로 전달될 인자*이며, 명령줄에 직접 쓴 인자가 아닙니다.

명령줄 인자를 직접 주면 `default_args`는 **덧붙지 않고 통째로 대체됩니다** —
`dva run rails test smoke`는 `bundle exec rails smoke`이지
`bundle exec rails -e test smoke`가 아닙니다.

단, `-`로 시작하는 인자는 `dva run` 자신의 플래그로 먼저 해석됩니다. `dva run rails test -p
4000`의 `-p`는 `--publish`로 먹히고 커맨드에는 아무것도 전달되지 않습니다 (`default_args`가
그대로 남습니다). 플래그처럼 생긴 인자를 넘길 때는 `--`로 끊습니다 —
`dva run rails test -- -p 4000`.

`default_args`를 실제로 소비하는 것은 단일 `command:` 실행 경로뿐입니다. `script:`,
`script_file:`, `steps:`로 실행되는 커맨드는 인자를 받지 않습니다 — 위 표에서 이들이
상속에서 제외되는 이유이기도 합니다.

> 여기서 말하는 상속은 **한 설정 파일 안에서 부모 노드 → 자식 노드** 방향입니다.
> 설정 레이어 사이(base ← modules ← subprojects)의 병합은 다른 축이며
> [docs/30-config-merge-semantics.md](docs/30-config-merge-semantics.md)가 다룹니다.

### subprojects

모노레포에서 서브프로젝트별 dva.yml을 참조합니다.
기본 연결 대상은 `plans`, `interactions`, 필요시 `provision`입니다.

```yaml
subprojects:
  backend:
    path: ./services/backend
    import:
      plans: [local-dev]
      interactions: [shell, logs]
      provision: [setup]
```

실행 이름은 canonical namespace를 사용합니다.

```bash
dva up backend/local-dev
dva run backend/shell
dva provision backend/setup
```

subproject의 `interaction`과 `provision`은 해당 subproject root 기준으로 실행됩니다.

### 특수 변수

| Variable | Description |
|----------|-------------|
| `DVA_OS` | 현재 OS (`linux`, `darwin`, `windows`) |
| `DVA_WORK_DIR_REL_PATH` | 작업 디렉토리 상대 경로 |
| `DVA_CURRENT_USER` | 현재 사용자명 (`username`) |
| `DVA_CURRENT_UID` | 현재 사용자 UID (숫자) |

### 설정 파일 로딩 순서

1. `DVA_FILE` 환경변수 (설정 시)
2. 현재 디렉토리에서 루트까지 `dva.yml` 탐색
3. `.sb/dva/*.yml` 모듈 병합
4. `dva.override.yml` 오버라이드 적용

## LLM Integration

DVA는 LLM 에이전트(Claude, Cursor 등)와의 통합을 위한 기능을 제공합니다.

- `am run dva-improve` — dva.yml AI 개선 (기존 파일 수정)
- `am run dva-improve param.mode=rewrite` — dva.yml AI 개선 (처음부터 재작성)
- `dva config docs` — CLAUDE.md/AGENTS.md 가이드 생성/갱신
- `am run dva-improve-guided` — Claude Code 대화형 가이드 모드
- `dva manifest` — 구조화된 커맨드 매니페스트 (JSON/YAML)
- `dva config show` — 병합된 최종 설정 출력
- `--json` 글로벌 플래그 — 모든 출력을 JSON으로
- `claude-plugin/` — Claude Code 플러그인

### 실행 디렉토리 요구사항

`dva-improve` 계열 플로우는 **대상 프로젝트 디렉토리에서 실행**해야 합니다.

```bash
cd /path/to/project
am run dva-improve            # target 기본값 "." — 정상
```

`target`을 다른 경로로 넘기면 플로우가 첫 쓰기 이전에 중단됩니다. agent-mesh는
`file`/`read_file` 경로를 **실행 디렉토리** 기준으로 해석하고 그 밖으로의 쓰기를
거부하는 반면 `shell` 스텝은 `cd`를 따르기 때문에, 두 경로가 갈리면 설정 백업이
기록되지 않은 채 에이전트가 설정을 수정하는 상태가 됩니다. 되돌릴 스냅샷이 없는
수정을 막기 위해 `check_run_dir` 가드가 실행 디렉토리와 target을 함께 출력하고
멈춥니다.

### 설정 백업과 복원

플로우는 설정을 수정하기 전에 `backups/dva/<이름>.<타임스탬프>.bak` 로 스냅샷을
남깁니다. git이 덮지 못하는 창 — 실행 시점의 **미커밋 로컬 수정** — 을 위한 것입니다.

```bash
ls -1 backups/dva/*.bak                       # 목록 (마지막 줄이 최근)
cp backups/dva/dva.yml.20260818-163000.bak dva.yml
dva validate
```

어느 스냅샷을 고를지, 스냅샷이 덮지 않는 변경, 보존 정리는
[docs/50-improve-flow-backup-and-restore.md](docs/50-improve-flow-backup-and-restore.md)
를 참조하세요.
