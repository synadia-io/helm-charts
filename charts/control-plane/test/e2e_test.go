//go:build e2e

// End-to-end tests against the cluster in the current kube context.
//
//	go test -tags e2e -timeout 20m
//
// E2E_VALUES optionally names a values file, for example to set the image registry and pull secret.
package test

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
)

const e2eRelease = "cp"

func e2eSetup(t *testing.T) (*k8s.KubectlOptions, *helm.Options, string) {
	t.Helper()
	ns := "cp-e2e-" + strings.ToLower(random.UniqueId())
	kubectl := k8s.NewKubectlOptions("", "", ns)
	k8s.CreateNamespace(t, kubectl, ns)
	t.Cleanup(func() { k8s.DeleteNamespace(t, kubectl, ns) })

	opts := &helm.Options{KubectlOptions: kubectl, SetValues: map[string]string{}}
	if f := os.Getenv("E2E_VALUES"); f != "" {
		opts.ValuesFiles = []string{f}
	}

	chart, err := filepath.Abs("..")
	require.NoError(t, err)
	return kubectl, opts, chart
}

// Installs with a KMS key from a Secret, then restarts the pod and checks that state on the PVC survives.
func TestE2EInstall(t *testing.T) {
	t.Parallel()
	kubectl, opts, chart := e2eSetup(t)

	k8s.RunKubectl(t, kubectl, "create", "secret", "generic", "cp-kms", "--from-literal=key.enc="+e2eKey(t))

	opts.SetValues["config.kms.key.secretName"] = "cp-kms"
	opts.ExtraArgs = map[string][]string{"install": {"--wait", "--timeout", "5m"}}
	helm.Install(t, opts, chart, e2eRelease)

	pod := e2eRelease + "-control-plane-0"
	k8s.RunKubectl(t, kubectl, "delete", "pod", pod)
	k8s.WaitUntilPodAvailable(t, kubectl, pod, 60, 5*time.Second)

	logs, err := k8s.RunKubectlAndGetOutputE(t, kubectl, "logs", pod)
	require.NoError(t, err)
	require.Contains(t, logs, "loaded existing bootstrap config from disk store")
}

// Upgrading a single-replica 1.x release must fail before it deletes the 1.x PVCs.
func TestE2EUpgradeGuard(t *testing.T) {
	t.Parallel()
	kubectl, opts, chart := e2eSetup(t)

	// the 1.x pod does not need to start, the guard only looks for the 1.x Deployment
	installV1(t, kubectl)

	err := helm.UpgradeE(t, opts, chart, e2eRelease)
	require.ErrorContains(t, err, "control-plane chart 1.x in single-replica mode")
	k8s.RunKubectl(t, kubectl, "get", "pvc", e2eRelease+"-control-plane-encryption")
}

// A multi-replica 1.x release upgrades with its 1.x values file, once that file sets config.mode.
func TestE2EUpgradeV1MultiReplica(t *testing.T) {
	t.Parallel()
	kubectl, opts, chart := e2eSetup(t)

	key, rotatedKey := e2eKey(t), e2eKey(t)
	v1Values := filepath.Join(t.TempDir(), "values.yaml")
	require.NoError(t, os.WriteFile(v1Values, []byte(`
singleReplicaMode:
  enabled: false
deployment:
  replicas: 3
config:
  kms:
    key:
      url: `+key+`
    rotatedKeys:
    - url: `+rotatedKey+`
  dataSources:
    postgres:
      dsn: postgres://cp:cp@postgres:5432/cp
    prometheus:
      url: http://prometheus:9090
`), 0o600))
	installV1(t, kubectl, "-f", v1Values)

	// the 2.x upgrade uses the same values file
	upgrade := func(values map[string]string) error {
		o := *opts
		o.ValuesFiles = append(append([]string{}, opts.ValuesFiles...), v1Values)
		o.SetValues = values
		return helm.UpgradeE(t, &o, chart, e2eRelease)
	}

	// failed upgrades leave the 1.x release as it was, so the cases run in order against it
	require.ErrorContains(t, upgrade(map[string]string{}),
		"where every user logs in")
	require.ErrorContains(t, upgrade(map[string]string{"config.mode": "standalone", "deployment.replicas": "1", "singleReplicaMode.enabled": "true"}),
		"upgrades only to a multi-replica cluster")
	require.ErrorContains(t, upgrade(map[string]string{"config.mode": "standalone", "config.dataSources.postgres.dsn": ""}),
		"Set config.dataSources.postgres.dsn")
	require.ErrorContains(t, upgrade(map[string]string{"config.mode": "standalone", "config.kms.key.url": e2eKey(t)}),
		"missing the 1.x kms.key_url")
	require.ErrorContains(t, upgrade(map[string]string{"config.mode": "standalone", "config.kms.rotatedKeys": "null"}),
		"missing the 1.x kms.rotated_key_urls[0]")

	require.NoError(t, upgrade(map[string]string{"config.mode": "managed", "config.merge.license.enabled": "true"}))
	_, err := k8s.GetDeploymentE(t, kubectl, e2eRelease+"-control-plane")
	require.Error(t, err)
	replicas, err := k8s.RunKubectlAndGetOutputE(t, kubectl, "get", "statefulset", e2eRelease+"-control-plane", "-o", "jsonpath={.spec.replicas}")
	require.NoError(t, err)
	require.Equal(t, "3", replicas)
}

// A 2.x release cannot switch between a single replica and a multi-replica cluster, but a cluster can change size.
func TestE2EReplicaMode(t *testing.T) {
	t.Parallel()
	key := e2eKey(t)
	multiReplica := func() map[string]string {
		return map[string]string{
			"statefulSet.replicas":              "3",
			"config.kms.key.url":                key,
			"config.dataSources.prometheus.url": "http://prometheus:9090",
		}
	}

	t.Run("single to multi", func(t *testing.T) {
		t.Parallel()
		_, opts, chart := e2eSetup(t)
		helm.Install(t, opts, chart, e2eRelease)

		opts.SetValues = multiReplica()
		err := helm.UpgradeE(t, opts, chart, e2eRelease)
		require.ErrorContains(t, err, "runs a single replica, and this upgrade would make it a multi-replica cluster")
	})

	t.Run("resize multi", func(t *testing.T) {
		t.Parallel()
		_, opts, chart := e2eSetup(t)
		opts.SetValues = multiReplica()
		helm.Install(t, opts, chart, e2eRelease)

		opts.SetValues["statefulSet.replicas"] = "5"
		require.NoError(t, helm.UpgradeE(t, opts, chart, e2eRelease))

		// with no values at all, helm upgrade reuses the release's values
		opts.SetValues = map[string]string{"statefulSet.replicas": "1"}
		err := helm.UpgradeE(t, opts, chart, e2eRelease)
		require.ErrorContains(t, err, "runs a multi-replica cluster, and this upgrade would make it a single replica")
	})
}

func installV1(t *testing.T, kubectl *k8s.KubectlOptions, args ...string) {
	t.Helper()
	args = append([]string{"install", e2eRelease, "control-plane",
		"--repo", "https://synadia-io.github.io/helm-charts", "--version", "1.14.1"}, args...)
	_, err := helm.RunHelmCommandAndGetOutputE(t, &helm.Options{KubectlOptions: kubectl}, args[0], args[1:]...)
	require.NoError(t, err)
}

func e2eKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return "base64key://" + base64.URLEncoding.EncodeToString(key)
}
