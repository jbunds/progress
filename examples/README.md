## Examples

The `progress` package includes standalone work simulators to demonstrate UI behavior in different modes.

### Weight-Based Accumulation

Simulates a workload where the total number of units comprising the workload may be unknown, or their relative weights may be unknown, by using the `AddTotal` method to enable the progress tracker to refine its denominator as work is incrementally completed:

```
go run -tags examples github.com/jbunds/progress/examples/weight-based
```

### Fractional Path Allocation

Simulates a recursively-discovered workload (i.e., files to be processed) via filesystem traversal using the `InitialBudget()` pattern:

```
go run -tags examples github.com/jbunds/progress/examples/fractional
```
