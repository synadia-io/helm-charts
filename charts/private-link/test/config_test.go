package test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestMTLS(t *testing.T) {
	test := DefaultTest()
	test.Values = `config:
  platformURL: https://my.control-plane.server
  natsURL: nats://my.nats.server
  token: agt_my_token
  tls:
    clientCert:
      enabled: true
      secretName: my-cert-secret
`

	expected := DefaultResources(t, test)

	expected.Deployment.Value.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{
			Name: "SPL_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "private-link-token",
					},
					Key: "token",
				},
			},
		},
	}

	expected.Deployment.Value.Spec.Template.Spec.Containers[0].Args = []string{
		"--nats-url=nats://my.nats.server",
		"--platform-url=https://my.control-plane.server",
		"--tlscert=/etc/private-link/certs/tls.crt",
		"--tlskey=/etc/private-link/certs/tls.key",
	}

	expected.Deployment.Value.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "tls-client",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "my-cert-secret",
				},
			},
		},
	}

	expected.Deployment.Value.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: "/etc/private-link/certs",
			Name:      "tls-client",
		},
	}

	RenderAndCheck(t, test, expected)
}

func TestMTLSWithCA(t *testing.T) {
	test := DefaultTest()
	test.Values = `config:
  platformURL: https://my.control-plane.server
  natsURL: nats://my.nats.server
  token: agt_my_token
  tls:
    clientCert:
      enabled: true
      secretName: my-cert-secret
    caCerts:
      enabled: true
      secretName: my-ca-secret
`

	expected := DefaultResources(t, test)

	expected.Deployment.Value.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{
			Name: "SPL_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "private-link-token",
					},
					Key: "token",
				},
			},
		},
	}

	expected.Deployment.Value.Spec.Template.Spec.Containers[0].Args = []string{
		"--nats-url=nats://my.nats.server",
		"--platform-url=https://my.control-plane.server",
		"--tlscert=/etc/private-link/certs/tls.crt",
		"--tlskey=/etc/private-link/certs/tls.key",
		"--tlsca=/etc/private-link/ca-cert/ca.crt",
	}

	expected.Deployment.Value.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "tls-ca",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "my-ca-secret",
				},
			},
		},
		{
			Name: "tls-client",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "my-cert-secret",
				},
			},
		},
	}

	expected.Deployment.Value.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: "/etc/private-link/ca-cert",
			Name:      "tls-ca",
		},
		{
			MountPath: "/etc/private-link/certs",
			Name:      "tls-client",
		},
	}

	RenderAndCheck(t, test, expected)

	test.Values = `config:
  platformURL: https://my.control-plane.server
  natsURL: nats://my.nats.server
  token: agt_my_token
  tls:
    clientCert:
      enabled: true
      secretName: my-cert-secret
    caCerts:
      enabled: true
      configMapName: my-ca-configMap
`

	expected.Deployment.Value.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "tls-ca",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "my-ca-configMap",
					},
				},
			},
		},
		{
			Name: "tls-client",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "my-cert-secret",
				},
			},
		},
	}

	expected.Deployment.Value.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: "/etc/private-link/ca-cert",
			Name:      "tls-ca",
		},
		{
			MountPath: "/etc/private-link/certs",
			Name:      "tls-client",
		},
	}

	RenderAndCheck(t, test, expected)
}

func TestMTLSInsecure(t *testing.T) {
	test := DefaultTest()
	test.Values = `config:
  platformURL: https://my.control-plane.server
  natsURL: nats://my.other.nats.server
  token: agt_my_other_token
  tls:
    clientCert:
      enabled: true
      secretName: my-cert-secret
    insecureSkipVerify: true
`

	expected := DefaultResources(t, test)

	expected.Deployment.Value.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{
			Name: "SPL_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "private-link-token",
					},
					Key: "token",
				},
			},
		},
	}

	expected.Deployment.Value.Spec.Template.Spec.Containers[0].Args = []string{
		"--nats-url=nats://my.other.nats.server",
		"--platform-url=https://my.control-plane.server",
		"--tlscert=/etc/private-link/certs/tls.crt",
		"--tlskey=/etc/private-link/certs/tls.key",
		"--insecure",
	}

	expected.Deployment.Value.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "tls-client",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "my-cert-secret",
				},
			},
		},
	}

	expected.Deployment.Value.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: "/etc/private-link/certs",
			Name:      "tls-client",
		},
	}

	RenderAndCheck(t, test, expected)
}
