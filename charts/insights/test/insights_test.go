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

func envVar(t *testing.T, c corev1.Container, name string) corev1.EnvVar {
	t.Helper()
	for _, e := range c.Env {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("env var %q not found", name)
	return corev1.EnvVar{}
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
	assert.Equal(t, "nats://nats.nats.svc.cluster.local:4222", env["INSIGHTS_SYS_SERVER"])
	// embedded sink binds all interfaces on 4222 by default
	assert.Equal(t, "0.0.0.0", env["INSIGHTS_SINK_HOST"])
	assert.Equal(t, "4222", env["INSIGHTS_SINK_PORT"])
	// production edition pulls no license; no credentials by default
	assert.NotContains(t, env, "INSIGHTS_LICENSE_FILE")
	assert.NotContains(t, env, "INSIGHTS_SYS_CREDS")
	assert.NotContains(t, env, "INSIGHTS_PROMETHEUS_ENABLED")

	// the embedded NATS (sink) port is declared on the container
	var hasNatsPort bool
	for _, p := range c.Ports {
		if p.Name == "nats" && p.ContainerPort == 4222 {
			hasNatsPort = true
		}
	}
	assert.True(t, hasNatsPort, "nats container port missing")

	// headless service exposes both http and nats
	headlessPorts := map[string]bool{}
	hsvc, _ := find[corev1.Service](t, out, "Service", fullName+"-headless")
	for _, p := range hsvc.Spec.Ports {
		headlessPorts[p.Name] = true
	}
	assert.True(t, headlessPorts["http"] && headlessPorts["nats"], "headless service should expose http + nats")

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

func TestSysCredsFileSecret(t *testing.T) {
	t.Parallel()
	values := minimalValues()
	values["config.sys.creds.secretName"] = "my-sys-creds"
	values["config.sys.creds.key"] = "creds"
	out := render(t, values)

	c := mainContainer(t, statefulSet(t, out))
	// the env points at the mounted file, at a stable path regardless of the Secret key
	assert.Equal(t, "/etc/insights/sys/creds/sys.creds", envMap(c)["INSIGHTS_SYS_CREDS"])
	// mounted from the referenced Secret; no chart-managed config Secret needed
	_, ok := find[corev1.Secret](t, out, "Secret", fullName+"-config")
	assert.False(t, ok, "no config secret when only file creds are referenced")

	spec := statefulSet(t, out).Spec.Template.Spec
	require.True(t, hasVolume(spec, "sys-creds"))
	for _, v := range spec.Volumes {
		if v.Name == "sys-creds" {
			require.NotNil(t, v.Secret)
			assert.Equal(t, "my-sys-creds", v.Secret.SecretName)
			require.Len(t, v.Secret.Items, 1)
			assert.Equal(t, "creds", v.Secret.Items[0].Key)
			assert.Equal(t, "sys.creds", v.Secret.Items[0].Path)
		}
	}
}

func TestSysBasicAuth(t *testing.T) {
	t.Parallel()
	values := minimalValues()
	values["config.sys.user"] = "sys"
	values["config.sys.password"] = "s3cret"
	out := render(t, values)

	c := mainContainer(t, statefulSet(t, out))
	assert.Equal(t, "sys", envMap(c)["INSIGHTS_SYS_USER"])
	// password is injected from the chart-managed config Secret, never set inline on the pod
	pw := envVar(t, c, "INSIGHTS_SYS_PASSWORD")
	require.NotNil(t, pw.ValueFrom)
	require.NotNil(t, pw.ValueFrom.SecretKeyRef)
	assert.Equal(t, fullName+"-config", pw.ValueFrom.SecretKeyRef.Name)
	assert.Equal(t, "sys-password", pw.ValueFrom.SecretKeyRef.Key)
	assert.Empty(t, pw.Value)

	secret, ok := find[corev1.Secret](t, out, "Secret", fullName+"-config")
	require.True(t, ok)
	assert.Contains(t, secret.StringData, "sys-password")
}

func TestSysNkeyJwt(t *testing.T) {
	t.Parallel()
	values := minimalValues()
	values["config.sys.nkey"] = "SUACSEED"
	values["config.sys.jwt"] = "eyJ.jwt.sig"
	out := render(t, values)

	c := mainContainer(t, statefulSet(t, out))
	assert.Equal(t, "sys-nkey", envVar(t, c, "INSIGHTS_SYS_NKEY").ValueFrom.SecretKeyRef.Key)
	assert.Equal(t, "sys-jwt", envVar(t, c, "INSIGHTS_SYS_JWT").ValueFrom.SecretKeyRef.Key)

	secret, _ := find[corev1.Secret](t, out, "Secret", fullName+"-config")
	assert.Contains(t, secret.StringData, "sys-nkey")
	assert.Contains(t, secret.StringData, "sys-jwt")
}

func TestSysTLS(t *testing.T) {
	t.Parallel()
	values := minimalValues()
	values["config.sys.tls.cert.secretName"] = "client-tls"
	values["config.sys.tls.key.secretName"] = "client-tls"
	values["config.sys.tls.caCert.secretName"] = "ca-bundle"
	out := render(t, values)

	env := envMap(mainContainer(t, statefulSet(t, out)))
	assert.Equal(t, "/etc/insights/sys/tls-cert/tls.crt", env["INSIGHTS_SYS_TLS_CERT"])
	assert.Equal(t, "/etc/insights/sys/tls-key/tls.key", env["INSIGHTS_SYS_TLS_KEY"])
	assert.Equal(t, "/etc/insights/sys/tls-ca/ca.crt", env["INSIGHTS_SYS_TLS_CA"])

	spec := statefulSet(t, out).Spec.Template.Spec
	assert.True(t, hasVolume(spec, "sys-tls-cert"))
	assert.True(t, hasVolume(spec, "sys-tls-key"))
	assert.True(t, hasVolume(spec, "sys-tls-ca"))
}

func TestRetentionDuration(t *testing.T) {
	t.Parallel()
	values := minimalValues()
	values["config.db.retention.duration"] = "720h"
	out := render(t, values)
	assert.Equal(t, "720h", envMap(mainContainer(t, statefulSet(t, out)))["INSIGHTS_DB_RETENTION_DURATION"])
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
