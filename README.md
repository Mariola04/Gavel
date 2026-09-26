# Gavel automatic grader for Go functions

[![qualify](https://github.com/Mariola04/Gavel/actions/workflows/qualify.yaml/badge.svg?branch=main)](https://github.com/Mariola04/Gavel/actions/workflows/qualify.yaml)

An academic prototype of an automatic grader for Go submissions. Each exercise
asks for **one function**: Gavel calls the student's function with the
arguments of each test and compares the returned value with the expected one.

- **Static analysis** checks the code without running it: syntax, import and
  signature rules, formatting, complexity, `go vet`, `gosec`, and compilation.
- **Dynamic analysis** runs the compiled code against every test in an
  isolated process with time limits, and records results, panics and timeouts.
- **Partial credit:** the score is the fraction of tests passed.
- **Exams run by the teacher:** they pick a difficulty, a custom mix or the
  exact exercises, and the duration; students join from the *Prova* tab and all get the same
  exercises. The best submission per exercise counts.
- **Three interfaces** over the same logic: a web app (practice, exams and a
  teacher area), a CLI and an HTTP API.

The user-facing text (web, CLI, reports) is in European Portuguese.

## How it works

![Gavel evaluation flow: pre-checks, static analysis (limits, parse, ast, gofmt and complexity, go vet and gosec, go build), dynamic analysis (run tests in the sandbox), then the report. Any blocking failure stops the pipeline and goes straight to the report.](docs/Gavel-Flow.excalidraw.png)

Stages run cheapest first and stop at the first blocking failure; every
outcome produces a saved report. Details in
[docs/pipeline.md](docs/pipeline.md).

## Quick start

You need **Go 1.22 or later**, `git` and `make` (Linux or macOS).

```sh
git clone https://github.com/Mariola04/Gavel.git
cd Gavel
make build
./bin/gavel serve          # open http://localhost:8080
```

Type a name, pick an exercise, write the function and press **Submeter**.
For the teacher area, start the server with
`GAVEL_ADMIN_PASSWORD='choose-a-password' ./bin/gavel serve`.

Prefer the terminal?

```sh
./bin/gavel exercises
./bin/gavel submit factorial my_solution.go
```

## Documentation

| Read this | For |
|-----------|-----|
| [CHEATSHEET.md](CHEATSHEET.md) | Every command and feature on one page |
| [docs/usage.md](docs/usage.md) | Running the server, the CLI, remote mode, using Gavel from other laptops |
| [docs/content.md](docs/content.md) | Exercise and exam formats, adding new ones, scoring, where the exercises come from |
| [docs/pipeline.md](docs/pipeline.md) | The nine evaluation stages: static and dynamic analysis, verdicts |
| [docs/api.md](docs/api.md) | HTTP API reference, including the teacher routes |
| [docs/teacher-area.md](docs/teacher-area.md) | Student names and the password-protected teacher area |
| [docs/security.md](docs/security.md) | How students' code is contained, and the known limitations |
| [docs/development.md](docs/development.md) | Tooling, `make` targets, tests and CI |

## License

Gavel's source code is released under the MIT License, see [`LICENSE`](LICENSE).
16 of the 21 exercises are converted from
[exercism/problem-specifications](https://github.com/exercism/problem-specifications)
and keep their original MIT License
([details](docs/content.md#exercise-origin-and-license)).
