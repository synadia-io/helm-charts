{{/*
Chart name.
*/}}
{{- define "insights.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Fully qualified resource name.
*/}}
{{- define "insights.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := include "insights.name" . }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "insights.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "insights.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "insights.selectorLabels" -}}
app.kubernetes.io/name: {{ include "insights.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Required application config. Insights needs a system to observe: the embedded
simulator, a sys connection, or a systems list. An opaque existing Secret cannot
be inspected, so its contents remain the operator's responsibility.
*/}}
{{- define "insights.validateConfig" -}}
{{- if not .Values.configSecret.existingSecret }}
  {{- $config := default dict .Values.config }}
  {{- $simulator := default dict (get $config "simulator") }}
  {{- if not (or (get $simulator "enabled") (get $config "sys") (get $config "systems")) }}
    {{- fail "insights: config observes no NATS system. Set config.simulator.enabled, config.sys, or config.systems, or point configSecret.existingSecret at a complete config.yaml." }}
  {{- end }}
{{- end }}
{{- end }}

{{/*
Config Secret name.
*/}}
{{- define "insights.configSecretName" -}}
{{- default (printf "%s-config" (include "insights.fullname" .)) .Values.configSecret.existingSecret }}
{{- end }}

{{/*
Image name. An explicit repository is authoritative. Otherwise, the presence
of a license token or file in the application config selects the licensed image.
*/}}
{{- define "insights.image" -}}
{{- $repository := .Values.image.repository }}
{{- if not $repository }}
  {{- $licensed := false }}
  {{- with .Values.config.license }}
    {{- $licensed = or (not (empty .token)) (not (empty .file)) }}
  {{- end }}
  {{- $repository = ternary "insights-licensed" "insights" $licensed }}
{{- end }}
{{- $image := $repository }}
{{- with .Values.image.registry }}
  {{- $image = printf "%s/%s" (trimSuffix "/" .) $repository }}
{{- end }}
{{- printf "%s:%s" $image (default .Chart.AppVersion .Values.image.tag) }}
{{- end }}

{{/*
Web port from application config, falling back to the Service port.
*/}}
{{- define "insights.webPort" -}}
{{- $port := .Values.service.port }}
{{- if .Values.service.targetPort }}
  {{- $port = .Values.service.targetPort }}
{{- else if not .Values.configSecret.existingSecret }}
  {{- with .Values.config.web }}
    {{- $port = default $port .port }}
  {{- end }}
{{- end }}
{{- $port }}
{{- end }}

{{/*
Whether the managed application config enables the web server. An opaque
existing Secret cannot be inspected and therefore defaults to enabled.
*/}}
{{- define "insights.webEnabled" -}}
{{- $enabled := true }}
{{- if not .Values.configSecret.existingSecret }}
  {{- with .Values.config.web }}
    {{- if hasKey . "enabled" }}
      {{- $enabled = .enabled }}
    {{- end }}
  {{- end }}
{{- end }}
{{- $enabled }}
{{- end }}

{{/*
Data mount path from application config. An opaque existing config Secret uses
the explicit persistence.mountPath fallback.
*/}}
{{- define "insights.dataDir" -}}
{{- if .Values.configSecret.existingSecret }}
{{- .Values.persistence.mountPath }}
{{- else }}
{{- default .Values.persistence.mountPath (get .Values.config "data-dir") }}
{{- end }}
{{- end }}
