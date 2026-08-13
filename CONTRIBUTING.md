# Contributing to AIStat

**English** | [简体中文](CONTRIBUTING.zh-CN.md)

Issues and pull requests are welcome. Bug reports from real Linux NVIDIA servers are especially useful, provided that logs and fixtures are sanitized before sharing.

## Open an issue

Use an issue to report a bug, suggest a feature, discuss a diagnostic rule, or describe a server configuration AIStat does not handle correctly.

A useful bug report includes:

- AIStat version from `aistat version`;
- Linux distribution, kernel, architecture, and GPU model;
- the exact command that was run;
- expected and actual behavior;
- a minimal, sanitized output or JSON excerpt;
- whether Docker, kernel logs, or process inspection were permission-restricted.

Before uploading output, remove hostnames, usernames, IP and MAC addresses, container IDs, model paths, prompts, tokens, and customer information. Do not post credentials or an unreviewed support bundle.

For feature requests, describe the operational problem first: what the engineer needs to decide, what evidence is available, and how the result could be verified. A proposed command name alone is usually not enough context.

Security vulnerabilities must not be reported in a public issue. Follow [SECURITY.md](SECURITY.md).

## Submit a pull request

Small, focused pull requests are easier to review. Open an issue before making a large architectural change, adding a host command, changing the JSON contract, or introducing a new diagnostic rule.

Before submitting:

```bash
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
```

If your environment supports it, also run:

```bash
go test -race ./...
staticcheck ./...
```

A pull request should:

- explain the user-visible problem and the chosen behavior;
- keep AIStat read-only and bounded;
- include tests for new behavior and failure states;
- update English and Chinese user documentation when applicable;
- update the JSON Schema and golden tests for report-model changes;
- avoid committing build artifacts, raw captures, host identities, or secrets.

## Collector changes

Collectors gather facts and diagnostics only. They must not recommend changes or mutate the host.

Keep parsers separate from filesystem or command traversal. New external commands require:

- a fixed executable allowlist entry;
- fixed arguments without shell interpolation;
- a timeout and output limit;
- tests for absence, permission denial, timeout, malformed output, and oversized output;
- narrow, documented collection of process arguments or environment variables.

## Rule changes

Rules consume the normalized snapshot, topology graph, profile, and clock. They must not read files or run commands.

Every rule change should include relevant cases for:

- trigger and pass;
- unknown or missing evidence;
- not-applicable/skip;
- threshold boundaries;
- false-positive regression.

Findings need actionable evidence, impact, recommendation, verification, confidence, and authoritative references. Missing evidence must never become PASS.

See [docs/contributing-rules.md](docs/contributing-rules.md) and [docs/rules.md](docs/rules.md).

## JSON and compatibility

The public report is a versioned contract. Changes to enums, required fields, or semantics require a schema-version decision. Additive optional fields must still be reflected in the schema, validation code, and tests.

Keep JSON enums lowercase and validate changes against [report-v0.1.schema.json](docs/schema/report-v0.1.schema.json).

## Review expectations

Maintainers may ask for smaller scope, additional evidence, sanitized fixtures, or a more conservative diagnostic state. This is intended to keep AIStat safe on unfamiliar production servers.

By contributing, you agree that your contribution is licensed under the repository's [Apache License 2.0](LICENSE).
