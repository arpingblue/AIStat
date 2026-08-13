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
| docker | version, info, ps, inspect; package database; standard CDI specs | `docker`, `nvidia-ctk`, `nvidia-container-cli`, `nvidia-container-runtime`, `dpkg-query`/`rpm` |
| runtime | sanitized process facts, bounded package metadata discovery, fixed isolated PyTorch probe | `python3 -I -c <fixed probe>` |

Optional tool absence is a fact state, not an internal error. Parsers are platform-neutral; traversal is injected. All command calls have deadlines and output limits.

The NVIDIA stack collector preserves multiple installed CUDA Toolkits, the active `/usr/local/cuda` version, forward-compatibility package presence, BAR1 counters, compute PIDs, Xid state, and topology affinity separately. It does not treat multiple Toolkits as a failure.

Runtime discovery records the runtime kind before command-line redaction, then separates installation state from execution state. Installation discovery reads `*.dist-info/METADATA` only: current-user and standard system/Conda prefixes plus readable roots of running containers. It is bounded to two seconds, 128 environments, 4096 directory entries, depth four, and 256 KiB per metadata file. It does not import runtime packages, execute inside containers, scan stopped images, or traverse other users' home directories.

NVIDIA Container Toolkit installation is evaluated independently from Docker GPU configuration. AIStat correlates the distro package database, three toolkit commands, Docker's registered runtimes, standard CDI directories, Docker-reported CDI directories, and running GPU device requests. Only a fully inspectable, consistently absent evidence set becomes `not_detected`; permission, timeout, or parse gaps remain explicit.

Docker client presence and daemon inspectability are separate facts. A denied Docker API produces `permission_denied`; it is never converted to “Docker/vLLM not installed.”
