# Architecture

AIStat uses a one-way, testable pipeline:

```text
/proc + /sys + bounded tools
        -> collectors/raw facts
        -> normalizer/typed snapshot
        -> base hardware graph
        -> runtime resolution
        -> enriched graph
        -> pure rule engine
        -> immutable report
        -> human or JSON renderer
```

Collectors implement `ID`, `Provides`, `Requires`, and `Collect(context, Env)`. The registry rejects duplicate providers, unknown requirements, and dependency cycles. `Env` injects the command runner, filesystem, clock, and platform so Linux fixtures run unchanged on Windows.

The normalizer is the only layer that maps fact keys into the typed snapshot. The first graph pass contains host, NUMA, CPU, PCI, GPU, NIC, and RDMA relationships. Runtime enrichment adds process, container, CPU-use, and GPU-use edges without I/O.

Rules are pure functions of snapshot, graph, profile, and injected time. The engine sorts deterministically and computes deployment and performance readiness once. Status, check, inventory, stack, runtime, topology, and JSON reporters consume that same report model; reporters never recollect or reinterpret host facts.
