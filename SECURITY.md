# Security policy

## Reporting a vulnerability

Please use [GitHub private vulnerability
reporting](https://github.com/rknightion/rfc6035-2otel/security/advisories/new), not a
public issue. Include the affected version or commit, a safe reproduction, impact, and
redacted configuration or packet material.

Only the latest release is supported. Fixes are made on `main` and released in the next
version; no older release line receives backports.

## Operational security

The service receives unauthenticated UDP SIP traffic from hosts that can reach its
listener. Restrict network access to expected source networks. The sender registry only
bounds telemetry cardinality and is not authentication.

Accepted report logs may contain call IDs, SIP identities, IP addresses, codec details,
and raw vendor fields. Treat the OTLP backend as a trusted sink and control read access.
Metrics carry only bounded dimensions and intentionally exclude those per-call values.

Keep OTLP authorization headers in deployment secrets or environment variables, not
committed configuration. Parsed authorization, password, cookie, API-key, and token
fields are suppressed from exporter-generated log attributes; do not rely on that as a
reason to include credentials in report bodies.
