{{/*
Expand the name of the chart.
*/}}
{{- define "sd.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "sd.fullname" -}}
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
{{- define "sd.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Print the namespace
*/}}
{{- define "sd.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride }}
{{- end }}

{{/*
Print the namespace for the metadata section
*/}}
{{- define "sd.metadataNamespace" -}}
{{- with .Values.namespaceOverride }}
namespace: {{ . | quote }}
{{- end }}
{{- end }}

{{/*
Set default values.
*/}}
{{- define "sd.defaultValues" }}
{{- if not .defaultValuesSet }}
  {{- $name := include "sd.fullname" . }}
  {{- include "sd.requiredValues" . }}
  {{- with .Values }}
    {{- $_ := set .tokenSecret         "name" (.tokenSecret.name         | default (printf "%s-token" $name)) }}
    {{- $_ := set .deployment          "name" (.deployment.name          | default $name) }}
    {{- $_ := set .serviceAccount      "name" (.serviceAccount.name      | default $name) }}
    {{- $_ := set .podDisruptionBudget "name" (.podDisruptionBudget.name | default $name) }}
  {{- end }}

  {{- $values := get (include "tplYaml" (dict "doc" .Values "ctx" $) | fromJson) "doc" }}
  {{- $_ := set . "Values" $values }}

  {{- $_ := set . "defaultValuesSet" true }}
{{- end }}
{{- end }}

{{/*
Set required values.
*/}}
{{- define "sd.requiredValues" }}
  {{- with .Values }}
    {{- $_ := (.config.token | required "config.token is required")}}
    {{- if and .config.tls.clientCert.cert (not .config.tls.clientCert.key) }}
      {{- fail "config.tls.clientCert.key is required if cert is defined" }}
    {{- end }}
    {{- if and .config.tls.clientCert.key (not .config.tls.clientCert.cert) }}
      {{- fail "config.tls.clientCert.cert is required if key is defined" }}
    {{- end }}
  {{- end }}
{{- end }}

{{/*
sd.labels
*/}}
{{- define "sd.labels" -}}
{{- with .Values.global.labels -}}
{{ toYaml . }}
{{ end -}}
helm.sh/chart: {{ include "sd.chart" . }}
{{ include "sd.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
sd.selector labels
*/}}
{{- define "sd.selectorLabels" -}}
app.kubernetes.io/name: {{ include "sd.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: synadia-deploy
{{- end }}

{{/*
Print the image
*/}}
{{- define "sd.image" }}
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
{{- define "sd.env" -}}
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
{{- define "sd.secretNames" -}}
{{- $secrets := list }}
  {{- with .Values.config.tls.clientCert }}
    {{- if and .enabled .secretName }}
      {{- $secrets = append $secrets (merge (dict "name" "tls-client") .) }}
    {{- end }}
  {{- end }}
{{- toJson (dict "secretNames" $secrets) }}
{{- end }}

{{- define "sd.tlsCAVolume" -}}
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

{{- define "sd.tlsCAVolumeMount" -}}
{{- with .Values.config.tls.caCerts }}
{{- if and .enabled (or .configMapName .secretName) }}
- name: tls-ca
  mountPath: {{ .dir | quote }}
{{- end }}
{{- end }}
{{- end }}

{{- /*
sd.loadMergePatch
input: map with 4 keys:
- file: name of file to load
- ctx: context to pass to tpl
- merge: interface{} to merge
- patch: []interface{} valid JSON Patch document
output: JSON encoded map with 1 key:
- doc: interface{} patched json result
*/}}
{{- define "sd.loadMergePatch" -}}
{{- $doc := tpl (.ctx.Files.Get (printf "files/%s" .file)) .ctx | fromYaml | default dict -}}
{{- $doc = mergeOverwrite $doc (deepCopy (.merge | default dict)) -}}
{{- get (include "jsonpatch" (dict "doc" $doc "patch" (.patch | default list)) | fromJson ) "doc" | toYaml -}}
{{- end }}
