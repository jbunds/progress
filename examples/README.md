## Examples

The `progress` package includes standalone work simulators to demonstrate UI behavior in different modes.

### Weight-Based Accumulation

Simulates a fixed-batch workload where the total number of units is known a priori:

```
go run weight-based/main.go
```

### Fractional Path Allocation

Simulates a recursively discovered workload via filesystem traversal using the `InitialBudget()` pattern:

```
go run fractional/main.go
```
