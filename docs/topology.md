# Topology

Graph nodes include host, CPU package, NUMA node, CPU, PCI root/bridge/device, GPU, NIC, RDMA device, block device, process, and container. Edges express containment, attachment, NUMA locality, runtime use, container membership, GPU P2P connectivity, and machine-reported accelerator/device paths.

The NVIDIA topology parser preserves GPU↔GPU and GPU↔NIC/NVMe paths plus GPU CPU/NUMA affinity. Unknown future tokens remain structured with status `unknown`. The graph builder performs no I/O. Unknown parents or topology tokens degrade gracefully. `RootForPCI`, `LocalNUMA`, `Neighbors`, and `Distance` are stable query primitives used by topology and placement rules.

`TOPO001` only recommends an alternative GPU set when a same-size visible set is no worse on every known pair and strictly better on at least one pair. AIStat never reorders devices automatically.
