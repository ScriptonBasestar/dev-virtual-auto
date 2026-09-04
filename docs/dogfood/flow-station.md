# flow-station dva 도입 적합성 분석 (2026-09-05, 읽기 전용)

## 현황
- 구조: devbox 루트(docs/, tasks/, draft/, SOUL/PRODUCT) + Git 하위 저장소 `flow-station/` 1개 (Go, `cmd/flow` 단일 바이너리).
- compose 파일: 없음. Dockerfile: 없음. PORT_MAPPINGS: 없음. mise.toml: 없음.
- 루트 Makefile: `prepare`/`status`/`validate` 3개 — 전부 `gz-git workspace` 래퍼. 하위 Makefile: `build`(bin/flow), `test`(go test + vet).
- 개발 시작 문서: README "make prepare → make status". 서버/포트 언급 없음(CLI 도구).

## dva 도입 적합성
**native-only(경량) 또는 도입 불필요.** 상주 프로세스도 인프라도 없다. dva가 줄 수 있는 것은 하위 저장소 build/test를 루트에서 부르는 interaction 매핑뿐이며, 이는 `make -C flow-station`으로도 충분하다. 향후 `flow serve` 같은 데몬이 생기면 native stack 엔트리 1개로 승격.

## 제안 dva.yml 골격
```yaml
version: "0.1.48"
interaction:
  build: {description: "Build flow binary", runner: local, command: "make -C flow-station build"}
  test:  {description: "go test + vet", runner: local, command: "make -C flow-station test"}
subprojects:
  flow-station: {path: flow-station}
```
stack/plans 없음(선언할 프로세스가 없다).

## init 생성기 결과 vs 골격
- `dva init --dry-run` (0.1.48, 루트 파일만 복사한 scratch 사본에서 실행): `ERROR: no Docker Compose file detected in .; dva.yml was not created` + "no recognized language manifest" — exit 1, 파일 미생성. `--recursive`도 동일 메시지로 즉시 종료.
- 하위 `flow-station/go.mod`가 있어도 루트에서 인식하지 않는다. 골격과의 격차: init은 0줄, 골격은 interaction 2개 + subproject 1개 (TASK-249 증거).

## 도입 난이도
**하** — 그러나 효용도 낮음. 우선순위 최하.
