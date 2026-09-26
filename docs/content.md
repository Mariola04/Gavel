# Exercises and exams

[← Back to the README](../README.md)

The formats of exercises and exam templates, how to add them, how exams are scored, and where the exercises come from.

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

## Exams

> Exams are run by the teacher. This replaces the original specification's
> self-service exams, where each student started their own attempt from a
> template; `POST /api/exams/{id}/attempts` and `gavel start` no longer exist.

### Presets

A preset is a file `data/exams/<id>.json`. The teacher picks one as a starting
point when opening an exam:

```json
{
  "title": "Teste fácil",
  "description": "2 exercícios fáceis e 1 médio.",
  "composition": { "easy": 2, "medium": 1 }
}
```

Included presets: `easy` (2 easy + 1 medium), `medium` (1 easy + 2 medium +
1 hard) and `hard` (1 medium + 3 hard).

**To add a preset:** create the file. At startup, Gavel checks that the levels
are valid, the counts are positive and there are enough exercises of each
level; otherwise startup fails.

### Exam sessions

1. **The teacher opens an exam** in the Docente area (*Provas* tab) with a
   title, a duration (1 to 480 minutes) and the exercises, chosen in one of
   three ways:
   - a **preset** (`easy`, `medium`, `hard`) — exercises drawn by level;
   - **Personalizada (sorteio)** — a custom number per level, drawn;
   - **Escolher exercícios** — the teacher ticks the exact exercises (no draw;
     they appear in list order: level, then title).

   Draws happen **once** (`math/rand/v2`, no repeats) and the session is saved
   in `data/sessions/<session_id>.json` (with the seed, for draws), so **every
   student gets the same exercises**.
2. **Students join** from the *Prova* tab, which lists the open exams with a
   countdown. Joining creates an attempt (`data/attempts/<attempt_id>.json`)
   with the student's name. Joining again with the same name (case-insensitive)
   returns the same attempt, so a student can switch browsers.
3. **Submissions** carry the `attempt_id`; the exercise must belong to the
   exam.
4. **Time is a fixed window** for everyone: when it ends, or when the teacher
   presses *Terminar agora*, joining and submitting are refused by the server
   and the web editor locks. Attempts and grades can still be viewed.

**Scoring.** For each exercise, the best submission counts (highest `score`,
which is the fraction of tests passed — 7 of 10 tests is 0.7). Points earned =
`score × level points`; the total is the sum of points earned divided by the
sum of possible points.

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
[`data/exercises/LICENSE-exercism.txt`](../data/exercises/LICENSE-exercism.txt)).
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
