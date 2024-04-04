package test

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGlobalOptions(t *testing.T) {
	t.Parallel()
	test := DefaultTest()
	test.Values = `
global:
  image:
    pullPolicy: Always
    registry: docker.io
  labels:
    global: global

# These are required options
config:
  token: agt_my_token
  natsURL: nats://connect.ngs.global
`
	expected := DefaultResources(t, test)
	meta := []*v1.ObjectMeta{
		&expected.Deployment.Value.ObjectMeta,
		&expected.Deployment.Value.Spec.Template.ObjectMeta,
	}
	for _, m := range meta {
		m.Labels["global"] = "global"
	}

	pts := &expected.Deployment.Value.Spec.Template.Spec

	ctr := &pts.Containers[0]
	imageSplit := strings.SplitN(ctr.Image, "/", 2)
	ctr.Image = "docker.io/" + imageSplit[1]
	ctr.ImagePullPolicy = corev1.PullAlways

	RenderAndCheck(t, test, expected)
}

func TestResourceOptions(t *testing.T) {
	t.Parallel()
	test := DefaultTest()
	test.Values = `
global:
  image:
    registry: docker.io
container:
  image:
    pullPolicy: Always
  env:
    GOMEMLIMIT: 1GiB
    TOKEN:
      valueFrom:
        secretKeyRef:
          name: token
          key: token
podTemplate:
  configChecksumAnnotation: false
  topologySpreadConstraints:
    kubernetes.io/hostname:
      maxSkew: 1

# These are required options
config:
  token: agt_my_token
  natsURL: nats://connect.ngs.global
`

	expected := DefaultResources(t, test)

	pts := &expected.Deployment.Value.Spec.Template.Spec
	pts.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{
		{
			MaxSkew:        1,
			TopologyKey:    "kubernetes.io/hostname",
			LabelSelector:  expected.Deployment.Value.Spec.Selector,
			MatchLabelKeys: []string{"pod-template-hash"},
		},
	}

	ctr := &pts.Containers[0]
	imageSplit := strings.SplitN(ctr.Image, "/", 2)
	ctr.Image = "docker.io/" + imageSplit[1]
	ctr.ImagePullPolicy = corev1.PullAlways
	ctr.Env = []corev1.EnvVar{
		{
			Name:  "GOMEMLIMIT",
			Value: "1GiB",
		},
		{
			Name: "TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "token",
					},
					Key: "token",
				},
			},
		},
	}

	RenderAndCheck(t, test, expected)
}

func TestResourcesMergePatch(t *testing.T) {
	t.Parallel()
	values := map[string]string{
		"merge": `
deployment:
  merge:
    metadata:
      labels:
        test: test
podTemplate:
  merge:
    metadata:
      labels:
        test: test
container:
  merge:
    stdin: true
serviceAccount:
  enabled: true
  merge:
    metadata:
      labels:
        test: test
# These are required options
config:
  token: agt_my_token
  natsURL: nats://connect.ngs.global
`,
		"patch": `
deployment:
  patch: [{op: add, path: /metadata/labels/test, value: test}]
podTemplate:
  patch: [{op: add, path: /metadata/labels/test, value: test}]
container:
  patch: [{op: add, path: /stdin, value: true}]
podDisruptionBudget:
  merge:
    metadata:
      annotations:
        test: test
  patch: [{op: add, path: /metadata/labels/test, value: "test"}]
serviceAccount:
  enabled: true
  patch: [{op: add, path: /metadata/labels/test, value: test}]
# These are required options
config:
  token: agt_my_token
  natsURL: nats://connect.ngs.global
`,
	}

	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			test := DefaultTest()
			test.Values = value

			expected := DefaultResources(t, test)
			meta := []*v1.ObjectMeta{
				&expected.Deployment.Value.ObjectMeta,
				&expected.Deployment.Value.Spec.Template.ObjectMeta,
				&expected.ServiceAccount.Value.ObjectMeta,
			}
			for _, m := range meta {
				m.Labels["test"] = "test"
			}

			pts := &expected.Deployment.Value.Spec.Template.Spec
			pts.ServiceAccountName = "private-link"

			ctr := &pts.Containers[0]
			ctr.Stdin = true

			expected.ServiceAccount.HasValue = true

			RenderAndCheck(t, test, expected)
		})
	}
}

func TestExtraResources(t *testing.T) {
	t.Parallel()
	test := DefaultTest()
	test.Values = `
extraResources:
- apiVersion: v1
  kind: Service
  metadata:
    name:
      $tplYaml: >
        {{ include "spl.fullname" $ }}-extra
    labels:
      $tplYaml: |
        {{ include "spl.labels" $ }}
  spec:
    selector:
      labels:
        $tplYamlSpread: |
          {{ include "spl.selectorLabels" $ | nindent 4 }}
    ports:
    - $tplYamlSpread: |
        - name: http
          port: 80
          targetPort: http
- $tplYaml: |
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: {{ include "spl.fullname" $ }}-extra
      labels:
        {{- include "spl.labels" $ | nindent 4 }}
    data:
      foo: bar

# These are required options
config:
  token: agt_my_token
  natsURL: nats://connect.ngs.global
`

	expected := DefaultResources(t, test)

	expected.ExtraConfigMap.HasValue = true
	expected.ExtraConfigMap.Value.Data = map[string]string{
		"foo": "bar",
	}

	expected.ExtraService.HasValue = true
	expected.ExtraService.Value.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			TargetPort: intstr.FromString("http"),
		},
	}

	RenderAndCheck(t, test, expected)
}
