# Security design

The command runner resolves only registered executable names, invokes no shell, applies context deadlines, captures bounded stdout/stderr, and records exit status. Unix children run in a new process group; Windows children run in a kill-on-close Job Object. Timeout tests verify descendant cleanup on Windows in addition to success, non-zero exit, oversized output, environment sanitization, and allowlist rejection.

Process collection serializes only explicit runtime flags and the narrow CUDA/NCCL environment allowlist. Container IDs are truncated, model paths are replaced with stable redacted forms, and HF/cloud/API credentials are covered by negative regression tests. The redaction library masks IP, MAC, token, password, secret, and API-key patterns. Raw fixtures are never publishable until sanitized and reviewed.

Permissions are evidence. A denied Docker socket, kernel log, procfs entry, or device query becomes an explicit state and may produce UNKNOWN. The tool never asks for elevation or mutates the host.
