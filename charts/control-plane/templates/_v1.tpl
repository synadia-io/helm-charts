{{- /*
Upgrades from control-plane chart 1.x. Removing 1.x upgrade support means deleting this file, the includes of
scp.v1ReuseValuesGuard, scp.v1Defaults, scp.v1Validate and scp.v1UpgradeGuard, the deployment.name fallback in
scp.defaultValues, the syn-cp.yaml key of the config Secret, and the Postgres config (files/config/postgres.yaml and
the postgres-tls secret).
*/}}

{{/*
helm upgrade --reuse-values from chart 1.x replaces the chart 2.x defaults with the 1.x values, so the keys that 2.x
added are missing
*/}}
{{- define "scp.v1ReuseValuesGuard" }}
{{- if not (and (kindIs "map" .Values.statefulSet) (kindIs "map" .Values.headlessService)) }}
  {{- fail "The values are missing statefulSet and headlessService, which chart 2.x adds. helm upgrade --reuse-values from chart 1.x does this. Pass the values file with -f, or use --reset-then-reuse-values instead." }}
{{- end }}
{{- end }}

{{/*
chart 1.x values set the replica count in deployment.replicas
*/}}
{{- define "scp.v1Defaults" }}
{{- if kindIs "invalid" .Values.statefulSet.replicas }}
  {{- $_ := set .Values.statefulSet "replicas" (dig "deployment" "replicas" 1 .Values) }}
  {{- $_ := set . "replicasKey" (ternary "deployment.replicas" "statefulSet.replicas" (hasKey (.Values.deployment | default dict) "replicas")) }}
{{- end }}
{{- end }}

{{/*
Reject chart 1.x values that 2.x cannot apply
*/}}
{{- define "scp.v1Validate" }}
{{- $replicas := int .Values.statefulSet.replicas }}
{{- $replicasKey := .replicasKey | default "statefulSet.replicas" }}
{{- $singleReplicaMode := dig "singleReplicaMode" "enabled" "" .Values }}
{{- if and (kindIs "bool" $singleReplicaMode) (ne $singleReplicaMode (eq $replicas 1)) }}
  {{- fail (printf "singleReplicaMode.enabled is %t, but %s is %d. Chart 2.x uses only statefulSet.replicas, so remove singleReplicaMode and set statefulSet.replicas." $singleReplicaMode $replicasKey $replicas) }}
{{- end }}
{{- if or (dig "deployment" "merge" (dict) .Values) (dig "deployment" "patch" (list) .Values) }}
  {{- fail "deployment.merge and deployment.patch changed the chart 1.x Deployment. Chart 2.x runs a StatefulSet, so move them to statefulSet.merge and statefulSet.patch, or to podTemplate for changes to the pods." }}
{{- end }}
{{- /* the 1.x data was created with a login for every user, so the mode must be chosen rather than defaulted */}}
{{- if and (dig "migration" "v1" "postgres_dsn" "" .config) (not .Values.config.mode) }}
  {{- fail "config.dataSources.postgres.dsn imports the data of a control-plane chart 1.x install, where every user must log in. Chart 2.x defaults config.mode to standalone, which requires no login and makes every user an admin. Set config.mode to managed to keep logins, which also requires config.merge.license.enabled: true, or to standalone to drop them." }}
{{- end }}
{{- if regexMatch "^v?1\\." (toString .Values.container.image.tag) }}
  {{- fail (printf "container.image.tag %s is a Control Plane 1.x image, and chart 2.x needs a 2.x image. Remove container.image.tag to use the chart's default." (toString .Values.container.image.tag)) }}
{{- end }}
{{- end }}

{{/*
chart 1.x used a Deployment
*/}}
{{- define "scp.v1UpgradeGuard" }}
{{- range (lookup "apps/v1" "Deployment" .Release.Namespace "").items }}
  {{- $labels := .metadata.labels | default dict }}
  {{- if and (eq (get $labels "app.kubernetes.io/instance") $.Release.Name) (eq (get $labels "app.kubernetes.io/component") "control-plane") }}
    {{- include "scp.v1DeploymentGuard" (dict "ctx" $ "deployment" .) }}
  {{- end }}
{{- end }}
{{- end }}

{{/*
Only a multi-replica 1.x release upgrades in place, into a multi-replica 2.x release that imports its Postgres data
input: map with 2 keys:
- ctx: root context
- deployment: the 1.x Deployment
*/}}
{{- define "scp.v1DeploymentGuard" }}
{{- $ctx := .ctx }}
{{- $name := .deployment.metadata.name }}

