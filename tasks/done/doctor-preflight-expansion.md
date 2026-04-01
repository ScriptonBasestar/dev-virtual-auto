# Doctor Preflight Expansion

## 배경

현재 `dva doctor`는 Docker 접근성, compose 파일 존재, devcontainer, `.gitignore` 중심입니다.
하지만 실제 사용자 실패는 실행 직전의 preflight 체크 부족에서 자주 발생합니다.

## 현재 상태

- doctor 구현: `internal/cli/doctor.go`
- 관련 테스트: `internal/cli/doctor_test.go`
- 참고 가능한 config 검사:
  - `internal/config/validate.go`
  - `internal/config/validate_warnings.go`

## 문제

- `doctor`가 stack/mode/env 관련 사전 문제를 충분히 포착하지 못합니다.
- 외부 바이너리 존재 여부나 health check 설정 이상 같은 실행 전 문제를 한 번에 보기 어렵습니다.
- 사용자가 `doctor` 결과만으로 "바로 실행 가능한 상태인지" 판단하기 어렵습니다.

## 이번 작업의 목표

`doctor`를 실행 전 preflight 진단 도구로 확장할 수 있도록, 어떤 체크를 추가할지와 어떤 체크가 `doctor`에 적합한지를 정리합니다.

## 범위

- doctor에 넣을 preflight 체크 후보 정리
- 현재 built-in check와 추가 후보의 우선순위 정리
- 테스트 관점에서 검증 가능한 후보 추리기

## 제외

- validation severity 정책 전면 재설계
- plugin별 상세 health 구현
- UI 전면 개편

## 참조 정보

- `internal/cli/doctor.go`
- `internal/cli/doctor_test.go`
- `internal/config/validate.go`
- `internal/config/validate_warnings.go`
- 연관 태스크: `tasks/done/validation-severity-policy.md`

## 완료 조건

- `doctor`에 추가할 preflight 체크 후보가 우선순위와 함께 정리되어 있다.
- 각 후보가 doctor에 적합한지, warning/validate 쪽이 적합한지 구분되어 있다.
- 빠르게 구현 가능한 항목과 추가 설계가 필요한 항목이 구분되어 있다.
- 테스트 보강 대상이 정리되어 있다.

## 체크리스트

- [x] mode/env/stack 관련 사전 진단 후보가 있다.
- [x] external binary / compose name / health check 관련 후보가 있다.
- [x] doctor에 둘 항목과 다른 계층으로 보낼 항목이 구분되어 있다.
- [x] 테스트 가능한 범위가 적혀 있다.
- [x] 후속 구현 순서가 적혀 있다.

---

# Doctor Preflight 체크 후보

## 현재 Built-in Checks

| 체크 | 설명 | 파일 |
|------|------|------|
| Docker daemon 접근 가능 | docker info 실행 가능 확인 | doctor.go:208-222 |
| Compose 파일 존재 | compose.files에 지정된 파일 존재 확인 | doctor.go:70-82 |
| 사용자 정의 checks | dva.yml `checks` 섹션 실행 | doctor.go:84-87 |
| devcontainer.json 존재 | devcontainer 설정 시 파일 확인 | doctor.go:89-97 |
| .gitignore 상태 | .sb/dva/가 제외되었는지 확인 | doctor.go:99-156 |

---

## 우선순위별 Preflight 체크 후보

### 우선순위 1: 실행 차단 가능성 높음 (즉시 구현 권장)

| 순위 | 체크 이름 | 설명 | Doctor 적합성 | 구현 복잡도 |
|------|-----------|------|---------------|-------------|
| 1 | Compose project name alignment | dva.yml project_name와 compose 파일 name: 일치 | ⚠️ 중복 고려 (validate에도 있음) | 간단 |
| 2 | Environment file 존재 | env_file에 지정된 파일 존재 확인 | ✅ 적합 (외부 상태) | 간단 |
| 3 | Stack entry file 존재 | stack.*.files에 지정된 경로 확인 | ✅ 적합 (외부 상태) | 간단 |
| 4 | External binary 존재 | provision/interaction.runner에서 참조하는 바이너리 확인 | ✅ 적합 (외부 상태) | 중간 |
| 5 | Default mode 유효성 | default_mode가 modes에 존재하는지 확인 | ❌ 하드 에러 (validate에 있음) | - |

