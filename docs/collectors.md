# Collectors

| Collector | Primary sources | External tools |
|---|---|---|
| system | `/etc/os-release`, kernel procfs | none |
| cpu/memory/numa | procfs plus CPU/cache/NUMA sysfs | none |
| pci | PCI sysfs | `lspci` optional ACS enrichment |
| network/RDMA | net and infiniband sysfs | none |
| storage | `/proc/mounts` | none |
| nvidia | NVIDIA query/topology output, CUDA/NCCL files | `nvidia-smi`, `dmesg` |
| process | allowlisted procfs status, cgroup, args, env | none |
| docker | version, info, ps, inspect | `docker`, `nvidia-ctk` |
| runtime | sanitized process facts and fixed isolated probe | `python3 -I -c <fixed probe>` |

Optional tool absence is a fact state, not an internal error. Parsers are platform-neutral; traversal is injected. All command calls have deadlines and output limits.

The NVIDIA stack collector preserves multiple installed CUDA Toolkits, the active `/usr/local/cuda` version, forward-compatibility package presence, BAR1 counters, compute PIDs, Xid state, and topology affinity separately. It does not treat multiple Toolkits as a failure.
