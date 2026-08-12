# Environment variables

Every supported override begins with `RFC6035_2OTEL_`. Convert a YAML nesting boundary
to `__` and preserve single underscores in a field name. Environment overrides have
the highest configuration precedence.

| YAML key | Environment variable |
| --- | --- |
| `listen.address` | `RFC6035_2OTEL_LISTEN__ADDRESS` |
| `listen.port` | `RFC6035_2OTEL_LISTEN__PORT` |
| `otlp.endpoint` | `RFC6035_2OTEL_OTLP__ENDPOINT` |
| `otlp.protocol` | `RFC6035_2OTEL_OTLP__PROTOCOL` |
| `otlp.headers.<header>` | `RFC6035_2OTEL_OTLP__HEADERS__<HEADER>` |
| `dedupe_window` | `RFC6035_2OTEL_DEDUPE_WINDOW` |
| `log.level` | `RFC6035_2OTEL_LOG__LEVEL` |
| `service.name` | `RFC6035_2OTEL_SERVICE__NAME` |
| `service.version` | `RFC6035_2OTEL_SERVICE__VERSION` |

For example:

```sh
export RFC6035_2OTEL_OTLP__ENDPOINT="https://collector.example.test"
export RFC6035_2OTEL_OTLP__HEADERS__AUTHORIZATION="Bearer replace-me"
export RFC6035_2OTEL_LISTEN__PORT="5060"
```

`senders` is a structured list and cannot be represented by the flat environment
mapping. Keep it in YAML. Unknown environment keys fail rather than being ignored, so
a misspelled secret variable cannot silently select unexpected defaults.

## Standard OpenTelemetry resource environment

`OTEL_RESOURCE_ATTRIBUTES` is also read and attached to both metrics and logs. Use it
for standard resource dimensions that are not application configuration, for example:

```sh
export OTEL_RESOURCE_ATTRIBUTES="deployment.environment.name=production"
```

The configured `service.name` and `service.version`, plus the runtime
`service.instance.id`, override those same keys from the OpenTelemetry environment.
`OTEL_SERVICE_NAME` is therefore accepted by the SDK but the explicit configured
service name remains authoritative. Telemetry SDK and host resource attributes are
added automatically.
