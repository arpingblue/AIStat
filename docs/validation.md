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

## Still required before `v0.1.0`

- At least one real NVIDIA Linux integration run covering GPU inventory, topology, Xid permission behavior, Docker/NVIDIA Container Toolkit, and an active framework/runtime.
- Review the generated report from that host for privacy and false positives.

The project owner will perform this NVIDIA validation manually when a suitable Linux host is available. Until its sanitized evidence is recorded here, the repository remains pre-release and must not be tagged `v0.1.0`.

Windows results and fixture coverage must not be represented as real Linux/NVIDIA validation. No `v0.1.0` tag should be published until the remaining gates are recorded here.
