package test

import (
	"testing"
)

func TestMTLS(t *testing.T) {
	test := DefaultTest()
	test.Values = `config:
  platformURL: https://my.control-plane.server
  natsURL: nats://my.nats.server
  token: agt_my_token
  tls:
    secretName: my-cert-secret
`

	expected := DefaultResources(t, test)
	expected.Deployment.Value.Spec.Template.Spec.Containers[0].Args = []string{
		"--token=agt_my_token",
		"--nats-url=nats://my.nats.server",
		"--platform-url=https://my.control-plane.server",
		"--tlscert=tls.crt",
		"--tlskey=tls.key",
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
    secretName: my-cert-secret
    insecure: true
`

	expected := DefaultResources(t, test)
	expected.Deployment.Value.Spec.Template.Spec.Containers[0].Args = []string{
		"--token=agt_my_other_token",
		"--nats-url=nats://my.other.nats.server",
		"--platform-url=https://my.control-plane.server",
		"--tlscert=tls.crt",
		"--tlskey=tls.key",
		"--insecure",
	}

	RenderAndCheck(t, test, expected)
}
