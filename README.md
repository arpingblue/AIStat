# AIStat

[English](README.md) | [简体中文](README.zh-CN.md)

AIStat is a lightweight, read-only inspection and performance-readiness tool for Linux NVIDIA compute nodes.

It collects host and workload facts, normalizes them into one node model, builds a topology graph, and evaluates 25 conservative rules. Every finding includes evidence, impact, a recommendation, and a verification step. AIStat does not tune the host, modify configuration, run benchmarks, require a daemon, or require network access during inspection.

> Know the node before you tune it.

## Project status

AIStat is preparing for `v0.1.0`.

- Windows portable tests and Linux-fixture tests pass.
- Ubuntu CPU-only GitHub Actions, race tests, static analysis, release cross-builds, installer simulation, and GoReleaser snapshot pass.
- A real NVIDIA Linux integration run is still required before publishing `v0.1.0`.
- No stable release tag has been published yet.

See [validation status](docs/validation.md) for the exact release gates.

## What AIStat inspects

| Layer | Coverage |
|---|---|
| Host | OS, kernel, architecture, CPU topology/cache/frequency, memory, huge pages, NUMA |
| I/O topology | PCIe hierarchy and link state, GPU-to-NUMA, GPU-to-GPU, GPU-to-NIC/RDMA, storage |
| NVIDIA stack | GPU inventory and health, driver, driver-supported CUDA, installed CUDA toolkits, NCCL, Xid events |
| Containers | Docker availability, NVIDIA Container Toolkit, cgroup/cpuset/memory, shared memory, GPU visibility |
| AI runtimes | Process affinity, PyTorch CUDA probe, vLLM and SGLang placement/parallelism |
| Judgment | Deployment readiness, performance readiness, and 25 evidence-backed rules |

The frozen rule catalog is documented in [docs/rules.md](docs/rules.md).

## Supported scope

- Linux `amd64` and `arm64`
- NVIDIA GPUs
- Docker and NVIDIA Container Toolkit
- PyTorch, vLLM, and SGLang best-effort discovery
- single-node inspection
- normal-user, read-only operation

Kubernetes, Slurm, multi-node inventory, AMD/Intel accelerators, monitoring, profiling, automatic tuning, host mutation, and benchmark execution are outside v0.1.

## Build from source

Go 1.26.5 is the pinned toolchain.

```bash
git clone https://github.com/arpingblue/AIStat.git
cd AIStat
go test ./...
CGO_ENABLED=0 go build -trimpath -o aistat ./cmd/aistat
./aistat version
```

Windows development commands:

```powershell
.\scripts\dev.ps1 test
.\scripts\dev.ps1 cross
```

The cross-build command produces Linux binaries; it does not claim to validate live Linux or NVIDIA collection.

## Installation

There is no stable binary release yet. After a version such as `v0.1.0` is published, the checksum-verifying installer can be used on Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/arpingblue/AIStat/main/scripts/install.sh | \
  AISTAT_VERSION=v0.1.0 sh
```

The installer needs network access. The installed `aistat` inspection command does not.

## Usage

Running `aistat` without a subcommand is equivalent to `aistat check`.

```text
aistat check [--format human|json] [--profile general|llm-inference]
aistat info [--format human|json]
aistat topology [--view tree|gpu|gpu-nic] [--format human|json]
aistat stack [--format human|json]
aistat runtime [--format human|json]
aistat explain RULE_ID [--format human|json]
aistat version [--format human|json]
```

Useful examples:

```bash
aistat check
aistat check --format json
aistat check --profile general --fail-on warn
aistat topology --view gpu-nic
aistat explain NUMA001
```

Every inspection has a bounded timeout (`10s` by default). `--fail-on warn` is useful in stricter CI policies.

### Exit codes

| Code | Meaning |
|---:|---|
| `0` | Inspection completed and no failing finding was produced |
| `1` | Inspection completed with a failing finding, or with a warning under `--fail-on warn` |
| `2` | Usage or internal execution error |

### Evidence states

Missing evidence is never treated as success. Collectors preserve states such as `available`, `not_detected`, `unsupported`, `permission_denied`, `timeout`, `parse_error`, and `unknown`. Rules can therefore return `pass`, `warn`, `fail`, `info`, `unknown`, or `skip` without inventing facts.

The versioned JSON contract is described in [docs/data-model.md](docs/data-model.md) and defined by [report-v0.1.schema.json](docs/schema/report-v0.1.schema.json).

## Architecture

```text
Collectors -> Facts -> Normalizer -> Snapshot -> Topology Graph
                                                    |
                                                    v
                                  Profile + Rules -> Report
```

- Collectors gather facts and diagnostics; they do not recommend changes.
- Rules consume only the normalized snapshot, topology graph, profile, and clock.
- Reporters render one canonical report model and do not recalculate rules.

More detail is available in [architecture](docs/architecture.md), [collectors](docs/collectors.md), and [topology](docs/topology.md).

## Privacy and safety

AIStat uses a fixed executable allowlist, never invokes a shell, caps command output, and terminates timed-out process trees. Process arguments and environments use narrow allowlists. Raw command lines, arbitrary environments, tokens, prompts, credentials, host mutation, and telemetry are excluded by design.

See [security design](docs/security.md) and [security policy](SECURITY.md).

## Testing

```bash
go test ./...
go vet ./...
make fuzz       # bounded parser fuzz runs
```

Normal CI does not require NVIDIA hardware. Sanitized fixtures cover portable parser, model, graph, rule, report, privacy, and failure-state behavior. Real NVIDIA validation remains a manual release gate and must be recorded in [docs/validation.md](docs/validation.md).

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md). Rule changes require trigger, pass, unknown, skip, boundary, and relevant false-positive regression tests.

## License

[Apache License 2.0](LICENSE).
