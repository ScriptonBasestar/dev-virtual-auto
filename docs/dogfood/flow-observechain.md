# flow-observechain dva 적용 분석

## 현황
- 파일: `dva.yml` (223줄), `version: "0.1.44"`
- 섹션: `env_file(files)`, `stack`(compose 단일, runners 형식 + 서비스 tags), `plans`(4개), `default_plan`, `checks`, `suggestion_ignore`, `interaction`, `provision`, `subprojects`(4개, `import.interactions` 활용), `endpoints`
- `dva validate`: **✅ valid, warning 0건** — 그룹 A 중 유일하게 완전 클린.

## 문제점
- 현행 구조 기준 위반 없음. removed/legacy 섹션(`modes`, `applications`, flat stack) 미사용, replace hook 없음, default_plan 설정됨.
- 굳이 꼽으면: native 앱(core/portal/ai/admin)이 루트 stack 엔트리가 아니라 subprojects `import.interactions`의 `dev` 커맨드로만 노출됨 (140–160행). 헤더 주석(3행)이 "Native service lifecycle belongs to each subproject"라고 의도를 명시했으므로 위반이 아니라 설계 선택. 다만 루트에서 `dva up <infra+native>` 조합 plan은 불가능한 구조.

## dva 개선 힌트
- **subprojects `import.interactions`의 모범 사용례** — canonical 예제(examples/)에 이 패턴이 없으므로 예제 후보.
- "native lifecycle은 subproject 소유" 설계와 "루트 plan에 native entry 포함"(knowchain/pipechain 방식) 두 패턴이 devbox 사이에 갈림 — dva 문서가 어느 쪽을 권장하는지 가이드가 없음. devbox 패턴 가이드 문서화 여지.

## 마이그레이션 난이도
**하(작업 없음)** — 현행 구조에 완전 부합. 그룹 A의 레퍼런스 설정으로 삼을 만함.
