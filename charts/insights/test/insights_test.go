package test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
)

func statefulSet(t *testing.T, out string) appsv1.StatefulSet {
	t.Helper()
	sts, ok := find[appsv1.StatefulSet](t, out, "StatefulSet", fullName)
	require.True(t, ok, "StatefulSet %q not rendered", fullName)
	return sts
}

func mainContainer(t *testing.T, sts appsv1.StatefulSet) corev1.Container {
	t.Helper()
	require.Len(t, sts.Spec.Template.Spec.Containers, 1)
	return sts.Spec.Template.Spec.Containers[0]
}

func envMap(c corev1.Container) map[string]string {
	m := map[string]string{}
	for _, e := range c.Env {
		m[e.Name] = e.Value // valueFrom entries carry an empty Value
	}
	return m
}

func hasVolume(spec corev1.PodSpec, name string) bool {
	for _, v := range spec.Volumes {
		if v.Name == name {
			return true
		}
	}
	return false
}

func TestDefaults(t *testing.T) {
	t.Parallel()
	out := render(t, minimalValues())

	sts := statefulSet(t, out)
	require.NotNil(t, sts.Spec.Replicas)
	assert.EqualValues(t, 1, *sts.Spec.Replicas)
	assert.Equal(t, fullName+"-headless", sts.Spec.ServiceName)

	c := mainContainer(t, sts)
	assert.True(t, strings.HasPrefix(c.Image, "registry.synadia.io/insights:"),
		"image should be the production repository on the synadia registry, got %q", c.Image)

	env := envMap(c)
	assert.Equal(t, "/data", env["INSIGHTS_DATA_DIR"])
	assert.Equal(t, "0.0.0.0", env["INSIGHTS_WEB_HOSTNAME"])
	assert.Equal(t, "tls://connect.ngs.global", env["INSIGHTS_SYS_SERVER"])
	assert.Equal(t, "false", env["INSIGHTS_PROMETHEUS_ENABLED"])
	// production edition pulls no license
	assert.NotContains(t, env, "INSIGHTS_LICENSE_FILE")
	assert.NotContains(t, env, "INSIGHTS_SYS_CREDS")

	// probes default to a TCP check on the web port
	require.NotNil(t, c.ReadinessProbe)
	require.NotNil(t, c.ReadinessProbe.TCPSocket)
	assert.Equal(t, "http", c.ReadinessProbe.TCPSocket.Port.String())
	require.NotNil(t, c.LivenessProbe)
	require.NotNil(t, c.StartupProbe)

	// data is backed by a volumeClaimTemplate mounted at /data
	require.Len(t, sts.Spec.VolumeClaimTemplates, 1)
	assert.Equal(t, "data", sts.Spec.VolumeClaimTemplates[0].Name)
	var dataMount *corev1.VolumeMount
	for i := range c.VolumeMounts {
		if c.VolumeMounts[i].Name == "data" {
			dataMount = &c.VolumeMounts[i]
		}
	}
	require.NotNil(t, dataMount, "data volume mount missing")
	assert.Equal(t, "/data", dataMount.MountPath)

	// fsGroup lets the non-root process write the volume
	require.NotNil(t, sts.Spec.Template.Spec.SecurityContext)
	require.NotNil(t, sts.Spec.Template.Spec.SecurityContext.FSGroup)

	// imagePullSecret is enabled by default and referenced by the pod
	regcred, ok := find[corev1.Secret](t, out, "Secret", fullName+"-regcred")
	require.True(t, ok, "image pull secret not rendered")
	assert.Equal(t, corev1.SecretTypeDockerConfigJson, regcred.Type)
	require.Len(t, sts.Spec.Template.Spec.ImagePullSecrets, 1)
	assert.Equal(t, fullName+"-regcred", sts.Spec.Template.Spec.ImagePullSecrets[0].Name)

	// services
	svc, ok := find[corev1.Service](t, out, "Service", fullName)
	require.True(t, ok, "service not rendered")
	require.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, "http", svc.Spec.Ports[0].Name)
	headless, ok := find[corev1.Service](t, out, "Service", fullName+"-headless")
	require.True(t, ok, "headless service not rendered")
	assert.Equal(t, "None", headless.Spec.ClusterIP)

	// nothing optional leaks in
	_, ok = find[corev1.Secret](t, out, "Secret", fullName+"-config")
	assert.False(t, ok, "config secret should not exist without inline secrets")
	_, ok = find[policyv1.PodDisruptionBudget](t, out, "PodDisruptionBudget", fullName)
	assert.False(t, ok, "PDB should be disabled by default")
	_, ok = find[networkingv1.Ingress](t, out, "Ingress", fullName)
	assert.False(t, ok, "ingress should be disabled by default")
}