{{- /* chart 1.x sets the Recreate strategy only in singleReplicaMode */}}
{{- if eq (dig "spec" "strategy" "type" "" .deployment) "Recreate" }}
  {{- fail (printf "Deployment %s is from control-plane chart 1.x in single-replica mode. Only a multi-replica 1.x release can upgrade to 2.x. Pin --version 1.x for this release." $name) }}
{{- end }}

{{- /* the 1.x PVCs leave the manifest, so the upgrade would delete them */}}
{{- range (lookup "v1" "PersistentVolumeClaim" $ctx.Release.Namespace "").items }}
  {{- if eq (get (.metadata.annotations | default dict) "meta.helm.sh/release-name") $ctx.Release.Name }}
    {{- fail (printf "PVC %s belongs to the control-plane chart 1.x release of Deployment %s. Upgrading to 2.x would delete it and its data. Pin --version 1.x for this release." .metadata.name $name) }}
  {{- end }}
{{- end }}

{{- /* the import reads the final 1.x database schema, which Control Plane 1.14.1 has; the image tag can differ from the chart's */}}
{{- $version := "" }}
{{- range (dig "spec" "template" "spec" "containers" (list) .deployment) }}
  {{- if eq .name "syn-cp" }}
    {{- $version = regexReplaceAll "^.*:" (regexReplaceAll "@.*$" .image "") "" | trimSuffix "-slim" }}
  {{- end }}
{{- end }}
{{- if not (regexMatch "^v?[0-9]+\\.[0-9]+\\.[0-9]+$" $version) }}
  {{- $version = get (.deployment.metadata.labels | default dict) "app.kubernetes.io/version" }}
{{- end }}
{{- if and (regexMatch "^v?[0-9]+\\.[0-9]+\\.[0-9]+$" $version) (not (semverCompare ">=1.14.1" $version)) }}
  {{- fail (printf "Deployment %s runs Control Plane %s. Control Plane 2.x imports the data of 1.14.1 or later, so upgrade the release to the final 1.x chart version first." $name $version) }}
{{- end }}

{{- if eq (include "scp.replicaMode" $ctx) "single" }}
  {{- fail (printf "Deployment %s is from a multi-replica control-plane chart 1.x release, which upgrades only to a multi-replica cluster. Set statefulSet.replicas to 3 or more." $name) }}
{{- end }}

{{- if not (dig "migration" "v1" "postgres_dsn" "" $ctx.config) }}
  {{- fail (printf "Deployment %s is from control-plane chart 1.x. Set config.dataSources.postgres.dsn to the Postgres database that 1.x uses, so that Control Plane imports the 1.x data." $name) }}
{{- end }}

{{- /* v2 decrypts the 1.x data only with the key URLs that 1.x used */}}
{{- $secretName := $ctx.Values.configSecret.name }}
{{- $raw := dig "data" "syn-cp.yaml" "" (lookup "v1" "Secret" $ctx.Release.Namespace $secretName) }}
{{- if not $raw }}
  {{- fail (printf "Cannot read the control-plane chart 1.x config from key syn-cp.yaml of Secret %s. If an earlier upgrade of this release to 2.x failed, run helm rollback to its last 1.x revision, then upgrade again." $secretName) }}
{{- end }}
{{- $_ := set $ctx "v1Config" ($raw | b64dec) }}
{{- $v1 := $ctx.v1Config | fromYaml }}
{{- $v1Keys := prepend (dig "kms" "rotated_key_urls" (list) $v1) (dig "kms" "key_url" "" $v1) }}
{{- $v2Keys := prepend (dig "kms" "rotated_key_urls" (list) $ctx.config) (dig "kms" "key_url" "" $ctx.config) }}
{{- range $i, $k := $v1Keys }}
  {{- if and $k (not (has $k $v2Keys)) }}
    {{- /* the URL can hold key material, so name it by position */}}
    {{- fail (printf "config.kms must include every key URL that chart 1.x used, as the key or a rotated key, so that Control Plane can decrypt the 1.x data. It is missing the 1.x %s." (ternary "kms.key_url" (printf "kms.rotated_key_urls[%d]" (sub $i 1)) (eq $i 0))) }}
  {{- end }}
{{- end }}
{{- end }}
