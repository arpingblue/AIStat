# Contributor and agent guide

- Treat `AIStat项目计划书.md` as product truth and `Implementation Plan.md` as the executable engineering contract.
- Preserve read-only behavior. Never add host mutation, automatic tuning, daemon behavior, shell interpolation, or unbounded external commands.
- Collectors gather facts and diagnostics only. Rules consume normalized `Snapshot + Graph + Profile`; reporters only render a completed report.
- Model missing, denied, timed-out, malformed, unsupported, and absent evidence explicitly. Never convert missing evidence to PASS.
- Keep JSON enums lowercase and validate report changes against `docs/schema/report-v0.1.schema.json`.
- Add sanitized fixtures and trigger/pass/missing-evidence tests for every rule change.
- Windows must run portable logic and Linux fixture tests. Linux CI and real NVIDIA validation remain authoritative for release.
- Do not commit `.tools`, build artifacts, raw captures, secrets, hostnames, IP addresses, MAC addresses, model paths, prompts, or arbitrary process environments.
