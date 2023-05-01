{{/*
Expand the name of the chart.
*/}}
{{- define "cp.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "cp.fullname" -}}
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
{{- define "cp.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Set default values.
*/}}
{{- define "cp.defaultValues" }}
{{- if not .defaultValuesSet }}
{{- $name := include "cp.fullname" . }}
{{- with .Values }}
{{- $_ := set .configSecret   "name" (.configSecret.name   | default (printf "%s-config" $name)) }}
{{- $_ := set .deployment              "name" (.deployment.name              | default (printf "%s" $name)) }}
{{- $_ := set .config.websocket.ingress         "name" (.config.websocket.ingress.name         | default (printf "%s-ws" $name)) }}
{{- $_ := set .configMap                        "name" (.configMap.name                        | default (printf "%s-config" $name)) }}
{{- $_ := set .headlessService                  "name" (.headlessService.name                  | default (printf "%s-headless" $name)) }}
{{- $_ := set .natsBox.contentsSecret           "name" (.natsBox.contentsSecret.name           | default (printf "%s-box-contents" $name)) }}
{{- $_ := set .natsBox.contextsSecret           "name" (.natsBox.contextsSecret.name           | default (printf "%s-box-contexts" $name)) }}
{{- $_ := set .natsBox.deployment               "name" (.natsBox.deployment.name               | default (printf "%s-box" $name)) }}
{{- $_ := set .natsBox.serviceAccount           "name" (.natsBox.serviceAccount.name           | default (printf "%s-box" $name)) }}
{{- $_ := set .service                          "name" (.service.name                          | default $name) }}
{{- $_ := set .serviceAccount                   "name" (.serviceAccount.name                   | default $name) }}
{{- $_ := set .statefulSet                      "name" (.statefulSet.name                      | default $name) }}
{{- $_ := set .promExporter.podMonitor          "name" (.promExporter.podMonitor.name          | default $name) }}
{{- end }}
{{- $values := get (include "tplYaml" (dict "doc" .Values "ctx" $) | fromJson) "doc" }}
{{- $_ := set . "Values" $values }}
{{- with .Values.config }}
{{- $config := include "cp.loadMergePatch" (merge (dict "file" "config/config.yaml" "ctx" $) .) | fromYaml }}
{{- $_ := set $ "config" $config }}
{{- end }}
{{- $_ := set . "defaultValuesSet" true }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "cp.labels" -}}
helm.sh/chart: {{ include "cp.chart" . }}
{{ include "cp.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "cp.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cp.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "cp.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "cp.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Define Image Pull Secret List
*/}}
{{- define "cp.imagePullSecrets" -}}
    {{ $imagePullSecrets := .Values.imagePullSecrets }}
    {{- if and .Values.imageCredentials.username .Values.imageCredentials.password }}
        {{- $imagePullSecrets = append $imagePullSecrets (dict "name" (printf "%s-regcred" (include "cp.fullname" .))) }}
    {{- end }}
    {{- toJson (dict "imagePullSecrets" $imagePullSecrets) }}
{{- end }}

{{/*
Define JSON string of the Helix configuration
*/}}
{{- define "cp.config" -}}
    {{- if .Values.helix.configFile -}}
        {{- fromJson .Values.helix.configFile | toJson -}}
    {{- else }}
        {{- toJson .Values.helix.config }}
    {{- end }}
{{- end }}

{{/*
Define JSON string of the Helix configuration
*/}}
{{- define "cp.secrets" -}}
    {{- if .Values.helix.secretsFile -}}
        {{- fromJson .Values.helix.secretsFile | toJson -}}
    {{- else }}
        {{- toJson .Values.helix.secrets }}
    {{- end }}
{{- end }}

{{/*
Define JSON string of Helix secret names
*/}}
{{- define "cp.secretNames" -}}
    {{- $secretNames := dict }}
    {{- $config := include "cp.config" . | fromJson }}
    {{- $secrets := include "cp.secrets" . | fromJson }}
    {{- if $secrets }}
        {{- range $name, $system := (get $secrets "nats_systems") }}
            {{- if kindIs "string" $system }}
                {{- $secretNames = merge $secretNames (dict $name $system) }}
            {{- else }}
                {{- $secretNames = merge $secretNames (dict $name (printf "helix-system-%s" $name)) }}
            {{- end }}
        {{- end }}
    {{- end }}
    {{- toJson $secretNames }}
{{- end }}

{{/*
Check if using default encryption key
*/}}
{{- define "cp.defaultEncryptionKey" -}}
    {{- if or (not (hasKey .config "encryption_key")) (eq (get .config "encryption_key") "") }}
    {{- true }}
    {{- end }}
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

{{/*
translates env var map to list
*/}}
{{- define "cp.env" -}}
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
cp.loadMergePatch
input: map with 4 keys:
- file: name of file to load
- ctx: context to pass to tpl
- merge: interface{} to merge
- patch: []interface{} valid JSON Patch document
output: JSON encoded map with 1 key:
- doc: interface{} patched json result
*/}}
{{- define "cp.loadMergePatch" -}}
{{- $doc := tpl (.ctx.Files.Get (printf "files/%s" .file)) .ctx | fromYaml -}}
{{- $doc = mergeOverwrite $doc (deepCopy .merge) -}}
{{- get (include "jsonpatch" (dict "doc" $doc "patch" .patch) | fromJson ) "doc" | toYaml -}}
{{- end }}
