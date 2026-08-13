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

## Still required before `v0.1.0`

- A real Ubuntu CPU-only CI run on GitHub Actions.
- At least one real NVIDIA Linux integration run covering GPU inventory, topology, Xid permission behavior, Docker/NVIDIA Container Toolkit, and an active framework/runtime.
- Review the generated report from that host for privacy and false positives.

Windows results and fixture coverage must not be represented as real Linux/NVIDIA validation. No `v0.1.0` tag should be published until the remaining gates are recorded here.
