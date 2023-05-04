package test

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"testing"
)

func TestDisableSingleReplicaMode(t *testing.T) {
	t.Parallel()
	test := DefaultTest()
	test.Values = `
config:
  kms:
    enabled: true
    key:
      url: base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=
  dataSources:
    postgres:
      enabled: true
      dsn: postgres://localhost:5432/localdb
    prometheus:
      enabled: true
      url: https://localhost:9090
deployment:
  replicas:
    2
singleReplicaMode:
  enabled: false
`
	expected := DefaultResources(t, test)
	expected.Conf.Value["kms"] = map[string]any{
		"key_url": "base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=",
	}
	expected.Conf.Value["data_sources"] = map[string]any{
		"postgres": map[string]any{
			"dsn": "postgres://localhost:5432/localdb",
		},
		"prometheus": map[string]any{
			"url": "https://localhost:9090",
		},
	}

	two := int32(2)
	expected.Deployment.Value.Spec.Strategy = appsv1.DeploymentStrategy{}
	expected.Deployment.Value.Spec.Replicas = &two

	pts := &expected.Deployment.Value.Spec.Template.Spec
	pts.Volumes = append(pts.Volumes[:2], pts.Volumes[4:]...)
	pts.Volumes[1].PersistentVolumeClaim = nil
	pts.Volumes[1].EmptyDir = &corev1.EmptyDirVolumeSource{}

	ctr := &pts.Containers[0]
	ctr.VolumeMounts = append(ctr.VolumeMounts[:2], ctr.VolumeMounts[4:]...)

	expected.SingleReplicaModeDataPvc.HasValue = false
	expected.SingleReplicaModePostgresPvc.HasValue = false
	expected.SingleReplicaModePrometheusPvc.HasValue = false

	RenderAndCheck(t, test, expected)
}

func TestConfigOptions(t *testing.T) {
	t.Parallel()
	test := DefaultTest()
	test.Values = `
config:
  server:
    url: https://cp.nats.io
    httpPort: 8081
  systems:
    TestContents:
      url: nats://localhost:4222
      systemUserCreds:
        contents: creds
      operatorSigningKey:
        contents: nk
    TestSecretName:
      url: nats://localhost:4222
      systemUserCreds:
        secretName: system-creds
      operatorSigningKey:
        secretName: system-sk
  kms:
    key:
      secretName: key
    rotatedKeys:
    - url: base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=
    - secretName: rotated-key
`
	expected := DefaultResources(t, test)
	expected.Conf.Value["server"] = map[string]any{
		"url":       "https://cp.nats.io",
		"http_addr": ":8081",
	}
	expected.Conf.Value["systems"] = map[string]any{
		"TestContents": map[string]any{
			"url":                       "nats://localhost:4222",
			"system_user_creds_file":    "/etc/syn-cp/contents/TestContents.sys-user.creds",
			"operator_signing_key_file": "/etc/syn-cp/contents/TestContents.operator-sk.nk",
		},
		"TestSecretName": map[string]any{
			"url":                       "nats://localhost:4222",
			"system_user_creds_file":    "/etc/syn-cp/systems/TestSecretName/sys-user-creds/sys-user.creds",
			"operator_signing_key_file": "/etc/syn-cp/systems/TestSecretName/operator-sk/operator-sk.nk",
		},
	}
	expected.Conf.Value["kms"] = map[string]any{
		"key_url": "file:///etc/syn-cp/kms/key.enc",
		"rotated_key_urls": []any{
			"base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=",
			"file:///etc/syn-cp/kms/rotated-key-1/key.enc",
		},
	}

	expected.ContentsSecret.HasValue = true
	expected.ContentsSecret.Value.StringData = map[string]string{
		"TestContents.sys-user.creds": "creds",
		"TestContents.operator-sk.nk": "nk",
	}

	pts := &expected.Deployment.Value.Spec.Template.Spec
	pts.Volumes = append(pts.Volumes,
		corev1.Volume{
			Name: "contents",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "control-plane-contents",
				},
			},
		},
		corev1.Volume{
			Name: "system-TestSecretName-operator-sk",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "system-sk",
				},
			},
		},
		corev1.Volume{
			Name: "system-TestSecretName-sys-user-creds",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "system-creds",
				},
			},
		},
	)

	ctr := &pts.Containers[0]
	ctr.VolumeMounts = append(ctr.VolumeMounts,
		corev1.VolumeMount{
			MountPath: "/etc/syn-cp/contents",
			Name:      "contents",
		},
		corev1.VolumeMount{
			MountPath: "/etc/syn-cp/systems/TestSecretName/operator-sk",
			Name:      "system-TestSecretName-operator-sk",
		},
		corev1.VolumeMount{
			MountPath: "/etc/syn-cp/systems/TestSecretName/sys-user-creds",
			Name:      "system-TestSecretName-sys-user-creds",
		},
	)

	ctr.Ports[0].ContainerPort = 8081

	RenderAndCheck(t, test, expected)
}