---

### 우선순위 2: UX/성능 개선 (단계적 구현 권장)

| 순위 | 체크 이름 | 설명 | Doctor 적합성 | 구현 복잡도 |
|------|-----------|------|---------------|-------------|
| 6 | Mode compose_services 존재 | mode.*.compose_services에 지정된 service가 compose에 있는지 | ⚠️ 중복 고려 (drift warning에 있음) | 중간 |
| 7 | Health check endpoint 접근 가능 | health_checks.*.URL에 지정된 엔드포인트 접근 가능 확인 | ✅ 적합 (외부 상태) | 중간 |
| 8 | Docker socket 권한 | docker.sock 읽기/쓰기 권한 확인 | ✅ 적합 (외부 상태) | 간단 |
| 9 | Devcontainer 설정 완전성 | .devcontainer/devcontainer.json 필수 필드 확인 | ✅ 적합 (외부 상태) | 중간 |
| 10 | Port 충돌 감지 | interaction/service에서 사용하는 포트 충돌 확인 | ✅ 적합 (외부 상태) | 복잡 |

---

### 우선순위 3: 향후 고려 (설계 필요)

| 순위 | 체크 이름 | 설명 | Doctor 적합성 | 구현 복잡도 |
|------|-----------|------|---------------|-------------|
| 11 | Resource 사용량 예상 | docker stats 기반 리소스 사용량 확인 | ✅ 적합 (외부 상태) | 복잡 |
| 12 | Docker 버전 호환성 | Docker 버전이 DVA 요구사항 충족하는지 확인 | ✅ 적합 (외부 상태) | 간단 |
| 13 | Compose service 상태 | docker compose ps 기반 서비스 실행 상태 확인 | ✅ 적합 (외부 상태) | 중간 |
| 14 | Network 설정 확인 | Docker network 연결성 확인 | ✅ 적합 (외부 상태) | 복잡 |
| 15 | Disk 공간 확인 | Docker 사용 가능한 디스크 공간 확인 | ✅ 적합 (외부 상태) | 간단 |

---

## 상세 분석

### 1. Compose project name alignment

**현재 상태**: validate에서 `[warn]`으로 출력됨

**Doctor 추가 제안**:
```go
// cli/doctor.go에 추가
func checkComposeProjectNameAlignment(c *config.Config) DoctorResult {
    warnings := c.ValidateComposeProjectNames()
    if len(warnings) == 0 {
        return DoctorResult{
            Name:   "Compose project name alignment",
            Passed:  true,
        }
    }

    w := warnings[0] // 첫 번째 경고만 표시
    msg := fmt.Sprintf("Compose file %s has %s", w.File,
        map[bool]string{
            true:  "missing project name",
            false: fmt.Sprintf("name '%s'", w.ComposeName),
        }[w.ComposeName == ""])

    return DoctorResult{
        Name:    msg,
        Passed:   false,
        FixHint:  fmt.Sprintf("Set 'name: %s' in %s", w.DvaName, w.File),
        Fixable:  true,
        fixFunc: func() error { return c.FixComposeProjectName(w) },
    }
}
```

**재사용**: `config.ValidateComposeProjectNames()` 이미 존재

**추가 필요**: `runDoctorChecks()`에 호출 추가

---

### 2. Environment file 존재

**목적**: env_file에 지정된 파일이 실제로 존재하는지 확인

