# Validation status

Validation recorded on 2026-08-13.

## Completed on Windows

- `go test ./...`
- `go vet ./...`
- staticcheck 2026.1
- Windows CLI build and JSON smoke check
- timeout descendant cleanup through a Windows Job Object
- Linux `amd64` and `arm64` cross-builds with `CGO_ENABLED=0`
- GoReleaser v2.17.1 configuration check and snapshot release
- non-publishing tagged `v0.1.0` GoReleaser dry run, including two archives and checksums

The cross-built uncompressed binaries are below 6 MiB. GoReleaser snapshot archives are below 2 MiB.

## CI gates

Ubuntu CI runs race-enabled tests, vet, staticcheck, CPU-only CLI JSON smoke, shell syntax checks, an offline installer simulation, both release builds, and a GoReleaser snapshot. Windows CI runs all portable logic and Linux-fixture tests and both cross-builds.

The first complete GitHub-hosted validation passed on 2026-08-13:

- [GitHub Actions run 31678644444](https://github.com/arpingblue/AIStat/actions/runs/31678644444)
- Linux job: formatting, race tests, vet, staticcheck, CPU-only JSON smoke, `amd64`/`arm64` builds, installer verification, and GoReleaser snapshot passed.
- Windows job: portable logic, Linux fixtures, vet, and both Linux cross-builds passed.

Bounded parser fuzz targets cover CPU lists, NUMA lists, meminfo, NVIDIA CSV/topology, PCI ACS, and allowlisted runtime arguments. CI executes short fuzz smoke sessions; longer runs are available through `make fuzz`.

## NVIDIA field validation

- Multiple normal-user runs on a four-GPU NVIDIA L20 node completed on 2026-08-13. They validated the static Linux binary, GPU/PCIe/NUMA inventory, all six GPU P2P pairs, Docker permission and daemon states, host Python package discovery, JSON output, the rule registry, and read-only execution.
- Those runs exposed ANSI-decorated topology headers, Docker permission UX, focused-topology output, NVIDIA Container Toolkit evidence gaps, and runtime classification gaps. Each issue has a sanitized regression test in the `v0.1.0` source.
- A complete active vLLM/SGLang workload run has not yet been recorded. Runtime process parsing and installation discovery are covered by sanitized fixtures, but that coverage must not be represented as an end-to-end production workload validation.
- Kernel Xid history can remain unavailable to an unprivileged user. AIStat reports this as an inspection gap rather than claiming that no Xid event exists.

## `v0.1.0` release decision

The maintainer accepted the initial public release with the active-workload limitation stated above. `v0.1.0` is a read-only diagnostic foundation, not a claim of benchmark accuracy, automatic optimization, or validation across every NVIDIA platform.

Future releases that add observation, profiling, benchmark, or automatic recommendation validation must add new real-hardware gates before publishing those capabilities as stable.
