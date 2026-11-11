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
Refuses values that render a pod the selected subcommand cannot start or serve
in. serve needs a system to observe and owns a ReadWriteOnce database, so it
runs alone. The other subcommands are clients of an Insights instance and need
a NATS connection to reach it; http also refuses to start without a bearer
token, mcp over stdio serves only a client that spawns it, and mcp sessions
live in one process. An opaque existing Secret cannot be inspected, so its
contents remain the operator's responsibility.
*/}}
{{- define "insights.validate" -}}
{{- $sub := .Values.subcommand }}
{{- if not (has $sub (list "serve" "web" "http" "mcp" "prometheus")) }}
  {{- fail (printf "insights: subcommand %q does not run as a service. Use serve, web, http, mcp, or prometheus." (toString $sub)) }}
{{- end }}
{{- if and (eq $sub "serve") (ne (int .Values.replicaCount) 1) }}
  {{- fail "insights: serve owns its database on a ReadWriteOnce volume and runs as one replica. replicaCount applies to the stateless subcommands." }}
{{- end }}
{{- if and (eq $sub "mcp") (ne (int .Values.replicaCount) 1) }}
  {{- fail "insights: mcp keeps each client session in process memory, so a request routed to another replica fails with session not found. Run one replica." }}
{{- end }}
{{- if not .Values.configSecret.existingSecret }}
  {{- $config := fromYaml (include "insights.config" .) }}
  {{- if eq $sub "serve" }}
    {{- $simulator := default dict (get $config "simulator") }}
    {{- if not (or (get $simulator "enabled") (get $config "sys") (get $config "systems")) }}
      {{- fail "insights: config observes no NATS system. Set config.simulator.enabled, config.sys, or config.systems, or point configSecret.existingSecret at a complete config.yaml." }}
    {{- end }}
  {{- else }}
    {{- $server := include "insights.isSet" (dict "config" $config "path" (list "nats" "server") "env" "INSIGHTS_NATS_SERVER" "flags" (list "server" "nats.server") "Values" .Values) }}
    {{- $context := include "insights.isSet" (dict "config" $config "path" (list "nats" "context") "env" "INSIGHTS_NATS_CONTEXT" "flags" (list "context" "nats.context") "Values" .Values) }}
    {{- if not (or $server $context) }}
      {{- fail (printf "insights: %s reaches Insights over NATS and has no connection. Set config.nats.server, INSIGHTS_NATS_SERVER in extraEnv, or --server in extraArgs." $sub) }}
    {{- end }}
  {{- end }}
  {{- if eq $sub "http" }}
    {{- $token := include "insights.isSet" (dict "config" $config "path" (list "http" "auth-token") "env" "INSIGHTS_HTTP_AUTH_TOKEN" "flags" (list "auth-token") "Values" .Values) }}
    {{- $open := include "insights.isSet" (dict "config" $config "path" (list "http" "allow-unauthenticated") "env" "INSIGHTS_HTTP_ALLOW_UNAUTHENTICATED" "flags" (list "allow-unauthenticated") "Values" .Values) }}
    {{- if not (or $token $open) }}
      {{- fail "insights: http refuses to start without a bearer token. Set INSIGHTS_HTTP_AUTH_TOKEN in extraEnv from a Secret, or config.http.allow-unauthenticated for a network-isolated gateway." }}
    {{- end }}
  {{- end }}
  {{- if eq $sub "mcp" }}
    {{- /* Resolved as Insights does: a flag outranks env, which outranks config. */}}
    {{- $transport := toString $config.mcp.transport }}
    {{- $env := default dict .Values.extraEnv }}
    {{- if hasKey $env "INSIGHTS_MCP_TRANSPORT" }}
      {{- $e := $env.INSIGHTS_MCP_TRANSPORT }}
      {{- if kindIs "string" $e }}
        {{- $transport = $e }}
      {{- else if and (kindIs "map" $e) (hasKey $e "value") }}
        {{- $transport = toString $e.value }}
      {{- end }}
    {{- end }}
    {{- $args := default list .Values.extraArgs }}
    {{- range $i, $arg := $args }}
      {{- if hasPrefix "--transport=" (toString $arg) }}
        {{- $transport = trimPrefix "--transport=" (toString $arg) }}
      {{- else if and (eq (toString $arg) "--transport") (lt (add1 $i) (len $args)) }}
        {{- $transport = toString (index $args (add1 $i)) }}
      {{- end }}
    {{- end }}
    {{- if ne $transport "http" }}
      {{- fail "insights: mcp in a pod must use the http transport; stdio serves only a client that spawns the process. Leave the transport unset or set it to http." }}
    {{- end }}
  {{- end }}
{{- end }}
{{- end }}