```go
func checkEnvFiles(c *config.Config) []DoctorResult {
    var results []DoctorResult
    cfgDir := c.FileDir()

    for _, envFile := range c.EnvFile {
        if envFile == "" {
            continue
        }
        path := envFile
        if !filepath.IsAbs(path) {
            path = filepath.Join(cfgDir, envFile)
        }

        passed := fileExists(path)
        result := DoctorResult{
            Name:    fmt.Sprintf("Environment file exists: %s", envFile),
            Passed:  passed,
            FixHint: condStr(!passed, fmt.Sprintf("Create or check path: %s", envFile)),
        }
        results = append(results, result)
    }

    return results
}
```

---

### 3. Stack entry file 존재

**목적**: stack.*.files에 지정된 경로 확인

```go
func checkStackFiles(c *config.Config) []DoctorResult {
    var results []DoctorResult
    cfgDir := c.FileDir()

    for name, entry := range c.Stack {
        var files []string
        if entry.Compose != nil {
            files = entry.Compose.Files
        } else if entry.Kubectl != nil {
            files = []string{entry.Kubectl.Kubeconfig}
        }

        for _, f := range files {
            if f == "" {
                continue
            }
            path := f
            if !filepath.IsAbs(path) {
                path = filepath.Join(cfgDir, f)
            }

            passed := fileExists(path)
            result := DoctorResult{
                Name:    fmt.Sprintf("Stack file exists: %s (%s)", name, f),
                Passed:  passed,
                FixHint: condStr(!passed, fmt.Sprintf("Create or check path: %s", f)),
            }
            results = append(results, result)
        }
    }

    return results
}
```

---

### 4. External binary 존재

**목적**: provision/interaction.runner에서 참조하는 바이너리 확인

**범위 제한**: runner: local/shell에서 command 실행 시 필요한 바이너리만 체크

```go
func checkExternalBinaries(c *config.Config) []DoctorResult {
    var results []DoctorResult

    // provision 단계의 바이너리 체크
    for name, provision := range c.Provision {
        for _, step := range provision.Steps {
            if step.Run == "" {
                continue
            }

            // 첫 번째 토큰이 바이너리인지 확인
            parts := strings.Fields(step.Run)
            if len(parts) == 0 {
                continue
            }

            binary := filepath.Base(parts[0])
            // 경로 포함인 경우 절대 경로로 사용
            if strings.Contains(parts[0], "/") || strings.Contains(parts[0], "\\") {
                binary = parts[0]
            }

            _, err := exec.LookPath(binary)
            result := DoctorResult{
                Name:    fmt.Sprintf("Provision binary exists: %s (%s)", binary, name),
                Passed:  err == nil,
                FixHint: condStr(err != nil, fmt.Sprintf("Install %s or verify PATH", binary)),
            }
            results = append(results, result)
        }
    }

    return results
}
```

**주의**: 모든 바이너리 체크는 과잉일 수 있음. 자주 사용되는 바이너리만 체크하거나 allowlist 사용 고려

---

### 5. Default mode 유효성

**분석**: 이미 validate.go:95-107에서 Hard Error로 처리됨

**결정**: Doctor에 추가하지 않음 (validate에서 이미 차단)

---

### 6. Mode compose_services 존재

**현재 상태**: validate.go:129-140에서 drift warning으로 처리됨

**Doctor 추가 제안**: mode.*.compose_services에 지정된 service 확인

```go
func checkModeComposeServices(c *config.Config) []DoctorResult {
    var results []DoctorResult
    cfgDir := c.FileDir()

    availableServices := configuredComposeServices(c)
    if len(availableServices) == 0 {
        return nil
    }

    for modeName, mode := range c.Modes {
        if mode.ComposeServices == nil {
            continue
        }

        for _, svc := range *mode.ComposeServices {
            if !availableServices[svc] {
                result := DoctorResult{
                    Name:    fmt.Sprintf("Mode service exists: %s in mode %s", svc, modeName),
                    Passed:   false,
                    FixHint:  fmt.Sprintf("Add service %s to compose files or remove from mode", svc),
                }
                results = append(results, result)
            }
        }
    }

    return results
}
```

