<div align="center">
  <img src="docs/assets/aistat-banner.svg" alt="AIStat — NVIDIA AI Infra diagnostics" width="100%">

  <p><strong>Understand an unfamiliar NVIDIA node in one command.</strong></p>
  <p>Read-only inspection and evidence-based diagnostics for LLM deployment and high-performance inference.</p>

  <p>
    <a href="https://github.com/arpingblue/AIStat/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/arpingblue/AIStat?style=flat-square&color=28c780"></a>
    <a href="https://github.com/arpingblue/AIStat/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/arpingblue/AIStat/ci.yml?branch=main&style=flat-square&label=CI"></a>
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue?style=flat-square"></a>
    <img alt="Linux" src="https://img.shields.io/badge/Linux-amd64%20%7C%20arm64-f0b90b?style=flat-square&logo=linux&logoColor=black">
    <img alt="NVIDIA" src="https://img.shields.io/badge/NVIDIA-GPU-76B900?style=flat-square&logo=nvidia&logoColor=white">
  </p>

  <p><a href="README.zh-CN.md">简体中文</a> · <a href="#install">Install</a> · <a href="#commands">Commands</a> · <a href="docs/architecture.md">Architecture</a> · <a href="docs/rules.md">Rules</a></p>
</div>

---

AIStat is an AI infrastructure inspection and diagnosis tool for Linux NVIDIA nodes. It connects hardware topology, the CUDA software stack, containers, and inference runtimes into one operator-focused report—so deployment blockers and performance risks are visible before they become an incident.

It is not another `nvidia-smi` wrapper. AIStat is the read-only foundation for a larger goal: turning GPU-server facts into a verifiable optimization workflow for large-model deployment and inference.

```text
Inspect  →  Model  →  Diagnose  →  Recommend  →  Validate  →  Optimize
```

## See the node, not a pile of commands

Run `aistat` after SSHing into a server:

```text
AIStat 0.1.0 — Node Status

Hardware
  GPUs       AVAILABLE 4 × NVIDIA L20
  CPU        AVAILABLE 2 sockets, 96 cores, 192 logical
  Memory     AVAILABLE 1007.4 GiB
  NUMA       AVAILABLE 2 nodes

NVIDIA Stack
  Driver     PASS      580.159.03
  CUDA       AVAILABLE driver capability 13.0; selected toolkit 13.0
  Xid log    PERMISSION DENIED kernel log could not be fully inspected

Containers
  Client     AVAILABLE docker 29.5.2
  Daemon     AVAILABLE 29.5.2
  NVIDIA CTK AVAILABLE 1.19.0

AI Runtimes
  PyTorch    installed=AVAILABLE running=NOT DETECTED instances=0
  vLLM       installed=AVAILABLE running=NOT DETECTED instances=0
  SGLang     installed=NOT DETECTED running=NOT DETECTED instances=0

Readiness
  Deployment  READY
  Performance UNKNOWN — runtime workload evidence is not available
```

The result is deliberately conservative: unavailable evidence is reported as unavailable, never converted into a false PASS.

## What AIStat connects

| Layer | What AIStat inspects |
|---|---|
| **Hardware** | CPU, memory, NUMA, PCIe, GPU, NIC/RDMA, storage |
| **NVIDIA stack** | Driver, CUDA capability and toolkits, NCCL, Xid visibility |
| **Containers** | Docker client/daemon, NVIDIA Container Toolkit, runtime/CDI, GPU requests |
| **AI runtimes** | PyTorch, vLLM, SGLang installations and active process context |
| **Topology** | GPU↔GPU P2P, GPU↔NUMA, GPU↔NIC/RDMA placement |
| **Diagnosis** | Deployment readiness, performance readiness, 25 evidence-backed rules |

Typical questions it answers:

- Can this node deploy a GPU inference workload right now?
- Is the driver/CUDA/runtime path coherent?
- Is Docker unavailable, stopped, or merely blocked by permissions?
- Is NVIDIA Container Toolkit installed and configured?
- Where are PyTorch, vLLM, and SGLang installed, and are they running?
- Are GPUs and network devices placed across NUMA boundaries?
- Which findings are confirmed, and which remain inspection gaps?

## Install

### User-only installation

No root access is required. The installer verifies the release checksum and writes only to `~/.local/bin`:

```bash
mkdir -p "$HOME/.local/bin"
curl -fsSL https://raw.githubusercontent.com/arpingblue/AIStat/v0.1.0/scripts/install.sh | \
  AISTAT_VERSION=v0.1.0 AISTAT_INSTALL_DIR="$HOME/.local/bin" sh
export PATH="$HOME/.local/bin:$PATH"
aistat version
```

Persist the path for future shells if necessary:

```bash
printf '\nexport PATH="$HOME/.local/bin:$PATH"\n' >> "$HOME/.bashrc"
```

