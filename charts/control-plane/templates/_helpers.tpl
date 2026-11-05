{{/*
Expand the name of the chart.
*/}}
{{- define "scp.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "scp.fullname" -}}
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
{{- define "scp.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Set default values.
*/}}
{{- define "scp.defaultValues" }}
{{- if not .defaultValuesSet }}
  {{- $name := include "scp.fullname" . }}
  {{- with .Values }}
    {{- $_ := set .configSecret    "name" (.configSecret.name    | default (printf "%s-config" $name)) }}
    {{- $_ := set .headlessService "name" (.headlessService.name | default (printf "%s-headless" $name)) }}
    {{- $_ := set .imagePullSecret "name" (.imagePullSecret.name | default (printf "%s-regcred" $name)) }}
    {{- $_ := set .ingress         "name" (.ingress.name         | default $name) }}
    {{- $_ := set .service         "name" (.service.name         | default $name) }}
    {{- $_ := set .serviceAccount  "name" (.serviceAccount.name  | default $name) }}
    {{- $_ := set .statefulSet     "name" (.statefulSet.name     | default $name) }}
  {{- end }}

  {{- /* chart 1.x used a Deployment, upgrading in place would delete its PVCs and the data on them */}}
  {{- range (lookup "apps/v1" "Deployment" .Release.Namespace "").items }}
    {{- $labels := .metadata.labels | default dict }}
    {{- if and (eq (get $labels "app.kubernetes.io/instance") $.Release.Name) (eq (get $labels "app.kubernetes.io/component") "control-plane") }}
      {{- fail (printf "Deployment %s is from control-plane chart 1.x. Upgrading this release to 2.x would delete its PVCs and data. Pin --version 1.x for this release." .metadata.name) }}
    {{- end }}
  {{- end }}

  {{- $values := get (include "tplYaml" (dict "doc" .Values "ctx" $) | fromJson) "doc" }}
  {{- $_ := set . "Values" $values }}

  {{- range $k, $v := .Values.config.kms.rotatedKeys }}
    {{- $_ := set $v "dir" ($v.dir | default (printf "/etc/syn-cp/kms/rotated-key-%d" $k)) }}
    {{- $_ := set $v "key" ($v.key | default "key.enc") }}
  {{- end }}

  {{- with .Values.config }}
    {{- $config := include "scp.loadMergePatch" (merge (dict "file" "config/config.yaml" "ctx" $) .) | fromYaml }}
    {{- $_ := set $ "config" $config }}
  {{- end }}

  {{- $_ := set . "defaultValuesSet" true }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "scp.labels" -}}
{{- with .Values.global.labels -}}
{{ toYaml . }}
{{ end -}}
helm.sh/chart: {{ include "scp.chart" . }}
{{ include "scp.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "scp.selectorLabels" -}}
app.kubernetes.io/name: {{ include "scp.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: control-plane
{{- end }}

{{/*
Print the image
*/}}
{{- define "scp.image" }}
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
List of external secretNames
*/}}
{{- define "scp.secretNames" -}}
{{- $secrets := list }}
  {{- with .Values.config }}
    {{- with .server.tls }}
      {{- if and .enabled .secretName }}
        {{- $secrets = append $secrets (merge (dict "name" "server-tls") .) }}
      {{- end }}
    {{- end }}
    {{- with .kms }}
      {{- with .key }}
        {{- if .secretName }}
          {{- $secrets = append $secrets (merge (dict "name" "kms-key") .) }}
        {{- end }}
      {{- end }}
      {{- range $k, $v := .rotatedKeys }}
        {{- if $v.secretName }}
          {{- $secrets = append $secrets (merge (dict "name" (printf "kms-rotated-key-%d" $k)) $v) }}
        {{- end }}
      {{- end }}
    {{- end }}
    {{- with .dataSources.prometheus.tls }}
      {{- if and .enabled .secretName }}
        {{- $secrets = append $secrets (merge (dict "name" "prometheus-tls") .) }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- toJson (dict "secretNames" $secrets) }}
{{- end }}

{{/*
Translates env var map to list
*/}}
{{- define "scp.env" -}}
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
scp.loadMergePatch
input: map with 4 keys:
- file: name of file to load
- ctx: context to pass to tpl
- merge: interface{} to merge
- patch: []interface{} valid JSON Patch document
output: JSON encoded map with 1 key:
- doc: interface{} patched json result
*/}}
{{- define "scp.loadMergePatch" -}}
{{- $doc := tpl (.ctx.Files.Get (printf "files/%s" .file)) .ctx | fromYaml | default dict -}}
{{- $doc = mergeOverwrite $doc (deepCopy (.merge | default dict)) -}}
{{- get (include "jsonpatch" (dict "doc" $doc "patch" (.patch | default list)) | fromJson ) "doc" | toYaml -}}
{{- end }}
