# Data model

The v0.1 JSON report has exactly these top-level fields:

```text
schema_version, aistat_version, collected_at, profile,
compatibility_version, readiness, summary, node, findings
```

Inventory sections carry one of: `available`, `not_detected`, `unsupported`, `permission_denied`, `timeout`, `parse_error`, or `unknown`. Optional scalars use pointers or an equivalent presence mechanism where zero is meaningful, such as ECC counters and PCIe widths.

Finding statuses are `pass`, `warn`, `fail`, `info`, `unknown`, and `skip`. `unknown` means the rule applies but evidence is insufficient; `skip` means the active context is not applicable.

`node.runtimes.products` always distinguishes PyTorch, vLLM, and SGLang installation and execution states. Each product carries host/container inspectability, instance count, any confirmed package-metadata installations, and explicit reasons for incomplete installation or execution evidence. Active instances retain a pre-redaction `runtime_kind`, while only allowlisted arguments and environment values reach the report.

Docker client, daemon, Toolkit installation, Toolkit package, Toolkit command, NVIDIA runtime, CDI, and effective Docker GPU-integration states are modeled separately. Evidence arrays retain safe source names, installed package versions, CDI spec locations, and detected integration modes. The legacy `toolkit_detected` field remains for schema-0.1 consumers and is derived only when installation is conclusively available or absent.

Deployment readiness is `not_ready` if any applicable deployment finding fails, otherwise `unknown` if any is unknown, otherwise `ready`. Performance readiness is `warn` if any performance finding warns or fails, otherwise `unknown` if any is unknown, otherwise `ready`.

The normative machine contract is [schema/report-v0.1.schema.json](schema/report-v0.1.schema.json).
