---
icon: lucide/gauge
---

# Benchmarks

The benchmark suite measures the picker paths users notice most: filtering the
session list, rendering the picker, and refreshing previews while the selection
moves fast. It uses Go's standard benchmark runner and
[`benchstat`](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) for
before-and-after comparisons.

!!! tip "Measure a change against its own baseline"

    Benchmark results depend on the machine, power mode, and background load.
    Compare repeated runs from the same machine instead of treating one result
    as a universal performance budget.

## Run the suite

Install the pinned development tools with `mise install` before you run these
commands.

=== "Quick check"

    Run every benchmark once:

    ```bash
    just bench
    ```

=== "Before and after"

    Collect several samples before and after the change, then compare them:

    ```bash
    just bench 10 | tee /tmp/rex-sesh-base.txt
    # Make the change.
    just bench 10 | tee /tmp/rex-sesh-candidate.txt
    just bench-compare /tmp/rex-sesh-base.txt /tmp/rex-sesh-candidate.txt
    ```

    `benchstat` reports the distributions, the relative change, and its
    statistical significance. Ten samples are a practical start; raise the
    count when results are noisy.

The `bench` recipe skips unit tests with `-run '^$'` and reports allocations
with `-benchmem`. It runs the benchmarks in `internal/sources` and
`internal/picker`; today, all of them are in `internal/picker`. The recipe also
prints `Unit` lines that tell `benchstat` to treat `canceled/op` and
`completed/op` as exact values. `bench-compare` runs `benchstat` through
`go tool`, so it needs no separate install.

## What is measured

All three benchmarks use generated Rex sessions with paths, session IDs, and a
mix of agent statuses.

| Benchmark | Workload | Why it exists |
| --- | --- | --- |
| `FilterSessions` | Filter 100 and 1,000 sessions with a query that matches none and one that matches 100. | Tracks filtering cost as the session list grows. |
| `RenderPicker` | Render 1,000 sessions, with icons, in a 120x40 picker, with the preview shown and hidden. | Measures the full view path and the cost of the preview layout. |
| `PreviewNavigationBurst` | Move the selection across eight sessions in a row, so each move starts a preview render. | Checks that a new selection cancels the preview it replaces. |

## Read the results

Go reports the standard time and allocation columns. `PreviewNavigationBurst`
also reports two behaviour metrics:

| Metric | Target | Meaning |
| --- | --- | --- |
| `ns/op` | Lower | Nanoseconds per benchmark operation. |
| `B/op` | Lower | Bytes allocated per operation. |
| `allocs/op` | Lower | Allocations per operation. |
| `canceled/op` | Exactly `7` | The seven replaced preview renders are canceled. |
| `completed/op` | Exactly `1` | The preview for the final selection completes. |

!!! info "Preview cancellation"

    Each new selection cancels the preview it replaces. The expected result is
    `7 canceled/op` and `1 completed/op`: seven stale renders stop, and the
    final selection still gets its preview. Any other pair is a behaviour
    regression, even if one value moved in a direction that looks better.

## Benchmark strategy

1. Benchmark user-facing hot paths with representative, generated data.
2. Check expected results where the workload has a known answer, and keep
   rendered output so the compiler cannot discard the measured work.
3. Compare repeated samples with `benchstat`. Do not judge a change from one
   run, or from results collected on different machines.
4. Review time, allocations, and behaviour metrics together. A lower `ns/op`
   does not justify missing the exact `7` canceled and `1` completed preview
   target.

The `benchstat` comparison is informational, not a CI threshold. Shared CI
runners are noisy, and fixed limits would fail on machine variation instead of
real regressions. The
[`Benchmark` workflow](https://github.com/fullerzz/rex-plugin-sesh/blob/main/.github/workflows/benchmark.yml)
runs on pull requests to `main` that change Go code, `go.mod`, `go.sum`,
`mise.toml`, the `justfile`, or the workflow. It runs `just bench 10` on the
base and the pull request merge revisions on the same runner. It writes the
`benchstat` comparison to the workflow summary. For branches in this
repository, it also adds or updates a "Benchmark results" comment on the pull
request. Confirm notable changes on one machine with `just bench` and
`just bench-compare`.

[Picker benchmarks](https://github.com/fullerzz/rex-plugin-sesh/blob/main/internal/picker/tea_benchmark_test.go){ .md-button }
