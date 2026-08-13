# Security policy

## Reporting a vulnerability

Do not open a public issue for a vulnerability. Send a private report to the repository security contact with the affected version, impact, reproduction, and any proposed mitigation. Maintainers should acknowledge the report within seven days.

## Security model

AIStat is read-only and should run without root for baseline inspection. Some kernel logs, process records, Docker sockets, or device data may be permission-restricted; AIStat reports that state as `permission_denied` or `unknown` instead of requesting elevation.

External commands are resolved from a fixed allowlist and executed directly without a shell. Each command has a context deadline and output cap. Reporters do not serialize arbitrary environment variables or command lines. Fixture sanitization must replace hostnames, MAC addresses, IP addresses, container IDs, usernames, model paths, tokens, prompts, and customer identifiers.

AIStat findings are diagnostic guidance, not authorization to weaken ACS/IOMMU, change drivers, alter Docker configuration, or modify workload placement automatically.
