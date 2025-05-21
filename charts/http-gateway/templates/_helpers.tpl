{{/*
Expand the name of the chart.
*/}}
{{- define "nhg.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "nhg.fullname" -}}
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
{{- define "nhg.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Print the namespace
*/}}
{{- define "nhg.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride }}
{{- end }}

{{/*
Print the namespace for the metadata section
*/}}
{{- define "nhg.metadataNamespace" -}}
{{- with .Values.namespaceOverride }}
namespace: {{ . | quote }}
{{- end }}
{{- end }}

{{/*
Set default values.
*/}}
{{- define "nhg.defaultValues" }}
{{- if not .defaultValuesSet }}
  {{- $name := include "nhg.fullname" . }}
  {{- include "nhg.requiredValues" . }}
  {{- with .Values }}
    {{- $_ := set .config              "tokensBucket" (.config.tokensBucket       | default "NHG_TOKENS" ) }}
    {{- $_ := set .deployment          "name"         (.deployment.name           | default $name) }}
    {{- $_ := set .ingress             "name"         (.ingress.name              | default $name) }}
    {{- $_ := set .service             "name"         (.service.name              | default $name) }}
    {{- $_ := set .serviceAccount      "name"         (.serviceAccount.name       | default $name) }}
    {{- $_ := set .podDisruptionBudget "name"         (.podDisruptionBudget.name  | default $name) }}
  {{- end }}

  {{- $values := get (include "tplYaml" (dict "doc" .Values "ctx" $) | fromJson) "doc" }}
  {{- $_ := set . "Values" $values }}

  {{- $_ := set . "defaultValuesSet" true }}
{{- end }}
{{- end }}

{{/*
Set required values.
*/}}
{{- define "nhg.requiredValues" }}
  {{- with .Values }}
    {{- $_ := (.config.url | required "config.url is required")}}
    {{- $_ := (.config.creds.secretName | required "config.creds.secretName is required")}}
    {{- if and .config.tls.cert.cert (not .config.tls.cert.key) }}
      {{- fail "config.tls.cert.key is required if cert is defined" }}
    {{- end }}
    {{- if and .config.tls.cert.key (not .config.tls.cert.cert) }}
      {{- fail "config.tls.cert.cert is required if key is defined" }}
    {{- end }}
  {{- end }}
{{- end }}

{{/*
nhg.labels
*/}}
{{- define "nhg.labels" -}}
{{- with .Values.global.labels -}}
{{ toYaml . }}
{{ end -}}
helm.sh/chart: {{ include "nhg.chart" . }}
{{ include "nhg.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
nhg.selector labels
*/}}
{{- define "nhg.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nhg.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: http-gateway
{{- end }}

{{/*
Print the image
*/}}
{{- define "nhg.image" }}
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
{{- define "nhg.env" -}}
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
{{- define "nhg.secretNames" -}}
{{- $secrets := list }}
  {{- $secrets = append $secrets (merge (dict "name" "creds") .Values.config.creds )}}
  {{- with .Values.config.tls.cert }}
    {{- if and .enabled .secretName }}
      {{- $secrets = append $secrets (merge (dict "name" "http-tls") .) }}
    {{- end }}
  {{- end }}
{{- toJson (dict "secretNames" $secrets) }}
{{- end }}

{{- define "nhg.tlsCAVolume" -}}
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

{{- define "nhg.tlsCAVolumeMount" -}}
{{- with .Values.config.tls.caCerts }}
{{- if and .enabled (or .configMapName .secretName) }}
- name: tls-ca
  mountPath: {{ .dir | quote }}
{{- end }}
{{- end }}
{{- end }}

{{- /*
nhg.loadMergePatch
input: map with 4 keys:
- file: name of file to load
- ctx: context to pass to tpl
- merge: interface{} to merge
- patch: []interface{} valid JSON Patch document
output: JSON encoded map with 1 key:
- doc: interface{} patched json result
*/}}
{{- define "nhg.loadMergePatch" -}}
{{- $doc := tpl (.ctx.Files.Get (printf "files/%s" .file)) .ctx | fromYaml | default dict -}}
{{- $doc = mergeOverwrite $doc (deepCopy (.merge | default dict)) -}}
{{- get (include "jsonpatch" (dict "doc" $doc "patch" (.patch | default list)) | fromJson ) "doc" | toYaml -}}
{{- end }}