func TestTrialEdition(t *testing.T) {
	t.Parallel()
	values := minimalValues()
	values["edition"] = "trial"
	values["config.license.token"] = "eyJhbGciOiJlZDI1NTE5In0.body.sig"
	out := render(t, values)

	c := mainContainer(t, statefulSet(t, out))
	assert.True(t, strings.HasPrefix(c.Image, "registry.synadia.io/insights-licensed:"),
		"trial edition should use the insights-licensed repository, got %q", c.Image)
	assert.Equal(t, "/etc/insights/license/license.jwt", envMap(c)["INSIGHTS_LICENSE_FILE"])

	secret, ok := find[corev1.Secret](t, out, "Secret", fullName+"-config")
	require.True(t, ok, "config secret should hold the inline license")
	assert.Contains(t, secret.StringData, "license.jwt")
	assert.True(t, hasVolume(statefulSet(t, out).Spec.Template.Spec, "license"))
}

func TestTrialEditionRequiresLicense(t *testing.T) {
	t.Parallel()
	values := minimalValues()
	values["edition"] = "trial"
	err := renderErr(t, values)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "license")
}

func TestSysCredsInline(t *testing.T) {
	t.Parallel()
	values := minimalValues()
	values["config.sys.creds"] = "-----BEGIN NATS USER JWT-----"
	out := render(t, values)

	c := mainContainer(t, statefulSet(t, out))
	assert.Equal(t, "/etc/insights/creds/sys.creds", envMap(c)["INSIGHTS_SYS_CREDS"])

	secret, ok := find[corev1.Secret](t, out, "Secret", fullName+"-config")
	require.True(t, ok, "config secret should hold the inline creds")
	assert.Contains(t, secret.StringData, "sys.creds")
	assert.True(t, hasVolume(statefulSet(t, out).Spec.Template.Spec, "sys-creds"))
}

func TestSysCredsExistingSecret(t *testing.T) {
	t.Parallel()
	values := minimalValues()
	values["config.sys.credsSecretName"] = "my-creds"
	out := render(t, values)

	c := mainContainer(t, statefulSet(t, out))
	assert.Equal(t, "/etc/insights/creds/sys.creds", envMap(c)["INSIGHTS_SYS_CREDS"])
	// no inline secret content, so no chart-managed config secret
	_, ok := find[corev1.Secret](t, out, "Secret", fullName+"-config")
	assert.False(t, ok)
	assert.True(t, hasVolume(statefulSet(t, out).Spec.Template.Spec, "sys-creds"))
}

func TestMetricsEnabled(t *testing.T) {
	t.Parallel()
	values := minimalValues()
	values["metrics.enabled"] = "true"
	values["podMonitor.enabled"] = "true"
	out := render(t, values)

	c := mainContainer(t, statefulSet(t, out))
	env := envMap(c)
	assert.Equal(t, "true", env["INSIGHTS_PROMETHEUS_ENABLED"])
	assert.Equal(t, "9091", env["INSIGHTS_PROMETHEUS_PORT"])
	var hasMetricsPort bool
	for _, p := range c.Ports {
		if p.Name == "metrics" {
			hasMetricsPort = true
		}
	}
	assert.True(t, hasMetricsPort, "metrics container port missing")

	svc, _ := find[corev1.Service](t, out, "Service", fullName)
	var svcMetrics bool
	for _, p := range svc.Spec.Ports {
		if p.Name == "metrics" {
			svcMetrics = true
		}
	}
	assert.True(t, svcMetrics, "metrics service port missing")

	require.Contains(t, out, "kind: PodMonitor")
}

func TestPodMonitorRequiresMetrics(t *testing.T) {
	t.Parallel()
	// podMonitor.enabled alone (metrics disabled) must not emit a PodMonitor
	values := minimalValues()
	values["podMonitor.enabled"] = "true"
	out := render(t, values)
	assert.NotContains(t, out, "kind: PodMonitor")
}

func TestIngressEnabled(t *testing.T) {
	t.Parallel()
	values := minimalValues()
	values["ingress.enabled"] = "true"
	values["ingress.hosts[0]"] = "insights.example.com"
	out := render(t, values)

	ing, ok := find[networkingv1.Ingress](t, out, "Ingress", fullName)
	require.True(t, ok, "ingress not rendered")
	require.Len(t, ing.Spec.Rules, 1)
	assert.Equal(t, "insights.example.com", ing.Spec.Rules[0].Host)
	backend := ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service
	assert.Equal(t, fullName, backend.Name)
	assert.EqualValues(t, 8080, backend.Port.Number)
}

func TestPersistenceDisabled(t *testing.T) {
	t.Parallel()
	values := minimalValues()
	values["persistence.enabled"] = "false"
	out := render(t, values)

	sts := statefulSet(t, out)
	assert.Empty(t, sts.Spec.VolumeClaimTemplates, "no volumeClaimTemplates when persistence is off")
	var dataIsEmptyDir bool
	for _, v := range sts.Spec.Template.Spec.Volumes {
		if v.Name == "data" && v.EmptyDir != nil {
			dataIsEmptyDir = true
		}
	}
	assert.True(t, dataIsEmptyDir, "data should fall back to an emptyDir")
}

func TestReplicasMustBeOne(t *testing.T) {
	t.Parallel()
	values := minimalValues()
	values["statefulSet.replicas"] = "2"
	err := renderErr(t, values)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be 1")
}

