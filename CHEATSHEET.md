# Gavel cheat sheet

Everything Gavel can do, in one page. Details in the [README](README.md) and the topic guides in [`docs/`](docs/).

## Setup
- Needs **Go 1.22+**. Optional: `gosec` (security stage), `firejail` (isolation), `golangci-lint` (dev).
- Install tools: `go install github.com/securego/gosec/v2/cmd/gosec@latest`
- Build: `make build` → `bin/gavel`
- Run commands from the repo root (data lives in `./data`; override with `GAVEL_DATA=<dir>`).

## Web
- Start: `./bin/gavel serve` (or `make run`) → open http://localhost:8080
- Students first type a **name / student number** (no password) → saved on every submission and attempt.
- Options: `-addr host:port`, `-sandbox auto|firejail|none`
- **Prática**: search/filter exercises, editor (auto-indent, `Tab`, `Ctrl+Enter` submits, drafts auto-saved), report with static + dynamic results.
- **Upload a file**: **Carregar .go** in the editor bar, or drag a `.go` file onto the editor → its code fills the editor → **Submeter**. Only `.go` text files up to 64 KB.
- **Prova**: always lists the exams the teacher opened (countdown) → **Entrar na prova** (or **Continuar a prova** if already in it) → same exercises for everyone → submit each → live score. Editor locks when time ends. Joining again with the same name resumes.
- Links are shareable: `#/exercicio/<id>`, `#/prova/<attempt_id>`, `#/docente`.

## Teacher area ("Docente" tab)
- Enable: `GAVEL_ADMIN_PASSWORD='choose-a-password' ./bin/gavel serve` (no env var → disabled). Never hardcoded.
- Log in on the **Docente** tab → totals + tabs **Provas** (open an exam: title, duration in minutes, and a preset, a custom count per level, or **pick the exact exercises**; follow time left and students; *Terminar agora*), **Submissões** (filter by student/exercise/attempt/verdict, click → code + full report), **Tentativas** (grade per attempt), **Alunos** (solved, best exam, last activity). Optional auto-refresh (15 s).
- API: `GET/POST /api/admin/sessions`, `POST /api/admin/sessions/{id}/close`, `GET /api/admin/submissions`, `GET /api/admin/attempts` with header `Authorization: Bearer <password>` (401 wrong, 404 disabled).

## Other laptops (class on a network)
- Default is **localhost only** (spec). To open it up: `./bin/gavel serve -addr 0.0.0.0:8080`
- Students open `http://<server-ip>:8080` (`hostname -I` shows the IP); firewall must allow port 8080.
- CLI from another laptop: `export SERVER=http://<server-ip>:8080`
- Caveats: code runs on the server machine (prefer `-sandbox firejail`), plain HTTP (password in clear text), names not verified.

## CLI
| Do | Command |
|----|---------|
| List exercises | `./bin/gavel exercises [-difficulty easy\|medium\|hard]` |
| Show one (signature, test inputs, skeleton) | `./bin/gavel show <id>` |
| List exam presets | `./bin/gavel exams` |
| List open exams | `./bin/gavel sessions` |
| Join an exam | `./bin/gavel join -student <name> <session_id>` → prints `attempt_id` |
| Submit (practice) | `./bin/gavel submit [-student <name>] <exercise_id> file.go` |
| Submit (exam) | `./bin/gavel submit -attempt <attempt_id> <exercise_id> file.go` |
| Attempt score | `./bin/gavel attempt <attempt_id>` |
| Saved report | `./bin/gavel report <submission_id>` |
| Server | `./bin/gavel serve [-addr ...] [-sandbox ...]` |
| Help | `./bin/gavel help` |

- `./bin/gavel` is the binary from `make build`; run it from the repo root. For a plain `gavel` command: `go install ./cmd/gavel` and put `$(go env GOPATH)/bin` on your `PATH`.
- Flags go **before** arguments.
- `submit` exit code: `0` = passed, `1` = anything else (handy in scripts/CI); `2` = bad usage.
- **Remote mode**: `export SERVER=http://localhost:8080` → same commands hit the server.

## HTTP API (JSON, CORS open)
| Method | Route | What |
|--------|-------|------|
| GET | `/api/exercises[?difficulty=easy]` | list |
| GET | `/api/exercises/{id}` | detail (no expected outputs) |
| GET | `/api/exams` | exam presets |
| GET | `/api/sessions` | open exams |
| POST | `/api/sessions/{id}/join` | `{"student"}` → attempt (same one if joined before) |
| GET | `/api/attempts/{id}` | attempt + score |
| POST | `/api/submissions` | `{"exercise_id","code","attempt_id"?,"student"?}` → report |
| GET | `/api/submissions/{id}` | saved report |
| GET/POST | `/api/admin/sessions` · POST `…/{id}/close` | teacher: list, open, close exams (password) |
| GET | `/api/admin/submissions` · `/api/admin/attempts` | teacher listings (password) |

