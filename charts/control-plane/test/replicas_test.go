package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestMultiReplica(t *testing.T) {
	t.Parallel()
	test := DefaultTest()
	test.Values = `
statefulSet:
  replicas: 3
config:
  kms:
    key:
      secretName: key
  dataSources:
    prometheus:
      url: https://prometheus:9090
`
	expected := DefaultResources(t, test)
	expected.Conf.Value["kms"] = map[string]any{
		"key_url": "file:///etc/syn-cp/kms/key.enc",
	}
	expected.Conf.Value["data_sources"] = map[string]any{
		"prometheus": map[string]any{
			"url": "https://prometheus:9090",
		},
	}
	expected.Conf.Value["replicas"] = []any{
		"control-plane-0.control-plane-headless.control-plane.svc.cluster.local",
		"control-plane-1.control-plane-headless.control-plane.svc.cluster.local",
		"control-plane-2.control-plane-headless.control-plane.svc.cluster.local",
	}

	expected.PDB.HasValue = true

	sts := &expected.StatefulSet.Value
	sts.Annotations["control-plane.synadia.io/replica-mode"] = "cluster"
	replicas3 := int32(3)
	sts.Spec.Replicas = &replicas3

	pts := &sts.Spec.Template.Spec
	pts.Volumes = append(pts.Volumes, corev1.Volume{
		Name: "kms-key",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "key",
			},
		},
	})
	ctr := &pts.Containers[0]
	ctr.VolumeMounts = append(ctr.VolumeMounts, corev1.VolumeMount{
		Name:      "kms-key",
		MountPath: "/etc/syn-cp/kms",
	})

	RenderAndCheck(t, test, expected)
}

func TestPodDisruptionBudgetDisabled(t *testing.T) {
	t.Parallel()
	test := DefaultTest()
	test.Values = `
statefulSet:
  replicas: 3
config:
  kms:
    key:
      url: base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=
  dataSources:
    prometheus:
      url: https://prometheus:9090
podDisruptionBudget:
  enabled: false
`
	actual := HelmRender(t, test)
	require.True(t, actual.StatefulSet.HasValue)
	require.False(t, actual.PDB.HasValue)
}

// config.dataSources.postgres keeps its chart 1.x options, and becomes the source of the v1 migration.
func TestV1Postgres(t *testing.T) {
	t.Parallel()
	test := DefaultTest()
	test.Values = `
config:
  dataSources:
    postgres:
      dsn: postgres://cp:pass@postgres:5432/cp?connect_timeout=10
      tls:
        enabled: true
        secretName: postgres-tls
        cert: tls.crt
        key: tls.key
`
	expected := DefaultResources(t, test)
	expected.Conf.Value["migration"] = map[string]any{
		"v1": map[string]any{
			"postgres_dsn": "postgres://cp:pass@postgres:5432/cp?connect_timeout=10&sslmode=verify-full" +
				"&sslcert=/etc/syn-cp/certs/postgres/tls.crt&sslkey=/etc/syn-cp/certs/postgres/tls.key" +
				"&sslrootcert=/etc/syn-cp/certs/postgres/tls.ca",
		},
	}

	pts := &expected.StatefulSet.Value.Spec.Template.Spec
	pts.Volumes = append(pts.Volumes, corev1.Volume{
		Name: "postgres-tls",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "postgres-tls",
			},
		},
	})
	ctr := &pts.Containers[0]
	ctr.VolumeMounts = append(ctr.VolumeMounts, corev1.VolumeMount{
		Name:      "postgres-tls",
		MountPath: "/etc/syn-cp/certs/postgres",
	})

	RenderAndCheck(t, test, expected)
}