**중복 고려**: validate.go의 drift warning과 중복 가능성. Doctor는 "실행 전 확인" 관점에서 추가

---

### 7. Health check endpoint 접근 가능

**목적**: health_checks.*.URL에 지정된 엔드포인트 접근 가능 확인

```go
func checkHealthCheckEndpoints(c *config.Config) []DoctorResult {
    var results []DoctorResult

    // Top-level health checks
    for name, hc := range c.HealthChecks {
        if hc.URL == "" || hc.Type != "http" {
            continue
        }

        result := checkHTTPEndpoint(name, hc.URL)
        results = append(results, result)
    }

    // Stack-nested health checks
    for entryName, entry := range c.Stack {
        for hcName, hc := range entry.HealthChecks {
            if hc.URL == "" || hc.Type != "http" {
                continue
            }

            result := checkHTTPEndpoint(fmt.Sprintf("%s.%s", entryName, hcName), hc.URL)
            results = append(results, result)
        }
    }

    return results
}

func checkHTTPEndpoint(name, url string) DoctorResult {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return DoctorResult{
            Name:    fmt.Sprintf("Health check endpoint accessible: %s", name),
            Passed:   false,
            FixHint:  fmt.Sprintf("Invalid URL: %s", url),
        }
    }

    client := &http.Client{Timeout: 5 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return DoctorResult{
            Name:    fmt.Sprintf("Health check endpoint accessible: %s", name),
            Passed:   false,
            FixHint:  fmt.Sprintf("Service not running or endpoint %s not accessible", url),
        }
    }
    defer resp.Body.Close()

    return DoctorResult{
        Name:   fmt.Sprintf("Health check endpoint accessible: %s", name),
        Passed: resp.StatusCode < 500,
        FixHint: condStr(resp.StatusCode >= 500, fmt.Sprintf("Endpoint returning %d", resp.StatusCode)),
    }
}
```

**주의**: 서비스가 실행 중이 아니면 실패 가능. 이것은 "preflight" 관점에서 실패로 간주

---

### 8. Docker socket 권한

**목적**: docker.sock 읽기/쓰기 권한 확인

```go
func checkDockerSocketPermissions() DoctorResult {
    sockPath := "/var/run/docker.sock"
    info, err := os.Stat(sockPath)
    if err != nil {
        return DoctorResult{
            Name:    "Docker socket accessible",
            Passed:   false,
            FixHint:  "Docker not running or socket path incorrect",
        }
    }

    // 현재 사용자가 소켓을 열 수 있는지 확인
    f, err := os.Open(sockPath)
    if err != nil {
        return DoctorResult{
            Name:    "Docker socket permissions",
            Passed:   false,
            FixHint:  "Add user to docker group or use sudo",
        }
    }
    f.Close()

    return DoctorResult{
        Name:   "Docker socket permissions",
        Passed:  true,
    }
}
```

---

### 9. Devcontainer 설정 완전성

**목적**: .devcontainer/devcontainer.json 필수 필드 확인

```go
func checkDevcontainerCompleteness(c *config.Config) DoctorResult {
    if len(c.Devcontainer) == 0 {
        return DoctorResult{
            Name:   "Devcontainer not configured",
            Passed:  true,
        }
    }

    cfgDir := c.FileDir()
    dcPath := filepath.Join(cfgDir, ".devcontainer", "devcontainer.json")

    data, err := os.ReadFile(dcPath)
    if err != nil {
        return DoctorResult{
            Name:    "Devcontainer file readable",
            Passed:   false,
            FixHint:  "Run: dva add devcontainer",
        }
    }

    var dc map[string]any
    if err := json.Unmarshal(data, &dc); err != nil {
        return DoctorResult{
            Name:    "Devcontainer JSON valid",
            Passed:   false,
            FixHint:  "Invalid JSON in devcontainer.json",
        }
    }

    // 필수 필드 확인
    requiredFields := []string{"image", "build"}
    var missing []string
    for _, field := range requiredFields {
        if _, ok := dc[field]; !ok {
            missing = append(missing, field)
        }
    }

    if len(missing) > 0 {
        return DoctorResult{
            Name:    "Devcontainer required fields",
            Passed:   false,
            FixHint:  fmt.Sprintf("Missing fields: %s", strings.Join(missing, ", ")),
        }
    }

    return DoctorResult{
        Name:   "Devcontainer completeness",
        Passed:  true,
    }
}
```

