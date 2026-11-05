//go:build e2e

// End-to-end tests against the cluster in the current kube context.
//
//	go test -tags e2e -timeout 20m
//
// E2E_VALUES optionally names a values file, for example to set the image registry and pull secret.
package test

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
)

const e2eRelease = "cp"

func e2eSetup(t *testing.T) (*k8s.KubectlOptions, *helm.Options, string) {
	t.Helper()
	ns := "cp-e2e-" + strings.ToLower(random.UniqueId())
	kubectl := k8s.NewKubectlOptions("", "", ns)
	k8s.CreateNamespace(t, kubectl, ns)
	t.Cleanup(func() { k8s.DeleteNamespace(t, kubectl, ns) })

	opts := &helm.Options{KubectlOptions: kubectl, SetValues: map[string]string{}}
	if f := os.Getenv("E2E_VALUES"); f != "" {
		opts.ValuesFiles = []string{f}
	}

	chart, err := filepath.Abs("..")
	require.NoError(t, err)
	return kubectl, opts, chart
}

// Installs with a KMS key from a Secret, then restarts the pod and checks that state on the PVC survives.
func TestE2EInstall(t *testing.T) {
	t.Parallel()
	kubectl, opts, chart := e2eSetup(t)

	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	k8s.RunKubectl(t, kubectl, "create", "secret", "generic", "cp-kms",
		"--from-literal=key.enc=base64key://"+base64.URLEncoding.EncodeToString(key))

	opts.SetValues["config.kms.key.secretName"] = "cp-kms"
	opts.ExtraArgs = map[string][]string{"install": {"--wait", "--timeout", "5m"}}
	helm.Install(t, opts, chart, e2eRelease)

	pod := e2eRelease + "-control-plane-0"
	k8s.RunKubectl(t, kubectl, "delete", "pod", pod)
	k8s.WaitUntilPodAvailable(t, kubectl, pod, 60, 5*time.Second)

	logs, err := k8s.RunKubectlAndGetOutputE(t, kubectl, "logs", pod)
	require.NoError(t, err)
	require.Contains(t, logs, "loaded existing bootstrap config from disk store")
}

// Upgrading a 1.x release must fail before it deletes the 1.x PVCs.
func TestE2EUpgradeGuard(t *testing.T) {
	t.Parallel()
	kubectl, opts, chart := e2eSetup(t)

	// the 1.x pod does not need to start, the guard only looks for the 1.x Deployment
	_, err := helm.RunHelmCommandAndGetOutputE(t, &helm.Options{KubectlOptions: kubectl}, "install", e2eRelease, "control-plane",
		"--repo", "https://synadia-io.github.io/helm-charts", "--version", "1.14.1")
	require.NoError(t, err)

	err = helm.UpgradeE(t, opts, chart, e2eRelease)
	require.ErrorContains(t, err, "control-plane chart 1.x")
	k8s.RunKubectl(t, kubectl, "get", "pvc", e2eRelease+"-control-plane-encryption")
}