- Errors: `{"error": "..."}` with 400 / 404 / 413 / 500.
- Try it: `curl -s localhost:8080/api/exercises?difficulty=hard`

## What a submission must be
- One file, `package solution`, max **64 KB**, defines the requested exported function with the **exact types**. Any parameter names and any algorithm are fine.
- Allowed imports: `fmt math strings strconv sort slices maps unicode unicode/utf8 errors`.
- Forbidden: other imports, `func main`, `func init`, `import "C"`, `//go:` directives.

## Static analysis (code is not run)
1. `limits` – size ≤ 64 KB → else **rejected**
2. `parse` – syntax (`go/parser`) → **rejected**
3. `ast` – package, imports, signature, forbidden constructs → **rejected**
4. `gofmt` – formatting → warning only
5. `complexity` – cyclomatic > 10 → warning only
6. `vet` – `go vet` → **rejected** (type errors → **compile_error**)
7. `gosec` – HIGH severity → **rejected**; lower → warning; not installed → skipped
8. `build` – `go build`, no cgo → **compile_error**
- 4‖5 and 6‖7 run in parallel. First blocking failure stops everything (`summary.stopped_at`).

## Dynamic analysis (code is run)
- Stage 9, only if stages 1–8 pass. Compiled program runs every test in a temp dir, clean env, Firejail if available.
- **Harness** = a `main.go` Gavel generates per submission (`internal/engine/harness.go`): reads the tests from stdin, calls the student's function once per test in a goroutine (`recover()` for panics, timer for timeouts), prints one `@@RESULT@@` JSON line per test tagged with a secret nonce. Details: `docs/pipeline.md#the-harness-how-the-code-is-run`.
- Per test: return value vs expected, time, **timeout**, **panic**, crash, output > 1 MB, unserialisable result.
- Student prints can't break or fake results (stdout redirected + secret nonce).
- Verdicts: **passed**, **failed**, **timeout**.

## Scoring
- Submission score = tests passed ÷ total (7/10 → **70%**). Rejected/compile errors → 0.
- Exam: best submission per exercise × level points (easy 1, medium 2, hard 3); total = points ÷ max.
- Presets: `easy` (2E+1M, 4 pts), `medium` (1E+2M+1H, 8 pts), `hard` (1M+3H, 11 pts); the teacher can also set a custom count per level, or pick the exact exercises.
- Exams are a **fixed time window** set by the teacher; after it ends (or *Terminar agora*) joins and submissions are refused.

## Content
- **Add an exercise**: `data/exercises/<id>/exercise.json` + `solution.go` → `make test` (reference solution must pass its own tests). Optional `param_names` for nicer signatures.
- **Add an exam preset**: `data/exams/<id>.json` with `title`, `description`, `composition` → validated at startup. Exams themselves are opened in the Docente area.
- 21 exercises (8 easy, 7 medium, 6 hard); 16 converted from Exercism (MIT).

## Where things are saved
- Reports: `data/reports/<submission_id>.json`
- Exam sessions: `data/sessions/<session_id>.json` (exercises + draw seed → reproducible)
- Attempts: `data/attempts/<attempt_id>.json` (student + session)
- Build cache: `data/.cache/` (safe to delete; slower first build)
- All of these are gitignored.

## Development & tests
| Do | Command |
|----|---------|
| Everything (CI gate) | `make check` (fmt-check + vet + lint + tests) |
| Tests only | `make test` (`go test -race ./...`) |
| Format / vet / lint | `make fmt` · `make vet` · `make lint` |
| Reference solutions only | `go test ./internal/engine -run TestReferenceSolutions` |
| Malicious/edge submissions | `go test ./internal/engine -run TestEvaluateSubmissions` |
| HTTP routes | `go test ./internal/server` |

- **CI** (`.github/workflows/qualify.yaml`): on every push and PR → fmt-check + vet + golangci-lint (on Go 1.24, the newest the pinned linter supports), and `make test` + `make build` on **Go 1.22** and **latest stable** (gosec only on stable). See the Actions tab / PR checks.
- Same checks locally before pushing: `make check`.
- Test submissions live in `testdata/submissions/` (correct, wrong, syntax/type error, `os`/`net/http` imports, infinite loop, panic, unformatted, `Println`, forged output, vet error).

## Security in one line each
- Import allowlist = main defence (no files, network or processes).
- Timeouts + process-group kill, 1 MB output cap, clean env, temp dir removed, max 2 evaluations at once.
- `go` runs offline (`GOPROXY=off`, `GOTOOLCHAIN=local`, `CGO_ENABLED=0`); `gosec` ignores `#nosec`.
- **Without Firejail there is no network/filesystem isolation**, and there is no memory limit.
- Teacher password: from env var only, kept per browser tab, compared as SHA-256 in constant time; no HTTPS.
