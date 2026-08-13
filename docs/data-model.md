# Data model

The v0.1 JSON report has exactly these top-level fields:

```text
schema_version, aistat_version, collected_at, profile,
compatibility_version, readiness, summary, node, findings
```

Inventory sections carry one of: `available`, `not_detected`, `unsupported`, `permission_denied`, `timeout`, `parse_error`, or `unknown`. Optional scalars use pointers or an equivalent presence mechanism where zero is meaningful, such as ECC counters and PCIe widths.

Finding statuses are `pass`, `warn`, `fail`, `info`, `unknown`, and `skip`. `unknown` means the rule applies but evidence is insufficient; `skip` means the active context is not applicable.

Deployment readiness is `not_ready` if any applicable deployment finding fails, otherwise `unknown` if any is unknown, otherwise `ready`. Performance readiness is `warn` if any performance finding warns or fails, otherwise `unknown` if any is unknown, otherwise `ready`.

The normative machine contract is [schema/report-v0.1.schema.json](schema/report-v0.1.schema.json).