// A chart 1.x multi-replica values file renders a multi-replica cluster without changes.
func TestV1Values(t *testing.T) {
	t.Parallel()
	test := DefaultTest()
	test.Values = `
singleReplicaMode:
  enabled: false
  encryptionPvc:
    size: 2Gi
deployment:
  replicas: 3
  name: scp
container:
  image:
    slim: true
config:
  kms:
    key:
      url: base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=
  dataSources:
    postgres:
      dsn: postgres://cp:pass@postgres:5432/cp
    prometheus:
      url: https://prometheus:9090
`
	actual := HelmRender(t, test)

	// the default ID uses the fullname, and deployment.name renames the StatefulSet
	require.False(t, actual.StatefulSet.HasValue)
	require.Equal(t, []any{
		"scp-0.control-plane-headless.control-plane.svc.cluster.local",
		"scp-1.control-plane-headless.control-plane.svc.cluster.local",
		"scp-2.control-plane-headless.control-plane.svc.cluster.local",
	}, actual.Conf.Value["replicas"])
	require.Equal(t, map[string]any{"v1": map[string]any{"postgres_dsn": "postgres://cp:pass@postgres:5432/cp"}},
		actual.Conf.Value["migration"])
	require.True(t, actual.PDB.HasValue)
}

func TestInvalidValues(t *testing.T) {
	t.Parallel()

	multiReplica := `
statefulSet:
  replicas: 3
config:
  kms:
    key:
      url: base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=
  dataSources:
    prometheus:
      url: https://prometheus:9090
`
	cases := map[string]struct {
		values string
		err    string
	}{
		"no replicas": {
			values: `{statefulSet: {replicas: 0}}`,
			err:    "statefulSet.replicas is 0",
		},
		"2 replicas": {
			values: `{statefulSet: {replicas: 2}}`,
			err:    "statefulSet.replicas is 2",
		},
		"multi-replica default key": {
			values: `{statefulSet: {replicas: 3}, config: {dataSources: {prometheus: {url: "https://prometheus:9090"}}}}`,
			err:    "requires config.kms.key",
		},
		"multi-replica internal prometheus": {
			values: `{statefulSet: {replicas: 3}, config: {kms: {key: {secretName: key}}}}`,
			err:    "requires an external Prometheus",
		},
		"1.x 2 replicas": {
			values: `{deployment: {replicas: 2}}`,
			err:    "deployment.replicas is 2",
		},
		"1.x single replica mode with replicas": {
			values: multiReplica + `
singleReplicaMode:
  enabled: true
`,
			err: "singleReplicaMode.enabled is true, but statefulSet.replicas is 3",
		},
		"1.x multi-replica mode with 1 replica": {
			values: `{singleReplicaMode: {enabled: false}}`,
			err:    "singleReplicaMode.enabled is false, but statefulSet.replicas is 1",
		},
		"1.x deployment merge": {
			values: `{deployment: {merge: {spec: {strategy: {type: Recreate}}}}}`,
			err:    "Chart 2.x runs a StatefulSet",
		},
		"1.x image": {
			values: `{container: {image: {tag: 1.14.1}}}`,
			err:    "container.image.tag 1.14.1 is a Control Plane 1.x image",
		},
		"managed without license": {
			values: `{config: {mode: managed}}`,
			err:    "requires config.merge.license.enabled",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := HelmRenderE(t, c.values)
			require.ErrorContains(t, err, c.err)
		})
	}
}

// HelmRenderE renders the chart and returns the error, for values that the chart must reject.
func HelmRenderE(t *testing.T, values string) error {
	t.Helper()
	test := DefaultTest()

	chart, err := filepath.Abs("..")
	require.NoError(t, err)

	valuesFile := filepath.Join(t.TempDir(), "values.yaml")
	require.NoError(t, os.WriteFile(valuesFile, []byte(values), 0o600))

	options := &helm.Options{
		ValuesFiles:    []string{valuesFile},
		KubectlOptions: k8s.NewKubectlOptions("", "", test.Namespace),
	}
	_, err = helm.RenderTemplateE(t, options, chart, test.ReleaseName, nil)
	return err
}
