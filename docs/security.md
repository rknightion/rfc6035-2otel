# Security

## Reporting a vulnerability

Report security issues privately through [GitHub private vulnerability
reporting](https://github.com/rknightion/rfc6035-2otel/security/advisories/new). Do
not open a public issue for a suspected vulnerability. Include the affected version or
commit, safe reproduction steps, expected and actual behaviour, and redact secrets and
call content.

Only the latest release is supported. Fixes land on `main` and ship in the next release;
older release lines do not receive backports.

## Trust boundaries

The UDP listener accepts SIP packets from the network path that can reach its bound
address. Place it behind network policy or a firewall that permits only expected report
sources. The configured sender registry bounds telemetry cardinality; it is not an
authentication or access-control mechanism.

The service returns SIP responses to the packet source. It does not initiate calls or
contact the endpoints named within a report. Bind deliberately: `0.0.0.0` accepts on
all local interfaces, whereas a private interface address limits exposure.

## Telemetry sensitivity

Every accepted report produces an OTLP log with its raw report body. Log attributes can
include call IDs, SIP identities, source address and port, codec details, and unrecognized
vendor fields. Treat the OTLP destination as a trusted data sink and limit read access
there accordingly.

Metrics deliberately exclude call IDs, addresses, SIP identities, parsed field values,
and other unbounded data. Their sender attribute comes only from the explicit registry
plus `unknown`. This is a cost and reliability boundary, not redaction: sensitive detail
is still present in the log record.

The exporter suppresses parsed field names that indicate authorization, passwords,
cookies, API keys, or tokens. Still avoid placing credentials in report payloads, and
review endpoint and header handling before adding new field mappings.

## OTLP credentials

OTLP headers are sent to both metric and log exporters. Supply sensitive headers via an
environment variable or deployment secret, avoid shell history where practical, and
never commit a production credential to `config.yaml`.
