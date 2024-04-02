package test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DynamicDefaults struct {
	VersionLabel     string
	HelmChartLabel   string
	PrivateLinkImage string
}

type DynamicDefaultsGetter struct {
	mu  sync.Mutex
	set bool
	dd  DynamicDefaults
}

var ddg DynamicDefaultsGetter

func (d *DynamicDefaultsGetter) Get(t *testing.T) DynamicDefaults {
	t.Helper()

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.set {
		return d.dd
	}

	test := DefaultTest()
	r := HelmRender(t, test)

	require.True(t, r.Deployment.HasValue)

	var ok bool
	d.dd.VersionLabel, ok = r.Deployment.Value.Labels["app.kubernetes.io/version"]
	require.True(t, ok)
	d.dd.HelmChartLabel, ok = r.Deployment.Value.Labels["helm.sh/chart"]
	require.True(t, ok)

	containers := r.Deployment.Value.Spec.Template.Spec.Containers
	require.Len(t, containers, 1)
	d.dd.PrivateLinkImage = containers[0].Image

	return d.dd
}

func DefaultResources(t *testing.T, test *Test) *Resources {
	fullName := test.FullName
	chartName := test.ChartName
	releaseName := test.ReleaseName

	dd := ddg.Get(t)
	dr := GenerateResources(fullName)

	plLabels := func() map[string]string {
		return map[string]string{
			"app.kubernetes.io/component":  "private-link",
			"app.kubernetes.io/instance":   releaseName,
			"app.kubernetes.io/managed-by": "Helm",
			"app.kubernetes.io/name":       chartName,
			"app.kubernetes.io/version":    dd.VersionLabel,
			"helm.sh/chart":                dd.HelmChartLabel,
		}
	}
	plSelectorLabels := func() map[string]string {
		return map[string]string{
			"app.kubernetes.io/component": "private-link",
			"app.kubernetes.io/instance":  releaseName,
			"app.kubernetes.io/name":      chartName,
		}
	}

	replicas1 := int32(1)
	falseBool := false
	fsGroup := int64(1000)
	fsGroupChangePolicy := corev1.FSGroupChangeOnRootMismatch

	return &Resources{
		Deployment: Resource[appsv1.Deployment]{
			ID:       dr.Deployment.ID,
			HasValue: true,
			Value: appsv1.Deployment{
				TypeMeta: v1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:   fullName,
					Labels: plLabels(),
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas1,
					Selector: &v1.LabelSelector{
						MatchLabels: plSelectorLabels(),
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: v1.ObjectMeta{
							Labels: plLabels(),
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Image: dd.PrivateLinkImage,
									Name:  "private-link",
									Args: []string{
										"--token=agt_my_token",
										"--nats-url=nats://connect.ngs.global",
									},
								},
							},
							EnableServiceLinks: &falseBool,
							SecurityContext: &corev1.PodSecurityContext{
								FSGroup:             &fsGroup,
								FSGroupChangePolicy: &fsGroupChangePolicy,
							},
						},
					},
				},
			},
		},
		ImagePullSecret: Resource[corev1.Secret]{
			ID:       dr.ImagePullSecret.ID,
			HasValue: false,
			Value: corev1.Secret{
				TypeMeta: v1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:   fullName + "-regcred",
					Labels: plLabels(),
				},
				Type: corev1.SecretTypeDockerConfigJson,
				StringData: map[string]string{
					corev1.DockerConfigJsonKey: `{"auths":{"registry.synadia.io":{}}}
`,
				},
			},
		},
		ServiceAccount: Resource[corev1.ServiceAccount]{
			ID:       dr.ServiceAccount.ID,
			HasValue: false,
			Value: corev1.ServiceAccount{
				TypeMeta: v1.TypeMeta{
					Kind:       "ServiceAccount",
					APIVersion: "v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:   fullName,
					Labels: plLabels(),
				},
			},
		},
		ExtraConfigMap: Resource[corev1.ConfigMap]{
			ID:       dr.ExtraConfigMap.ID,
			HasValue: false,
			Value: corev1.ConfigMap{
				TypeMeta: v1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:   fullName + "-extra",
					Labels: plLabels(),
				},
			},
		},
		ExtraService: Resource[corev1.Service]{
			ID:       dr.ExtraService.ID,
			HasValue: false,
			Value: corev1.Service{
				TypeMeta: v1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:   fullName + "-extra",
					Labels: plLabels(),
				},
				Spec: corev1.ServiceSpec{
					Selector: plSelectorLabels(),
				},
			},
		},
	}
}

func TestDefaultValues(t *testing.T) {
	t.Parallel()
	test := DefaultTest()
	expected := DefaultResources(t, test)
	RenderAndCheck(t, test, expected)
}
