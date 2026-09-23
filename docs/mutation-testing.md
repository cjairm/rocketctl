# Mutation testing

`make mutate` runs [gremlins](https://github.com/go-gremlins/gremlins) over `internal/version` and
`internal/config` — the two packages with no I/O, and therefore the only ones currently testable in
isolation.

## Reading the output

| Status        | Meaning                                         | Action                                                    |
| ------------- | ----------------------------------------------- | --------------------------------------------------------- |
| `KILLED`      | A test caught the mutation.                     | None. This is the goal.                                   |
| `LIVED`       | The code was changed and **no test noticed**.   | Add a test case. Never change production code to kill it. |
| `NOT COVERED` | No test exercises that line at all.             | Add coverage, or accept it deliberately.                  |
| `TIMED OUT`   | The mutated test run exceeded gremlins' budget. | Ignore — see below.                                       |

**Only `LIVED` matters.** It is the one status that reports a real gap in the test suite.

## Why this is advisory and not a CI gate

Measured 2026-09-20 with gremlins v0.6.0: two identical back-to-back runs of
`gremlins unleash ./internal/config/` returned `Killed: 2, efficacy 100.00%` and then
`Killed: 0, efficacy 0.00%`. Mutants land in `TIMED OUT` at random.

The suspected cause is that gremlins derives its per-mutant timeout from the baseline test run, and
this suite finishes in well under a second — faster than Go's compile-and-run overhead for the
mutated binary. Raising `--timeout-coefficient` made the problem worse rather than better, so the
mechanism is not fully understood.

Until that is resolved, `make mutate` never fails the build and CI does not run it. Run it locally
when changing `internal/version` or `internal/config`, and act on `LIVED` results only.

## Re-evaluating

Worth revisiting if the suite grows substantially slower (which may stabilise the timeout
calculation), or if gremlins ships a fix. Alternatives if it stays unusable:
[ooze](https://github.com/gtramontina/ooze), which runs as an ordinary Go test, or
[avito-tech/go-mutesting](https://github.com/avito-tech/go-mutesting).
