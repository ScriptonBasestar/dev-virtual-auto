---
id: TASK-120
title: "`output.PrintYAML` panics on a value it cannot marshal, so its `error` return can never carry that failure"
type: fix
priority: P3
effort: S
status: done
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/output/output.go:67 PrintYAML — yaml.Marshal panics rather than returning; four call sites in internal/cli"
verified-at: 2026-08-03T14:45:00+09:00
archived-at: 2026-08-03T14:45:00+09:00
verification-summary: |
  Deliverable is committed and clean: 90eb28c `fix(output): recover the panic PrintYAML used to let through`
  touches internal/output/output.go, output_test.go, and the task file; `git status --porcelain` on
  internal/output is empty.
  The panic-vs-error claim was exercised, not read: a standalone reconstruction of the pre-fix
  PrintYAML in the scratchpad (yaml.v3 v3.0.1, module cache) let `cannot marshal type: chan int`
  escape as a panic, confirming both the original defect and the mutation-kill recorded in the task.
  Current code returns an error for the same input and both subtests pass.
  Coverage re-measured from scratch: package 100.0%, PrintYAML 100.0% — the 75%/dead-`return err`
  gap is gone, and the coverage binding is non-vacuous (it would regress if the recover were removed).
  All four named call sites still `return` the printer's error and so propagate it:
  internal/cli/manifest.go:27, internal/cli/list.go:229, internal/cli/list.go:250,
  internal/cli/config_dump.go:27.
  No follow-up work is outstanding: `grep -rln "TASK-120\|PrintYAML" tasks/` matches only 120 and its
  parent 114 — nothing in tasks/todo (incl. 134..152), blocked, decision, or plan.
  One observation, not a defect: the recover is function-wide, so it also spans printDocument and any
  user MarshalYAML, and would relabel an unrelated runtime panic as `yaml marshal: ...`. That is
  inside the fail-fast trade-off the Resolution explicitly chose and documents; printDocument is three
  lines over os.Stdout and cannot realistically panic.
---

# Task 120: the error return that cannot report the error

Found while writing the tests for [TASK-114](../_archive/114-output-package-has-no-tests-and-drops-write-errors.md).
A test that fed the same unmarshalable value to both printers passed on the JSON side and
crashed the test binary on the YAML side.

## Measured

```
go test ./internal/output/ -run MarshalFailure -v
  --- FAIL: TestMarshalFailureLeavesStdoutClean (0.00s)
  panic: cannot marshal type: chan int [recovered, repanicked]
    gopkg.in/yaml.v3.(*encoder).marshal   encode.go:182
    gopkg.in/yaml.v3.Marshal              yaml.go:222
    …/internal/output.PrintYAML           output.go:68

go tool cover -func=cov.out
  PrintJSON    100.0%
  PrintYAML     75.0%      <- the uncovered statement is `return err`
```

The coverage gap is the defect. `PrintYAML`'s error branch sits at 75% while every other
function in the package reaches 100%, because the statement it guards is unreachable.

## Why

`gopkg.in/yaml.v3` raises a plain string panic for an unsupported kind
(`encode.go:182`: `panic("cannot marshal type: " + in.Type().String())`), and its own
recovery helper only converts its private error type back into a return value
(`yaml.go:288-296`):

```go
func handleErr(err *error) {
	if v := recover(); v != nil {
		if e, ok := v.(yamlError); ok {
			*err = e.err
		} else {
			panic(v)          // <- everything else keeps going up
		}
	}
}
```

So `yaml.Marshal` returns an error for a malformed document and terminates the process for
an unsupported type. `encoding/json` makes the opposite choice and returns
`UnsupportedTypeError` for the same input, which is why the two printers diverge.

## Reachability

Four call sites: `internal/cli/manifest.go:27`, `internal/cli/list.go:229`,
`internal/cli/list.go:250`, `internal/cli/config_dump.go:27`. The first three marshal
purpose-built structs. `config_dump` marshals a whole `*config.Config`, so its exposure is
whatever that type grows to hold — the risk is a future field, not a present one. Nothing
here is known to crash today; what is wrong today is that the signature promises a report
it cannot make.

