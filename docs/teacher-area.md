# Students and teacher area

[← Back to the README](../README.md)

How students identify themselves and how teachers see every submission.

> This goes beyond the original specification, which asks for no
> authentication. It is deliberately minimal: there are no accounts.

## Students

The web interface asks for a name or student number before
practising. It is stored with every submission (`student`) and attempt; a
submission inside an exam always takes the attempt's student. There is no
password, so names are self-declared. In the CLI, use `-student <name>` with
`start` and `submit`. Names are trimmed, limited to 80 characters and may not
contain control characters.

## Teacher area ("Docente" tab)

Start the server with a password in the
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

The teacher routes are listed in the [API reference](api.md#teacher-routes).
