# AIStat v0.1 rule catalog

This document describes the frozen v0.1 catalog implemented by `internal/rules`. The product contract in `AIStat项目计划书.md` remains authoritative for rule semantics; the registry and tests are authoritative for the executable implementation.

## Evaluation contract

Rules are deterministic and read-only. They receive only:

```text
Normalized Snapshot + Topology Graph + Profile + Evaluation Time
```

They never read `/proc` or `/sys`, execute a command, access the network, or change the host. Every result includes evidence, impact/why, recommendation, verification, confidence, and references.

### Result states

| State | Meaning |
|---|---|
| `pass` | Sufficient evidence was available and the trigger condition was not present |
| `warn` | A supported performance-risk condition was established |
| `fail` | A supported deployment or correctness blocker was established |
| `info` | Relevant evidence was found without a blocker or supported risk conclusion |
| `unknown` | The rule applies, but evidence is missing, denied, timed out, malformed, or unsupported |
| `skip` | The rule is not applicable to the detected hardware/workload context |

`unknown` is never converted to `pass`. `skip` is not a statement about readiness; it only means that the rule's applicability condition was absent.

### Readiness dimensions

- **Deployment:** `READY`, `NOT READY`, or `UNKNOWN`. A deployment `fail` makes it `NOT READY`.
- **Performance:** `READY`, `WARN`, or `UNKNOWN`. A performance `warn` makes it `WARN`.

AIStat intentionally does not produce an arbitrary numeric score.

## GPU and NVIDIA health

| ID | Dimension / priority | Trigger | Result |
|---|---|---|---|
| `GPU001` | deployment / P0 | An NVIDIA display/compute PCI device exists, but the driver stack is confirmed unusable | `fail` |
| `GPU002` | deployment / P0 | A selected/visible GPU reports compute mode `PROHIBITED` | `fail` |
| `GPU003` | deployment / P0 | A reliable volatile uncorrectable ECC counter is greater than zero | `fail` |
| `GPU004` | deployment / P0 | A critical, catalogued NVIDIA Xid occurred in the 24-hour evaluation window | `fail` |

Important evidence boundaries:

- No NVIDIA PCI device makes GPU-specific rules `skip` rather than `pass`.
- Unreadable ECC or Xid evidence produces `unknown`.
- AIStat does not reset GPUs, change compute mode, or clear counters.

## PCIe, NUMA, and topology

| ID | Dimension / priority | Trigger | Result |
|---|---|---|---|
| `PCIE001` | performance / P1 | An active GPU negotiates a narrower PCIe width than its reported maximum | `warn` |
| `PCIE002` | performance / P1 | A P2P/GDR context exists and ACS redirect is enabled on a relevant inspected path | `warn` |
| `NUMA001` | performance / P1 | An AI runtime's effective CPU set has no overlap with CPUs local to a selected GPU | `warn` |
| `NUMA002` | performance / P1 | Effective runtime memory nodes exclude the selected GPU's local NUMA node | `warn` |
| `TOPO001` | performance / P1 | A visible same-size GPU group is no worse on every known pair and strictly better on at least one pair | `warn` |

`PCIE001` requires an active GPU because idle links may legitimately downshift. `TOPO001` uses strict dominance; it does not claim that every alternative with a different topology token is faster. All placement recommendations require workload A/B validation.

## Network, RDMA, and NCCL

| ID | Dimension / priority | Trigger | Result |
|---|---|---|---|
| `NET001` | deployment / P0 | A runtime explicitly selects a NIC/HCA that is absent or confirmed unavailable | `fail` |
| `NET002` | performance / P1 | Confirmed GDR context crosses a clearly unsupported PCIe-root arrangement | `warn` |
| `NET003` | deployment / P0 | An RDMA/NCCL context has a confirmed insufficient effective memlock limit | `fail` |
| `NCCL001` | deployment / P0 | `NCCL_SOCKET_IFNAME` or `NCCL_IB_HCA` matches no usable inventoried device | `fail` |

Permission-denied RDMA inventory is `unknown`, not a missing-device failure. AIStat reports explicit selection problems but does not rewrite NCCL environment variables or network configuration.

## CUDA and driver compatibility

