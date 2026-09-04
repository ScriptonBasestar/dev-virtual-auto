# Migration And Compatibility

기존 설정에서 새 구조로 옮기는 원칙과 호환 레이어.
선언/용어 배경은 [40-declarative-stack-and-plans.md](40-declarative-stack-and-plans.md)를 본다.
실행 계획·CLI는 [41-execution-plans-and-cli.md](41-execution-plans-and-cli.md)를 본다.

## 11. 마이그레이션 원칙

기존 구조에서 새 구조로 옮길 때의 기본 원칙:

- 기존 `stack` 엔트리는 최대한 유지
- `order`, `tags`, `services` 같은 실행 계획 성격 필드는 plan 쪽으로 이동
- 기존 `modes`는 삭제하거나 점진적으로 `plans` / `sites` / `environments`로 분해
- 기존 `applications`는 가능하면 `stack` 안의 runner 선언으로 통합

### 11-1. 기존 설정에서 사라지거나 이동하는 항목

아래 표는 기존 주요 설정이 새 구조에서 어떻게 처리되는지 정리한 것입니다.

| 기존 개념/키 | 새 구조 상태 | 설명 |
|---|---|---|
| `modes` | 제거 후 분해 | 기존 `mode`의 책임을 `plans`, `environments`, `sites`로 분리 |
| `applications` | 최상위 섹션 제거 | 앱 정의를 `stack` 내부 runner 선언으로 통합 |
| `stack.*.order` | `plans.*.entries[].order` 로 이동 | 실행 순서는 선언이 아니라 실행 계획 책임 |
| `stack.*.tags` | 축소 또는 선택적 메타데이터화 | 핵심 실행 제어 수단으로는 사용하지 않음 |
| `stack` 내부 compose 서비스 선택 | `plans.*.entries[].services` 로 이동 | compose 부분 선택은 선언이 아니라 계획 책임 |
| `mode` 기반 app strategy | `plan` / `site` 해석으로 이동 | `native` / `docker` / 기타 runner 선택을 mode에서 분리 |
| `environments.*.stack` | 제거 | environment는 stack 선택 책임을 갖지 않음 |
| `environments.*.stack_overrides` | 제거 또는 대폭 축소 | environment는 vars 중심으로 단순화 |
| `dva stack up/down/stop/status` | `dva up/down/stop/status <name>` 으로 대체 | 실행 대상은 stack이 아니라 named execution entry |

### 11-2. 유지되지만 의미가 바뀌는 항목

| 기존 개념/키 | 새 구조 상태 | 설명 |
|---|---|---|
| `stack` | 유지 | 실행 대상 집합이 아니라 선언 저장소 |
| `environments` | 유지 | stack 필터가 아니라 `vars` 중심 구성 |
| `subprojects` | 유지 | 여러 DVA 설정 공간을 연결하는 계층 |
| `interaction` | 유지 | 단발성 편의 명령 |
| `provision` | 유지 | 준비/초기화 절차 |

## 12. Compatibility Layers

새 구조에서도 기존의 핵심 편의 요소는 유지해야 합니다.
다만 실행 모델과 동일 레이어에 두지 않고, 보조 레이어로 분리해서 다뤄야 합니다.

### 12-1. subprojects

`subprojects`는 계속 유지할 가치가 있습니다.
역할은 "여러 DVA 설정 공간을 연결하는 네임스페이스/집계 계층"입니다.

즉 `subprojects`는 실행 계획 자체를 정의하기보다, 여러 프로젝트의 `stack` / `plans` / `interactions`를 연결하는 구조입니다.

예상 활용:

- 상위 프로젝트에서 하위 프로젝트의 실행 가능한 이름을 함께 노출
- 특정 subproject의 실행 계획만 대상으로 실행
- `backend/local-dev` 같은 qualified name 지원

기본 연결 대상:

- `plans`
- `interactions`

선택 연결 대상:

- `provision`
- `stack` 조회용 노출

권장 원칙:

- `subprojects`는 설정 공간 분리와 재사용을 담당
- 실행 모델 자체는 각 subproject 내부에서 동일한 구조를 따름
- 기본 canonical name은 항상 `subproject/name` 형식을 사용
- parent top-level로 자동 flatten 하지 않음
- alias는 선택 사항으로만 허용
- 이름 충돌은 자동 해결하지 않고 hard error로 처리
- subproject 선언은 미리 둘 수 있지만, non-empty `import` 대상이 있거나 직접 실행할 때는 해당 subproject의 `dva.yml`이 존재해야 함

예:

```yaml
subprojects:
  backend:
    path: services/backend
    import:
      plans: [local-dev]
      interactions: [shell, logs]
      provision: [setup]
```

parent에서 노출되는 canonical name:

- `backend/local-dev`
- `backend/shell`
- `backend/logs`
- `backend/setup`

필요하면 alias를 명시적으로 둘 수 있습니다.

```yaml
subprojects:
  backend:
    path: services/backend
    import:
      plans:
        - name: local-dev
          as: backend-dev
```

이 경우에도 canonical name은 여전히 `backend/local-dev` 이고, `backend-dev` 는 추가 alias일 뿐입니다.

#### Subproject execution path

Imported 항목은 subproject 설정 디렉터리를 기준으로 실행합니다.

즉:

- command resolution 기준 = subproject config
- relative path 기준 = subproject config dir
- default working directory = subproject root

이 규칙이 필요한 이유:

- subproject 내부 상대 경로가 parent 위치에 영향을 받지 않도록 하기 위해
- subproject가 독립적으로도 동일하게 실행되도록 하기 위해
- parent에서 import해도 동작 의미가 바뀌지 않도록 하기 위해

