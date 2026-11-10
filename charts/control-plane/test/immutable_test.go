package test

import (
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestStatefulSetImmutableFields pins the StatefulSet fields that Kubernetes does not allow to change after install.
// A chart that renders them differently fails every upgrade of an existing 2.x release, so only change the expected
// values together with an upgrade path for those releases.
func TestStatefulSetImmutableFields(t *testing.T) {
	t.Parallel()

	// the default test release is named control-plane
	selectorLabels := func() map[string]string {
		return map[string]string{
			"app.kubernetes.io/component": "control-plane",
			"app.kubernetes.io/instance":  "control-plane",
			"app.kubernetes.io/name":      "control-plane",
		}
	}
	resource20Gi := resource.MustParse("20Gi")
	expectedSelector := &v1.LabelSelector{
		MatchLabels: selectorLabels(),
	}
	expectedVolumeClaimTemplates := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: v1.ObjectMeta{
				Name:   "data",
				Labels: selectorLabels(),
				Annotations: map[string]string{
					"helm.sh/resource-policy": "keep",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					"ReadWriteOnce",
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						"storage": resource20Gi,
					},
				},
			},
		},
	}

	// values that change other parts of the release must leave the immutable fields alone
	values := map[string]string{
		"defaults": `{}`,
		"labels": `
global:
  labels:
    global: global
statefulSet:
  merge:
    metadata:
      labels:
        test: test
podTemplate:
  merge:
    metadata:
      labels:
        test: test
`,
		"config": `
config:
  mode: managed
  server:
    url: https://cp.example.com
  kms:
    key:
      secretName: control-plane-kms
  merge:
    license:
      enabled: true
`,
		"image": `
container:
  image:
    tag: 2.99.0
`,
		"multi-replica": `
statefulSet:
  replicas: 5
config:
  kms:
    key:
      secretName: control-plane-kms
  dataSources:
    prometheus:
      url: https://prometheus:9090
`,
	}

	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			test := DefaultTest()
			test.Values = value

			actual := HelmRender(t, test)
			require.True(t, actual.StatefulSet.HasValue)

			spec := actual.StatefulSet.Value.Spec
			require.Equal(t, expectedSelector, spec.Selector)
			require.Equal(t, test.FullName+"-headless", spec.ServiceName)
			require.Equal(t, appsv1.ParallelPodManagement, spec.PodManagementPolicy)
			require.Equal(t, expectedVolumeClaimTemplates, spec.VolumeClaimTemplates)
		})
	}
}
