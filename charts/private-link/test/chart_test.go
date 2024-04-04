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
	policyv1 "k8s.io/api/policy/v1"
)

type Resources struct {
	Deployment          Resource[appsv1.Deployment]
	PodDisruptionBudget Resource[policyv1.PodDisruptionBudget]
	ServiceAccount      Resource[corev1.ServiceAccount]
	ExtraConfigMap      Resource[corev1.ConfigMap]
	ExtraService        Resource[corev1.Service]
}

func (r *Resources) Iter() []MutableResource {
	return []MutableResource{
		r.Deployment.Mutable(),
		r.ServiceAccount.Mutable(),
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
		Deployment: Resource[appsv1.Deployment]{
			ID: "Deployment/" + fullName,
		},
		PodDisruptionBudget: Resource[policyv1.PodDisruptionBudget]{
			ID: "PodDisruptionBudget/" + fullName,
		},
		ServiceAccount: Resource[corev1.ServiceAccount]{
			ID: "ServiceAccount/" + fullName,
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
		ChartName:   "private-link",
		ReleaseName: "private-link",
		Namespace:   "private-link",
		FullName:    "private-link",
		Values: `config:
  token: agt_my_token
  natsURL: nats://connect.ngs.global
`,
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

	resources := GenerateResources("private-link")
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

	return resources
}

func RenderAndCheck(t *testing.T, test *Test, expected *Resources) {
	t.Helper()
	actual := HelmRender(t, test)
	a := assert.New(t)

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
