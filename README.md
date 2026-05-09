[![Go Version](https://img.shields.io/badge/go-%20v1.26.2-00ADD8?logo=go)](https://github.com/jbunds/progress/blob/main/go.mod) &nbsp;
[![Go Reference](https://pkg.go.dev/badge/github.com/jbunds/progress.svg)](https://pkg.go.dev/github.com/jbunds/progress) &nbsp;
[![pre-commit](https://img.shields.io/badge/pre--commit-enabled-brightgreen?logo=pre-commit)](https://github.com/pre-commit/pre-commit) &nbsp;
[![tests](https://github.com/jbunds/progress/actions/workflows/test-go.yml/badge.svg)](https://github.com/jbunds/progress/actions/workflows/test-go.yml) &nbsp;
[![lint](https://github.com/jbunds/progress/actions/workflows/lint-go.yml/badge.svg)](https://github.com/jbunds/progress/actions/workflows/lint-go.yml) &nbsp;
[![coverage](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/jbunds/75dfc308f6a4adc36db4340cc020c713/raw/coverage.json)](https://github.com/jbunds/coverage/actions/workflows/test-go.yml)

#### Simple Terminal Progress Tracker

The `progress` package provides status updates to the terminal as units of work are incrementally completed.

Incremental calculations retain high-precision, while imposing minimal overhead upon callers.

---

#### Example Usage

See the [`examples`](./examples) directory for examples of the modal API.

---

Features include:

- race-free and lock-free by design:
  - concurrency-safe and well-suited to highly-scaled concurrent processing systems
  - uses Go concurrency primitives for synchronization
  - uses `atomic.Uint64` and `atomic.Pointer` types to:
    - provide lock-free updates of internal state for highly-scaled concurrent workloads
    - obviate mutex contention
    - minimize memory allocation and impose minimal GC overhead
    - enable fast and efficient UI synchronization via comparisons of:
      - string pointers (`atomic.Pointer[string]`)
      - string handles (`unique.Handle[string]`)
      - `atomic.Uint64` values
- context-aware:
  - correctly handles cancellation of the parent context, ensuring a clean exit under reasonable circumstances
- very efficient with minimal memory footprint:
  - optimized for low CPU usage, even at refresh rates beyond human perception
  - the use of a background rendering loop throttled at ~60 FPS combined with `atomic` types:
    - decouples the progress status tracker from the workers processing the workload
    - allows workers to report progress updates to the tracker asynchronously
    - ensures that UI rendering (which involves I/O and syscalls) never blocks the workers
  - dynamically pre-allocates a concurrency-safe, optimally-sized reusable `atomic.Pointer[[]byte]` buffer for all I/O:
    - increased dynamically as needed by a terminal window resize event listener
  - condenses all I/O operations into a single, atomic system call per frame, minimizing I/O latency
  - skips redundant UI redraws, further minimizing I/O and ensuring the terminal is never overwhelmed
  - uses bit-packed `atomic.Uint32` types and bitwise operations to further reduce memory allocation and efficiently handle updates of internal state
- supports two tracking modes:
  - **weight-based accumulation**: callers specify the total known amount of work (e.g., 100 tasks)
  - **fractional allocation**: callers add the relative share of the total budget as work is discovered (e.g., recursively traversing a directory to process its contents)
- supports multiple progress status tracking implementations which are well-suited to different sets of inputs:
  - `progress.Standard`: suitable for mostly unique status updates (uses `atomic.Pointer[string]`)
  - `progress.Unique`:   suitable for mostly repetitive status updates (uses `atomic.Value` and `unique.Handle[string]` to canonicalize status, further reducing memory footprint)
- supports multiple progress status formats:
  - `progress.Fraction`: writes progress status as a proper fraction (`x/y`) given a prescribed fixed total units of work (`y`)
  - `progress.Percent`:  writes only the percentage calculation to the terminal
- transparently handles pipes, redirections, and non-TTY environments
- correctly handles UTF-8 strings passed by callers
- supports concurrency-safe terminal window resizing, dynamically adapting the layout, formatting the rendered output accordingly, and ensuring layout and output integrity during concurrent writes to the terminal

Limitations:

- the precision of percentage calculations starts to progressively degrade at ~1 quadrillion (1e15) units of work
  - a workload capacity limited to 1 quadrillion units still allows for extremely fine-grained budget splitting and / or extremely deep recursion
- probably won't work on Windows, e.g., correctly handling terminal resize events
- truncated status updates may render incorrectly in terminals which lack UTF-8 support

See [![Go Reference](https://pkg.go.dev/badge/github.com/jbunds/progress.svg)](https://pkg.go.dev/github.com/jbunds/progress) for API documentation and [![DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/jbunds/progress) for technical internal implementation details.

---

#### Motivation

This library originated as an experiement to satisfy the author's curiosity about the practical application of modern, idiomatic Go features (e.g., `context`, `sync/atomic`, `unique`) in a context where concurrency is paramount after a nearly eight-year break from coding Go, back when 1.10 was the latest release.

The implementation was then iteratively optimized well beyond the point of overengineering, for fun and as a learning exercise.

---

#### Further Reading // Viewing

- [Zig progress bar](https://andrewkelley.me/post/zig-new-cli-progress-bar-explained.html) - [Andrew Kelley](https://andrewkelley.me/)
- [Why Progress Bars Don't Move Smoothly](https://www.youtube.com/watch?v=iZnLZFRylbs) - [Tom Scott](https://www.youtube.com/@TomScottGo)
- [Progress Bars](https://www.youtube.com/watch?v=uHh0qpc1BR4) - [Computerphile](https://www.youtube.com/@Computerphile)