---

### 10. Port 충돌 감지

**목적**: interaction/service에서 사용하는 포트 충돌 확인

**복잡도**: 높음 (compose 파일 파싱 필요)

```go
func checkPortConflicts(c *config.Config) []DoctorResult {
    cfgDir := c.FileDir()
    portMap := make(map[int][]string)

    // compose 파일에서 포트 추출
    for _, composeFile := range c.AllComposeFiles() {
        path := composeFile
        if !filepath.IsAbs(path) {
            path = filepath.Join(cfgDir, path)
        }

        services, err := extractComposeServicesAndPorts(path)
        if err != nil {
            continue
        }

        for svcName, ports := range services {
            for _, port := range ports {
                portMap[port] = append(portMap[port], svcName)
            }
        }
    }

    // 충돌 감지
    var results []DoctorResult
    for port, services := range portMap {
        if len(services) > 1 {
            result := DoctorResult{
                Name:    fmt.Sprintf("Port conflict: port %d", port),
                Passed:   false,
                FixHint:  fmt.Sprintf("Services %s use the same port %d", strings.Join(services, ", "), port),
            }
            results = append(results, result)
        }
    }

    return results
}

func extractComposeServicesAndPorts(path string) (map[string][]int, error) {
    // compose 파일 YAML 파싱
    // services.*.ports 추출
    // "80:8080" 형식 또는 "8080:80" 형식 고려
    // TODO: 구현 필요
    return nil, nil
}
```

**추가 설계 필요**: compose 파일 포트 파싱 로직 구현

---

## Doctor vs Validate/Warning 경계 결정

| 체크 후보 | 현재 위치 | Doctor 적합성 | 결정 | 이유 |
|-----------|----------|---------------|------|------|
| Compose project name | validate (warning) | ⚠️ | Doctor에 추가 | 실행 전 환경 확인 |
| Environment file 존재 | 없음 | ✅ | Doctor에 추가 | 외부 파일 상태 |
| Stack entry file 존재 | 없음 | ✅ | Doctor에 추가 | 외부 파일 상태 |
| External binary 존재 | 없음 | ✅ | Doctor에 추가 | 외부 환경 상태 |
| Default mode 유효성 | validate (hard error) | ❌ | 유지 (validate) | dva.yml 내부 구조 |
| Mode compose_services 존재 | validate (drift warning) | ⚠️ | Doctor에 추가 고려 | 실행 전 확인 강화 |
| Health check endpoint | 없음 | ✅ | Doctor에 추가 | 외부 네트워크 상태 |
| Docker socket 권한 | 없음 | ✅ | Doctor에 추가 | 외부 환경 상태 |
| Devcontainer 완전성 | 없음 | ✅ | Doctor에 추가 | 외부 파일 상태 |
| Port 충돌 감지 | 없음 | ✅ | Doctor에 추가 | 외부 실행 상태 |

**중복 관리 전략**:
- **Validate**: dva.yml 내부 구조 검증 유지
- **Doctor**: 외부 상태 확인 강화 (환경 프리플라이트)
- **재사용**: 공통 함수를 validate에서도 호출 가능 (예: ValidateComposeProjectNames)

---

## 구현 복잡도 분류

### 간단 (즉시 구현 가능)

