# Security and limitations

[← Back to the README](../README.md)

How Gavel protects the machine that runs students' code, and what it does not protect against.

## Defences

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

## Known limitations

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
