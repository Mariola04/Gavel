# Gavel automatic grader for Go functions

[![qualify](https://github.com/Mariola04/Gavel/actions/workflows/qualify.yaml/badge.svg?branch=main)](https://github.com/Mariola04/Gavel/actions/workflows/qualify.yaml)

An academic prototype of an automatic grader for Go submissions. Each exercise
asks for **one function**: Gavel calls the student's function with the
arguments of each test and compares the returned value with the expected one.
There is no I/O (stdin/stdout) grading.

Every submission goes through two kinds of analysis:

- **static analysis** — the code is inspected without running it (syntax,
  import and signature rules, formatting, cyclomatic complexity, `go vet`,
  `gosec`) and then compiled;
- **dynamic analysis** — the compiled program runs against every test case in
  an isolated process with time limits, and its behaviour is observed (return
  values, execution time, panics, timeouts, crashes).

It offers three interfaces over the same logic:

- a **CLI** (`gavel`), which works locally or against a remote server;
- an **HTTP API** (`gavel serve`);
- a **web interface** (plain HTML/CSS/JS, served by the binary itself), with a
  practice mode, an exam mode and a password-protected teacher area.

For a one-page summary of every command and feature, see
[CHEATSHEET.md](CHEATSHEET.md).

The user-facing text (CLI output, web interface, report messages) is in
European Portuguese.

## Requirements

- **Go 1.22 or later** (required: Gavel uses `go` to build submissions).
- [gosec](https://github.com/securego/gosec) (optional): if it is not on the
  `PATH`, the `gosec` stage is marked as `skipped`.
  `go install github.com/securego/gosec/v2/cmd/gosec@latest`
- [Firejail](https://firejail.wordpress.com/) (optional, Linux): isolates
  execution with no network and a private filesystem.
- For development: [golangci-lint](https://golangci-lint.run/) v1.x.

> The `Makefile` adds `$(go env GOPATH)/bin` to the `PATH`, so tools installed
> with `go install` are found by the `make` targets. Outside `make`, make sure
> `gosec` is on your `PATH` if you want that stage to run.

## Running

```sh
make build                     # builds bin/gavel
./bin/gavel serve              # server at http://localhost:8080
```

Open <http://localhost:8080> in a browser. Alternatively, `make run` starts the
server with `go run`.

Commands run from the repository root (they use `./data`). To use a different
data directory, set `GAVEL_DATA`.

### CLI

```
gavel exercises [-difficulty easy]          # table: Id | Level | Title | Description
gavel show <id>                             # exercise details
gavel exams                                 # list exam templates
gavel start [-student <name>] <exam_id>     # start an attempt, show attempt_id and exercises
gavel submit [-attempt <id>] [-student <name>] <exercise_id> <file.go>
gavel attempt <attempt_id>                  # attempt score
gavel report <submission_id>                # saved report
gavel serve [-addr localhost:8080] [-sandbox auto|firejail|none]
```

Flags come before arguments (`gavel submit -attempt X factorial sol.go`).
`submit` exits with code 0 if the verdict is `passed` and 1 otherwise.

By default the CLI works directly on the local disk. If the `SERVER`
environment variable is set, the same commands use the remote API:

```sh
export SERVER=http://localhost:8080
gavel exercises -difficulty hard
```

An exam example:

```sh
gavel start easy                          # shows the attempt_id and the drawn exercises
gavel submit -attempt <attempt_id> leap leap.go
gavel attempt <attempt_id>                # current score
```

### Development

```sh
make fmt      # gofmt -w
make vet      # go vet
make lint     # golangci-lint
make test     # go test -race ./...
make check    # fmt-check + vet + lint + test
```

Run `make check` before pushing: it is what CI checks.

### Continuous integration

[`.github/workflows/qualify.yaml`](.github/workflows/qualify.yaml) runs on
every push (any branch) and every pull request, with two jobs:

| Job | What it runs |
|-----|--------------|
| **Format, vet and lint** (Go 1.24) | `make fmt-check`, `make vet`, golangci-lint v1.64.8 with `.golangci.yml` |
| **Tests** (Go 1.22 and latest stable) | `make test` (`go test -race ./...`, including the full evaluation pipeline and every reference solution) and `make build` |

- Go 1.22 is the minimum supported version, so it is tested explicitly.
- The lint job is pinned to Go 1.24 because the golangci-lint v1.64.8 binary
  is built with Go 1.24 and cannot type-check code against newer Go releases
  (it fails with "export data version … is greater than maximum supported
  version"). Moving to a newer Go for linting requires golangci-lint v2 and
  migrating `.golangci.yml` to the v2 format.
- `gosec` is installed only in the stable job, so the security stage runs for
  real there, and the "gosec not installed → skipped" path is covered by the
  Go 1.22 job.
- Firejail is not available on the runners; the tests use the `none` sandbox
  (the report shows the usual warning).
- The engine's build cache (`data/.cache/go-build`) is cached between runs to
  keep the tests fast.
- A newer push to the same branch cancels the previous run.

Results appear in the repository's **Actions** tab and as checks on pull
requests. To block merging when they fail, enable branch protection on
`main` (Settings → Branches → require the "Format, vet and lint" and "Tests"
checks).

## Exercise format

Each exercise is a directory `data/exercises/<id>/` (the directory name is the
`id`: lowercase letters, digits and hyphens) with two files:

- `exercise.json` — the definition;
- `solution.go` — the reference solution, in `package solution`.

```json
{
  "title": "Fatorial",
  "description": "Defina a função Factorial(n int) int que devolve n!.",
  "difficulty": "easy",
  "function": "Factorial",
  "params": ["int"],
  "param_names": ["n"],
  "returns": "int",
  "timeout_ms": 1000,
  "tests": [
    { "input": [0], "output": 1, "hint": "Ver caso base" },
    { "input": [5], "output": 120 }
  ]
}
```

| Field | Rules |
|-------|-------|
| `difficulty` | `easy` (1 point), `medium` (2 points) or `hard` (3 points) |
| `function` | exported identifier |
| `params`, `returns` | JSON-serialisable types: basic types (`int`, `float64`, `string`, `bool`, …), slices and maps with `string` keys, in any combination. A single return value. |
| `param_names` | optional; parameter names shown in the signature and the editor skeleton (e.g. `func Distance(a, b string) int`). Defaults to `p0`, `p1`, … Students may use any names. |
| `timeout_ms` | per-test limit, positive |
| `tests[].input` | array with one value per parameter |
| `tests[].hint` | optional; shown only when the test fails |

**To add an exercise:** create the directory with both files and run
`make test`. `TestReferenceSolutions` runs the reference solution through the
whole pipeline and fails if it does not pass its own tests. Exercises are
validated at startup; an invalid exercise stops startup with a clear message.

For comparison, `got` and `output` are decoded into generic JSON values and
compared with `reflect.DeepEqual` (numbers are compared as `float64`). A `nil`
slice or map is considered equal to `[]` or `{}`.

## Exam format

An exam template is a file `data/exams/<id>.json`:

```json
{
  "title": "Teste fácil",
  "description": "2 exercícios fáceis e 1 médio.",
  "composition": { "easy": 2, "medium": 1 }
}
```

Included templates: `easy` (2 easy + 1 medium), `medium` (1 easy + 2 medium +
1 hard) and `hard` (1 medium + 3 hard).

**To add a template:** create the file. At startup, Gavel checks that the
levels are valid, the counts are positive and there are enough exercises of
each level; otherwise startup fails.

**Attempts.** Starting an exam draws the exercises (`math/rand/v2`, no
repeats) and saves the attempt in `data/attempts/<attempt_id>.json` together
with the seed, which makes the draw reproducible. Submissions may include an
`attempt_id`; in that case the exercise must belong to the attempt.

**Scoring.** For each exercise, the best submission counts (highest `score`,
which is the fraction of tests passed — 7 of 10 tests is 0.7). Points earned =
`score × level points`; the total is the sum of points earned divided by the
sum of possible points.

## Evaluation pipeline

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

### Dynamic analysis

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
forge the results (see [Security](#security-and-limitations)). The score is the
fraction of tests passed, so partial solutions get partial credit.

Allowed imports: `fmt`, `math`, `strings`, `strconv`, `sort`, `slices`,
`maps`, `unicode`, `unicode/utf8`, `errors`.

## API

All responses are JSON. Errors have the form `{"error": "..."}` with the
appropriate status (400, 404, 413 or 500). CORS is open
(`Access-Control-Allow-Origin: *`).

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/exercises` | list (`id`, `title`, `description`, `difficulty`); accepts `?difficulty=easy` |
| GET | `/api/exercises/{id}` | details, with the signature and test inputs, **without** the outputs |
| GET | `/api/exams` | exam templates |
| POST | `/api/exams/{id}/attempts` | start an attempt (201) → `attempt_id`, drawn exercises and score; optional body `{"student": "..."}` |
| GET | `/api/attempts/{id}` | attempt with the current score and per-exercise details |
| POST | `/api/submissions` | body `{"exercise_id": "...", "code": "...", "attempt_id": "optional", "student": "optional"}` → full report (synchronous) |
| GET | `/api/submissions/{id}` | saved report |
| GET | `/` | web interface |

Example:

```sh
curl -s localhost:8080/api/submissions \
  -d '{"exercise_id":"sum","code":"package solution\n\nfunc Sum(n []int) int {\n\tt := 0\n\tfor _, x := range n {\n\t\tt += x\n\t}\n\treturn t\n}\n"}'
```

The report has `static.checks` (status `ok`, `warning`, `error` or `skipped`
per stage), `dynamic.tests` (input, expected, got, error, duration and, for
failed tests, the hint) and `summary` (`verdict`, `passed`, `total`, `score`,
`stopped_at`). Reports are saved in `data/reports/<submission_id>.json`.

## Students and teacher area

> This goes beyond the original specification, which asks for no
> authentication. It is deliberately minimal: there are no accounts.

**Students.** The web interface asks for a name or student number before
practising. It is stored with every submission (`student`) and attempt; a
submission inside an exam always takes the attempt's student. There is no
password, so names are self-declared. In the CLI, use `-student <name>` with
`start` and `submit`. Names are trimmed, limited to 80 characters and may not
contain control characters.

**Teacher area ("Docente" tab).** Start the server with a password in the
environment — it is never stored in the code or on disk:

```sh
GAVEL_ADMIN_PASSWORD='choose-a-password' ./bin/gavel serve
```

Without `GAVEL_ADMIN_PASSWORD` the area is disabled. After logging in, the
teacher sees:

- totals (submissions, students, attempts, pass rate);
- **Submissões** — every submission, newest first, filterable by student,
  exercise, attempt and verdict; clicking one opens the submitted code and the
  full report;
- **Tentativas** — every exam attempt with its student, points and grade;
- **Alunos** — per student: submissions, exercises solved, best exam grade and
  last activity;
- optional auto-refresh every 15 seconds.

The password is kept only for the browser tab (`sessionStorage`) and sent in
the `Authorization: Bearer <password>` header; the server compares SHA-256
hashes in constant time.

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/admin/submissions` | all submissions (without code), oldest first |
| GET | `/api/admin/attempts` | all attempts with their current score, oldest first |

Both return 401 with a wrong or missing password, and 404 when the area is
disabled.

## Using Gavel from other devices

By default the server only listens on `localhost`, as the specification
requires, so only the machine running it can open the page. To let other
laptops on the same network use it (e.g. a class):

```sh
GAVEL_ADMIN_PASSWORD='choose-a-password' ./bin/gavel serve -addr 0.0.0.0:8080
```

Students then open `http://<server-ip>:8080` (find the IP with
`hostname -I` on Linux or `ipconfig` on Windows). The firewall must allow port
8080. Remote CLI users can set `SERVER=http://<server-ip>:8080`.

Before doing this, keep in mind:

- every submission is compiled and **run on the server machine**; use
  `--sandbox=firejail` (Linux) if possible;
- traffic is plain HTTP, so the teacher password travels unencrypted on the
  network — use it only on a trusted network;
- student names are not verified, and reports and attempts are readable by
  anyone who knows their id.

## Security and limitations

Running third-party code is dangerous. Gavel's defences, in order of
importance:

1. **Import allowlist (main defence).** Without `os`, `net`, `syscall`,
   `unsafe`, `reflect` or `C`, the student's code cannot access files, the
   network or processes. `//go:` directives (e.g. `go:linkname`, `go:embed`),
   `init` and `main` are also forbidden.
2. **Separate harness.** The student's code is compiled as the
   `sandbox/solution` package, imported by a generated `main`; the student
   cannot see the grader's identifiers. Before calling the function, the
   harness redirects `os.Stdout` to stderr, and every result line carries a
   random nonce passed through an environment variable the student cannot
   read. So whatever the student prints can neither break the harness nor
   forge results.
3. **Resource limits.** Per-test timeout (goroutine + `select`) and a global
   timeout that kills the whole process group; stdout and stderr capped at
   1 MB; clean environment (minimal `PATH` only); a dedicated temporary
   directory, removed at the end; at most 2 concurrent evaluations.
4. **Tools.** `go vet` and `gosec` (which ignores the student's `#nosec`
   annotations); `go` runs with `GOPROXY=off`, `GOTOOLCHAIN=local` and
   `CGO_ENABLED=0`.
5. **Firejail** (`--sandbox=auto|firejail|none`). In `auto` mode, Firejail is
   used if available (`--net=none --private=<tmp> --noroot`); otherwise
   execution runs without isolation and the report includes a warning
   (`sandbox`).

Known limitations:

- **Without Firejail there is no guaranteed network or filesystem isolation.**
  Security then relies on the allowlist and the compiler. A bug in the Go
  runtime or in the allowlist could allow an escape.
- There is no memory limit: a submission can allocate a lot of memory until
  the timeout kills it. With Firejail you can add `--rlimit-as`.
- Firejail was not tested in this development environment (it was not
  installed); the tests exercised the `none` mode.
- `gosec` rule G115 (integer conversion with possible overflow, e.g.
  `uint64(n)` with `n int`) has HIGH severity and therefore blocks the
  submission, as the specification requires.
- An attempt's score reads every saved report; this is fine for a prototype
  but does not scale to many thousands of submissions.
- Numbers are compared as `float64`, so integers above 2^53 may be considered
  equal if they differ only slightly.
- There are no real accounts: student names are self-declared, and anyone with
  access to the server can see an attempt or report if they know its id. Only
  the listings in the teacher area are password-protected, and without HTTPS
  that password is sent in clear text.

## Exercise origin and license

The exercises `factorial`, `fibonacci`, `reverse`, `sum` and `palindrome` were
written from scratch for this project.

The others (`leap`, `hamming`, `raindrops`, `isogram`, `pangram`,
`collatz-conjecture`, `darts`, `grains`, `armstrong-numbers`, `acronym`,
`roman-numerals`, `run-length-encoding`, `matching-brackets`, `nth-prime`,
`sieve`, `change`) were converted from the
[exercism/problem-specifications](https://github.com/exercism/problem-specifications)
repository, distributed under the **MIT License** (Copyright (c) Exercism;
full text in
[`data/exercises/LICENSE-exercism.txt`](data/exercises/LICENSE-exercism.txt)).
The tests come from each exercise's `canonical-data.json`, with these
adaptations:

- cases that expect an error were omitted;
- cases superseded by others (`reimplements`) were omitted;
- cases with integers that do not fit in a 64-bit `int` were omitted
  (`armstrong-numbers`);
- `grains` uses only the `square` property and `run-length-encoding` only
  `encode`;
- the descriptions were written in Portuguese, and the reference solutions
  from scratch.

The reference solutions live inside the module, so they are also compiled and
checked by `go vet` and `golangci-lint` in `make check`.

## License

Gavel's source code is released under the MIT License — see [`LICENSE`](LICENSE).
The exercises converted from Exercism keep their original MIT License (see above).
