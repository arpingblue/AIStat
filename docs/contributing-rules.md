# Contributing rules

A rule must have a stable ID, title, domain, deployment/performance dimension, priority, confidence, description, and authoritative references. It may only inspect `Snapshot`, `Graph`, `Profile`, and injected `Now`.

Every rule requires three minimum tests:

1. trigger evidence produces the documented FAIL or WARN;
2. complete healthy evidence produces PASS;
3. applicable but missing/denied/malformed evidence produces UNKNOWN.

Use SKIP only when the workload context is genuinely inapplicable. Recommendations must be reversible guidance and include a verification path. Do not add speculative tuning rules or encode one platform topology as universal truth.
