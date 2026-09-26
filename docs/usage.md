# Using Gavel

[← Back to the README](../README.md)

How to run Gavel and use it from the browser, the CLI and other devices.

## Running the server

```sh
make build                     # builds bin/gavel
./bin/gavel serve              # server at http://localhost:8080
```

Open <http://localhost:8080> in a browser. Alternatively, `make run` starts the
server with `go run`.

Commands run from the repository root (they use `./data`). To use a different
data directory, set `GAVEL_DATA`.

## CLI

`make build` creates the binary at `./bin/gavel`, which is what the examples
below use. Run them from the repository root, because Gavel reads `./data`
(or set `GAVEL_DATA=/path/to/Gavel/data`).

To type just `gavel` instead, install it into your Go bin folder and make sure
that folder is on your `PATH`:

```sh
go install ./cmd/gavel                     # installs to $(go env GOPATH)/bin/gavel
export PATH="$PATH:$(go env GOPATH)/bin"   # add this line to ~/.bashrc to keep it
```

```
./bin/gavel exercises [-difficulty easy]          # table: Id | Level | Title | Description
./bin/gavel show <id>                             # exercise details
./bin/gavel exams                                 # list exam templates
./bin/gavel start [-student <name>] <exam_id>     # start an attempt, show attempt_id and exercises
./bin/gavel submit [-attempt <id>] [-student <name>] <exercise_id> <file.go>
./bin/gavel attempt <attempt_id>                  # attempt score
./bin/gavel report <submission_id>                # saved report
./bin/gavel serve [-addr localhost:8080] [-sandbox auto|firejail|none]
```

Flags come before arguments (`./bin/gavel submit -attempt X factorial sol.go`).
`submit` exits with code 0 if the verdict is `passed` and 1 otherwise.

By default the CLI works directly on the local disk. If the `SERVER`
environment variable is set, the same commands use the remote API:

```sh
export SERVER=http://localhost:8080
./bin/gavel exercises -difficulty hard
```

An exam example:

```sh
./bin/gavel start easy                          # shows the attempt_id and the drawn exercises
./bin/gavel submit -attempt <attempt_id> leap leap.go
./bin/gavel attempt <attempt_id>                # current score
```

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