| 체크 | 이유 |
|------|------|
| Compose project name alignment | 공통 함수 이미 존재 |
| Environment file 존재 | fileExists() 사용 |
| Stack entry file 존재 | fileExists() 사용 |
| Docker socket 권한 | os.Stat() + os.Open() |
| Docker 버전 호환성 | docker version 파싱 |
| Disk 공간 확인 | df 명령 실행 |

### 중간 (약간의 추가 설계 필요)

| 체크 | 이유 |
|------|------|
| External binary 존재 | provision/interaction 파싱 필요 |
| Mode compose_services 존재 | availableServices 이미 존재 |
| Health check endpoint 접근 가능 | HTTP request 처리 |
| Devcontainer 완전성 | JSON 파싱 + 필드 확인 |
| Compose service 상태 | docker compose ps 파싱 |

### 복잡 (설계 후 구현)

| 체크 | 이유 |
|------|------|
| Port 충돌 감지 | compose 파일 포트 파싱 로직 필요 |
| Resource 사용량 예상 | docker stats 파싱 + 계산 |
| Network 설정 확인 | 네트워크 연결성 테스트 로직 |

---

## 테스트 보강 대상

### 신규 테스트 필요

| 테스트 | 설명 |
|--------|------|
| TestCheckComposeProjectNameAlignment | compose project name 체크 |
| TestCheckEnvFiles | env_file 존재 체크 |
| TestCheckStackFiles | stack entry file 존재 체크 |
| TestCheckExternalBinaries | provision 바이너리 체크 |
| TestCheckModeComposeServices | mode compose_services 체크 |
| TestCheckHealthCheckEndpoints | HTTP health check endpoint 체크 |
| TestCheckDockerSocketPermissions | docker.sock 권한 체크 |
| TestCheckDevcontainerCompleteness | devcontainer 필수 필드 체크 |

### 기존 테스트 확장

| 기존 테스트 | 확장 필요 |
|-----------|----------|
| TestRunDoctorChecks_AllPass | 새 체크 추가 후 통합 테스트 |
| TestApplyDoctorFixes | fixFunc 호출 순서 확인 |

---

## 후속 구현 순서

### 단계 1: 우선순위 1 - 간단한 체크 (이번 태스크 범위)
- [ ] Compose project name alignment 추가
- [ ] Environment file 존재 체크 추가
- [ ] Stack entry file 존재 체크 추가
- [ ] Docker socket 권한 체크 추가

### 단계 2: 우선순위 2 - 중간 복잡도 체크
- [ ] External binary 존재 체크 추가
- [ ] Mode compose_services 존재 체크 추가
- [ ] Health check endpoint 접근 가능 체크 추가
- [ ] Devcontainer 완전성 체크 추가

### 단계 3: 우선순위 3 - 복잡한 체크
- [ ] Port 충돌 감지 (추가 설계 후 구현)
- [ ] Resource 사용량 예상 (추가 설계 후 구현)
- [ ] Network 설정 확인 (추가 설계 후 구현)

### 단계 4: 테스트 보강
- [ ] 각 새 체크별 테스트 추가
- [ ] 통합 테스트 확장
- [ ] fixture 파일 추가 (필요 시)

### 단계 5: 문서 반영
- [ ] README.md에 doctor 체크 목록 추가
- [ ] 사용자 가이드: "dva doctor로 문제 진단하기" 추가
- [ ] 개발자 가이드: 새 doctor 체크 추가 방법 추가

---

## 결론

이 문서는 `dva doctor`를 확장하기 위한 preflight 체크 후보를 정리합니다. 우선순위와 구현 복잡도를 고려하여 단계적으로 구현을 진행하세요.

**첫 번째 구현 단계 (단계 1)**는 간단한 체크 위주로, 사용자가 자주 겪는 실행 전 문제를 빠르게 포착하는 데 중점을 둡니다.

이 확장은 사용자가 `dva doctor` 실행 후 "바로 실행 가능한 상태인지" 명확히 판단할 수 있도록 돕습니다.
