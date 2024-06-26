> [!WARNING]  
> Control Plane 1.4.0 introduces native support for `nats-server`, and `synadia-server` is now deprecated in favor of `nats-server`.  [View the FAQ](https://docs.synadia.com/platform/faq#synadia-server-deprecation) for more information.

---

# Synadia Server Helm Chart

## Accessing the Helm Chart

```bash
# add the synadia repo (only needs to be run once)
helm repo add synadia https://synadia-io.github.io/helm-charts

# update the synadia repo index (run to get updated chart versions)
helm repo update synadia

# now you can install the synadia/synadia-server chart
helm upgrade --install nats synadia/synadia-server
```

---

## Values

There are a handful of explicitly defined options which are documented with comments in the [values.yaml](values.yaml)
file.

Everything in the NATS Config or Kubernetes Resources can be overridden by `merge` and `patch`, which is supported for
the following values:

| key                              | type                                                                                                                        | enabled by default                      |
|----------------------------------|-----------------------------------------------------------------------------------------------------------------------------|-----------------------------------------|
| `config`                         | [NATS Config](https://docs.nats.io/running-a-nats-service/configuration)                                                    | yes                                     |
| `config.cluster`                 | [NATS Cluster](https://docs.nats.io/running-a-nats-service/configuration/clustering/cluster_config)                         | no                                      |
| `config.cluster.tls`             | [NATS TLS](https://docs.nats.io/running-a-nats-service/configuration/securing_nats/tls)                                     | no                                      |
| `config.jetstream`               | [NATS JetStream](https://docs.nats.io/running-a-nats-service/configuration#jetstream)                                       | no                                      |
| `config.jetstream.fileStore.pvc` | [k8s PVC](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#persistentvolumeclaim-v1-core)               | yes, when `config.jetstream` is enabled |
| `config.nats.tls`                | [NATS TLS](https://docs.nats.io/running-a-nats-service/configuration/securing_nats/tls)                                     | no                                      |
| `config.leafnodes`               | [NATS LeafNodes](https://docs.nats.io/running-a-nats-service/configuration/leafnodes/leafnodes_conf)                        | no                                      |
| `config.leafnodes.tls`           | [NATS TLS](https://docs.nats.io/running-a-nats-service/configuration/securing_nats/tls)                                     | no                                      |
| `config.websocket`               | [NATS WebSocket](https://docs.nats.io/running-a-nats-service/configuration/websocket/websocket_conf)                        | no                                      |
| `config.websocket.tls`           | [NATS TLS](https://docs.nats.io/running-a-nats-service/configuration/securing_nats/tls)                                     | no                                      |
| `config.websocket.ingress`       | [k8s Ingress](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#ingress-v1-networking-k8s-io)            | no                                      |
| `config.mqtt`                    | [NATS MQTT](https://docs.nats.io/running-a-nats-service/configuration/mqtt/mqtt_config)                                     | no                                      |
| `config.mqtt.tls`                | [NATS TLS](https://docs.nats.io/running-a-nats-service/configuration/securing_nats/tls)                                     | no                                      |
| `config.gateway`                 | [NATS Gateway](https://docs.nats.io/running-a-nats-service/configuration/gateways/gateway#gateway-configuration-block)      | no                                      |
| `config.gateway.tls`             | [NATS TLS](https://docs.nats.io/running-a-nats-service/configuration/securing_nats/tls)                                     | no                                      |
| `config.resolver`                | [NATS Resolver](https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_intro/jwt/resolver)            | no                                      |
| `config.resolver.pvc`            | [k8s PVC](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#persistentvolumeclaim-v1-core)               | yes, when `config.resolver` is enabled  |
| `container`                      | nats [k8s Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#container-v1-core)                | yes                                     |
| `promExporter`                   | prometheus exporter [k8s Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#container-v1-core) | no                                      |
| `promExporter.podMonitor`        | [prometheus PodMonitor](https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.PodMonitor)             | no                                      |
| `service`                        | [k8s Service](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#service-v1-core)                         | yes                                     |
| `statefulSet`                    | [k8s StatefulSet](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#statefulset-v1-apps)                 | yes                                     |
| `podTemplate`                    | [k8s PodTemplate](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#pod-v1-core)                         | yes                                     |
| `headlessService`                | [k8s Service](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#service-v1-core)                         | yes                                     |
| `configMap`                      | [k8s ConfiegMap](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#configmap-v1-core)                    | yes                                     |
| `optsSecret`                     | [k8s Secret](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secret-v1-core)                           | yes                                     |
| `natsBox.contexts.default`       | [NATS Context](https://docs.nats.io/using-nats/nats-tools/nats_cli#nats-contexts)                                           | yes                                     |
| `natsBox.contexts.[name]`        | [NATS Context](https://docs.nats.io/using-nats/nats-tools/nats_cli#nats-contexts)                                           | no                                      |
| `natsBox.container`              | nats-box [k8s Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#container-v1-core)            | yes                                     |
| `natsBox.deployment`             | [k8s Deployment](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#deployment-v1-apps)                   | yes                                     |
| `natsBox.podTemplate`            | [k8s PodTemplate](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#pod-v1-core)                         | yes                                     |
| `natsBox.contextsSecret`         | [k8s Secret](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secret-v1-core)                           | yes                                     |
| `natsBox.contentsSecret`         | [k8s Secret](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secret-v1-core)                           | yes                                     |

### Merge

Merging is performed using the Helm `merge` function. Example - add NATS accounts and container resources:

```yaml
config:
  merge:
    accounts:
      A:
        users:
          - { user: a, password: a }
      B:
        users:
          - { user: b, password: b }
natsBox:
  contexts:
    a:
      merge: { user: a, password: a }
    b:
      merge: { user: b, password: b }
  defaultContextName: a
```

## Patch

Patching is performed using [JSON Patch](https://jsonpatch.com/). Example - add additional route to end of route list:

```yaml
config:
  cluster:
    enabled: true
    patch:
      - op: add
        path: /routes/-
        value: nats://demo.nats.io:6222
```

## Common Configurations

### Connect to Synadia Control Plane

Provisioning instructions in Synadia Control Plane will provide configuration similar to this:

```yaml
opts:
  # Synadia Control Plane URL
  url: https://cp.nats.io
  # Synadia Control Plane Token
  token: agt_foobar
```

### JetStream Cluster on 3 separate hosts

```yaml
config:
  cluster:
    enabled: true
    replicas: 3
  jetstream:
    enabled: true
    fileStore:
      pvc:
        size: 10Gi

podTemplate:
  topologySpreadConstraints:
    kubernetes.io/hostname:
      maxSkew: 1
      whenUnsatisfiable: DoNotSchedule
```

### NATS Container Resources

```yaml
container:
  env:
    # different from k8s units, suffix must be B, KiB, MiB, GiB, or TiB
    # should be ~90% of memory limit
    GOMEMLIMIT: 7GiB
  merge:
    # recommended limit is at least 2 CPU cores and 8Gi Memory for production JetStream clusters
    resources:
      requests:
        cpu: "2"
        memory: 8Gi
      limits:
        cpu: "2"
        memory: 8Gi
```

### Specify Image Version

```yaml
container:
  image:
    tag: x.y.z
```

### Operator Mode with NATS Resolver

Provisioning instructions in Synadia Control Plane will provide configuration similar to this:

```
config:
  resolver:
    enabled: true
    merge:
      type: full
      interval: 2m
      timeout: 1.9s
  merge:
    operator: OPERATOR_JWT
    system_account: SYS_ACCOUNT_ID
    resolver_preload:
      SYS_ACCOUNT_ID: SYS_ACCOUNT_JWT
```

## Accessing NATS

The chart contains 2 services by default, `service` and `headlessService`.

### `service`

The `service` is intended to be accessed by NATS Clients. It is a `ClusterIP` service by default, however it can easily
be changed to a different service type.

The `nats`, `websocket`, `leafnodes`, and `mqtt` ports will be exposed through this service by default if they are
enabled.

Example: change this service type to a `LoadBalancer`:

```yaml
service:
  merge:
    spec:
      type: LoadBalancer
```

### `headlessService`

The `headlessService` is used for NATS Servers in the Stateful Set to discover one another. It is primarily intended to
be used for Cluster Route connections.

### TLS Considerations

The TLS Certificate used for Client Connections should have a SAN covering DNS Name that clients access the `service`
at.

The TLS Certificate used for Cluster Route Connections should have a SAN covering the DNS Name that routes access each
other on the `headlessService` at. This is `*.<headless-service-name>` by default.

## Advanced Features

### Templating Values

Anything in `values.yaml` can be templated:

- maps matching the following syntax will be templated and parsed as YAML:
  ```yaml
  $tplYaml: |
    yaml template
  ```
- maps matching the follow syntax will be templated, parsed as YAML, and spread into the parent map/slice
  ```yaml
  $tplYamlSpread: |
    yaml template
  ```

Example - change service name:

```yaml
service:
  name:
    $tplYaml: >-
      {{ include "nats.fullname" . }}-svc
```

### NATS Config Units and Variables

NATS configuration extends JSON, and can represent Units and Variables. They must be wrapped in `<< >>` in order to
template correctly. Example:

```yaml
config:
  merge:
    authorization:
      # variable
      token: << $TOKEN >>
    # units
    max_payload: << 2MB >>
```

templates to the `nats.conf`:

```
{
  "authorization": {
    "token": $TOKEN
  },
  "max_payload": 2MB,
  "port": 4222,
  ...
}
```

### NATS Config Includes

Any NATS Config key ending in `$include` will be replaced with an include directive. Included files should be in paths
relative to `/etc/nats-config`. Multiple `$include` keys are supported by using a prefix, and will be sorted
alphabetically. Example:

```yaml
config:
  merge:
    00$include: auth.conf
    01$include: params.conf
configMap:
  merge:
    data:
      auth.conf: |
        accounts: {
          A: {
            users: [
              {user: a, password: a}
            ]
          },
          B: {
            users: [
              {user: b, password: b}
            ]
          },
        }
      params.conf: |
        max_payload: 2MB
```

templates to the `nats.conf`:

```
include auth.conf;
"port": 4222,
...
include params.conf;
```

### Extra Resources

Enables adding additional arbitrary resources. Example - expose WebSocket via VirtualService in Istio:

```yaml
config:
  websocket:
    enabled: true
extraResources:
  - apiVersion: networking.istio.io/v1beta1
    kind: VirtualService
    metadata:
      namespace:
        $tplYamlSpread: >
          {{ include "nats.metadataNamespace" $ }}
      name:
        $tplYaml: >
          {{ include "nats.fullname" $ | quote }}
      labels:
        $tplYaml: |
          {{ include "nats.labels" $ }}
    spec:
      hosts:
        - demo.nats.io
      gateways:
        - my-gateway
      http:
        - name: default
          match:
            - name: root
              uri:
                exact: /
          route:
            - destination:
                host:
                  $tplYaml: >
                    {{ .Values.service.name | quote }}
                port:
                  number:
                    $tplYaml: >
                      {{ .Values.config.websocket.port }}
```