세부 owner·path 계약은 [resolution 문서](31-execution-plan-resolution.md#6-subproject-resolution)가 소유합니다.

예를 들어 `services/backend/dva.yml` 에 정의된:

```yaml
interactions:
  shell:
    command: ./scripts/dev-shell.sh
```

이를 parent에서 `dva run backend/shell` 로 실행해도, 실제 실행 기준은 `services/backend` 여야 합니다.

`provision`도 동일합니다.
`dva provision backend/setup` 이 호출되면, 해당 provision step들의 상대 경로와 기본 실행 위치는 subproject root를 기준으로 해석합니다.

예외적으로 parent 기준 실행이 필요하다면, 명시적 옵션이나 절대 경로를 사용해야 합니다.

### 12-2. interactions

`interactions`도 유지해야 합니다.
이는 실행 계획이 아니라, 단발성 편의 명령 레이어입니다.

예:

- `dva shell`
- `dva logs`
- `dva db:migrate`
- `dva test`
- `dva k8s`

권장 원칙:

- `plans`는 장기 실행과 오케스트레이션 담당
- `interactions`는 단발성 작업과 접속/운영 편의 담당
- `up` / `down` / `stop` / `status` 같은 생명주기 명령은 `interactions`로 우회하지 않음

즉:

- 실행은 `plans`
- 작업은 `interactions`

### 12-3. provision

`provision`은 계속 필요합니다.
이는 실행 모델이 아니라 "환경 준비와 초기화" 레이어입니다.

예:

- 개발 도구 설치 확인
- 인증/로그인 준비
- 디렉토리/볼륨 초기화
- 초기 데이터 seed
- 클러스터 시작 전 선행 작업

권장 원칙:

- `provision`은 `up`의 대체가 아님
- `provision`은 실행 전에 필요한 반복 가능한 준비 절차
- `plans`가 장기 실행 환경을 올리고 내리는 역할을 담당한다면, `provision`은 그 전에 필요한 준비를 담당

### 12-4. 계층 요약

새 구조에서 각 요소의 역할은 아래와 같이 구분합니다.

- `stack`: 재사용 가능한 실행 대상 선언
- `plans`: 실제 실행 가능한 조합
- `environments`: dev/stg/prd 같은 환경 차이
- `sites`: local/office/remote/cloud 같은 실행 host 차이
- `subprojects`: 여러 DVA 설정 공간 연결
- `interactions`: 단발성 편의 명령
- `provision`: 환경 준비/초기화 절차

### 12-5. 설정 변환기

`dva config migrate`는 이전 선언을 `stack`/`plans` 형태로 옮기는 opt-in
호환 레이어다.
`dva config migrate`는 다음의 기계적으로 판단 가능한 변환만 수행한다.

- legacy compose 선언을 `stack.<name>.runners.compose`로 이동
- `applications` 선언을 native runner를 가진 `stack` 선언으로 이동
- `stack.*.order`를 해당 선언을 참조하는 `plans.*.entries[].order`로 이동
- stack 선택만 하는 mode를 같은 이름의 plan으로 이동

불가능한 선언은 `Left for you` 아래에 남긴다. 그 밖의 `modes`는 사유와 대상 위치만 보고한다.
`stack.*.order`는 참조 plan entry가 없으면 삭제하지 않는다. plan 없는 설정에서는 이
값이 기존 stack 실행 순서를 결정하므로, 사람이 named plan과 해당 entries를 선언한 뒤
다시 `dva config migrate`를 실행해야 한다. 이는 수동 한계다. 이 경우
`Left for you`는 변환 실패나 자동 수정 대상이 아니다.

이 설명은 TASK-007의 legacy command/section deprecation 정책 중 변환기 부분을
충족한다. 변환기는 영구 호환 약속이 아니며, 다음을 모두 만족하면 제거 후보가 된다.

1. 각 릴리스 후보에서 repository와 유지보수자가 관리하는 실제 설정 코퍼스의 모든
   설정에 `dva config migrate` preview를 실행한다(`--write` 금지).
2. 두 연속 릴리스 후보의 sweep에서 `Converted:` 항목 수가 모두 0이다.
3. 같은 두 sweep에서 `Left for you` 항목은 위의 plan 없는 `stack.*.order` 수동 전환
   한계로 분류되어 기록된 것뿐이며, `modes`, `applications` 또는 plan이 있는 설정의
   order처럼 변환기가 안내할 다른 legacy 선언은 0이다.

각 sweep의 총 설정 수, `Converted:` 수, `Left for you` 수, plan 없는 order 예외 수를
release 기록에 남긴다. 이 yes/no predicate가 두 번 연속 참이면 다음 릴리스에서
`dva config migrate`를 제거할 수 있다. 그 전에는 경고의 탈출 경로로 유지한다.

## 13. 결정 사항 요약

- `stack`은 유지한다.
- `stack`은 선언 저장소다.
- `stack` 엔트리는 multi-runner logical unit이 될 수 있다.
- 실행 명령의 대상은 named execution entry다.
- 문서에서는 이를 `plans`로 부르지만, CLI에는 굳이 드러내지 않는다.
- `profiles`는 도입하지 않는다.
- `environments`는 환경 변수와 용도 차이 담당이다.
- `sites`는 host 기준 실행 위치 담당이다.
- `env_file`은 유지한다.
- `compose`는 여러 파일과 여러 서비스를 함께 다루는 묶음 단위로 지원한다.
- runner 다양성은 유지하되, 선언 레이어와 계획 레이어를 분리한다.
- `subprojects`, `interactions`, `provision`은 유지하되 실행 모델과 분리된 보조 레이어로 취급한다.
