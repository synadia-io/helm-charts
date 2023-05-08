package test

import (
	"strings"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
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
imagePullSecret:
  enabled: false
`
	expected := DefaultResources(t, test)
	meta := []*v1.ObjectMeta{
		&expected.ConfigSecret.Value.ObjectMeta,
		&expected.Deployment.Value.ObjectMeta,
		&expected.Deployment.Value.Spec.Template.ObjectMeta,
		&expected.Service.Value.ObjectMeta,
		&expected.SingleReplicaModeDataPvc.Value.ObjectMeta,
		&expected.SingleReplicaModePostgresPvc.Value.ObjectMeta,
		&expected.SingleReplicaModePrometheusPvc.Value.ObjectMeta,
	}
	for _, m := range meta {
		m.Labels["global"] = "global"
	}

	pts := &expected.Deployment.Value.Spec.Template.Spec
	pts.ImagePullSecrets = nil

	ctr := &pts.Containers[0]
	imageSplit := strings.SplitN(ctr.Image, "/", 2)
	ctr.Image = "docker.io/" + imageSplit[1]
	ctr.ImagePullPolicy = corev1.PullAlways

	expected.ImagePullSecret.HasValue = false

	RenderAndCheck(t, test, expected)
}

func TestResourceOptions(t *testing.T) {
	t.Parallel()
	test := DefaultTest()
	test.Values = `
imagePullSecret:
  registry: docker.io
  username: a
  password: b
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
ingress:
  enabled: true
  hosts:
  - cp.nats.io
  tlsSecretName: cp-tls
podTemplate:
  configChecksumAnnotation: false
  topologySpreadConstraints:
    kubernetes.io/hostname:
      maxSkew: 1
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

	expected.ImagePullSecret.Value.StringData[corev1.DockerConfigJsonKey] = `{"auths":{"docker.io":{"auth":"YTpi","password":"b","username":"a"}}}
`

	expected.Ingress.HasValue = true
	expected.Ingress.Value.Spec.TLS = []networkingv1.IngressTLS{
		{
			Hosts:      []string{"cp.nats.io"},
			SecretName: "cp-tls",
		},
	}

	RenderAndCheck(t, test, expected)
}

func TestResourcesMergePatch(t *testing.T) {
	t.Parallel()
	values := map[string]string{
		"merge": `
config:
  systems:
    TestContents:
      url: nats://localhost:4222
      systemUserCreds:
        contents: creds
      operatorSigningKey:
        contents: nk
configSecret:
  merge:
    metadata:
      labels:
        test: test
contentsSecret:
  merge:
    metadata:
      labels:
        test: test
imagePullSecret:
  merge:
    metadata:
      labels:
        test: test
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
service:
  merge:
    metadata:
      labels:
        test: test
serviceAccount:
  enabled: true
  merge:
    metadata:
      labels:
        test: test
singleReplicaMode:
  dataPvc:
    merge:
      metadata:
        labels:
          test: test
  postgresPvc:
    merge:
      metadata:
        labels:
          test: test
  prometheusPvc:
    merge:
      metadata:
        labels:
          test: test
`,
		"patch": `
config:
  systems:
    TestContents:
      url: nats://localhost:4222
      systemUserCreds:
        contents: creds
      operatorSigningKey:
        contents: nk
configSecret:
  patch: [{op: add, path: /metadata/labels/test, value: test}]
contentsSecret:
  patch: [{op: add, path: /metadata/labels/test, value: test}]
imagePullSecret:
  patch: [{op: add, path: /metadata/labels/test, value: test}]
deployment:
  patch: [{op: add, path: /metadata/labels/test, value: test}]
podTemplate:
  patch: [{op: add, path: /metadata/labels/test, value: test}]
container:
  patch: [{op: add, path: /stdin, value: true}]
service:
  patch: [{op: add, path: /metadata/labels/test, value: test}]
serviceAccount:
  enabled: true
  patch: [{op: add, path: /metadata/labels/test, value: test}]
singleReplicaMode:
  dataPvc:
    patch: [{op: add, path: /metadata/labels/test, value: test}]
  postgresPvc:
    patch: [{op: add, path: /metadata/labels/test, value: test}]
  prometheusPvc:
    patch: [{op: add, path: /metadata/labels/test, value: test}]
`,
	}

	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			test := DefaultTest()
			test.Values = value

			expected := DefaultResources(t, test)
			meta := []*v1.ObjectMeta{
				&expected.ConfigSecret.Value.ObjectMeta,
				&expected.ContentsSecret.Value.ObjectMeta,
				&expected.Deployment.Value.ObjectMeta,
				&expected.Deployment.Value.Spec.Template.ObjectMeta,
				&expected.ImagePullSecret.Value.ObjectMeta,
				&expected.Service.Value.ObjectMeta,
				&expected.ServiceAccount.Value.ObjectMeta,
				&expected.SingleReplicaModeDataPvc.Value.ObjectMeta,
				&expected.SingleReplicaModePostgresPvc.Value.ObjectMeta,
				&expected.SingleReplicaModePrometheusPvc.Value.ObjectMeta,
			}
			for _, m := range meta {
				m.Labels["test"] = "test"
			}
			expected.Conf.Value["systems"] = map[string]any{
				"TestContents": map[string]any{
					"url":                       "nats://localhost:4222",
					"system_user_creds_file":    "/etc/syn-cp/contents/TestContents.sys-user.creds",
					"operator_signing_key_file": "/etc/syn-cp/contents/TestContents.operator-sk.nk",
				},
			}

			expected.ContentsSecret.HasValue = true
			expected.ContentsSecret.Value.StringData = map[string]string{
				"TestContents.sys-user.creds": "creds",
				"TestContents.operator-sk.nk": "nk",
			}

			pts := &expected.Deployment.Value.Spec.Template.Spec
			pts.ServiceAccountName = "control-plane"
			pts.Volumes = append(pts.Volumes,
				corev1.Volume{
					Name: "contents",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "control-plane-contents",
						},
					},
				},
			)

			ctr := &pts.Containers[0]
			ctr.Stdin = true
			ctr.VolumeMounts = append(ctr.VolumeMounts,
				corev1.VolumeMount{
					MountPath: "/etc/syn-cp/contents",
					Name:      "contents",
				},
			)

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
        {{ include "scp.fullname" $ }}-extra
    labels:
      $tplYaml: |
        {{ include "scp.labels" $ }}
  spec:
    selector:
      labels:
        $tplYamlSpread: |
          {{ include "scp.selectorLabels" $ | nindent 4 }}
    ports:
    - $tplYamlSpread: |
        - name: http
          port: 80
          targetPort: http
- $tplYaml: |
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: {{ include "scp.fullname" $ }}-extra
      labels:
        {{- include "scp.labels" $ | nindent 4 }}
    data:
      foo: bar
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
