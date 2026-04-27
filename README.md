#### Simple Terminal Progress Tracker

The `progress` package provides status updates to the terminal as units of work are incrementally completed.

Incremental calculations retain high-precision, while imposing minimal overhead upon callers.

- context-aware
- concurrency-safe:
  - uses `sync/atomic` to provide lock-free updates of internal state for highly-scaled concurrent workloads
- very efficient:
  - throttles status updates at ~60 FPS
  - uses `atomic.Uint32` and `atomic.Pointer[string]` to minimize memory allocation and impose minimal GC overhead
  - optimized to minimize CPU consumption
- correctly handles UTF-8 strings passed by callers to provide status updates
- supports two tracking modes:
  - weight-based accumulation: callers specify the total known amount of work (e.g., 100 tasks, known a prioi)
  - fractional allocation: callers add the relative share of the total budget as work is discovered (e.g., recursively walking a directory to process its contents)
- limited to ~18.4 million units of work
- transparently handles pipes, redirects, and non-TTY environments

#### Example Usage

See the [`examples`](tree/main/examples) directory for examples of the modal API.
