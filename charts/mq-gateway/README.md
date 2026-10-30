# MQ Gateway Helm chart

This chart runs MQ Gateway, which serves MQTT 3.1.1, MQTT 5.0 and AMQP 1.0
clients on NATS. It runs two or more gateways behind one Service, with a
PodDisruptionBudget, and writes their config file into a Secret.

It does not run NATS or create the streams and KV buckets the gateway
stores messages and sessions in. Install NATS with JetStream first, for
example with the [nats chart](https://github.com/nats-io/k8s/tree/main/helm/charts/nats),
and create the streams and buckets as described below.

## Install

Give the gateway its NATS servers, a credential, and what to serve:

```sh
kubectl create secret generic mq-gateway-nats --from-file=nats.creds=gateway.creds
```

```yaml
# values.yaml
nats:
  url: nats://nats.nats.svc.cluster.local:4222
  credentials:
    existingSecret: mq-gateway-nats

config:
  monitor:
    addr: ":8222"
  bindings:
    - { subjects: ["orders.>"], stream: ORDERS }
  mqtt:
    bindings: [ORDERS]
    retained: { bucket: MQTT_RETAINED }
    sessions: { bucket: MQTT_SESSIONS }
  amqp:
    bindings: [ORDERS]
    delivery-counts: { bucket: AMQP_COUNTS }
```

```sh
helm repo add synadia https://synadia-io.github.io/helm-charts
helm repo update synadia
helm upgrade --install mq-gateway synadia/mq-gateway -f values.yaml
```

MQTT clients connect to `mq-gateway:1883` and AMQP clients to
`mq-gateway:5672`. To reach the gateway from outside the cluster, set
`service.type: LoadBalancer`.

## Create the streams and buckets

The gateway never creates a stream or a bucket. Until those its config
names exist it refuses the clients that need them and logs `account's
startup check failed; trying again`; once they are created it serves them,
with no restart. Each pod stays ready meanwhile, since every replica would
fail the same way; `/healthz` names what is missing.

`mq-gateway generate` prints them as nats CLI commands, from the config the
chart wrote. Run it in a gateway pod, fill in the limits it asks for, and
run the script with the nats CLI, `NATS_URL` and `NATS_CREDS` naming a NATS
user allowed to create streams (`kubectl port-forward` reaches a NATS server
inside the cluster):

```sh
kubectl exec deploy/mq-gateway -- mq-gateway generate \
  --config /etc/mq-gateway/config/config.yaml --format nats-cli > provision.sh
bash provision.sh
kubectl exec deploy/mq-gateway -- mq-gateway check \
  --config /etc/mq-gateway/config/config.yaml
```

`--format nack` and `--format terraform` print NACK manifests and Terraform
resources instead. The MQ Gateway guide *Provision the streams and buckets*
explains each limit.

## Configuration mapping

Everything below `config` is written to the gateway's config file, so keys,
nesting and future settings work as they do in a standalone file. The chart
fills in only what it manages, and refuses a value set both ways:

| Value | Written as |
| --- | --- |
| `nats.url` | `nats.url` |
| `nats.credentials.existingSecret` | `nats.creds`, the Secret's key mounted at `/etc/mq-gateway/nats/nats.creds` |
| `nats.tls.existingSecret` | `nats.tls`, the Secret's keys mounted under `/etc/mq-gateway/nats-tls` |
| `tls.existingSecret` | `tls`, the listeners' certificate, key and optional client CA, under `/etc/mq-gateway/tls` |
| `license.existingSecret` | `license.file`, the Secret's key mounted at `/etc/mq-gateway/license/license.jwt` |

Each pod's gateway id is its pod name, set as `MQGW_ID`, since gateways
sharing a sessions bucket need distinct ids; `config.id` is refused. A
config must serve MQTT or AMQP (an `mqtt` or `amqp` block, even `{}`) and
keep `monitor.addr`, whose `/readyz` and `/livez` are the probes.

The pod and the Service expose one port per listener in `config.mqtt.listeners`
and `config.amqp.listeners`, or the protocol's default (`1883`, `5672`) for a
block without listeners. A listener's `name` is its port name, so it must be a
valid Kubernetes port name; one without a name is called `mqtt`, `amqp`,
`mqtt-1` and so on. The health port is not on the Service.

```yaml
config:
  mqtt:
    listeners:
      - { name: mqtt, addr: ":1883" }
      - { name: mqtt-ws, addr: ":8080", ws-path: /mqtt }
```

Every serving setting with a flag also has an `MQGW_` variable, which
outranks the file. `extraEnv` sets them, and the variables an `env://`
location in the config reads, as a literal or a whole `valueFrom` body:

```yaml
config:
  nats:
    user: gateway
    password: env://NATS_PASSWORD
extraEnv:
  MQGW_LOG_FORMAT: json
  NATS_PASSWORD:
    valueFrom:
      secretKeyRef:
        name: mq-gateway-nats
        key: password
```

## Images and licensing

| Values | Default image |
| --- | --- |
| No license | `registry.synadia.io/mq-gateway` |
| `license.existingSecret`, `config.license.token` or `config.license.file` | `registry.synadia.io/mq-gateway-licensed` |

The licensed image serves only under a Synadia license that entitles MQ
Gateway:

```sh
kubectl create secret generic mq-gateway-license --from-file=license.jwt=license.jwt
```

```yaml
license:
  existingSecret: mq-gateway-license
```

Pulling `registry.synadia.io/mq-gateway` needs authentication: create a pull
secret with the registry credentials issued to you and reference it.

```sh
kubectl create secret docker-registry synadia-registry \
  --docker-server registry.synadia.io \
  --docker-username "$SYNADIA_REGISTRY_USERNAME" \
  --docker-password "$SYNADIA_REGISTRY_PASSWORD"
```

```yaml
image:
  pullSecrets:
    - name: synadia-registry
```

Set `image.tag` to pin a version and `image.registry` for a mirror; an
explicit `image.repository` overrides the licensed or unlicensed choice.

## High availability

Any gateway can serve any client: sessions, retained messages and stored
messages live in NATS. With `mqtt.sessions.bucket` in the config, a client
that reconnects to another replica resumes its session there. So the chart
runs two replicas by default, prefers to put them on different nodes
(`podAntiAffinity: soft`, or `hard` to require it) and spreads them over
zones, and its PodDisruptionBudget lets a drain take one at a time.

A rollout starts each new gateway and waits for its `/readyz` before it
stops an old one (`maxSurge: 1`, `maxUnavailable: 0`). A config change
changes the config Secret's checksum on the pod template, so `helm upgrade`
rolls the gateways; the gateway reads its config only at start.

When a pod stops, its preStop hook runs `mq-gateway signal ldm`, which puts
the gateway into lame duck mode: it closes its listeners at once, then after
`ldm.grace-period` (10s) its connections, spread over the rest of
`ldm.duration` (2m), writing each session for another gateway to resume, and
exits. Its clients reconnect through the Service to the other replicas a few
at a time rather than all at once. The kubelet's SIGTERM is ignored
meanwhile, and `terminationGracePeriodSeconds` follows `ldm.duration`, plus
20 seconds, so the drain is not cut short. With `configSecret.existingSecret`
the chart cannot read the duration: set `terminationGracePeriodSeconds` if
it is not the default.

`/readyz` answers 503 from the moment the gateway starts to stop, and while
its NATS connections reconnect, so the Service stops routing to it. The
liveness probe is `/livez`, which fails only when the process stops
answering.

## Existing config Secret

To manage the whole config file yourself, put it in a Secret and set
`configSecret.existingSecret`. The chart cannot read it, so it changes
nothing in it: the file names its NATS servers, and the paths above for any
credential, TLS or license Secret you still mount through the chart. Name
its listeners and health port so the pod and Service expose them, and set
`image.repository: mq-gateway-licensed` if it carries a license:

```yaml
configSecret:
  existingSecret: mq-gateway-config
  key: config.yaml
  listeners:
    - { name: mqtt, port: 1883 }
  monitorPort: 8222
```

The pods do not restart when that Secret changes:
`kubectl rollout restart deployment/mq-gateway` applies it.

## Secrets

The generated config is a Kubernetes Secret because it may hold credentials.
Kubernetes Secrets are not encrypted unless the cluster encrypts them at
rest, and values passed to Helm are kept in its release history. Reference
Secrets you manage (`nats.credentials`, `license`, `extraEnv` with
`secretKeyRef`) rather than putting credentials in values.

A `creds` file is read again on every NATS reconnect, so a rotated
credentials Secret needs no restart: the gateway presents it the next time
it reconnects. A license is read at start: restart the deployment after
replacing it.

## Security context

The pod runs as the image's unprivileged user 65532 with `runAsNonRoot`, a
read-only root filesystem, no capabilities and no privilege escalation. The
gateway keeps nothing on disk and calls no Kubernetes API, so the pod mounts
no ServiceAccount token.
