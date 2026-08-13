# AIStat

AIStat is a lightweight, read-only AI infrastructure inspection and performance-readiness tool for Linux NVIDIA compute nodes.

It inventories host, NUMA, PCIe, GPU, network, RDMA, storage, container, and AI-runtime state; builds a normalized topology graph; and evaluates 25 conservative, evidence-backed readiness rules. It does not tune the host, change configuration, start benchmarks, or require a daemon.

## Status

The repository targets `v0.1.0`. Portable logic and Linux-fixture tests run on Windows. Linux CPU-only CI and real NVIDIA Linux validation are separate release gates; Windows results alone are not evidence that NVIDIA collection works on production hardware.

Current verification evidence and the remaining release gates are recorded in [docs/validation.md](docs/validation.md).

## Build

Go 1.26.5 is the pinned toolchain.

```bash
go test ./...
go vet ./...
CGO_ENABLED=0 go build -trimpath -o bin/aistat ./cmd/aistat
```

Windows developers can run:

```powershell
.\scripts\dev.ps1 test
.\scripts\dev.ps1 cross
```

## Usage

```text
aistat check [--format human|json] [--profile general|llm-inference]
aistat info [--format human|json]
aistat topology [--view tree|gpu|gpu-nic] [--format human|json]
aistat stack [--format human|json]
aistat runtime [--format human|json]
aistat explain RULE_ID [--format human|json]
aistat version [--format human|json]
```

Every inspection has a bounded timeout (10 seconds by default). `--fail-on warn` makes warnings fail CI; the default exit code is non-zero only for a rule failure or an internal/usage error.

The versioned JSON contract is documented in [docs/data-model.md](docs/data-model.md) and defined by [docs/schema/report-v0.1.schema.json](docs/schema/report-v0.1.schema.json). JSON enums are lowercase. Missing evidence becomes `unknown`, never an implicit pass.

## Supported scope

- Linux amd64 and arm64
- NVIDIA GPUs
- Docker and NVIDIA Container Toolkit
- PyTorch, vLLM, and SGLang best-effort runtime discovery
- single-node inspection

Kubernetes, Slurm, cluster inventory, AMD/Intel accelerators, profiling, automatic tuning, configuration mutation, and benchmark execution are outside v0.1.

## Privacy and safety

AIStat uses fixed command allowlists and arguments, never invokes a shell, caps output, and redacts sensitive fixture content. Runtime environment and arguments use narrow allowlists; tokens, model prompts, arbitrary environment variables, IP addresses, and MAC addresses are not intended for reports. See [SECURITY.md](SECURITY.md).

## License

Apache-2.0.
