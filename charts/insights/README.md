# Insights Helm chart

This chart runs one persistent Insights instance, or one of the stateless
Insights services that reach an instance over NATS. It intentionally contains
only the Kubernetes resources the application needs: a StatefulSet and data
volume for the instance or a Deployment for a stateless service, a Service, and
a config Secret.

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

An instance needs a system to observe, so `config` must set one of
`simulator.enabled`, `sys`, or `systems`. The chart refuses to render without
one rather than deploying a pod that cannot start.

The complete application configuration is documented in the
[Insights configuration reference](https://docs.synadia.com/insights/reference/configuration).

## Subcommands

`subcommand` selects the Insights command the container runs. The default,
`serve`, is the instance above. The others are clients of an instance that
reach its API over NATS, hold no state, and run as a Deployment of
`replicaCount` pods:

| `subcommand` | Runs | Workload | Listens on | Probe |
| --- | --- | --- | --- | --- |
| `serve` | Scraper, indexer, database, and web UI | StatefulSet, one replica, data volume | `web.port`, 8080 | `/healthz` while the web UI is on |
| `web` | The web UI alone | Deployment | `web.port`, 8080 | `/healthz` |
| `http` | HTTP gateway over the read-only query API | Deployment | `http.listen`, 0.0.0.0:8080 | `/healthz` |
| `mcp` | MCP server over the read-only query API | Deployment, one replica | `mcp.http`, 0.0.0.0:8090 | TCP |
| `prometheus` | One `/metrics` endpoint for every node | Deployment | `prometheus.port`, 9091 | TCP |

A client needs a NATS connection to the instance: `config.nats`, an
`INSIGHTS_NATS_*` variable in `extraEnv`, or a flag in `extraArgs`. The chart
refuses to render without a server or context. Insights binds these services to
loopback by default, so the chart writes the pod-network address above into
`config` wherever it is left unset.

A web tier beside an instance:

```yaml
subcommand: web
replicaCount: 2
config:
  nats:
    server: nats://nats.nats.svc.cluster.local:4222
```

The HTTP gateway refuses to start without a bearer token. Take it from a Secret:

```yaml
subcommand: http
config:
  nats:
    server: nats://nats.nats.svc.cluster.local:4222
extraEnv:
  INSIGHTS_HTTP_AUTH_TOKEN:
    valueFrom:
      secretKeyRef:
        name: insights-http
        key: token
```

Set `config.http.allow-unauthenticated: true` instead only for a gateway no
untrusted client can reach.

`mcp` always runs its http transport. Stdio serves only a client that spawns the
process, so the chart refuses a stdio transport. The server keeps each client
session in process memory, so it runs as one replica and is replaced rather
than rolled on upgrade. It applies no authorization: anyone who reaches the
Service can query Insights through it, so keep it off untrusted networks.

`extraArgs` appends flags after the subcommand and `--config`. A flag outranks
the same setting in `extraEnv` and in `config`:

```yaml
extraArgs:
  - --log-level=debug
```

The chart reads the container port from `config`. Set `service.targetPort` too
when the listener comes from anywhere else: a flag in `extraArgs`, a variable in
`extraEnv`, or a config in `configSecret.existingSecret`, which the chart cannot
read. That listener must bind the pod network, not loopback.

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

Every config key also has an `INSIGHTS_`-prefixed environment variable, which
outranks the config file. `extraEnv` sets them, taking either a literal or a
whole `valueFrom` body so a value can come from a Secret you already keep:

```yaml
extraEnv:
  INSIGHTS_LOG_LEVEL: debug
  INSIGHTS_LICENSE_TOKEN:
    valueFrom:
      secretKeyRef:
        name: insights-license
        key: token
```

## Images and licensing

The default image is selected from the application config:

| Application config | Default image |
| --- | --- |
| No `license.token` or `license.file` | `registry.synadia.io/insights` |
| `license.token` or `license.file` set | `registry.synadia.io/insights-licensed` |

Set `image.tag` to pin a version. An explicit `image.repository` takes
precedence over automatic edition selection.

`insights` requires authentication to `registry.synadia.io`; without a pull
secret the pod fails to start with `ImagePullBackOff`. `insights-licensed` is
public, so a trial needs no pull secret. Create the Secret outside the chart,
using the registry credentials Synadia issued you:

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

When `configSecret.existingSecret` is used, Helm cannot inspect that Secret to
detect a license or its web settings. Select the licensed image explicitly and,
if needed, set `service.targetPort` to the port in that config. Set
`startupProbe`, `livenessProbe`, and `readinessProbe` to `null` if its web server
is disabled. Insights reads the config at startup, so restart the StatefulSet,
or the Deployment of a stateless subcommand, after changing an externally
managed Secret:

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
      defaultMode: 0400
extraVolumeMounts:
  - name: credentials
    mountPath: /etc/insights/credentials
    readOnly: true
```

Mounting under `/etc/insights`, where the config Secret is mounted, is fine: the
kubelet creates the subdirectory inside that volume and mounts the second Secret
there, leaving `config.yaml` readable alongside it.

Insights can observe several NATS systems at once through `config.systems`, and
each entry carries its own connection — so each needs its own credentials file
and its own TLS material, and the four filenames would otherwise collide. Give
each system a Secret and a directory named for it:

```yaml
config:
  systems:
    - id: east
      scrape:
        nats:
          server: nats://east.example.com:4222
          creds: /etc/insights/systems/east/sys.creds
    - id: west
      scrape:
        nats:
          server: nats://west.example.com:4222
          creds: /etc/insights/systems/west/sys.creds

extraVolumes:
  - name: creds-east
    secret: { secretName: insights-nats-east, defaultMode: 0400 }
  - name: creds-west
    secret: { secretName: insights-nats-west, defaultMode: 0400 }
extraVolumeMounts:
  - name: creds-east
    mountPath: /etc/insights/systems/east
    readOnly: true
  - name: creds-west
    mountPath: /etc/insights/systems/west
    readOnly: true
```

## Web session seed

Insights signs web session cookies with `web.session-seed`. Its application
default is the literal string `insights`, so every deployment that leaves it
alone signs with the same publicly known value; Insights warns about this at
startup.

To sign with your own, create a Secret and name it in `sessionSeed`:

```sh
kubectl create secret generic insights-session \
  --from-literal=seed="$(head -c 48 /dev/urandom | base64)"
```

```yaml
sessionSeed:
  existingSecret: insights-session
  key: seed
```

The chart never reads the Secret, only references it in a `secretKeyRef`, so the
seed stays out of your values, out of the rendered manifests, and out of git.

`INSIGHTS_WEB_SESSION_SEED` outranks the config file, so set `sessionSeed` or
`config.web.session-seed`, not both. Changing the seed signs out every session
the deployment has issued.

## Persistence

Persistence applies to `serve`. The stateless subcommands mount no data volume.

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

## Memory

`resources` is empty by default, so the pod has no memory limit. Set one sized
to the application's DuckDB limit, `config.db.memory-limit` (default `4GiB`):

```
limits.memory >= db.memory-limit + 2Gi + Go allowance
```

DuckDB keeps the memory it tracks under `db.memory-limit`, but the process
holds up to about 2 GiB more in freed memory the allocator has not yet returned.
The Go allowance covers everything outside DuckDB, including the embedded NATS
server: 2 GiB, or 4 GiB for a large fleet. The default limit therefore needs
8 GiB:

```yaml
config:
  db:
    memory-limit: 4GiB
resources:
  requests:
    memory: 8Gi
  limits:
    memory: 8Gi
```

Keep the request equal to the limit so the pod is only scheduled on a node that
can hold its peak. To fit a smaller pod, lower `db.memory-limit` rather than the
headroom. With `systems`, each system has its own DuckDB, so add each system's
`memory-limit` and 2 GiB per system.

The chart does not set `GOMEMLIMIT`. It bounds the Go heap only, so the pod's
memory limit is the wrong value for it. To set it, use the Go allowance:

```yaml
extraEnv:
  GOMEMLIMIT: 2GiB
```

## Security context

The pod runs as UID and GID 1000 with `runAsNonRoot`, and `fsGroup` gives the
data volume to that group. Override `podSecurityContext` for a cluster that
assigns its own UID range, such as OpenShift.

The container drops all capabilities and disallows privilege escalation.
