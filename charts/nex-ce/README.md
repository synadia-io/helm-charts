# Synadia Nex CE Helm Chart

## Accessing the Helm Chart

```bash
# add the synadia repo (only needs to be run once)
helm repo add synadia https://synadia-io.github.io/helm-charts

# update the synadia repo index (run to get updated chart versions)
helm repo update synadia

# now you can install the synadia/nex-ce chart
helm upgrade --install nex-ce synadia/nex-ce
```

### Useful Tools and References

- [Chart Values file](https://github.com/synadia-io/helm-charts/blob/main/charts/nex-ce/values.yaml) - lists all possible configuration options

## Common Configuration

The chart runs one Nex CE node with the Kubernetes nexlets. The node creates a Deployment and a Secret for each workload in `config.workloadsNamespace` (Workloads) and `config.connectorsNamespace` (Connectors). Both default to the release namespace. The chart creates a Role and RoleBinding in each of these namespaces.

Generate a node seed with `nk -gen server`. Back it up: the node finds its workloads by the public key of this seed.

### Control Plane registration

Enable the Workloads platform component in Control Plane system settings and copy its component token. Control Plane then supplies the nexus, the control account and the NATS connection.

```yaml
config:
  nodeSeed: SN...
  workloadsNamespace: nex-workloads
  connectorsNamespace: nex-workloads
  platform:
    enabled: true
    url: https://cloud.synadia.com
    token: <workloads component token>
```

To host the connector catalog, enable the Catalog platform component and add:

```yaml
config:
  catalog:
    enabled: true
    id: <catalog NUID>
    token: <catalog component token>
container:
  env:
    YES_SEED: "true"
```

### Direct NATS connection

```yaml
config:
  nodeSeed: SN...
  nexus: nexus
  url: nats://nats.nats.svc.cluster.local:4222
  creds:
    jwt: eyJ...
    seed: SU...
  credsSigning:
    signingKey: SA...
    signingKeyAccount: A...
```

### Private registry for workload images

Nex CE starts workload pods with the `default` ServiceAccount of the workload namespace. To pull workload images from a private registry, attach a pull secret to that ServiceAccount:

```bash
kubectl -n nex-workloads patch serviceaccount default \
  -p '{"imagePullSecrets":[{"name":"synadia-registry"}]}'
```

Workload images must set a numeric, non-root `USER`.
