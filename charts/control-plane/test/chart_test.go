package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

type Resources struct {
	Conf                           Resource[map[string]any]
	ConfigSecret                   Resource[corev1.Secret]
	ContentsSecret                 Resource[corev1.Secret]
	Deployment                     Resource[appsv1.Deployment]
	ImagePullSecret                Resource[corev1.Secret]
	Ingress                        Resource[networkingv1.Ingress]
	Service                        Resource[corev1.Service]
	ServiceAccount                 Resource[corev1.ServiceAccount]
	SingleReplicaModeDataPvc       Resource[corev1.PersistentVolumeClaim]
	SingleReplicaModePostgresPvc   Resource[corev1.PersistentVolumeClaim]
	SingleReplicaModePrometheusPvc Resource[corev1.PersistentVolumeClaim]
	ExtraConfigMap                 Resource[corev1.ConfigMap]
	ExtraService                   Resource[corev1.Service]
}

func (r *Resources) Iter() []MutableResource {
	return []MutableResource{
		r.Conf.Mutable(),
		r.ConfigSecret.Mutable(),
		r.ContentsSecret.Mutable(),
		r.Deployment.Mutable(),
		r.ImagePullSecret.Mutable(),
		r.Ingress.Mutable(),
		r.Service.Mutable(),
		r.ServiceAccount.Mutable(),
		r.SingleReplicaModeDataPvc.Mutable(),
		r.SingleReplicaModePostgresPvc.Mutable(),
		r.SingleReplicaModePrometheusPvc.Mutable(),
		r.ExtraConfigMap.Mutable(),
		r.ExtraService.Mutable(),
	}
}

type Resource[T any] struct {
	ID       string
	HasValue bool
	Value    T
}

func (r *Resource[T]) Mutable() MutableResource {
	return MutableResource{
		ID:        r.ID,
		HasValueP: &r.HasValue,
		ValueP:    &r.Value,
	}
}

type MutableResource struct {
	ID        string
	HasValueP *bool
	ValueP    any
}

type K8sResource struct {
	Kind     string      `yaml:"kind"`
	Metadata K8sMetadata `yaml:"metadata"`
}

type K8sMetadata struct {
	Name string `yaml:"name"`
}

func GenerateResources(fullName string) *Resources {
	return &Resources{
		Conf: Resource[map[string]any]{
			ID: "syn-cp.yaml",
		},
		ConfigSecret: Resource[corev1.Secret]{
			ID: "Secret/" + fullName + "-config",
		},
		ContentsSecret: Resource[corev1.Secret]{
			ID: "Secret/" + fullName + "-contents",
		},
		Deployment: Resource[appsv1.Deployment]{
			ID: "Deployment/" + fullName,
		},
		ImagePullSecret: Resource[corev1.Secret]{
			ID: "Secret/" + fullName + "-regcred",
		},
		Ingress: Resource[networkingv1.Ingress]{
			ID: "Ingress/" + fullName,
		},
		Service: Resource[corev1.Service]{
			ID: "Service/" + fullName,
		},
		ServiceAccount: Resource[corev1.ServiceAccount]{
			ID: "ServiceAccount/" + fullName,
		},
		SingleReplicaModeDataPvc: Resource[corev1.PersistentVolumeClaim]{
			ID: "PersistentVolumeClaim/" + fullName + "-data",
		},
		SingleReplicaModePostgresPvc: Resource[corev1.PersistentVolumeClaim]{
			ID: "PersistentVolumeClaim/" + fullName + "-postgres",
		},
		SingleReplicaModePrometheusPvc: Resource[corev1.PersistentVolumeClaim]{
			ID: "PersistentVolumeClaim/" + fullName + "-prometheus",
		},
		ExtraConfigMap: Resource[corev1.ConfigMap]{
			ID: "ConfigMap/" + fullName + "-extra",
		},
		ExtraService: Resource[corev1.Service]{
			ID: "Service/" + fullName + "-extra",
		},
	}
}

type Test struct {
	ChartName   string
	ReleaseName string
	Namespace   string
	FullName    string
	Values      string
}

func DefaultTest() *Test {
	return &Test{
		ChartName:   "control-plane",
		ReleaseName: "control-plane",
		Namespace:   "control-plane",
		FullName:    "control-plane",
		Values:      "{}",
	}
}

func HelmRender(t *testing.T, test *Test) *Resources {
	t.Helper()

	helmChartPath, err := filepath.Abs("..")
	require.NoError(t, err)

	tmpFile, err := os.CreateTemp("", "values.*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(test.Values)); err != nil {
		tmpFile.Close()
		require.NoError(t, err)
	}
	err = tmpFile.Close()
	require.NoError(t, err)

	options := &helm.Options{
		ValuesFiles:    []string{tmpFile.Name()},
		KubectlOptions: k8s.NewKubectlOptions("", "", test.Namespace),
	}
	output := helm.RenderTemplate(t, options, helmChartPath, test.ReleaseName, nil)
	outputs := strings.Split(output, "---")

	resources := GenerateResources("control-plane")
	for _, o := range outputs {
		meta := K8sResource{}
		err := yaml.Unmarshal([]byte(o), &meta)
		require.NoError(t, err)

		id := meta.Kind + "/" + meta.Metadata.Name
		for _, r := range resources.Iter() {
			if id == r.ID {
				helm.UnmarshalK8SYaml(t, o, r.ValueP)
				*r.HasValueP = true
				break
			}
		}
	}

	require.True(t, resources.ConfigSecret.HasValue)
	confStr, ok := resources.ConfigSecret.Value.StringData["syn-cp.yaml"]
	require.True(t, ok)

	err = yaml.Unmarshal([]byte(confStr), &resources.Conf.Value)
	require.NoError(t, err)
	resources.Conf.HasValue = true

	return resources
}

func RenderAndCheck(t *testing.T, test *Test, expected *Resources) {
	t.Helper()
	actual := HelmRender(t, test)
	a := assert.New(t)

	if actual.ConfigSecret.Value.StringData != nil {
		conf, ok := actual.ConfigSecret.Value.StringData["syn-cp.yaml"]
		if ok {
			if expected.ConfigSecret.Value.StringData == nil {
				expected.ConfigSecret.Value.StringData = map[string]string{}
			}
			expected.ConfigSecret.Value.StringData["syn-cp.yaml"] = conf
		}
	}

	if actual.Deployment.Value.Spec.Template.Annotations != nil {
		configMapHash, ok := actual.Deployment.Value.Spec.Template.Annotations["checksum/config"]
		if ok {
			if expected.Deployment.Value.Spec.Template.Annotations == nil {
				expected.Deployment.Value.Spec.Template.Annotations = map[string]string{}
			}
			expected.Deployment.Value.Spec.Template.Annotations["checksum/config"] = configMapHash
		}
	}

	expectedResources := expected.Iter()
	actualResources := actual.Iter()
	require.Len(t, actualResources, len(expectedResources))

	for i := range expectedResources {
		expectedResource := expectedResources[i]
		actualResource := actualResources[i]
		if a.Equal(expectedResource.HasValueP, actualResource.HasValueP) && *actualResource.HasValueP {
			a.Equal(expectedResource.ValueP, actualResource.ValueP)
		}
	}
}
