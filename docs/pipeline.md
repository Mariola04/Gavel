# Evaluation pipeline

[← Back to the README](../README.md)

What happens to a submission: the static analysis (stages 1–8), the dynamic analysis (stage 9) and how results are compared.

![Gavel evaluation flow](Gavel-Flow.excalidraw.png)

## Stages

Stages run in this order. A blocking stage that fails ends the evaluation, and
the report shows where it stopped in `summary.stopped_at`. Stages 1–8 are the
**static analysis** (reported in `static.checks`); stage 9 is the **dynamic
analysis** (reported in `dynamic.tests`).

| # | Stage (`name`) | Blocking | Verdict on failure |
|---|----------------|----------|--------------------|
| 1 | `limits` — code up to 64 KB | yes | `rejected` |
| 2 | `parse` — `go/parser` | yes | `rejected` |
| 3 | `ast` — `package solution`, allowlisted imports, signature, no `main`/`init`/`import "C"`/`//go:` directives | yes | `rejected` |
| 4 | `gofmt` | no (warning) | — |
| 5 | `complexity` — cyclomatic > 10 | no (warning) | — |
| 6 | `vet` — `go vet` | yes | `rejected` (or `compile_error` if the code does not type-check) |
| 7 | `gosec` | only HIGH severity | `rejected` |
| 8 | `build` — `go build`, `CGO_ENABLED=0` | yes | `compile_error` |
| 9 | execution | — | `passed`, `failed` or `timeout` |

Stages 4 and 5 run in parallel, as do 6 and 7. An unknown exercise, or one
outside the attempt, is refused before the pipeline (HTTP 404/400).

## Dynamic analysis

Stage 9 runs only if the code passed the static analysis and compiled. The
program is run once, receiving all the tests as JSON on stdin, inside a
temporary directory with a clean environment (and inside Firejail when
available). For each test it records:

| Observed | How | Shown as |
|----------|-----|----------|
| Return value | serialised to JSON and compared with the expected output | `got` / `expected`, `passed` |
| Execution time | measured around the call | `duration_ms` |
| Timeout | each call runs in a goroutine with a timer (`timeout_ms`); the whole run also has a global timeout that kills the process group | error `timeout`, verdict `timeout`; later tests are reported as not run |
| Panic | `recover()` around each call, so the other tests still run | error `panic: …` |
| Crash | the process exits without reporting some results | error `o programa terminou inesperadamente…` |
| Output flood | stdout/stderr capped at 1 MB | error `a saída excedeu 1 MB` |
| Unserialisable result | e.g. `NaN` | error `resultado não serializável…` |

Anything the student prints goes to stderr and is ignored; it cannot break or
forge the results (see [Security](security.md)). The score is the
fraction of tests passed, so partial solutions get partial credit.

Allowed imports: `fmt`, `math`, `strings`, `strconv`, `sort`, `slices`,
`maps`, `unicode`, `unicode/utf8`, `errors`.
