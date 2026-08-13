# Changelog

All notable changes to AIStat are documented in this file.

## [0.1.0] - 2026-08-13

AIStat's first public release establishes the read-only diagnostic foundation for Linux NVIDIA inference nodes.

### Highlights

- Added `aistat status` and made it the default operator overview.
- Added hardware, NVIDIA stack, container, runtime, readiness, and top-action sections.
- Added compact NUMA/GPU/NIC topology trees and GPU P2P matrices.
- Added bounded installation and process discovery for PyTorch, vLLM, and SGLang.
- Distinguished Docker client, daemon, permission, NVIDIA runtime, CDI, and package evidence.
- Added explicit inspection-gap reasons instead of treating unavailable evidence as PASS.
- Added terminal-aware status colors with ANSI-free JSON and redirected output.
- Added 25 evidence-backed diagnostic rules and a versioned JSON Schema 0.1 report.
- Added Linux `amd64` and `arm64` release builds, checksums, provenance attestation, and a checksum-verifying installer.
- Added a one-command, user-local installer that selects the host architecture and downloads the latest release.

### Safety

- Read-only inspection with no daemon, telemetry, automatic host changes, or benchmark execution.
- Fixed external-command allowlist, no shell interpolation, bounded output, and process-tree timeout cleanup.
- Narrow process/environment collection and credential-safe runtime classification.

### Known limits

- Linux and NVIDIA only; single-node scope.
- Performance readiness can remain unknown without an active workload or time-series evidence.
- Kubernetes, Slurm, multi-node diagnosis, profiling, benchmarks, and automatic tuning are not part of v0.1.0.

[0.1.0]: https://github.com/arpingblue/AIStat/releases/tag/v0.1.0
