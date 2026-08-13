# Contributing

Use Go 1.26.5 and keep production dependencies minimal. Before opening a change, run:

```bash
gofmt -w ./cmd ./internal
go test -race ./...
go vet ./...
staticcheck ./...
```

Collector parsers must be separated from filesystem or command traversal and tested with sanitized fixtures. New external commands require a fixed executable allowlist entry, fixed arguments, a timeout, an output limit, and tests for absence, denial, timeout, malformed output, and oversized output.

Rule changes must include trigger, pass, and missing-evidence cases, actionable remediation, verification steps, confidence, and authoritative references. Rules may not read files, execute commands, or mutate state.

Public JSON changes require a schema-version decision and matching schema/golden tests. See [docs/contributing-rules.md](docs/contributing-rules.md).
