{{/*
Expand the name of the chart.
*/}}
{{- define "helix.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "helix.fullname" -}}
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
{{- define "helix.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "helix.labels" -}}
helm.sh/chart: {{ include "helix.chart" . }}
{{ include "helix.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "helix.selectorLabels" -}}
app.kubernetes.io/name: {{ include "helix.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "helix.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "helix.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Define Image Pull Secret List
*/}}
{{- define "helix.imagePullSecrets" -}}
    {{ $imagePullSecrets := .Values.imagePullSecrets }}
    {{- if and .Values.imageCredentials.username .Values.imageCredentials.password }}
        {{- $imagePullSecrets = append $imagePullSecrets (dict "name" (printf "%s-regcred" (include "helix.fullname" .))) }}
    {{- end }}
    {{- toJson (dict "imagePullSecrets" $imagePullSecrets) }}
{{- end }}

{{/*
Define JSON string of the Helix configuration
*/}}
{{- define "helix.config" -}}
    {{- if .Values.helix.configFile -}}
        {{- fromJson .Values.helix.configFile | toJson -}}
    {{- else }}
        {{- toJson .Values.helix.config }}
    {{- end }}
{{- end }}

{{/*
Define JSON string of the Helix configuration
*/}}
{{- define "helix.secrets" -}}
    {{- if .Values.helix.secretsFile -}}
        {{- fromJson .Values.helix.secretsFile | toJson -}}
    {{- else }}
        {{- toJson .Values.helix.secrets }}
    {{- end }}
{{- end }}

{{/*
Define JSON string of Helix secret names
*/}}
{{- define "helix.secretNames" -}}
    {{- $secretNames := dict }}
    {{- $config := include "helix.config" . | fromJson }}
    {{- $secrets := include "helix.secrets" . | fromJson }}
    {{- if $secrets }}
        {{- range $name, $system := (get $secrets "nats_systems") }}
            {{- if kindIs "string" $system }}
                {{- $secretNames = merge $secretNames (dict $name $system) }}
            {{- else }}
                {{- $secretNames = merge $secretNames (dict $name (printf "helix-%s" $name)) }}
            {{- end }}
        {{- end }}
    {{- end }}
    {{- toJson $secretNames }}
{{- end }}

{{/*
Define a Registry Credential Secret
*/}}
{{- define "imagePullSecret" }}
{{- with .Values.imageCredentials }}
{{- $auth := dict "username" .username "password" .password "auth" (printf "%s:%s" .username .password | b64enc) }}
{{- if .email }}
  {{- $auth = merge $auth (dict "email" .email) }}
{{- end }}
{{- $auths := dict .registry $auth }}
{{- printf "{\"auths\":%s}" (toJson $auths) | b64enc }}
{{- end }}
{{- end }}