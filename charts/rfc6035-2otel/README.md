# rfc6035-2otel

<!-- x-release-please-start-version -->
![Version](https://img.shields.io/static/v1?label=Version&message=0.2.1&color=informational&style=flat-square)
![AppVersion](https://img.shields.io/static/v1?label=AppVersion&message=0.2.1&color=informational&style=flat-square)
<!-- x-release-please-end -->
![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square)

Receive Poly RTCP-XR SIP PUBLISH reports over UDP and export metrics and logs over OTLP.

**Homepage:** <https://github.com/rknightion/rfc6035-2otel>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| rknightion |  |  |

## Source Code

* <https://github.com/rknightion/rfc6035-2otel>

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
appropriate for off-cluster phones only when the cluster load balancer supports
UDP. `NodePort` also supports UDP, but phones must target a reachable node IP and
port. `ClusterIP` supports UDP only for callers that can reach the cluster Service
IP. `ExternalName` has no proxy or port exposure, and HTTP/TCP-only ingress does
not carry this traffic.

`service.externalTrafficPolicy: Local` is the Kubernetes control most likely to
retain the original source IP for `LoadBalancer` and `NodePort`, but it sends
traffic only to nodes with ready local endpoints. Source-IP behavior still
depends on the CNI and load balancer. If the source is translated, configured
sender mappings cannot distinguish the original handset.

## Security posture

The defaults run as non-root user `65532`, use a read-only root filesystem,
disable privilege escalation, drop every Linux capability, select the runtime
default seccomp profile, and do not mount a service-account token. The process is
OTLP-push-only and intentionally has no HTTP health endpoint or fabricated probe.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| config.dedupe_window | string | `"32s"` |  |
| config.listen.address | string | `"0.0.0.0"` |  |
| config.listen.port | int | `5060` |  |
| config.log.level | string | `"info"` |  |
| config.otlp.endpoint | string | `"https://otlp.example.invalid"` |  |
| config.otlp.headers | object | `{}` |  |
| config.otlp.protocol | string | `"http"` |  |
| config.service.name | string | `"rfc6035-2otel"` |  |
| config.service.version | string | `""` |  |
| extraEnv | list | `[]` |  |
| extraVolumeMounts | list | `[]` |  |
| extraVolumes | list | `[]` |  |
| fullnameOverride | string | `""` |  |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"ghcr.io/rknightion/rfc6035-2otel"` |  |
| image.tag | string | `""` |  |
| imagePullSecrets | list | `[]` |  |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` |  |
| podAnnotations | object | `{}` |  |
| podLabels | object | `{}` |  |
| podSecurityContext.fsGroup | int | `65532` |  |
| podSecurityContext.fsGroupChangePolicy | string | `"OnRootMismatch"` |  |
| podSecurityContext.runAsGroup | int | `65532` |  |
| podSecurityContext.runAsNonRoot | bool | `true` |  |
| podSecurityContext.runAsUser | int | `65532` |  |
| podSecurityContext.seccompProfile.type | string | `"RuntimeDefault"` |  |
| replicaCount | int | `1` |  |
| resources.limits.cpu | string | `"500m"` |  |
| resources.limits.memory | string | `"256Mi"` |  |
| resources.requests.cpu | string | `"50m"` |  |
| resources.requests.memory | string | `"64Mi"` |  |
| securityContext.allowPrivilegeEscalation | bool | `false` |  |
| securityContext.capabilities.drop[0] | string | `"ALL"` |  |
| securityContext.readOnlyRootFilesystem | bool | `true` |  |
| securityContext.runAsGroup | int | `65532` |  |
| securityContext.runAsUser | int | `65532` |  |
| service.annotations | object | `{}` |  |
| service.externalTrafficPolicy | string | `"Local"` |  |
| service.loadBalancerIP | string | `""` |  |
| service.loadBalancerSourceRanges | list | `[]` |  |
| service.nodePort | string | `nil` |  |
| service.port | int | `5060` |  |
| service.type | string | `"LoadBalancer"` |  |
| serviceAccount.automountServiceAccountToken | bool | `false` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `""` |  |
| tolerations | list | `[]` |  |