## The decision this needs

Not mechanical, which is why it was not folded into TASK-114:

- **Recover and return.** Matches `PrintJSON`, makes the signature honest, and makes the
  75% statement reachable. Costs the fail-fast: a `chan` reaching a config dump is a
  programming error, and converting it into a returned error means it surfaces as a tidy
  CLI message in production instead of a loud crash in a test.
- **Leave the panic and drop the pretence.** Document that an unmarshalable value is a
  programming error and that the error return covers write failures only. Cheaper, and
  arguably the correct reading of what yaml.v3 intends — but it leaves the two printers
  behaving differently on identical input, which is the thing that caused this to be found
  in the first place.

Pick one before writing code. Whichever is chosen, `PrintJSON` and `PrintYAML` must
document the same contract.

## Acceptance criteria

- [x] The divergence is reproduced first | verify: `go test ./internal/output/ -run MarshalFailure -v` — a test feeding `make(chan int)` to both printers, recording which returns and which panics``
- [x] The chosen contract is implemented | verify: `go test ./internal/output/ -run 'MarshalFailure' -v`
- [x] Both printers document the same contract | verify: `human — read the two doc comments side by side; they must not disagree about what the error return covers`
- [x] No unreachable statement is left behind | verify: `go test ./internal/output/ -coverprofile=/tmp/c.out && go tool cover -func=/tmp/c.out` — PrintYAML must not sit below PrintJSON because of dead code`
- [x] Full suite passes | verify: `make test`

## Related

- [TASK-114](../_archive/114-output-package-has-no-tests-and-drops-write-errors.md) — added the
  first tests this package ever had, which is how this surfaced.

## Resolution

**Decision: recover and return (option A).** PrintYAML now wraps `yaml.Marshal` in a deferred
recover that converts the unsupported-kind panic into a returned error. The named return carries
it; the signature was already `error`.

The other choice — leave the panic, document that the error return covers writes only — was rejected
because the signature already promises an error and the two-call printer pair should not diverge on
identical input. TASK-114 made `output.PrintJSON` honest about write failures; TASK-121 carried that
honesty to `runner.Explain`. Letting PrintYAML terminate the process where PrintJSON returns
`UnsupportedTypeError` breaks that line at the cheapest point to keep it. The fail-fast this loses is
marginal: a `chan` reaching a config dump is a programming error, and a tidy message in production
beats a stack trace there. None of the four call sites is known to hit this today.

Both doc comments now open with the same sentence — the error return covers marshal failures and
write failures alike — and PrintYAML's adds the one library difference (yaml.v3 panics, its own
recover re-panics) that makes that non-trivial.

### Measured

- `go test ./internal/output/ -run MarshalFailure -v` → exit 0, both `PrintJSON` and `PrintYAML`
  subtests PASS (the YAML twin is now writable; before TASK-120 it crashed the binary)
- coverage: `PrintYAML` 75.0% → **100.0%**, package total **100.0%**. The dead statement was the
  `return err`; the recover made it reachable and `TestPrintYAMLReturnsMarshalError` (a
  `MarshalYAML` that returns a plain error) covers the non-panic marshal-error branch alongside the
  panic branch the channel drives.
- `make test` → exit 0, 6 packages ok; `internal/output` at 100.0%
- `make lint` → exit 0, 0 issues
- the two doc comments agree on the contract (criterion 3, human read)

### Mutation testing

One mutant, reverted and confirmed byte-identical with `diff -q`:

| mutant | killed by | what the failure said |
|---|---|---|
| PrintYAML reverted to the pre-TASK-120 panic version (recover removed, `fmt` import dropped) | `TestMarshalFailureLeavesStdoutClean/PrintYAML` | `--- PASS: PrintJSON` then `--- FAIL: PrintYAML` and `panic: cannot marshal type: chan int [recovered, repanicked]` — the exact defect this task was filed from |

The mutant is the whole function rather than a single line because `fmt.Errorf` is the file's only
use of `fmt`: a one-line mutation that dropped it left `fmt` imported-and-unused and failed at
build, which says nothing about the test. Reverting the whole function keeps the build honest and
lets the panic reach the assertion.
