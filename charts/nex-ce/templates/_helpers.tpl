{{/*
Expand the name of the chart.
*/}}
{{- define "nce.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "nce.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "nce.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Print the namespace
*/}}
{{- define "nce.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride }}
{{- end }}

{{/*
Print the namespace for the metadata section
*/}}
{{- define "nce.metadataNamespace" -}}
{{- with .Values.namespaceOverride }}
namespace: {{ . | quote }}
{{- end }}
{{- end }}

{{/*
Set default values.
*/}}
{{- define "nce.defaultValues" }}
{{- if not .defaultValuesSet }}
  {{- $name := include "nce.fullname" . }}
  {{- include "nce.requiredValues" . }}
  {{- with .Values }}
    {{- $_ := set .configSecret        "name" (.configSecret.name        | default (printf "%s-config" $name)) }}
    {{- $_ := set .deployment          "name" (.deployment.name          | default $name) }}
    {{- $_ := set .serviceAccount      "name" (.serviceAccount.name      | default $name) }}
    {{- $_ := set .podDisruptionBudget "name" (.podDisruptionBudget.name | default $name) }}
    {{- $_ := set .workloadServiceAccount "name" (.workloadServiceAccount.name | default (printf "%s-workload" $name)) }}
  {{- end }}

  {{- $values := get (include "tplYaml" (dict "doc" .Values "ctx" $) | fromJson) "doc" }}
  {{- $_ := set . "Values" $values }}

  {{- $_ := set . "defaultValuesSet" true }}
{{- end }}
{{- end }}

{{/*
Set required values.
*/}}
{{- define "nce.requiredValues" }}
  {{- with .Values }}
    {{- if and .config.tls.clientCert.cert (not .config.tls.clientCert.key) }}
      {{- fail "config.tls.clientCert.key is required if cert is defined" }}
    {{- end }}
    {{- if and .config.tls.clientCert.key (not .config.tls.clientCert.cert) }}
      {{- fail "config.tls.clientCert.cert is required if key is defined" }}
    {{- end }}
    {{- if not .config.nodeSeed }}
      {{- fail "config.nodeSeed is required (go run github.com/nats-io/nkeys/nk@latest -gen server)" }}
    {{- end }}
    {{- if .config.platform.enabled }}
      {{- if not .config.platform.token }}
        {{- fail "config.platform.token is required when config.platform.enabled is true" }}
      {{- end }}
    {{- else if not .config.url }}
      {{- fail "set config.platform.enabled (Control Plane registration) or config.url (direct NATS connection)" }}
    {{- end }}
    {{- if and .config.tls.clientCert.enabled (not .config.tls.clientCert.secretName) }}
      {{- fail "config.tls.clientCert.secretName is required when config.tls.clientCert.enabled is true (nex-ce checks the files at startup)" }}
    {{- end }}
    {{- if and .config.tls.caCerts.enabled (not (or .config.tls.caCerts.configMapName .config.tls.caCerts.secretName)) }}
      {{- fail "config.tls.caCerts.configMapName or secretName is required when config.tls.caCerts.enabled is true" }}
    {{- end }}
    {{- if not (or .config.nexlets.connectors.enabled .config.nexlets.containers.enabled) }}
      {{- fail "enable at least one of config.nexlets.connectors or config.nexlets.containers" }}
    {{- end }}
  {{- end }}
{{- end }}

{{/*
nce.labels
*/}}
{{- define "nce.labels" -}}
{{- with .Values.global.labels -}}
{{ toYaml . }}
{{ end -}}
helm.sh/chart: {{ include "nce.chart" . }}
{{ include "nce.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
nce.selector labels
*/}}
{{- define "nce.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nce.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: nex-ce
{{- end }}

{{/*
Print the image
*/}}
{{- define "nce.image" }}
{{- $image := printf "%s:%s" .repository .tag }}
{{- if or .registry .global.image.registry }}
{{- $image = printf "%s/%s" (.registry | default .global.image.registry) $image }}
{{- end -}}
image: {{ $image }}
{{- if or .pullPolicy .global.image.pullPolicy }}
imagePullPolicy: {{ .pullPolicy | default .global.image.pullPolicy }}
{{- end }}
{{- end }}

{{/*
translates env var map to list
*/}}
{{- define "nce.env" -}}
{{- range $k, $v := . }}
{{- if kindIs "string" $v }}
- name: {{ $k | quote }}
  value: {{ $v | quote }}
{{- else if kindIs "map" $v }}
- {{ merge (dict "name" $k) $v | toYaml | nindent 2 }}
{{- else }}
{{- fail (cat "env var" $k "must be string or map, got" (kindOf $v)) }}
{{- end }}
{{- end }}
{{- end }}

{{/*
List of external secretNames
*/}}
{{- define "nce.secretNames" -}}
{{- $secrets := list }}
  {{- with .Values.config.tls.clientCert }}
    {{- if and .enabled .secretName }}
      {{- $secrets = append $secrets (merge (dict "name" "tls-client") .) }}
    {{- end }}
  {{- end }}
{{- toJson (dict "secretNames" $secrets) }}
{{- end }}

{{- define "nce.tlsCAVolume" -}}
{{- with .Values.config.tls.caCerts }}
{{- if and .enabled (or .configMapName .secretName) }}
- name: tls-ca
{{- if .configMapName }}
  configMap:
    name: {{ .configMapName | quote }}
{{- else if .secretName }}
  secret:
    secretName: {{ .secretName | quote }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}

{{- define "nce.tlsCAVolumeMount" -}}
{{- with .Values.config.tls.caCerts }}
{{- if and .enabled (or .configMapName .secretName) }}
- name: tls-ca
  mountPath: {{ .dir | quote }}
{{- end }}
{{- end }}
{{- end }}

{{- /*
nce.loadMergePatch
input: map with 4 keys:
- file: name of file to load
- ctx: context to pass to tpl
- merge: interface{} to merge
- patch: []interface{} valid JSON Patch document
output: JSON encoded map with 1 key:
- doc: interface{} patched json result
*/}}
{{- define "nce.loadMergePatch" -}}
{{- $doc := tpl (.ctx.Files.Get (printf "files/%s" .file)) .ctx | fromYaml | default dict -}}
{{- $doc = mergeOverwrite $doc (deepCopy (.merge | default dict)) -}}
{{- get (include "jsonpatch" (dict "doc" $doc "patch" (.patch | default list)) | fromJson ) "doc" | toYaml -}}
{{- end }}

{{/*
nce.config renders the nex-ce config file as JSON.
Top-level and nested group keys are snake_case; nexlet keys are camelCase.
Keys are only written when set, so empty values do not override nex-ce defaults.
*/}}
{{- define "nce.config" -}}
{{- $c := .Values.config }}
{{- $cfg := dict "name" ($c.name | default "nex-ce") "node_seed" ($c.nodeSeed | default "") }}
{{- with $c.tags }}
{{- /* nex-ce takes map[string]string; --set tags.x=1 would otherwise arrive as a number */}}
{{- $tags := dict }}
{{- range $k, $v := . }}
{{- $_ := set $tags $k (toString $v) }}
{{- end }}
{{- $_ := set $cfg "tags" $tags }}
{{- end }}
{{- $_ := set $cfg "logger" (dict "level" ($c.logLevel | default "info" | lower)) }}

{{- $nats := dict }}
{{- with $c.url }}
{{- $_ := set $nats "servers" (list .) }}
{{- end }}
{{- with $c.creds.seed }}
{{- $_ := set $nats "seed" . }}
{{- end }}
{{- with $c.creds.jwt }}
{{- $_ := set $nats "jwt" . }}
{{- end }}
{{- with $c.tls.clientCert }}
{{- if .enabled }}
{{- $_ := set $nats "tlscert" (printf "%s/%s" .dir .cert) }}
{{- $_ := set $nats "tlskey" (printf "%s/%s" .dir .key) }}
{{- end }}
{{- end }}
{{- with $c.tls.caCerts }}
{{- if .enabled }}
{{- $_ := set $nats "tlsca" (printf "%s/%s" .dir .key) }}
{{- end }}
{{- end }}
{{- if $nats }}
{{- $_ := set $cfg "nats" $nats }}
{{- end }}

{{- if $c.platform.enabled }}
{{- /* Control Plane supplies the nexus and control account in platform mode */}}
{{- $platform := dict "enabled" true }}
{{- with $c.platform.url }}
{{- $_ := set $platform "url" . }}
{{- end }}
{{- with $c.platform.token }}
{{- $_ := set $platform "token" . }}
{{- end }}
{{- $_ := set $cfg "platform" $platform }}
{{- else }}
{{- $_ := set $cfg "nexus" ($c.nexus | default "nexus") }}
{{- with $c.credsSigning.signingKey }}
{{- $_ := set $cfg "creds_signing_key" . }}
{{- end }}
{{- with $c.credsSigning.signingKeyAccount }}
{{- $_ := set $cfg "control_account" . }}
{{- end }}
{{- end }}
{{- if $c.allowRemoteRegister }}
{{- $_ := set $cfg "allow_remote_register" true }}
{{- end }}

{{- if $c.catalog.enabled }}
{{- $catalog := dict "enabled" true }}
{{- range $k := list "name" "id" "token" }}
{{- with get $c.catalog $k }}
{{- $_ := set $catalog $k . }}
{{- end }}
{{- end }}
{{- $_ := set $cfg "catalog" $catalog }}
{{- end }}

{{- $ns := include "nce.namespace" . | trim }}
{{- $nexlets := dict }}
{{- $_ := set $nexlets "connectors-kubernetes" (include "nce.nexletConfig" (dict "ctx" . "nexlet" $c.nexlets.connectors "registerType" "connector" "namespace" ($c.connectorsNamespace | default $ns)) | fromJson) }}
{{- $_ := set $nexlets "containers-kubernetes" (include "nce.nexletConfig" (dict "ctx" . "nexlet" $c.nexlets.containers "registerType" "container" "namespace" ($c.workloadsNamespace | default $ns)) | fromJson) }}
{{- $_ := set $cfg "nexlets" $nexlets }}

{{- toPrettyJson $cfg }}
{{- end }}

{{/*
nce.nexletNamespaces prints a JSON list of the namespaces the enabled nexlets create workloads in.
*/}}
{{- define "nce.nexletNamespaces" -}}
{{- $c := .Values.config }}
{{- $ns := include "nce.namespace" . | trim }}
{{- $list := list }}
{{- if $c.nexlets.connectors.enabled }}
{{- $list = append $list ($c.connectorsNamespace | default $ns) }}
{{- end }}
{{- if $c.nexlets.containers.enabled }}
{{- $list = append $list ($c.workloadsNamespace | default $ns) }}
{{- end }}
{{- $list | uniq | toJson }}
{{- end }}

{{/*
nce.nexletConfig renders one nexlet entry of the nex-ce config.
input: dict with ctx, nexlet (values), registerType, namespace
Keys are only written when set; k8sServiceAccountName falls back to the workload ServiceAccount.
*/}}
{{- define "nce.nexletConfig" -}}
{{- $cfg := dict "enabled" (.nexlet.enabled | default false) "registerType" .registerType "k8sNamespace" .namespace }}
{{- $sa := .nexlet.serviceAccountName }}
{{- if and (not $sa) .ctx.Values.workloadServiceAccount.enabled }}
{{- $sa = .ctx.Values.workloadServiceAccount.name }}
{{- end }}
{{- with $sa }}
{{- $_ := set $cfg "k8sServiceAccountName" . }}
{{- end }}
{{- with .nexlet.imagePullSecrets }}
{{- if kindIs "slice" . }}
{{- $_ := set $cfg "k8sImagePullSecrets" . }}
{{- else }}
{{- $_ := set $cfg "k8sImagePullSecrets" (list (toString .)) }}
{{- end }}
{{- end }}
{{- /* --set-string leaves numbers as strings; nex-ce wants a number, an integer and an integer */}}
{{- with .nexlet.defaultCpu }}
{{- $_ := set $cfg "k8sDefaultCpu" (float64 .) }}
{{- end }}
{{- with .nexlet.defaultMemoryMb }}
{{- $_ := set $cfg "k8sDefaultMemoryMb" (int .) }}
{{- end }}
{{- with .nexlet.metricsPort }}
{{- $_ := set $cfg "k8sMetricsPort" (int .) }}
{{- end }}
{{- toJson $cfg }}
{{- end }}