func TestConfigMergePatch(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"merge": `
config:
  merge:
    data_dir: /mnt/data
  server:
    merge:
      url: cp.nats.io
  systems:
    test:
      merge:
        url: nats://localhost:4222
  kms:
    merge:
      key_url: base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=
  dataSources:
    postgres:
      merge:
        dsn: postgres://localhost:5432/localdb
    prometheus:
      merge:
        url: https://localhost:9090
singleReplicaMode:
  enabled: false
`,
		"patch": `
config:
  patch: [{op: add, path: /data_dir, value: /mnt/data}]
  server:
    patch: [{op: add, path: /url, value: cp.nats.io}]
  systems:
    test:
      patch: [{op: add, path: /url, value: nats://localhost:4222}]
  kms:
     patch: [{op: add, path: /key_url, value: base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=}]
  dataSources:
    postgres:
      patch: [{op: add, path: /dsn, value: postgres://localhost:5432/localdb}]
    prometheus:
      patch: [{op: add, path: /url, value: https://localhost:9090}]
singleReplicaMode:
  enabled: false
`,
	}

	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			test := DefaultTest()
			test.Values = value

			expected := DefaultResources(t, test)
			expected.Conf.Value["data_dir"] = "/mnt/data"
			expected.Conf.Value["server"] = map[string]any{
				"url":       "cp.nats.io",
				"http_addr": ":8080",
			}
			expected.Conf.Value["systems"] = map[string]any{
				"test": map[string]any{
					"url": "nats://localhost:4222",
				},
			}
			expected.Conf.Value["kms"] = map[string]any{
				"key_url": "base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=",
			}
			expected.Conf.Value["data_sources"] = map[string]any{
				"postgres": map[string]any{
					"dsn": "postgres://localhost:5432/localdb",
				},
				"prometheus": map[string]any{
					"url": "https://localhost:9090",
				},
			}

			expected.Deployment.Value.Spec.Strategy = appsv1.DeploymentStrategy{}

			pts := &expected.Deployment.Value.Spec.Template.Spec
			pts.Volumes = append(pts.Volumes[:2], pts.Volumes[4:]...)
			pts.Volumes[1].PersistentVolumeClaim = nil
			pts.Volumes[1].EmptyDir = &corev1.EmptyDirVolumeSource{}

			ctr := &pts.Containers[0]
			ctr.VolumeMounts = append(ctr.VolumeMounts[:2], ctr.VolumeMounts[4:]...)
			ctr.VolumeMounts[1].MountPath = "/mnt/data"

			expected.SingleReplicaModeDataPvc.HasValue = false
			expected.SingleReplicaModePostgresPvc.HasValue = false
			expected.SingleReplicaModePrometheusPvc.HasValue = false

			RenderAndCheck(t, test, expected)
		})
	}
}

func TestConfigTls(t *testing.T) {
	t.Parallel()
	test := DefaultTest()
	test.Values = `
config:
  server:
    tls:
      enabled: true
      secretName: server-tls
  systems:
    test:
      url: nats://localhost:4222
      tls:
        enabled: true
        secretName: system-tls
  dataSources:
    postgres:
      dsn: postgres://localhost:5432/localdb
      tls:
        enabled: true
        secretName: postgres-tls
    prometheus:
      url: https://localhost:9090
      tls:
        enabled: true
        secretName: prometheus-tls
`

	expected := DefaultResources(t, test)
	expected.Conf.Value["server"] = map[string]any{
		"http_addr":  ":8080",
		"https_addr": ":8443",
		"tls": map[string]any{
			"cert_file": "/etc/syn-cp/certs/server/tls.crt",
			"key_file":  "/etc/syn-cp/certs/server/tls.key",
		},
	}
	expected.Conf.Value["systems"] = map[string]any{
		"test": map[string]any{
			"url": "nats://localhost:4222",
			"tls": map[string]any{
				"ca_file": "/etc/syn-cp/systems/test/cert/tls.ca",
			},
		},
	}
	expected.Conf.Value["data_sources"] = map[string]any{
		"postgres": map[string]any{
			"dsn": "postgres://localhost:5432/localdb?sslmode=verify-full&sslrootcert=/etc/syn-cp/certs/postgres/tls.ca",
		},
		"prometheus": map[string]any{
			"url": "https://localhost:9090",
			"tls": map[string]any{
				"ca_file": "/etc/syn-cp/certs/prometheus/tls.ca",
			},
		},
	}

	pts := &expected.Deployment.Value.Spec.Template.Spec
	pts.Volumes = append(pts.Volumes,
		corev1.Volume{
			Name: "server-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "server-tls",
				},
			},
		},
		corev1.Volume{
			Name: "postgres-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "postgres-tls",
				},
			},
		},
		corev1.Volume{
			Name: "prometheus-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "prometheus-tls",
				},
			},
		},
		corev1.Volume{
			Name: "system-test-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "system-tls",
				},
			},
		},
	)

	ctr := &pts.Containers[0]
	ctr.VolumeMounts = append(ctr.VolumeMounts,
		corev1.VolumeMount{
			Name:      "server-tls",
			MountPath: "/etc/syn-cp/certs/server",
		},
		corev1.VolumeMount{
			Name:      "postgres-tls",
			MountPath: "/etc/syn-cp/certs/postgres",
		},
		corev1.VolumeMount{
			Name:      "prometheus-tls",
			MountPath: "/etc/syn-cp/certs/prometheus",
		},
		corev1.VolumeMount{
			Name:      "system-test-tls",
			MountPath: "/etc/syn-cp/systems/test/cert",
		},
	)
	ctr.Ports = append(ctr.Ports, corev1.ContainerPort{
		ContainerPort: 8443,
		Name:          "https",
	})

	expected.Service.Value.Spec.Ports = append(expected.Service.Value.Spec.Ports, corev1.ServicePort{
		Name:       "https",
		Port:       443,
		TargetPort: intstr.FromString("https"),
	})

	RenderAndCheck(t, test, expected)
}
