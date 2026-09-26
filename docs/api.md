# HTTP API

[← Back to the README](../README.md)

The JSON API served by `./bin/gavel serve`, including the teacher routes.

## Public routes

All responses are JSON. Errors have the form `{"error": "..."}` with the
appropriate status (400, 404, 413 or 500). CORS is open
(`Access-Control-Allow-Origin: *`).

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/exercises` | list (`id`, `title`, `description`, `difficulty`); accepts `?difficulty=easy` |
| GET | `/api/exercises/{id}` | details, with the signature and test inputs, **without** the outputs |
| GET | `/api/exams` | exam presets |
| GET | `/api/sessions` | open exam sessions (without their exercises), with `remaining_seconds` |
| POST | `/api/sessions/{id}/join` | body `{"student": "..."}` (required) → the student's attempt; joining again returns the same attempt. 400 if the exam has ended |
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

## Teacher routes

These need the header `Authorization: Bearer <GAVEL_ADMIN_PASSWORD>`; see [Students and teacher area](teacher-area.md).

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/admin/submissions` | all submissions (without code), oldest first |
| GET | `/api/admin/attempts` | all attempts with their current score, oldest first |
| GET | `/api/admin/sessions` | all exam sessions, open or closed, with `students` joined |
| POST | `/api/admin/sessions` | open a session (201): `{"title", "duration_minutes", …}` plus one of `"exercise_ids": ["sum", "hamming"]` (picked, in that order), `"composition": {"easy": 2, "hard": 1}` (drawn) or `"preset": "medium"` (drawn from the preset). `exercise_ids` wins over the others |
| POST | `/api/admin/sessions/{id}/close` | end a session now |

Both return 401 with a wrong or missing password, and 404 when the area is
disabled.
