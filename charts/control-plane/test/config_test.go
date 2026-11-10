package test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestConfigOptions(t *testing.T) {
	t.Parallel()
	test := DefaultTest()
	test.Values = `
config:
  mode: managed
  merge:
    license:
      enabled: true
  server:
    url: https://cp.nats.io
    httpPort: 8081
  kms:
    key:
      secretName: key
    rotatedKeys:
    - url: base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=
    - secretName: rotated-key
`
	expected := DefaultResources(t, test)
	expected.Conf.Value["mode"] = "managed"
	expected.Conf.Value["license"] = map[string]any{"enabled": true}
	expected.Conf.Value["server"] = map[string]any{
		"url":       "https://cp.nats.io",
		"http_addr": ":8081",
	}
	expected.Conf.Value["kms"] = map[string]any{
		"key_url": "file:///etc/syn-cp/kms/key.enc",
		"rotated_key_urls": []any{
			"base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=",
			"file:///etc/syn-cp/kms/rotated-key-1/key.enc",
		},
	}

	pts := &expected.StatefulSet.Value.Spec.Template.Spec
	pts.Volumes = append(pts.Volumes,
		corev1.Volume{
			Name: "kms-key",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "key",
				},
			},
		},
		corev1.Volume{
			Name: "kms-rotated-key-1",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "rotated-key",
				},
			},
		},
	)

	ctr := &pts.Containers[0]
	ctr.Ports[0].ContainerPort = 8081
	ctr.VolumeMounts = append(ctr.VolumeMounts,
		corev1.VolumeMount{
			Name:      "kms-key",
			MountPath: "/etc/syn-cp/kms",
		},
		corev1.VolumeMount{
			Name:      "kms-rotated-key-1",
			MountPath: "/etc/syn-cp/kms/rotated-key-1",
		},
	)

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
  kms:
    merge:
      key_url: base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=
  dataSources:
    prometheus:
      merge:
        url: https://localhost:9090
`,
		"patch": `
config:
  patch: [{op: add, path: /data_dir, value: /mnt/data}]
  server:
    patch: [{op: add, path: /url, value: cp.nats.io}]
  kms:
     patch: [{op: add, path: /key_url, value: base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=}]
  dataSources:
    prometheus:
      patch: [{op: add, path: /url, value: https://localhost:9090}]
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
			expected.Conf.Value["kms"] = map[string]any{
				"key_url": "base64key://smGbjm71Nxd1Ig5FS0wj9SlbzAIrnolCz9bQQ6uAhl4=",
			}
			expected.Conf.Value["data_sources"] = map[string]any{
				"prometheus": map[string]any{
					"url": "https://localhost:9090",
				},
			}

			ctr := &expected.StatefulSet.Value.Spec.Template.Spec.Containers[0]
			ctr.VolumeMounts[1].MountPath = "/mnt/data"

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
  dataSources:
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
	expected.Conf.Value["data_sources"] = map[string]any{
		"prometheus": map[string]any{
			"url": "https://localhost:9090",
			"tls": map[string]any{
				"ca_file": "/etc/syn-cp/certs/prometheus/tls.ca",
			},
		},
	}

	pts := &expected.StatefulSet.Value.Spec.Template.Spec
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
			Name: "prometheus-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "prometheus-tls",
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
			Name:      "prometheus-tls",
			MountPath: "/etc/syn-cp/certs/prometheus",
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
