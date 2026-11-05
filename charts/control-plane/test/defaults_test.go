package test

import (
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

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

	require.True(t, r.StatefulSet.HasValue)

	var ok bool
	d.dd.VersionLabel, ok = r.StatefulSet.Value.Labels["app.kubernetes.io/version"]
	require.True(t, ok)
	d.dd.HelmChartLabel, ok = r.StatefulSet.Value.Labels["helm.sh/chart"]
	require.True(t, ok)

	containers := r.StatefulSet.Value.Spec.Template.Spec.Containers
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

	resource20Gi, _ := resource.ParseQuantity("20Gi")
	replicas1 := int32(1)
	falseBool := false
	prefixPath := networkingv1.PathTypePrefix
	fsGroup := int64(1000)
	fsGroupChangePolicy := corev1.FSGroupChangeOnRootMismatch

	return &Resources{
		Conf: Resource[map[string]any]{
			ID:       dr.Conf.ID,
			HasValue: true,
			Value: map[string]any{
				"data_dir": "/data",
				"mode":     "standalone",
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
		StatefulSet: Resource[appsv1.StatefulSet]{
			ID:       dr.StatefulSet.ID,
			HasValue: true,
			Value: appsv1.StatefulSet{
				TypeMeta: v1.TypeMeta{
					Kind:       "StatefulSet",
					APIVersion: "apps/v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:   fullName,
					Labels: cpLabels(),
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: &replicas1,
					Selector: &v1.LabelSelector{
						MatchLabels: cpSelectorLabels(),
					},
					ServiceName:         fullName + "-headless",
					PodManagementPolicy: appsv1.ParallelPodManagement,
					PersistentVolumeClaimRetentionPolicy: &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
						WhenDeleted: appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
						WhenScaled:  appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
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
									Args: []string{
										"server",
										"-c",
										"/etc/syn-cp/syn-cp.json",
									},
									Ports: []corev1.ContainerPort{
										{
											Name:          "http",
											ContainerPort: 8080,
										},
									},
									StartupProbe: &corev1.Probe{
										PeriodSeconds:    5,
										FailureThreshold: 60,
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path: "/healthz",
												Port: intstr.FromString("http"),
											},
										},
									},
									ReadinessProbe: &corev1.Probe{
										PeriodSeconds:    10,
										FailureThreshold: 3,
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path: "/healthz",
												Port: intstr.FromString("http"),
											},
										},
									},
									LivenessProbe: &corev1.Probe{
										PeriodSeconds:    10,
										FailureThreshold: 3,
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path: "/healthz",
												Port: intstr.FromString("http"),
											},
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
									},
								},
							},
							EnableServiceLinks: &falseBool,
							ImagePullSecrets: []corev1.LocalObjectReference{
								{
									Name: "control-plane-regcred",
								},
							},
							SecurityContext: &corev1.PodSecurityContext{
								FSGroup:             &fsGroup,
								FSGroupChangePolicy: &fsGroupChangePolicy,
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
							},
						},
					},
					VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:   "data",
								Labels: cpSelectorLabels(),
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
					},
				},
			},
		},
		HeadlessService: Resource[corev1.Service]{
			ID:       dr.HeadlessService.ID,
			HasValue: true,
			Value: corev1.Service{
				TypeMeta: v1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:   fullName + "-headless",
					Labels: cpLabels(),
				},
				Spec: corev1.ServiceSpec{
					ClusterIP:                "None",
					PublishNotReadyAddresses: true,
					Selector:                 cpSelectorLabels(),
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
					corev1.DockerConfigJsonKey: `{"auths":{"registry.synadia.io":{}}}
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
					Name:   fullName,
					Labels: cpLabels(),
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "cp.nats.io",
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
														Name: "http",
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
