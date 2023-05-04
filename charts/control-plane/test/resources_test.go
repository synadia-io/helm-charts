package test

import (
	networkingv1 "k8s.io/api/networking/v1"
	"strings"
	"testing"

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

//func TestResourcesMergePatch(t *testing.T) {
//	t.Parallel()
//	test := DefaultTest()
//	test.Values = `
//config:
//  websocket:
//    enabled: true
//    ingress:
//      enabled: true
//      hosts:
//      - demo.nats.io
//      merge:
//        metadata:
//          annotations:
//            test: test
//      patch: [{op: add, path: /metadata/labels/test, value: "test"}]
//container:
//  merge:
//    stdin: true
//  patch: [{op: add, path: /tty, value: true}]
//reloader:
//  merge:
//    stdin: true
//  patch: [{op: add, path: /tty, value: true}]
//promExporter:
//  enabled: true
//  merge:
//    stdin: true
//  patch: [{op: add, path: /tty, value: true}]
//  podMonitor:
//    enabled: true
//    merge:
//      metadata:
//        annotations:
//          test: test
//    patch: [{op: add, path: /metadata/labels/test, value: "test"}]
//service:
//  enabled: true
//  merge:
//    metadata:
//      annotations:
//        test: test
//  patch: [{op: add, path: /metadata/labels/test, value: "test"}]
//statefulSet:
//  merge:
//    metadata:
//      annotations:
//        test: test
//  patch: [{op: add, path: /metadata/labels/test, value: "test"}]
//podTemplate:
//  merge:
//    metadata:
//      annotations:
//        test: test
//  patch: [{op: add, path: /metadata/labels/test, value: "test"}]
//headlessService:
//  merge:
//    metadata:
//      annotations:
//        test: test
//  patch: [{op: add, path: /metadata/labels/test, value: "test"}]
//configMap:
//  merge:
//    metadata:
//      annotations:
//        test: test
//  patch: [{op: add, path: /metadata/labels/test, value: "test"}]
//serviceAccount:
//  enabled: true
//  merge:
//    metadata:
//      annotations:
//        test: test
//  patch: [{op: add, path: /metadata/labels/test, value: "test"}]
//natsBox:
//  contexts:
//    default:
//      merge:
//        user: foo
//      patch: [{op: add, path: /password, value: "bar"}]
//  container:
//    merge:
//      stdin: true
//    patch: [{op: add, path: /tty, value: true}]
//  podTemplate:
//    merge:
//      metadata:
//        annotations:
//          test: test
//    patch: [{op: add, path: /metadata/labels/test, value: "test"}]
//  deployment:
//    merge:
//      metadata:
//        annotations:
//          test: test
//    patch: [{op: add, path: /metadata/labels/test, value: "test"}]
//  contextsSecret:
//    merge:
//      metadata:
//        annotations:
//          test: test
//    patch: [{op: add, path: /metadata/labels/test, value: "test"}]
//  contentsSecret:
//    merge:
//      metadata:
//        annotations:
//          test: test
//    patch: [{op: add, path: /metadata/labels/test, value: "test"}]
//  serviceAccount:
//    enabled: true
//    merge:
//      metadata:
//        annotations:
//          test: test
//    patch: [{op: add, path: /metadata/labels/test, value: "test"}]
//`
//	expected := DefaultResources(t, test)
//
//	expected.Conf.Value["websocket"] = map[string]any{
//		"port":        int64(8080),
//		"no_tls":      true,
//		"compression": true,
//	}
//
//	annotations := func() map[string]string {
//		return map[string]string{
//			"test": "test",
//		}
//	}
//
//	dd := ddg.Get(t)
//	ctr := expected.StatefulSet.Value.Spec.Template.Spec.Containers
//	ctr[0].Stdin = true
//	ctr[0].TTY = true
//	ctr[1].Stdin = true
//	ctr[1].TTY = true
//	ctr = append(ctr, corev1.Container{
//		Args: []string{
//			"-port=7777",
//			"-connz",
//			"-routez",
//			"-subz",
//			"-varz",
//			"-prefix=nats",
//			"-use_internal_server_id",
//			"http://localhost:8222/",
//		},
//		Image: dd.PromExporterImage,
//		Name:  "prom-exporter",
//		Ports: []corev1.ContainerPort{
//			{
//				Name:          "prom-metrics",
//				ContainerPort: 7777,
//			},
//		},
//		Stdin: true,
//		TTY:   true,
//	})
//	expected.StatefulSet.Value.Spec.Template.Spec.Containers = ctr
//	expected.StatefulSet.Value.Spec.Template.Spec.ServiceAccountName = test.FullName
//
//	expected.NatsBoxDeployment.Value.Spec.Template.Spec.Containers[0].Stdin = true
//	expected.NatsBoxDeployment.Value.Spec.Template.Spec.Containers[0].TTY = true
//
//	expected.StatefulSet.Value.ObjectMeta.Annotations = annotations()
//	expected.StatefulSet.Value.ObjectMeta.Labels["test"] = "test"
//
//	expected.StatefulSet.Value.Spec.Template.ObjectMeta.Annotations = annotations()
//	expected.StatefulSet.Value.Spec.Template.ObjectMeta.Labels["test"] = "test"
//
//	expected.NatsBoxDeployment.Value.ObjectMeta.Annotations = annotations()
//	expected.NatsBoxDeployment.Value.ObjectMeta.Labels["test"] = "test"
//
//	expected.NatsBoxDeployment.Value.Spec.Template.ObjectMeta.Annotations = annotations()
//	expected.NatsBoxDeployment.Value.Spec.Template.ObjectMeta.Labels["test"] = "test"
//	expected.NatsBoxDeployment.Value.Spec.Template.Spec.ServiceAccountName = test.FullName + "-box"
//
//	expected.PodMonitor.HasValue = true
//	expected.PodMonitor.Value.ObjectMeta.Annotations = annotations()
//	expected.PodMonitor.Value.ObjectMeta.Labels["test"] = "test"
//
//	expected.Ingress.HasValue = true
//	expected.Ingress.Value.ObjectMeta.Annotations = annotations()
//	expected.Ingress.Value.ObjectMeta.Labels["test"] = "test"
//
//	expected.NatsBoxContextsSecret.Value.ObjectMeta.Annotations = annotations()
//	expected.NatsBoxContextsSecret.Value.ObjectMeta.Labels["test"] = "test"
//	expected.NatsBoxContextsSecret.Value.StringData["default.json"] = `{
//  "password": "bar",
//  "url": "nats://` + test.FullName + `",
//  "user": "foo"
//}
//`
//
//	expected.NatsBoxContentsSecret.Value.ObjectMeta.Annotations = annotations()
//	expected.NatsBoxContentsSecret.Value.ObjectMeta.Labels["test"] = "test"
//
//	expected.NatsBoxServiceAccount.HasValue = true
//	expected.NatsBoxServiceAccount.Value.ObjectMeta.Annotations = annotations()
//	expected.NatsBoxServiceAccount.Value.ObjectMeta.Labels["test"] = "test"
//
//	expected.Service.Value.ObjectMeta.Annotations = annotations()
//	expected.Service.Value.ObjectMeta.Labels["test"] = "test"
//
//	expected.ServiceAccount.HasValue = true
//	expected.ServiceAccount.Value.ObjectMeta.Annotations = annotations()
//	expected.ServiceAccount.Value.ObjectMeta.Labels["test"] = "test"
//
//	expected.HeadlessService.Value.ObjectMeta.Annotations = annotations()
//	expected.HeadlessService.Value.ObjectMeta.Labels["test"] = "test"
//
//	expected.ConfigMap.Value.ObjectMeta.Annotations = annotations()
//	expected.ConfigMap.Value.ObjectMeta.Labels["test"] = "test"
//
//	expected.StatefulSet.Value.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
//		{
//			Name:          "nats",
//			ContainerPort: 4222,
//		},
//		{
//			Name:          "websocket",
//			ContainerPort: 8080,
//		},
//		{
//			Name:          "monitor",
//			ContainerPort: 8222,
//		},
//	}
//
//	expected.HeadlessService.Value.Spec.Ports = []corev1.ServicePort{
//		{
//			Name:       "nats",
//			Port:       4222,
//			TargetPort: intstr.FromString("nats"),
//		},
//		{
//			Name:       "websocket",
//			Port:       8080,
//			TargetPort: intstr.FromString("websocket"),
//		},
//		{
//			Name:       "monitor",
//			Port:       8222,
//			TargetPort: intstr.FromString("monitor"),
//		},
//	}
//
//	expected.Service.Value.Spec.Ports = []corev1.ServicePort{
//		{
//			Name:       "nats",
//			Port:       4222,
//			TargetPort: intstr.FromString("nats"),
//		},
//		{
//			Name:       "websocket",
//			Port:       8080,
//			TargetPort: intstr.FromString("websocket"),
//		},
//	}
//
//	RenderAndCheck(t, test, expected)
//}