{{/*
Whether a setting reaches the process from any of its three sources: the
application config, an extraEnv variable, or an extraArgs flag. Renders "true"
or nothing.
*/}}
{{- define "insights.isSet" -}}
{{- $v := .config }}
{{- range .path }}
  {{- if kindIs "map" $v }}{{ $v = get $v . }}{{ else }}{{ $v = "" }}{{ end }}
{{- end }}
{{- $set := not (empty $v) }}
{{- if hasKey (default dict .Values.extraEnv) .env }}{{ $set = true }}{{ end }}
{{- range $arg := .Values.extraArgs }}
  {{- range $flag := $.flags }}
    {{- if or (eq (toString $arg) (printf "--%s" $flag)) (hasPrefix (printf "--%s=" $flag) (toString $arg)) }}{{ $set = true }}{{ end }}
  {{- end }}
{{- end }}
{{- if $set }}true{{ end }}
{{- end }}

{{/*
Application config as written to config.yaml. Every subcommand binds loopback
by default, which nothing outside the pod can reach, so the chart fills in a
pod-network listener for the client subcommands where config leaves it unset.
serve and web take theirs from config.web in values.yaml. mcp also defaults to
its http transport, the only one that serves a remote client.
*/}}
{{- define "insights.config" -}}
{{- $defaults := dict }}
{{- if eq .Values.subcommand "http" }}
  {{- $defaults = dict "http" (dict "listen" "0.0.0.0:8080") }}
{{- else if eq .Values.subcommand "mcp" }}
  {{- $defaults = dict "mcp" (dict "transport" "http" "http" "0.0.0.0:8090") }}
{{- else if eq .Values.subcommand "prometheus" }}
  {{- $defaults = dict "prometheus" (dict "hostname" "0.0.0.0") }}
{{- end }}
{{- toYaml (merge (deepCopy (default dict .Values.config)) $defaults) }}
{{- end }}

{{/*
ServiceAccount name. A created account defaults to the release's fullname. An
account the operator manages has to be named, since the chart cannot guess it,
and falls back to the namespace default.
*/}}
{{- define "insights.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "insights.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
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
Port the subcommand listens on, read from the application config it binds
from, falling back to the Service port.
*/}}
{{- define "insights.port" -}}
{{- $port := .Values.service.port }}
{{- if .Values.service.targetPort }}
  {{- $port = .Values.service.targetPort }}
{{- else if not .Values.configSecret.existingSecret }}
  {{- $config := fromYaml (include "insights.config" .) }}
  {{- $sub := .Values.subcommand }}
  {{- if has $sub (list "serve" "web") }}
    {{- with $config.web }}
      {{- $port = default $port .port }}
    {{- end }}
  {{- else if eq $sub "http" }}
    {{- $port = $config.http.listen | toString | splitList ":" | last }}
  {{- else if eq $sub "mcp" }}
    {{- $port = $config.mcp.http | toString | splitList ":" | last }}
  {{- else if eq $sub "prometheus" }}
    {{- $port = default 9091 $config.prometheus.port }}
  {{- end }}
{{- end }}
{{- $port }}
{{- end }}

{{/*
A probe as configured, except that mcp and prometheus serve no /healthz: the
default /healthz check becomes a TCP check on the same port for them. A probe
the operator wrote with another handler is left alone.
*/}}
{{- define "insights.probe" -}}
{{- $probe := .probe }}
{{- if and (has .subcommand (list "mcp" "prometheus")) $probe.httpGet (eq (toString $probe.httpGet.path) "/healthz") }}
  {{- $probe = merge (dict "tcpSocket" (dict "port" $probe.httpGet.port)) (omit $probe "httpGet") }}
{{- end }}
{{- toYaml $probe }}
{{- end }}

{{/*
Whether serve's managed application config enables the web server. An opaque
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
Container env from a map keyed by variable name. A string value becomes a
literal, a map is spliced in whole so valueFrom and friends work without the
chart enumerating them.
*/}}
{{- define "insights.env" -}}
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
