# AIStat

[English](README.md) | [简体中文](README.zh-CN.md)

**AI infrastructure inspection, diagnostics, and optimization intelligence for LLM deployment and high-performance inference.**

AIStat unifies the analysis of hardware topology, the CUDA software stack, containers, and inference runtimes on Linux NVIDIA nodes. Its purpose is to find deployment blockers and performance bottlenecks, explain the evidence behind them, and produce optimization recommendations that engineers can verify.

The long-term goal is not another GPU inventory command. AIStat is being built to turn GPU-server facts into an explainable optimization workflow for large-model deployment and inference: understand every layer of the node, diagnose cross-layer problems, recommend changes, and verify whether those changes actually improve the workload.

> **Understand the node. Diagnose the stack. Optimize the inference path.**

## Project status

AIStat is preparing for `v0.1.0`.

- Windows portable tests and Linux-fixture tests pass.
- Ubuntu CPU-only GitHub Actions, race tests, static analysis, release cross-builds, installer simulation, and GoReleaser snapshot pass.
- A real NVIDIA Linux integration run is still required before publishing `v0.1.0`.
- No stable release tag has been published yet.

See [validation status](docs/validation.md) for the exact release gates.

## Product vision

```text
Inspect -> Model -> Diagnose -> Recommend -> Validate -> Optimize
```

AIStat is designed around a progressive optimization loop:

1. **Inspect:** collect trustworthy facts from hardware, Linux, NVIDIA, containers, and inference runtimes.
2. **Model:** connect CPU, NUMA, PCIe, GPU, NIC/RDMA, CUDA, containers, and processes in one topology-aware node model.
3. **Diagnose:** identify deployment blockers, compatibility failures, resource-placement mistakes, and performance bottlenecks.
4. **Recommend:** produce evidence-backed optimization plans instead of generic tuning advice.
5. **Validate:** compare the relevant facts and workload results before and after a change.
6. **Optimize:** evolve toward a controlled, auditable optimization workflow for GPU servers running large-model inference.

The current `v0.1` is the read-only foundation of that vision. It focuses on node visibility, normalized modeling, topology, runtime context, and conservative diagnostics. It does **not** yet claim automatic tuning or autonomous host modification.

## Current v0.1 foundation

| Layer | Coverage |
|---|---|
| Host | OS, kernel, architecture, CPU topology/cache/frequency, memory, huge pages, NUMA |
| I/O topology | PCIe hierarchy and link state, GPU-to-NUMA, GPU-to-GPU, GPU-to-NIC/RDMA, storage |
| NVIDIA stack | GPU inventory and health, driver, driver-supported CUDA, installed CUDA toolkits, NCCL, Xid events |
| Containers | Docker availability, NVIDIA Container Toolkit, cgroup/cpuset/memory, shared memory, GPU visibility |
| AI runtimes | Process affinity, PyTorch CUDA probe, vLLM and SGLang placement/parallelism |
| Diagnosis | Deployment readiness, performance readiness, and 25 evidence-backed rules |

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
