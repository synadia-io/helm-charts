package test

import (
	"k8s.io/apimachinery/pkg/api/resource"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type DynamicDefaults struct {
	VersionLabel      string
	HelmChartLabel    string
	ControlPlaneImage string
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
	d.dd.ControlPlaneImage = containers[0].Image

	return d.dd
}

func DefaultResources(t *testing.T, test *Test) *Resources {
	fullName := test.FullName
	chartName := test.ChartName
	releaseName := test.ReleaseName

	dd := ddg.Get(t)
	dr := GenerateResources(fullName)

	cpLabels := func() map[string]string {
		return map[string]string{
			"app.kubernetes.io/component":  "control-plane",
			"app.kubernetes.io/instance":   releaseName,
			"app.kubernetes.io/managed-by": "Helm",
			"app.kubernetes.io/name":       chartName,
			"app.kubernetes.io/version":    dd.VersionLabel,
			"helm.sh/chart":                dd.HelmChartLabel,
		}
	}
	cpSelectorLabels := func() map[string]string {
		return map[string]string{
			"app.kubernetes.io/component": "control-plane",
			"app.kubernetes.io/instance":  releaseName,
			"app.kubernetes.io/name":      chartName,
		}
	}

	resource1Gi, _ := resource.ParseQuantity("1Gi")
	resource10Gi, _ := resource.ParseQuantity("10Gi")
	replicas1 := int32(1)
	falseBool := false
	prefixPath := networkingv1.PathTypePrefix

	return &Resources{
		Conf: Resource[map[string]any]{
			ID:       dr.Conf.ID,
			HasValue: true,
			Value: map[string]any{
				"data_dir": "/data",
				"server": map[string]any{
					"http_addr": ":8080",
				},
			},
		},
		ConfigSecret: Resource[corev1.Secret]{
			ID:       dr.ConfigSecret.ID,
			HasValue: true,
			Value: corev1.Secret{
				TypeMeta: v1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				Type: corev1.SecretTypeOpaque,
				ObjectMeta: v1.ObjectMeta{
					Name:   fullName + "-config",
					Labels: cpLabels(),
				},
			},
		},
		ContentsSecret: Resource[corev1.Secret]{
			ID:       dr.ConfigSecret.ID,
			HasValue: false,
			Value: corev1.Secret{
				TypeMeta: v1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				Type: corev1.SecretTypeOpaque,
				ObjectMeta: v1.ObjectMeta{
					Name:   fullName + "-contents",
					Labels: cpLabels(),
				},
			},
		},
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
					Labels: cpLabels(),
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas1,
					Selector: &v1.LabelSelector{
						MatchLabels: cpSelectorLabels(),
					},
					Strategy: appsv1.DeploymentStrategy{
						Type: appsv1.RecreateDeploymentStrategyType,
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: v1.ObjectMeta{
							Labels: cpLabels(),
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Image: dd.ControlPlaneImage,
									Name:  "syn-cp",
									Ports: []corev1.ContainerPort{
										{
											Name:          "http",
											ContainerPort: 8080,
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											MountPath: "/etc/syn-cp",
											Name:      "config",
										},
										{
											MountPath: "/data",
											Name:      "data",
										},
										{
											MountPath: "/data/postgres",
											Name:      "postgres",
										},
										{
											MountPath: "/data/prometheus",
											Name:      "prometheus",
										},
									},
								},
							},
							EnableServiceLinks: &falseBool,
							ImagePullSecrets: []corev1.LocalObjectReference{
								{
									Name: "control-plane-regcred",
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "config",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "control-plane-config",
										},
									},
								},
								{
									Name: "data",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: "control-plane-data",
										},
									},
								},
								{
									Name: "postgres",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: "control-plane-postgres",
										},
									},
								},
								{
									Name: "prometheus",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: "control-plane-prometheus",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		ImagePullSecret: Resource[corev1.Secret]{
			ID:       dr.ConfigSecret.ID,
			HasValue: true,
			Value: corev1.Secret{
				TypeMeta: v1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:   fullName + "-regcred",
					Labels: cpLabels(),
				},
				Type: corev1.SecretTypeDockerConfigJson,
				StringData: map[string]string{
					corev1.DockerConfigJsonKey: `{"auths":{"registry.helix-dev.synadia.io":{}}}
`,
				},
			},
		},
		Ingress: Resource[networkingv1.Ingress]{
			ID:       dr.Ingress.ID,
			HasValue: false,
			Value: networkingv1.Ingress{
				TypeMeta: v1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: "networking.k8s.io/v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:   fullName + "-ws",
					Labels: cpLabels(),
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "demo.nats.io",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path:     "/",
											PathType: &prefixPath,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: fullName,
													Port: networkingv1.ServiceBackendPort{
														Name: "websocket",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Service: Resource[corev1.Service]{
			ID:       dr.Service.ID,
			HasValue: true,
			Value: corev1.Service{
				TypeMeta: v1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:   fullName,
					Labels: cpLabels(),
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "http",
							Port:       80,
							TargetPort: intstr.FromString("http"),
						},
					},
					Selector: cpSelectorLabels(),
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
					Labels: cpLabels(),
				},
			},
		},
		SingleReplicaModeDataPvc: Resource[corev1.PersistentVolumeClaim]{
			ID:       dr.ServiceAccount.ID,
			HasValue: true,
			Value: corev1.PersistentVolumeClaim{
				TypeMeta: v1.TypeMeta{
					Kind:       "PersistentVolumeClaim",
					APIVersion: "v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:   fullName + "-data",
					Labels: cpLabels(),
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						"ReadWriteOnce",
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"storage": resource1Gi,
						},
					},
				},
			},
		},
		SingleReplicaModePostgresPvc: Resource[corev1.PersistentVolumeClaim]{
			ID:       dr.ServiceAccount.ID,
			HasValue: true,
			Value: corev1.PersistentVolumeClaim{
				TypeMeta: v1.TypeMeta{
					Kind:       "PersistentVolumeClaim",
					APIVersion: "v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:   fullName + "-postgres",
					Labels: cpLabels(),
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						"ReadWriteOnce",
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"storage": resource10Gi,
						},
					},
				},
			},
		},
		SingleReplicaModePrometheusPvc: Resource[corev1.PersistentVolumeClaim]{
			ID:       dr.ServiceAccount.ID,
			HasValue: true,
			Value: corev1.PersistentVolumeClaim{
				TypeMeta: v1.TypeMeta{
					Kind:       "PersistentVolumeClaim",
					APIVersion: "v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:   fullName + "-prometheus",
					Labels: cpLabels(),
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						"ReadWriteOnce",
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"storage": resource10Gi,
						},
					},
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
					Labels: cpLabels(),
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
					Labels: cpLabels(),
				},
				Spec: corev1.ServiceSpec{
					Selector: cpSelectorLabels(),
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