### Manual installation

Download the archive for `linux-amd64` or `linux-arm64` from the [latest release](https://github.com/arpingblue/AIStat/releases/latest), verify it with `checksums.txt`, extract `aistat`, and place it in a directory on your `PATH`.

AIStat is a statically linked single binary. The inspection itself does not access the network, install packages, start services, or modify the host.

## Commands

| Command | Use it for |
|---|---|
| `aistat` / `aistat status` | Fast operator overview and top actions |
| `aistat check` | Detailed findings, evidence, impact, recommendations, and verification |
| `aistat info` | Hardware and node inventory |
| `aistat stack` | NVIDIA, CUDA, Docker, and Container Toolkit state |
| `aistat runtime` | PyTorch, vLLM, and SGLang installations and instances |
| `aistat topology` | Compact NUMA/GPU/NIC tree and GPU P2P matrix |
| `aistat explain RULE_ID` | Explain one diagnostic rule |
| `aistat version` | Build version, commit, and time |

Useful examples:

```bash
# Fast human-readable status
aistat

# Full machine-readable report
aistat check --format json > aistat-report.json

# Detailed diagnosis; warnings also fail CI
aistat check --profile llm-inference --fail-on warn

# GPU and physical NIC/RDMA locality
aistat topology --view gpu-nic

# Explain a finding
aistat explain CTR002
```

Human output uses color only on an interactive terminal. JSON, redirected output, `--no-color`, and `NO_COLOR` never contain ANSI control sequences.

## Designed for trustworthy diagnostics

**Evidence before conclusions.** Facts distinguish `available`, `not_detected`, `unsupported`, `permission_denied`, `timeout`, `parse_error`, and `unknown`. Inspection gaps stay visible.

**Read-only by default.** AIStat does not tune sysctls, alter drivers, change Docker groups, launch containers, run benchmarks, or write configuration.

**Bounded collection.** External commands use a fixed allowlist, no shell, strict deadlines, output limits, and process-tree cleanup.

**Runtime-aware without execution.** Package discovery reads bounded metadata. It does not `import vllm`, scan other users' homes, use `docker exec`, or start a workload.

**One canonical JSON report.** Human views are projections of the same versioned model defined by the [0.1 JSON Schema](docs/schema/report-v0.1.schema.json).

## Topology that operators can read

```text
Host
├── NUMA 0  CPUs 0-47,96-143
│   ├── GPU0  NVIDIA L20  0000:3b:00.0
│   └── NIC   mlx5_0 / ens6f0  0000:41:00.0
└── NUMA 1  CPUs 48-95,144-191
    ├── GPU2  NVIDIA L20  0000:9a:00.0
    └── NIC   mlx5_2 / ens12f0 0000:a1:00.0

GPU P2P
      GPU0 GPU1 GPU2 GPU3
GPU0   —   PIX  SYS  SYS
GPU1  PIX   —   SYS  SYS
GPU2  SYS  SYS   —   PIX
GPU3  SYS  SYS  PIX   —
```

The default human view stays compact. The JSON report retains the complete CPU, process, PCIe, and topology graph for automation.

## Scope of v0.1.0

The first public release supports Linux `amd64` and `arm64`, NVIDIA GPUs, Docker/NVIDIA Container Toolkit, and best-effort discovery of PyTorch, vLLM, and SGLang on a single node.

It is a diagnostic foundation—not yet a benchmark suite, profiler, monitoring daemon, Kubernetes/Slurm inventory system, or automatic tuning engine. Performance readiness may remain `UNKNOWN` when there is no active workload or time-series evidence; the report explains why.

See [validation](docs/validation.md) for tested environments and current limits.

## Roadmap

- **v0.2 — Verify:** compare before/after reports and validate optimization changes.
- **v0.3 — Observe:** bounded short-window sampling for GPU, PCIe, NVLink, NUMA, CPU pressure, and RDMA.
- **v0.4 — Diagnose inference:** correlate vLLM/SGLang/PyTorch placement and parallelism with bottleneck evidence.
- **Later:** optional DCGM/eBPF adapters, controlled benchmarks, offline HTML reports, and a read-only MCP interface.

The product direction remains: **understand the node, diagnose the stack, optimize the inference path—then prove the change helped.**

## Build and contribute

AIStat uses Go 1.26.5:

```bash
git clone https://github.com/arpingblue/AIStat.git
cd AIStat
go test ./...
CGO_ENABLED=0 go build -trimpath -o aistat ./cmd/aistat
```

Read [CONTRIBUTING.md](CONTRIBUTING.md), the [architecture](docs/architecture.md), [collector contract](docs/collectors.md), [rule catalog](docs/rules.md), and [security model](docs/security.md) before contributing.

## License

AIStat is available under the [Apache License 2.0](LICENSE).
