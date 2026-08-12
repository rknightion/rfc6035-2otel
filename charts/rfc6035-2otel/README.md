# rfc6035-2otel

Receive Poly RTCP-XR SIP PUBLISH reports over UDP and export OpenTelemetry
metrics and logs over OTLP. The collector auto-detects RFC 6035 standard and
Poly pre-standard report dialects. It is Go-based, UDP-only, has no syslog hop,
and emits no traces.

## Install

Set an OTLP endpoint during installation (or set `config.otlp.endpoint` in a
values file):

```sh
helm install rfc6035 oci://ghcr.io/rknightion/charts/rfc6035-2otel \
  --set config.otlp.endpoint=https://otlp.example.invalid
```

For secret OTLP headers, use `extraEnv`, for example an `EnvVar` sourced from a
Kubernetes Secret with the name `RFC6035_2OTEL_OTLP__HEADERS__AUTHORIZATION`.
The chart has no secret value and does not render a Secret.

## Inbound UDP Service exposure

The Service always declares UDP port 5060. `LoadBalancer` is the default and is
appropriate for off-cluster phones only when the cluster's load-balancer
implementation supports and provisions UDP. `NodePort` also supports UDP, but
phones must target a node IP and the allocated node port (or external routing
must forward to it). `ClusterIP` supports UDP only for callers that can reach
the cluster Service IP, normally in-cluster workloads.

`ExternalName` has no proxy or port exposure and therefore does not expose this
listener. `ClusterIP` does not make an off-cluster listener reachable. A
`LoadBalancer` whose provider does not provision UDP, a `NodePort` without a
reachable node address/port, or an ingress configured for HTTP/TCP only can
silently leave SIP PUBLISH traffic unable to reach the collector.

There is no Kubernetes `network_mode: host` equivalent in this chart. For
`LoadBalancer` or `NodePort`, `service.externalTrafficPolicy: Local` asks
Kubernetes to keep external traffic on nodes with ready local endpoints and is
the applicable setting for retaining source IP where the implementation
supports it. That can reduce availability because nodes without a local ready
endpoint do not receive traffic. `Cluster` is the usual cross-node balancing
mode but may source-NAT traffic, so the collector may not see the handset's
original IP. The actual source-IP behavior remains dependent on the CNI and
load-balancer implementation.

## Values

Every value is documented inline in [values.yaml](values.yaml). Key settings:

| Key | Default | Description |
| --- | --- | --- |
| `image.repository` | `ghcr.io/rknightion/rfc6035-2otel` | Container image repository. |
| `config.listen.port` | `5060` | UDP listener port and container port. |
| `service.type` | `LoadBalancer` | Kubernetes UDP Service exposure mode. |
| `service.externalTrafficPolicy` | `Local` | Source-IP/availability trade-off for `LoadBalancer` and `NodePort`. |
| `config.otlp.endpoint` | `https://otlp.example.invalid` | Required OTLP endpoint; override for a real collector. |
| `config.otlp.protocol` | `http` | OTLP transport: `http` or `grpc`. |
| `config.otlp.headers` | `{}` | Non-secret OTLP headers. |
| `extraEnv` | `[]` | Extra EnvVar entries, including Secret-backed OTLP credentials. |

The default security posture is non-root (`65532`), `readOnlyRootFilesystem:
true`, `allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]`, and the
runtime-default seccomp profile. The collector is stateless and needs no PVC.
