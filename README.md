# AIStat

**English** | [简体中文](README.zh-CN.md)

AIStat checks Linux NVIDIA servers for GPU topology, the CUDA stack, containers, AI runtimes, and deployment problems.

It is a single, read-only binary for engineers who need to understand an unfamiliar GPU node before deploying or troubleshooting PyTorch, vLLM, or SGLang.

[![Release](https://img.shields.io/github/v/release/arpingblue/AIStat)](https://github.com/arpingblue/AIStat/releases/latest)
[![CI](https://github.com/arpingblue/AIStat/actions/workflows/ci.yml/badge.svg)](https://github.com/arpingblue/AIStat/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

## Install

Linux `amd64` and `arm64` are supported. This installs the latest release for the current user:

```bash
curl -fsSL https://raw.githubusercontent.com/arpingblue/AIStat/main/scripts/install.sh | sh
```

The binary is installed to `~/.local/bin/aistat`. If that directory is not on your `PATH`, the installer prints the command to add it.

To install a specific version or directory:

```bash
curl -fsSL https://raw.githubusercontent.com/arpingblue/AIStat/main/scripts/install.sh | \
  AISTAT_VERSION=v0.1.0 AISTAT_INSTALL_DIR=/your/bin sh
```

Release archives and checksums are also available on the [Releases page](https://github.com/arpingblue/AIStat/releases/latest).

## Usage

Run `aistat` for a quick node overview:

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

Common commands:

```bash
aistat                             # quick status
aistat check                       # detailed diagnosis
aistat check --format json         # complete JSON report
aistat info                        # hardware inventory
aistat stack                       # NVIDIA and container stack
aistat runtime                     # PyTorch, vLLM and SGLang
aistat topology --view gpu-nic     # GPU/NIC/NUMA topology
aistat explain CTR002              # explain one rule
```

## What it checks

| Area | Collected and diagnosed |
|---|---|
| Hardware | CPU, memory, NUMA, PCIe, NVIDIA GPUs, storage |
| GPU topology | GPU↔GPU P2P, GPU↔NUMA, GPU↔NIC/RDMA |
| NVIDIA stack | Driver, CUDA capability and toolkits, NCCL, Xid visibility |
| Containers | Docker client/daemon, NVIDIA Container Toolkit, runtime/CDI, GPU requests |
| AI runtimes | PyTorch, vLLM, SGLang installations and active processes |
| Diagnosis | Deployment readiness, performance readiness, 25 rules |

AIStat distinguishes an absent component from an inspection failure. For example, Docker permission denial is not reported as “Docker is not installed,” and an unreadable kernel log is not reported as “no Xid errors.”

## Commands

| Command | Purpose |
|---|---|
| `aistat` / `aistat status` | Short node summary and top actions |
| `aistat check` | Findings, evidence, impact, recommendations, and verification |
| `aistat info` | Hardware inventory |
| `aistat topology` | Compact topology tree and GPU P2P matrix |
| `aistat stack` | NVIDIA, CUDA, Docker, and Container Toolkit |
| `aistat runtime` | Runtime installations and running instances |
| `aistat explain RULE_ID` | Rule details |
| `aistat version` | Version and build metadata |

All commands support bounded execution through `--timeout`. Human output uses color only in a terminal; JSON, redirected output, `--no-color`, and `NO_COLOR` contain no ANSI sequences.

Exit codes:

| Code | Meaning |
|---:|---|
| `0` | Inspection completed without a failing finding |
| `1` | A FAIL was found, or a WARN with `--fail-on warn` |
| `2` | Invalid arguments or an internal execution error |

## Safety

AIStat is read-only. It does not:

- change drivers, sysctls, Docker configuration, groups, or workload placement;
- start containers, import Python packages, or run benchmarks;
- use a shell for external commands;
- upload reports or send telemetry.

External commands use a fixed allowlist, timeout, output limit, and process-tree cleanup. Runtime discovery reads bounded package metadata and does not scan other users' home directories. See [security](docs/security.md) for details.

## Scope

Version `0.1.0` covers a single Linux NVIDIA node. Kubernetes, Slurm, multi-node diagnosis, monitoring, profiling, benchmarks, and automatic tuning are not included yet.

Performance readiness may remain `UNKNOWN` when no active workload or time-series evidence exists. AIStat reports the missing evidence instead of guessing.

The next major step is before/after report comparison so recommendations can be verified. See [validation](docs/validation.md), [architecture](docs/architecture.md), [collectors](docs/collectors.md), [rules](docs/rules.md), and the [JSON Schema](docs/schema/report-v0.1.schema.json).

## Build

Go 1.26.5 is used for development:

```bash
git clone https://github.com/arpingblue/AIStat.git
cd AIStat
go test ./...
CGO_ENABLED=0 go build -trimpath -o aistat ./cmd/aistat
```

## Contributing

Issues and pull requests are welcome. Reports from real Linux NVIDIA servers are especially useful when the shared output has been sanitized.

- **Found a bug?** Open an [issue](https://github.com/arpingblue/AIStat/issues) with the AIStat version, system information, command, expected behavior, and sanitized output.
- **Have an idea?** Describe the operational problem, the evidence AIStat could collect, and how the result could be verified.
- **Want to contribute code?** Keep the pull request focused, add tests, preserve read-only behavior, and update both languages when user-facing behavior changes.

### Before opening an issue

Include the following when applicable:

- output from `aistat version`;
- Linux distribution, kernel, architecture, and GPU model;
- the exact command, expected behavior, and actual behavior;
- a minimal output or JSON excerpt;
- whether Docker, kernel logs, or process inspection were permission-restricted.

Remove hostnames, usernames, IP and MAC addresses, container IDs, model paths, prompts, tokens, and customer information. Do not upload an unreviewed support bundle. Security vulnerabilities should follow [SECURITY.md](SECURITY.md), not a public issue.

### Before opening a pull request

Run:

```bash
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
```

A pull request should explain the user-visible problem, include tests, and avoid unrelated changes. Large architecture changes, new external commands, report-schema changes, and new diagnostic rules should be discussed in an issue first.

Collector changes must remain read-only and bounded. New external commands require a fixed allowlist entry, fixed arguments without shell interpolation, timeout and output limits, and tests for absence, denial, timeout, and malformed output.

Rule changes must cover trigger, pass, missing evidence, not-applicable behavior, boundaries, and relevant false positives. Missing evidence must never become PASS. Public report changes must update the [JSON Schema](docs/schema/report-v0.1.schema.json) and matching tests.

By contributing, you agree that your contribution is licensed under the repository's Apache License 2.0.

## License

[Apache License 2.0](LICENSE)
