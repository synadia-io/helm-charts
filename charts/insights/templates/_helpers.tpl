{{/*
Expand the name of the chart.
*/}}
{{- define "ins.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "ins.fullname" -}}
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
{{- define "ins.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Print the namespace
*/}}
{{- define "ins.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride }}
{{- end }}

{{/*
Print the namespace for the metadata section
*/}}
{{- define "ins.metadataNamespace" -}}
{{- with .Values.namespaceOverride }}
namespace: {{ . | quote }}
{{- end }}
{{- end }}

{{/*
Set default values.
*/}}
{{- define "ins.defaultValues" }}
{{- if not .defaultValuesSet }}
  {{- $name := include "ins.fullname" . }}
  {{- with .Values }}
    {{- $_ := set .configSecret        "name" (.configSecret.name        | default (printf "%s-config" $name)) }}
    {{- $_ := set .statefulSet         "name" (.statefulSet.name         | default $name) }}
    {{- $_ := set .imagePullSecret     "name" (.imagePullSecret.name     | default (printf "%s-regcred" $name)) }}
    {{- $_ := set .ingress             "name" (.ingress.name             | default $name) }}
    {{- $_ := set .service             "name" (.service.name             | default $name) }}
    {{- $_ := set .headlessService     "name" (.headlessService.name     | default (printf "%s-headless" $name)) }}
    {{- $_ := set .serviceAccount      "name" (.serviceAccount.name      | default $name) }}
    {{- $_ := set .podDisruptionBudget "name" (.podDisruptionBudget.name | default $name) }}
    {{- $_ := set .podMonitor          "name" (.podMonitor.name          | default $name) }}
    {{- /* edition selects the default image repository: production -> insights, trial -> insights-licensed */}}
    {{- $repo := ternary "insights-licensed" "insights" (eq .edition "trial") }}
    {{- $_ := set .container.image "repository" (.container.image.repository | default $repo) }}
    {{- $_ := set .container.image "tag"        (.container.image.tag        | default $.Chart.AppVersion) }}
  {{- end }}

  {{- include "ins.requiredValues" . }}

  {{- $values := get (include "tplYaml" (dict "doc" .Values "ctx" $) | fromJson) "doc" }}
  {{- $_ := set . "Values" $values }}

  {{- $_ := set . "defaultValuesSet" true }}
{{- end }}
{{- end }}

{{/*
Set required values.
*/}}
{{- define "ins.requiredValues" }}
  {{- with .Values }}
    {{- if not (or (eq .edition "production") (eq .edition "trial")) }}
      {{- fail (cat "edition must be \"production\" or \"trial\", got" (.edition | quote)) }}
    {{- end }}
    {{- if ne (int .statefulSet.replicas) 1 }}
      {{- fail "statefulSet.replicas must be 1: insights runs as a single instance (single-writer DuckDB + embedded NATS sink)" }}
    {{- end }}
    {{- if eq .edition "trial" }}
      {{- if not (or .config.license.token .config.license.secretName) }}
        {{- fail "config.license.token or config.license.secretName is required when edition is \"trial\" (the insights-licensed image validates a license JWT)" }}
      {{- end }}
    {{- end }}
    {{- if not .config.sys.server }}
      {{- fail "config.sys.server is required: the NATS system URL to monitor (e.g. tls://connect.ngs.global)" }}
    {{- end }}
  {{- end }}
{{- end }}

{{/*
ins.labels
*/}}
{{- define "ins.labels" -}}
{{- with .Values.global.labels -}}
{{ toYaml . }}
{{ end -}}
helm.sh/chart: {{ include "ins.chart" . }}
{{ include "ins.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
ins.selectorLabels
*/}}
{{- define "ins.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ins.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: insights
{{- end }}

{{/*
Print the image.
When imagePullSecret is enabled the registry defaults to imagePullSecret.registry,
otherwise it falls back to global.image.registry. An explicit image.registry always wins.
*/}}
{{- define "ins.image" }}
{{- $image := printf "%s:%s" .repository .tag }}
{{- if or .registry .imagePullSecret.enabled .global.image.registry }}
{{- $image = printf "%s/%s" (.registry | default (ternary .imagePullSecret.registry .global.image.registry .imagePullSecret.enabled)) $image }}
{{- end -}}
image: {{ $image }}
{{- if or .pullPolicy .global.image.pullPolicy }}
imagePullPolicy: {{ .pullPolicy | default .global.image.pullPolicy }}
{{- end }}
{{- end }}

{{/*
Translates env var map to list.
*/}}
{{- define "ins.env" -}}
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
ins.loadMergePatch
input: map with 4 keys:
- file: name of file to load
- ctx: context to pass to tpl
- merge: interface{} to merge
- patch: []interface{} valid JSON Patch document
output: JSON encoded map with 1 key:
- doc: interface{} patched json result
*/}}
{{- define "ins.loadMergePatch" -}}
{{- $doc := tpl (.ctx.Files.Get (printf "files/%s" .file)) .ctx | fromYaml | default dict -}}
{{- $doc = mergeOverwrite $doc (deepCopy (.merge | default dict)) -}}
{{- get (include "jsonpatch" (dict "doc" $doc "patch" (.patch | default list)) | fromJson ) "doc" | toYaml -}}
{{- end }}
