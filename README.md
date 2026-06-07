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

[Run](./examples/README.md) the example programs to see the UI in action.

Here are a couple of real-world examples:
- [github.com/jbunds/coverage/main.go](https://github.com/jbunds/coverage/blob/main/main.go#:~:text=progress.New)
- [github.com/jbunds/coverage/tree.go](https://github.com/jbunds/coverage/blob/main/tree.go#:~:text=progress.New)

---

Features include:

- race-free and lock-free by design:
  - concurrency-safe and well-suited to highly-scaled concurrent processing systems
  - uses Go concurrency primitives for synchronization
  - uses atomic operations to:
    - provide lock-free updates of internal state for highly-scaled concurrent workloads
    - obviate mutex contention
    - minimize memory allocation and impose minimal GC overhead
    - enable fast and efficient UI synchronization via comparisons of:
      - precise internal progress values via `atomic.Uint32` and `atomic.Uint64` types
      - progress status strings (`atomic.Pointer[string]` in the `progress.Standard` tracker; `atomic.Uint64` in the `progress.Fraction` tracker), or
      - canonicalized (interned) progress status string handles (`unique.Handle[string]` guarded by `atomic.Value` in the `progress.Unique` tracker)
- context-aware:
  - correctly handles cancellation of the parent context, ensuring a clean exit under reasonable circumstances
- very efficient with minimal memory footprint:
  - optimized for very low CPU usage, even at refresh rates beyond human perception
  - the use of a background rendering loop throttled at ~60 FPS combined with `atomic` types:
    - decouples the progress status tracker from the workers processing the workload
    - allows workers to report progress updates to the tracker asynchronously
    - ensures that UI rendering (which involves I/O and syscalls) never blocks workers
  - condenses all I/O operations into a single, atomic system call per frame, minimizing I/O latency
  - skips redundant UI redraws, further minimizing I/O and ensuring the terminal is never overwhelmed with I/O
  - uses bit-packed `atomic.Uint32` types and bitwise operations to further reduce memory allocation and efficiently handle precise updates of internal state
- supports two tracking modes:
  - **weight-based accumulation**: callers specify the total known amount of work (e.g., 100 tasks)
  - **fractional allocation**: callers add the relative share of the total budget as work is discovered (e.g., recursively traversing a directory to process its contents)
- supports multiple progress status tracking implementations which are well-suited to different sets of inputs:
  - `progress.Standard`: suitable for mostly unique status updates (uses `atomic.Value`)
  - `progress.Unique`:   suitable for mostly repetitive status updates (uses `atomic.Value` and `unique.Handle[string]` to canonicalize (intern) status
- supports multiple progress status formats:
  - `progress.Fraction`: writes progress status as a proper fraction (`x/y`) given a prescribed fixed total units of work (`y`)
  - `progress.Percent`:  writes only the percentage calculation to the terminal
- transparently handles pipes, redirections, and non-TTY environments
- correctly and efficiently handles UTF-8 status strings passed by callers
- supports concurrency-safe terminal window resizing by:
  - dynamically adapting the layout
  - formatting the rendered output accordingly
  - ensuring layout and output integrity during concurrent writes to the terminal
- supports [various color schemes](./themes.go) for rendering the background colors of the progress bar
  - the [`all_themes.sh`](./all_themes.sh) script demonstrates all available themes

Limitations:

- the precision of percentage calculations starts to progressively degrade at ~1 quadrillion (1e15) units of work
  - a workload capacity limited to 1 quadrillion units still allows for extremely fine-grained budget splitting and / or extremely deep recursion
- handling of terminal resize events on Windows systems is not supported
- truncated status updates may render incorrectly in terminal emulators which lack UTF-8 support
- has not been tested on terminal emulators which lack support for rendering 24-bit colors

See [![Go Reference](https://pkg.go.dev/badge/github.com/jbunds/progress.svg)](https://pkg.go.dev/github.com/jbunds/progress) for API documentation and [![DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/jbunds/progress) for technical internal implementation details.

---

#### Motivation

This library originated as an experiement to satisfy the author's curiosity about the practical application of modern, idiomatic Go features (e.g., `context` for modern handling of cancelation and deadlines across API boundaries, lock-free synchronization and thread-safety via `sync/atomic` and CAS loops, set canonicalization (interning) via `unique` and `comparable`, etc) in a context where concurrency is paramount after a nearly eight-year break from coding Go, back when 1.10 was the latest release.

The implementation was then iteratively optimized well beyond the point of overengineering, for fun and as a learning exercise.

---

#### Further Reading // Viewing

- [Zig progress bar](https://andrewkelley.me/post/zig-new-cli-progress-bar-explained.html) - [Andrew Kelley](https://andrewkelley.me/)
- [Why Progress Bars Don't Move Smoothly](https://www.youtube.com/watch?v=iZnLZFRylbs) - [Tom Scott](https://www.youtube.com/@TomScottGo)
- [Progress Bars](https://www.youtube.com/watch?v=uHh0qpc1BR4) - [Computerphile](https://www.youtube.com/@Computerphile)
