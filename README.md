[![Go Version](https://img.shields.io/badge/go-%20v1.26.2-blue?logo=go)](https://github.com/jbunds/progress/blob/main/go.mod) &nbsp;
[![pre-commit](https://img.shields.io/badge/pre--commit-enabled-brightgreen?logo=pre-commit)](https://github.com/pre-commit/pre-commit) &nbsp;
[![tests](https://github.com/jbunds/progress/actions/workflows/test-go.yml/badge.svg)](https://github.com/jbunds/progress/actions/workflows/test-go.yml) &nbsp;
[![lint](https://github.com/jbunds/progress/actions/workflows/lint-go.yml/badge.svg)](https://github.com/jbunds/progress/actions/workflows/lint-go.yml) &nbsp;
[![coverage](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/jbunds/75dfc308f6a4adc36db4340cc020c713/raw/coverage.json)](https://github.com/jbunds/coverage/actions/workflows/test-go.yml)

#### Simple Terminal Progress Tracker

The `progress` package provides status updates to the terminal as units of work are incrementally completed.

Incremental calculations retain high-precision, while imposing minimal overhead upon callers.

Features include:

- context-aware
- concurrency-safe:
  - uses `sync/atomic` to provide lock-free updates of internal state for highly-scaled concurrent workloads
- very efficient:
  - throttles status updates at ~60 FPS
  - uses `atomic.Uint32` and `atomic.Pointer` to minimize memory allocation and impose minimal GC overhead
  - optimized to minimize CPU consumption
- correctly handles UTF-8 strings passed by callers to provide status updates
- supports two tracking modes:
  - weight-based accumulation: callers specify the total known amount of work (e.g., 100 tasks, known a prioi)
  - fractional allocation: callers add the relative share of the total budget as work is discovered (e.g., recursively walking a directory to process its contents)
- transparently handles pipes, redirects, and non-TTY environments
- limited to ~18.4 million units of work

#### Example Usage

See the [`examples`](./examples) directory for examples of the modal API.
