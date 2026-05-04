[![Go Version](https://img.shields.io/badge/go-%20v1.26.2-blue?logo=go)](https://github.com/jbunds/progress/blob/main/go.mod) &nbsp;
[![pre-commit](https://img.shields.io/badge/pre--commit-enabled-brightgreen?logo=pre-commit)](https://github.com/pre-commit/pre-commit) &nbsp;
[![tests](https://github.com/jbunds/progress/actions/workflows/test-go.yml/badge.svg)](https://github.com/jbunds/progress/actions/workflows/test-go.yml) &nbsp;
[![lint](https://github.com/jbunds/progress/actions/workflows/lint-go.yml/badge.svg)](https://github.com/jbunds/progress/actions/workflows/lint-go.yml) &nbsp;
[![coverage](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/jbunds/75dfc308f6a4adc36db4340cc020c713/raw/coverage.json)](https://github.com/jbunds/coverage/actions/workflows/test-go.yml)

#### Simple Terminal Progress Tracker

The `progress` package provides status updates to the terminal as units of work are incrementally completed.

Incremental calculations retain high-precision, while imposing minimal overhead upon callers.

Features include:

- context-aware:
  - correctly handles cancellation of the parent context, ensuring a clean exit under reasonable circumstances
- concurrency-safe and well-suited to highly-scaled concurrent processing systems:
  - uses `sync/atomic` to provide lock-free updates of internal state for highly-scaled concurrent workloads
  - optionally supports an implementation via the `unique` package to reduce the memory footprint for suitable workloads (repetitive status updates)
- very efficient:
  - throttles status updates at ~60 FPS, decoupling the progress status tracker from the caller program processing the workload
  - uses packed `atomic.Uint32` types and bitwise operations to further reduce memory allocation and efficiently handle updates of internal state
  - uses `atomic.Uint64` and `atomic.Pointer` types to:
    - minimize memory allocation
    - impose minimal GC overhead
    - enable fast comparisons via pointers
    - obviate mutex contention
  - optimized to minimize CPU resource consumption
- supports two tracking modes:
  - weight-based accumulation: callers specify the total known amount of work (e.g., 100 tasks, known a prioi)
  - fractional allocation: callers add the relative share of the total budget as work is discovered (e.g., recursively walking a directory to process its contents)
- supports multiple progress status tracking implementations which are well-suited to different sets of inputs:
  - `progress.Standard`: suitable for mostly unique status updates
  - `progress.Unique`:   suitable for mostly repetitive status updates
- supports multiple progress status formats:
  - `progress.Percent`:  writes only the percentage calculation to the terminal
  - `progress.Fraction`: writes progress status as a proper fraction (x/y) given a prescribed fixed total units of work (y)
- transparently handles pipes, redirections, and non-TTY environments
- correctly handles UTF-8 strings passed by callers to provide status updates

Limitations:

- the precision of percentage calculations starts to progressively degrade at ~18.4 million units of work

#### Example Usage

See the [`examples`](./examples) directory for examples of the modal API.
