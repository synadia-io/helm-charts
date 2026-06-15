package test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/stretchr/testify/require"
)

// releaseName is fixed so resource names are deterministic across tests:
// the chart's fullname becomes "<release>-<chart>" = "ins-insights".
const (
	releaseName = "ins"
	namespace   = "insights"
	fullName    = "ins-insights"
)

func chartDir(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("..")
	require.NoError(t, err)
	return p
}

// render templates the chart with the given --set values and returns the
// multi-document YAML output. It fails the test if rendering errors.
func render(t *testing.T, values map[string]string) string {
	t.Helper()
	opts := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", namespace),
	}
	return helm.RenderTemplate(t, opts, chartDir(t), releaseName, nil)
}

// renderErr templates the chart and returns the error (used to assert that
// invalid configurations fail fast in the chart's required-values checks).
func renderErr(t *testing.T, values map[string]string) error {
	t.Helper()
	opts := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", namespace),
	}
	_, err := helm.RenderTemplateE(t, opts, chartDir(t), releaseName, nil)
	return err
}

type docMeta struct {
	Kind     string `json:"kind"`
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
}

// find locates a single rendered resource by kind and name and unmarshals it
// into T. The bool reports whether the resource was present in the output.
func find[T any](t *testing.T, out, kind, name string) (T, bool) {
	t.Helper()
	var result T
	for _, doc := range strings.Split(out, "\n---\n") {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		var meta docMeta
		if err := yaml.Unmarshal([]byte(doc), &meta); err != nil {
			continue
		}
		if meta.Kind == kind && meta.Metadata.Name == name {
			helm.UnmarshalK8SYaml(t, doc, &result)
			return result, true
		}
	}
	return result, false
}

// minimalValues is the smallest config that renders: production edition only
// requires the system to monitor.
func minimalValues() map[string]string {
	return map[string]string{
		"config.sys.server": "tls://connect.ngs.global",
	}
}
