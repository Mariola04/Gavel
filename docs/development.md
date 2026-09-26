# Development

[← Back to the README](../README.md)

Tooling, checks and continuous integration for people working on Gavel itself.

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

## Make targets

```sh
make fmt      # gofmt -w
make vet      # go vet
make lint     # golangci-lint
make test     # go test -race ./...
make check    # fmt-check + vet + lint + test
```

Run `make check` before pushing: it is what CI checks.

## Tests

- Unit tests live next to the code (`internal/*/…_test.go`).
- `internal/engine` runs real submissions from `testdata/submissions/` (correct, wrong answer, syntax and type errors, forbidden imports, infinite loop, panic, unformatted code, `fmt.Println`, forged output, vet error).
- `TestReferenceSolutions` runs every `data/exercises/*/solution.go` through the full pipeline.
- `internal/server` tests every HTTP route with `httptest`, including the teacher routes.

```sh
go test ./internal/engine -run TestReferenceSolutions   # reference solutions only
go test ./internal/engine -run TestEvaluateSubmissions  # edge-case submissions
go test ./internal/server                               # HTTP routes
```

## Continuous integration

[`.github/workflows/qualify.yaml`](../.github/workflows/qualify.yaml) runs on
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
