{{/*
Expand the name of the chart.
*/}}
{{- define "synack.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "synack.fullname" -}}
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
{{- define "synack.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Print the namespace
*/}}
{{- define "synack.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride }}
{{- end }}

{{/*
Print the namespace for the metadata section
*/}}
{{- define "synack.metadataNamespace" -}}
{{- with .Values.namespaceOverride }}
namespace: {{ . | quote }}
{{- end }}
{{- end }}

{{/*
Set default values.
*/}}
{{- define "synack.defaultValues" }}
{{- if not .defaultValuesSet }}
  {{- $name := include "synack.fullname" . }}
  {{- include "synack.requiredValues" . }}
  {{- with .Values }}
    {{- $_ := set .tokenSecret          "name" (.tokenSecret.name          | default (printf "%s-token" $name)) }}
    {{- $_ := set .deployment           "name" (.deployment.name           | default $name) }}
    {{- $_ := set .serviceAccount       "name" (.serviceAccount.name       | default $name) }}
    {{- $_ := set .clusterRole          "name" (.clusterRole.name          | default $name) }}
    {{- $_ := set .clusterRoleBinding   "name" (.clusterRoleBinding.name   | default $name) }}
  {{- end }}

  {{- $values := get (include "tplYaml" (dict "doc" .Values "ctx" $) | fromJson) "doc" }}
  {{- $_ := set . "Values" $values }}

  {{- $_ := set . "defaultValuesSet" true }}
{{- end }}
{{- end }}

{{/*
Set required values.
*/}}
{{- define "synack.requiredValues" }}
  {{- with .Values }}
    {{- $_ := (.config.token | required "config.token is required") }}
    {{- $_ := (.config.controlPlaneBaseURL | required "config.controlPlaneBaseURL is required") }}
  {{- end }}
{{- end }}

{{/*
synack.labels
*/}}
{{- define "synack.labels" -}}
{{- with .Values.global.labels -}}
{{ toYaml . }}
{{ end -}}
helm.sh/chart: {{ include "synack.chart" . }}
{{ include "synack.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
synack.selector labels
*/}}
{{- define "synack.selectorLabels" -}}
app.kubernetes.io/name: {{ include "synack.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: synack
{{- end }}

{{/*
Print the image
*/}}
{{- define "synack.image" }}
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
{{- define "synack.env" -}}
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

{{- /*
synack.loadMergePatch
input: map with 4 keys:
- file: name of file to load
- ctx: context to pass to tpl
- merge: interface{} to merge
- patch: []interface{} valid JSON Patch document
output: JSON encoded map with 1 key:
- doc: interface{} patched json result
*/}}
{{- define "synack.loadMergePatch" -}}
{{- $doc := tpl (.ctx.Files.Get (printf "files/%s" .file)) .ctx | fromYaml | default dict -}}
{{- $doc = mergeOverwrite $doc (deepCopy (.merge | default dict)) -}}
{{- get (include "jsonpatch" (dict "doc" $doc "patch" (.patch | default list)) | fromJson ) "doc" | toYaml -}}
{{- end }}