func TestRequiresSysServer(t *testing.T) {
	t.Parallel()
	err := renderErr(t, map[string]string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config.sys.server")
}

func TestSimulatorMode(t *testing.T) {
	t.Parallel()
	// the simulator provides its own embedded system: no sys.server, creds, or
	// license required, even on the default production image.
	out := render(t, map[string]string{"config.simulator.enabled": "true"})

	c := mainContainer(t, statefulSet(t, out))
	env := envMap(c)
	assert.Equal(t, "true", env["INSIGHTS_SIMULATOR_ENABLED"])
	assert.Equal(t, "js-small", env["INSIGHTS_SIMULATOR_PROFILE"])
	assert.NotContains(t, env, "INSIGHTS_SYS_SERVER")
	assert.True(t, strings.HasPrefix(c.Image, "registry.synadia.io/insights:"))
}

func TestSimulatorTrialNeedsNoLicense(t *testing.T) {
	t.Parallel()
	// simulator skips licensing in the app, so the chart must not require a
	// license even when the trial image is selected.
	err := renderErr(t, map[string]string{
		"edition":                  "trial",
		"config.simulator.enabled": "true",
	})
	require.NoError(t, err)
}

func TestPodDisruptionBudgetEnabled(t *testing.T) {
	t.Parallel()
	values := minimalValues()
	values["podDisruptionBudget.enabled"] = "true"
	out := render(t, values)

	pdb, ok := find[policyv1.PodDisruptionBudget](t, out, "PodDisruptionBudget", fullName)
	require.True(t, ok, "PDB not rendered")
	require.NotNil(t, pdb.Spec.MaxUnavailable)
	assert.Equal(t, "1", pdb.Spec.MaxUnavailable.String())
}

func TestServiceAccountEnabled(t *testing.T) {
	t.Parallel()
	values := minimalValues()
	values["serviceAccount.enabled"] = "true"
	out := render(t, values)

	_, ok := find[corev1.ServiceAccount](t, out, "ServiceAccount", fullName)
	require.True(t, ok, "service account not rendered")
	assert.Equal(t, fullName, statefulSet(t, out).Spec.Template.Spec.ServiceAccountName)
}

func TestStatefulSetMerge(t *testing.T) {
	t.Parallel()
	// merge deep-merges onto the rendered StatefulSet
	values := minimalValues()
	values["statefulSet.merge.spec.minReadySeconds"] = "5"
	sts := statefulSet(t, render(t, values))
	assert.EqualValues(t, 5, sts.Spec.MinReadySeconds)
}

func TestStatefulSetPatch(t *testing.T) {
	t.Parallel()
	// patch applies a JSON Patch document to the rendered StatefulSet
	values := minimalValues()
	values["statefulSet.patch[0].op"] = "add"
	values["statefulSet.patch[0].path"] = "/spec/minReadySeconds"
	values["statefulSet.patch[0].value"] = "9"
	sts := statefulSet(t, render(t, values))
	assert.EqualValues(t, 9, sts.Spec.MinReadySeconds)
}

func TestContainerMerge(t *testing.T) {
	t.Parallel()
	// container.merge merges onto the insights container (resources are empty by default)
	values := minimalValues()
	values["container.merge.resources.requests.cpu"] = "250m"
	c := mainContainer(t, statefulSet(t, render(t, values)))
	assert.Equal(t, "250m", c.Resources.Requests.Cpu().String())
}

func TestPodTemplateMerge(t *testing.T) {
	t.Parallel()
	// podTemplate.merge merges onto the pod template (e.g. scheduling fields)
	values := minimalValues()
	values["podTemplate.merge.spec.nodeSelector.disktype"] = "ssd"
	sts := statefulSet(t, render(t, values))
	assert.Equal(t, "ssd", sts.Spec.Template.Spec.NodeSelector["disktype"])
}

func TestGlobalLabels(t *testing.T) {
	t.Parallel()
	// global.labels propagate onto every chart-managed resource
	values := minimalValues()
	values["global.labels.team"] = "platform"
	out := render(t, values)
	assert.Equal(t, "platform", statefulSet(t, out).Labels["team"])
	svc, _ := find[corev1.Service](t, out, "Service", fullName)
	assert.Equal(t, "platform", svc.Labels["team"])
}

func TestImagePullSecretDisabledUsesGlobalRegistry(t *testing.T) {
	t.Parallel()
	values := minimalValues()
	values["imagePullSecret.enabled"] = "false"
	values["global.image.registry"] = "ghcr.io/connecteverything"
	out := render(t, values)

	_, ok := find[corev1.Secret](t, out, "Secret", fullName+"-regcred")
	assert.False(t, ok, "regcred should not be rendered when imagePullSecret is disabled")

	c := mainContainer(t, statefulSet(t, out))
	assert.True(t, strings.HasPrefix(c.Image, "ghcr.io/connecteverything/insights:"),
		"image should use the global registry when pull secret is off, got %q", c.Image)
	assert.Empty(t, statefulSet(t, out).Spec.Template.Spec.ImagePullSecrets)
}
