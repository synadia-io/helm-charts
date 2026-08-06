# Insights Helm chart

This chart runs one persistent Insights instance. It intentionally contains
only the Kubernetes resources the application needs: a StatefulSet, Service,
config Secret, and data volume.

## Install

For a simulator deployment:

```yaml
# values.yaml
config:
  data-dir: /var/lib/insights
  web:
    hostname: 0.0.0.0
  simulator:
    enabled: true
```

```sh
helm repo add synadia https://synadia-io.github.io/helm-charts
helm repo update synadia
helm upgrade --install insights synadia/insights -f values.yaml
kubectl port-forward service/insights 8080:8080
```

To monitor a NATS system, replace `simulator` with the same configuration you
would put in an Insights `config.yaml`:

```yaml
config:
  data-dir: /var/lib/insights
  web:
    hostname: 0.0.0.0
  sys:
    server: nats://nats.nats.svc.cluster.local:4222
    user: system
    password: secret
```

Insights needs a system to observe, so `config` must set one of
`simulator.enabled`, `sys`, or `systems`. The chart refuses to render without
one rather than deploying a pod that cannot start.

The complete application configuration is documented in the
[Insights configuration reference](https://docs.synadia.com/insights/reference/configuration).

## Configuration mapping

Everything below `config` is serialized directly to `/etc/insights/config.yaml`.
There is no chart-specific translation layer, so application keys, nesting,
lists, booleans, durations, and future configuration additions work as they do
in a standalone config file. The `config` wrapper is the only extra level.

The chart does not copy every application default into `values.yaml`. Doing so
would make defaults drift between the chart and Insights. It includes only the
two container-specific settings: a persistent `data-dir` and a web listener on
all pod interfaces.

File paths in the application config still need corresponding Kubernetes
mounts. Use `extraVolumes` and `extraVolumeMounts` for NATS credentials, TLS
files, an embedded NATS config, or a file-based license.

## Images and licensing

The default image is selected from the application config:

| Application config | Default image |
| --- | --- |
| No `license.token` or `license.file` | `registry.synadia.io/insights` |
| `license.token` or `license.file` set | `registry.synadia.io/insights-licensed` |

Set `image.tag` to pin a version. Set `image.registry` for a registry mirror.
Set both `image.registry` and `image.repository` when the mirror uses a custom
layout. An explicit repository name takes precedence over automatic edition
selection.

```yaml
image:
  registry: mirror.example.com
  tag: 0.1.9
```

The default registry may require authentication. Create the registry Secret
outside the chart:

```sh
kubectl create secret docker-registry synadia-registry \
  --namespace insights \
  --docker-server registry.synadia.io \
  --docker-username "$SYNADIA_REGISTRY_USERNAME" \
  --docker-password "$SYNADIA_REGISTRY_PASSWORD"
```

Then reference it:

```yaml
image:
  pullSecrets:
    - name: synadia-registry
```

Registry credentials and an Insights license are separate concerns. Adding
`config.license.token` or `config.license.file` selects the licensed image but
does not authenticate the pull.

When `configSecret.existingSecret` is used, Helm cannot inspect that Secret to
detect a license or its web settings. Select the licensed image explicitly and,
if needed, set `service.targetPort` to the port in that config. Set
`startupProbe`, `livenessProbe`, and `readinessProbe` to `null` if its web server
is disabled. Insights reads the config at startup, so restart the StatefulSet
after changing an externally managed Secret:

```sh
kubectl rollout restart statefulset/insights
```

```yaml
configSecret:
  existingSecret: insights-config
image:
  repository: insights-licensed
service:
  targetPort: 8888
```

## Secrets

The generated config is a Kubernetes Secret because an Insights config may
contain credentials. Kubernetes Secrets are not encrypted unless the cluster
is configured for encryption at rest, and values passed to Helm may be retained
in Helm release history.

For production credentials, use a secret-management workflow and point
`configSecret.existingSecret` at a Secret containing the complete `config.yaml`.
The Secret key is configurable with `configSecret.key`.

For file-based credentials, mount an existing Secret:

```yaml
config:
  data-dir: /var/lib/insights
  web:
    hostname: 0.0.0.0
  sys:
    server: nats://nats.nats.svc.cluster.local:4222
    creds: /etc/insights/credentials/sys.creds

extraVolumes:
  - name: credentials
    secret:
      secretName: insights-sys-creds
extraVolumeMounts:
  - name: credentials
    mountPath: /etc/insights/credentials
    readOnly: true
```

## Persistence

Persistence is enabled by default with a 10 GiB `ReadWriteOnce` volume claim.
Set `persistence.size`, `persistence.storageClass`, and
`persistence.accessModes` to match the cluster, or set
`persistence.existingClaim` to reuse a PVC.

Disabling persistence uses `emptyDir`; all indexed history is then lost with
the pod. StatefulSet-created PVCs are intentionally retained by Kubernetes when
the Helm release is deleted and must be removed separately when their data is
no longer needed.

Leaving `persistence.storageClass` empty uses the cluster default. Managed
clusters differ in what they provide:

| Cluster | Notes |
| --- | --- |
| Amazon EKS | Auto Mode provides the EBS CSI driver but creates no StorageClass; define one (for example encrypted `gp3`) before installing. |
| Azure AKS | `managed-csi` (Standard SSD) and `managed-csi-premium` (Premium SSD) are preinstalled. |
| Google GKE | `standard-rwo` is preinstalled and is the default. |

The volume claim template is immutable once the StatefulSet exists, so
`persistence.size`, `persistence.storageClass`, and `persistence.accessModes`
cannot be changed by upgrading the release. Expanding a volume is a change to
the PVC, made through a StorageClass that allows expansion.

## Security context

The pod runs as UID and GID 1000 with `runAsNonRoot`, and `fsGroup` gives the
data volume to that group. Override `podSecurityContext` for a cluster that
assigns its own UID range, such as OpenShift.

The container drops all capabilities and disallows privilege escalation. A
`memory-limit` in the application config bounds DuckDB's buffer pool only, so
size `resources.limits.memory` above it to leave headroom for Go, embedded
NATS, and allocations outside that pool.
