# Synadia Insights Helm Chart

[Synadia Insights](https://www.synadia.com/insights) observes a NATS system: it
scrapes the system-account monitoring endpoints, indexes the data into an
embedded DuckDB database, and serves a web UI plus audit checks.

Insights runs as a **single, stateful instance** — the chart deploys a
1-replica StatefulSet with a persistent volume for the DuckDB database and
JetStream data. It is not horizontally scalable (single-writer DB + embedded
NATS sink).

## Accessing the Helm Chart

```bash
# add the synadia repo (only needs to be run once)
helm repo add synadia https://synadia-io.github.io/helm-charts

# update the synadia repo index (run to get updated chart versions)
helm repo update synadia

# note: you must configure an image pull secret (see below) and the NATS system to monitor
helm upgrade --install insights synadia/insights -f values.yaml
```

### Useful Tools and References

- [Chart Values file](https://github.com/synadia-io/helm-charts/blob/main/charts/insights/values.yaml) — lists all possible configuration options

## Common Configuration

### Image Pull Secret

The Insights image is hosted on the private `registry.synadia.io`. By default the
chart creates an image pull secret from the credentials you provide:

```yaml
imagePullSecret:
  username: my-user
  password: my-password
```

To use an existing pull secret instead, disable the managed one and reference
yours via the pod template:

```yaml
imagePullSecret:
  enabled: false

podTemplate:
  merge:
    spec:
      imagePullSecrets:
      - name: my-existing-regcred
```

### Quick Start (built-in simulator)

To evaluate Insights without wiring up a real NATS system, enable the built-in
simulator. It starts an embedded NATS system with synthetic JetStream traffic
and monitors that — no `config.sys`, no credentials, and no license required
(it works with the default production image). For demos/evaluation only.

```yaml
imagePullSecret:
  username: my-user
  password: my-password

config:
  simulator:
    enabled: true
    # profile: js-small   # or e.g. super-medium
```

```bash
helm upgrade --install insights synadia/insights -f values.yaml
# then port-forward the web UI:
kubectl port-forward svc/insights 8080:8080
```

### Connecting to a System

To monitor a real system, set its server URL and the system-account credentials
in whichever auth mode your system uses.

**Credentials file (`.creds`)** — the common case. The file is mounted from an
existing Secret you create:

```bash
kubectl create secret generic my-sys-creds --from-file=sys.creds=./sys.creds
```

```yaml
imagePullSecret:
  username: my-user
  password: my-password

config:
  sys:
    server: nats://nats.nats.svc.cluster.local:4222
    creds:
      secretName: my-sys-creds   # existing Secret
      key: sys.creds             # key within it (default)

  # recommended: a random seed for secure session cookies
  web:
    sessionSeed: <random-string>
```

**Basic auth / NKey** — sensitive string values. Provide inline and the chart
stores them in a managed Secret (`<release>-config`) and injects them via
`secretKeyRef` — they are never placed inline on the pod spec:

```yaml
config:
  sys:
    server: nats://nats.nats.svc.cluster.local:4222
    user: sys
    password: s3cret            # -> chart-managed Secret
    # or NKey seed + user JWT:
    # nkey: SUACSEED...
    # jwt: eyJ...
```

To reference an **existing Secret** for any of those string values instead of
inlining, use the `container.env` escape hatch:

```yaml
config:
  sys:
    server: nats://nats.nats.svc.cluster.local:4222
    user: sys
container:
  env:
    INSIGHTS_SYS_PASSWORD:
      valueFrom:
        secretKeyRef:
          name: my-sys-secret
          key: password
```

**Client TLS** — each file is mounted from an existing Secret (e.g. cert + key
from a `kubernetes.io/tls` Secret, CA from another):

```yaml
config:
  sys:
    server: tls://nats.example.com:4222
    tls:
      cert:   { secretName: client-tls, key: tls.crt }
      key:    { secretName: client-tls, key: tls.key }
      caCert: { secretName: ca-bundle,  key: ca.crt }
```

### Edition: Production vs Trial

`edition` selects which image is deployed and whether a license is required:

| edition | image | license |
| --- | --- | --- |
| `production` (default) | `registry.synadia.io/insights` | not required |
| `trial` | `registry.synadia.io/insights-licensed` | **required** |

For a trial, supply the license JWT:

```yaml
edition: trial
config:
  license:
    token: <license-jwt>
    # or reference an existing Secret:
    # secretName: my-license
    # fileKey: license.jwt
```

### Persistence

Persistence is enabled by default. Size the volume to your retention needs;
disabling persistence uses an `emptyDir` and **all indexed history is lost on
restart**.

```yaml
persistence:
  enabled: true
  size: 50Gi
  storageClassName: fast-ssd
```

The DuckDB memory limit should be sized to the host and the monitored system:

```yaml
config:
  db:
    memoryLimit: "8GB"   # DuckDB units; caps the engine buffer pool
    threads: 4
    retention:
      duration: 2160h    # keep 90 days; app default 768h (~32 days), 0 disables
container:
  resources:
    requests:
      cpu: "2"
      memory: 8Gi
    limits:
      memory: 16Gi
```

### Exposing the Web UI via Ingress

> **The Insights web UI ships no built-in authentication.** When exposing it,
> front it with an authenticating ingress or proxy (e.g. oauth2-proxy, your
> ingress controller's auth, or a service mesh).

```yaml
ingress:
  enabled: true
  className: nginx
  hosts:
  - insights.example.com
  tlsSecretName: insights-tls   # optional
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt
```

### Advanced Topologies

The API server and data sink default to the embedded NATS server. For advanced
setups (a leaf-node sink config, separate API credentials, extra TLS material),
use the escape hatches: set any `INSIGHTS_*` variable via `container.env`, mount
additional material via `extraVolumes`/`extraVolumeMounts`, and adjust any
resource with its `merge`/`patch` keys.

```yaml
container:
  env:
    INSIGHTS_LOG_LEVEL: debug
```
