# Synadia Nex CE Helm Chart

## Accessing the Helm Chart

```bash
# add the synadia repo (only needs to be run once)
helm repo add synadia https://synadia-io.github.io/helm-charts

# update the synadia repo index (run to get updated chart versions)
helm repo update synadia

# now you can install the synadia/nex-ce chart (see Common Configuration for values.yaml)
helm upgrade --install nex-ce synadia/nex-ce -f values.yaml
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

### Workload namespaces, RBAC and ServiceAccounts

For every workload namespace (`workloadsNamespace`, `connectorsNamespace`) the chart creates:

- a Role and RoleBinding granting the node's ServiceAccount exactly what the Kubernetes nexlet uses there: Deployments (get/list/watch/create/delete/patch), pods (get/list/watch), `pods/log`, Secrets (create/get/list/delete/patch) and `metrics.k8s.io` pods. Nothing cluster-scoped.
- a token-less ServiceAccount for workload pods (`workloadServiceAccount`), passed to the nexlets as `k8sServiceAccountName` unless `config.nexlets.<nexlet>.serviceAccountName` is set.

Label workload namespaces with PodSecurity `restricted`; the nexlet renders every workload pod for that profile, and the node pod runs as uid 65532 with a read-only root filesystem.

### Private registry for workload images

nex-ce `0.3.3` reads only `k8sKubeconfig` and `k8sNamespace` from the nexlet config and starts workload pods with the `default` ServiceAccount of the workload namespace. To pull workload images from a private registry on that release, attach a pull secret to that ServiceAccount:

```bash
kubectl -n nex-workloads patch serviceaccount default \
  -p '{"imagePullSecrets":[{"name":"synadia-registry"}]}'
```

The per-nexlet keys below map onto the nexlet's `k8sServiceAccountName`, `k8sImagePullSecrets`, `k8sDefaultCpu` / `k8sDefaultMemoryMb` and `k8sMetricsPort` and take effect with the first nex-ce release that parses them (they are in nexlet.kubernetes `v0.1.1`; the nex-ce glue is pending). Older nex-ce builds ignore them silently, and the `workloadServiceAccount` stays unused until then.

```yaml
config:
  nexlets:
    connectors:
      imagePullSecrets: [synadia-registry]
      defaultCpu: 1
      defaultMemoryMb: 1024
```

Workload images must set a numeric, non-root `USER`.

### Validation

The chart refuses to render a node that cannot connect: set `config.platform.enabled` (with a token) or `config.url`, and enable at least one nexlet.