| ID | Dimension / priority | Trigger | Result |
|---|---|---|---|
| `CUDA001` | deployment / P0 | Versioned compatibility data finds no valid driver/runtime or compatibility-package path for the active CUDA runtime | `fail` |
| `CUDA002` | deployment / P2 | The embedded, source-attributed lifecycle dataset marks the installed NVIDIA driver branch EOL | `warn` |

Installed CUDA toolkits, the driver's supported CUDA level, and a framework's active CUDA build are distinct facts. Multiple installed toolkits do not fail by themselves. Unsupported or incomplete version evidence produces `unknown`.

## PyTorch

| ID | Dimension / priority | Trigger | Result |
|---|---|---|---|
| `TORCH001` | deployment / P0 | The active GPU runtime expects CUDA, but the bounded probe in the same interpreter reports `torch.cuda.is_available() == false` | `fail` |
| `TORCH002` | deployment / P0 | Runtime local world size exceeds the PyTorch probe's effective CUDA device count | `fail` |

The probe uses fixed built-in code, an isolated interpreter invocation, a minimal environment, empty stdin, and bounded time/output. It never imports user code, loads a model, or downloads data.

## Docker and NVIDIA Container Toolkit

| ID | Dimension / priority | Trigger | Result |
|---|---|---|---|
| `CTR001` | deployment / P0 | A detected workflow requires Docker and the daemon is confirmed unavailable | `fail` |
| `CTR002` | deployment / P0 | A required Docker GPU workflow has conclusive evidence that Toolkit packages/components are absent, or Toolkit installation is confirmed but neither NVIDIA runtime nor CDI is configured | `fail` |
| `CTR003` | deployment / P0 | A container expects GPU access but has zero effective visible GPUs | `fail` |
| `CTR004` | deployment / P0 | A multi-process/multi-GPU NCCL container uses default or tiny (`<=64 MiB`) `/dev/shm` | `fail` |

Docker socket permission denial produces `unknown`; it is not treated as an unavailable daemon. Toolkit absence requires consistent results from package, component-command, Docker-runtime, and CDI probes. A permission, timeout, parse, or coverage gap cannot produce “not installed.” AIStat never starts Docker, pulls a validation image, or edits daemon/runtime configuration.

## vLLM

| ID | Dimension / priority | Trigger | Result |
|---|---|---|---|
| `VLLM001` | deployment / P0 | vLLM local world size exceeds effective visible GPU count | `fail` |
| `VLLM002` | deployment / P0 | Explicit vLLM GPU selection contains a duplicate, nonexistent, or unmappable device reference | `fail` |

Runtime discovery retains only allowlisted infrastructure flags and environment variables. Raw command lines, arbitrary environment variables, model names/paths, API keys, and tokens are not retained.

## SGLang

| ID | Dimension / priority | Trigger | Result |
|---|---|---|---|
| `SGL001` | deployment / P0 | SGLang local world size exceeds effective visible GPU count | `fail` |
| `SGL002` | deployment / P0 | SGLang disaggregation explicitly references an absent or unavailable HCA | `fail` |

An incomplete disaggregation/HCA mapping is `unknown`; the rule fails only when the bad mapping is established by runtime and RDMA evidence.

## Profiles and applicability

The default profile is `llm-inference`; `general` is available for inventory-oriented checks. Runtime evidence can enrich profile context—for example, detected containers can require Docker, selected HCAs can require RDMA, and multi-worker runtimes can establish a multi-process context.

Profiles only control applicability. They do not fabricate facts and cannot turn missing evidence into a pass.

## Explain and machine output

Inspect rule metadata:

```bash
aistat explain GPU001
aistat explain NUMA001 --format json
```

Evaluate the full catalog:

```bash
aistat check
aistat check --format json
```

The JSON schema is versioned at [schema/report-v0.1.schema.json](schema/report-v0.1.schema.json). Official source links used by findings are listed in [REFERENCES.md](REFERENCES.md).

## Changing a rule

The 25 IDs and their v0.1 meanings are frozen. A change must preserve deterministic ordering and add or update:

- trigger test;
- pass test;
- unknown/missing-evidence test;
- skip/not-applicable test;
- boundary test;
- false-positive regression test for complex rules;
- complete evidence, impact, recommendation, verification, confidence, and references.

New semantics require a new Rule ID; an existing ID must not be silently repurposed.
