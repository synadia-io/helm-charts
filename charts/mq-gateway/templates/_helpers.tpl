{{/*
Chart name.
*/}}
{{- define "mq-gateway.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Fully qualified resource name.
*/}}
{{- define "mq-gateway.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := include "mq-gateway.name" . }}
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
{{- define "mq-gateway.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "mq-gateway.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "mq-gateway.selectorLabels" -}}
app.kubernetes.io/name: {{ include "mq-gateway.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
ServiceAccount name. A created account defaults to the release's fullname. An
account the operator manages has to be named, since the chart cannot guess it,
and falls back to the namespace default.
*/}}
{{- define "mq-gateway.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "mq-gateway.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Config Secret name.
*/}}
{{- define "mq-gateway.configSecretName" -}}
{{- default (printf "%s-config" (include "mq-gateway.fullname" .)) .Values.configSecret.existingSecret }}
{{- end }}

{{/*
The config file the chart writes: values.config with the NATS servers and the
files the chart mounts filled in. A key set both ways is refused rather than
one silently winning. Each replica takes its pod name as its gateway id, so a
fixed id, which every replica would share, is refused too.
*/}}
{{- define "mq-gateway.config" -}}
{{- $config := deepCopy (default dict .Values.config) }}
{{- if not (or (hasKey $config "mqtt") (hasKey $config "amqp")) }}
  {{- fail "mq-gateway: config serves no protocol. Add an mqtt or amqp block (even {}), or point configSecret.existingSecret at a complete config file." }}
{{- end }}
{{- if hasKey $config "id" }}
  {{- fail "mq-gateway: config.id is set, but replicas sharing a sessions bucket need distinct ids; each takes its pod name. Remove config.id." }}
{{- end }}
{{- $nats := deepCopy (default dict (get $config "nats")) }}
{{- if get $nats "embed" }}
  {{- fail "mq-gateway: config.nats.embed is not supported; set nats.url to a NATS server you run." }}
{{- end }}
{{- if hasKey $nats "url" }}
  {{- fail "mq-gateway: set the NATS servers with nats.url, not config.nats.url." }}
{{- end }}
{{- $url := .Values.nats.url }}
{{- if kindIs "slice" $url }}
  {{- $url = join "," $url }}
{{- end }}
{{- if not $url }}
  {{- fail "mq-gateway: nats.url is required: the NATS servers the gateway connects to." }}
{{- end }}
{{- $_ := set $nats "url" $url }}
{{- with .Values.nats.credentials.existingSecret }}
  {{- range $k := list "user" "password" "creds" "nkey" "jwt" "seed" "context" }}
    {{- if hasKey $nats $k }}
      {{- fail (printf "mq-gateway: nats.credentials.existingSecret and config.nats.%s are both set; the gateway takes one credential." $k) }}
    {{- end }}
  {{- end }}
  {{- $_ := set $nats "creds" "/etc/mq-gateway/nats/nats.creds" }}
{{- end }}
{{- with .Values.nats.tls }}
  {{- if .existingSecret }}
    {{- if hasKey $nats "tls" }}
      {{- fail "mq-gateway: nats.tls.existingSecret and config.nats.tls are both set." }}
    {{- end }}
    {{- $_ := set $nats "tls" (include "mq-gateway.tlsFiles" (dict "values" . "dir" "/etc/mq-gateway/nats-tls") | fromYaml) }}
  {{- end }}
{{- end }}
{{- $_ := set $config "nats" $nats }}
{{- if .Values.tls.existingSecret }}
  {{- if hasKey $config "tls" }}
    {{- fail "mq-gateway: tls.existingSecret and config.tls are both set." }}
  {{- end }}
  {{- $_ := set $config "tls" (include "mq-gateway.tlsFiles" (dict "values" .Values.tls "dir" "/etc/mq-gateway/tls") | fromYaml) }}
{{- end }}
{{- if .Values.license.existingSecret }}
  {{- if hasKey $config "license" }}
    {{- fail "mq-gateway: license.existingSecret and config.license are both set." }}
  {{- end }}
  {{- $_ := set $config "license" (dict "file" "/etc/mq-gateway/license/license.jwt") }}
{{- end }}
{{- toYaml $config }}
{{- end }}

{{/*
A tls block's cert, key and ca paths in dir, for the keys the values name.
*/}}
{{- define "mq-gateway.tlsFiles" -}}
{{- range $k := list "ca" "cert" "key" }}
{{- if get $.values $k }}
{{ $k }}: {{ printf "%s/%s" $.dir (get $.values $k) | quote }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Secret volume items for a tls block's keys, each under its own name.
*/}}
{{- define "mq-gateway.tlsItems" -}}
{{- range $k := list "ca" "cert" "key" }}
{{- with get $ $k }}
- key: {{ . | quote }}
  path: {{ . | quote }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Whether a license is configured, which selects the licensed image. An opaque
existing config Secret cannot be inspected.
*/}}
{{- define "mq-gateway.licensed" -}}
{{- $licensed := not (empty .Values.license.existingSecret) }}
{{- if not .Values.configSecret.existingSecret }}
  {{- with .Values.config.license }}
    {{- $licensed = or $licensed (not (empty .token)) (not (empty .file)) }}
  {{- end }}
{{- end }}
{{- $licensed }}
{{- end }}

{{/*
Image name. An explicit repository is authoritative. Otherwise a license
selects the licensed image.
*/}}
{{- define "mq-gateway.image" -}}
{{- $repository := .Values.image.repository }}
{{- if not $repository }}
  {{- $repository = ternary "mq-gateway-licensed" "mq-gateway" (eq (include "mq-gateway.licensed" .) "true") }}
{{- end }}
{{- $image := $repository }}
{{- with .Values.image.registry }}
  {{- $image = printf "%s/%s" (trimSuffix "/" .) $repository }}
{{- end }}
{{- printf "%s:%s" $image (default .Chart.AppVersion .Values.image.tag) }}
{{- end }}

{{/*
The listeners the pod and Service expose, as a YAML list of name and port:
read from the config's mqtt and amqp blocks, each of which listens on its
protocol's default port when it lists no listeners, or taken from
configSecret.listeners for an opaque config. A listener's name becomes its
port name, so it must be one Kubernetes accepts.
*/}}
{{- define "mq-gateway.listeners" -}}
{{- $listeners := list }}
{{- if .Values.configSecret.existingSecret }}
  {{- $listeners = .Values.configSecret.listeners }}
  {{- if not $listeners }}
    {{- fail "mq-gateway: configSecret.listeners is empty; name the listeners in the config Secret so the pod and Service expose them." }}
  {{- end }}
{{- else }}
  {{- range $protocol, $default := dict "mqtt" 1883 "amqp" 5672 }}
    {{- if hasKey $.Values.config $protocol }}
      {{- $block := default dict (get $.Values.config $protocol) }}
      {{- $defined := default (list (dict "addr" (printf ":%d" $default))) (get $block "listeners") }}
      {{- range $i, $l := $defined }}
        {{- $port := regexFind "[0-9]+$" (toString (get $l "addr")) }}
        {{- if not $port }}
          {{- fail (printf "mq-gateway: config.%s.listeners[%d].addr %q names no port." $protocol $i (toString (get $l "addr"))) }}
        {{- end }}
        {{- $name := get $l "name" | default (ternary $protocol (printf "%s-%d" $protocol $i) (eq $i 0)) }}
        {{- $listeners = append $listeners (dict "name" $name "port" (atoi $port)) }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
{{- $seen := dict "monitor" true }}
{{- range $listeners }}
  {{- if or (gt (len .name) 15) (not (regexMatch "^[a-z0-9]([a-z0-9-]*[a-z0-9])?$" .name)) (not (regexMatch "[a-z]" .name)) }}
    {{- fail (printf "mq-gateway: listener name %q is not a valid Kubernetes port name: at most 15 lowercase letters, digits and '-', with a letter." .name) }}
  {{- end }}
  {{- if hasKey $seen .name }}
    {{- fail (printf "mq-gateway: listener name %q is used twice (\"monitor\" is the health port)." .name) }}
  {{- end }}
  {{- $_ := set $seen .name true }}
{{- end }}
{{- toYaml $listeners }}
{{- end }}

{{/*
Health endpoint port, from config.monitor.addr, or configSecret.monitorPort
for an opaque config.
*/}}
{{- define "mq-gateway.monitorPort" -}}
{{- if .Values.configSecret.existingSecret }}
{{- .Values.configSecret.monitorPort }}
{{- else }}
{{- $addr := "" }}
{{- with .Values.config.monitor }}
  {{- $addr = toString (get . "addr") }}
{{- end }}
{{- $port := regexFind "[0-9]+$" $addr }}
{{- if not $port }}
  {{- fail "mq-gateway: config.monitor.addr is required: the probes use its /livez and /readyz." }}
{{- end }}
{{- $port }}
{{- end }}
{{- end }}

{{/*
Container env from a map keyed by variable name. A string value becomes a
literal, a map is spliced in whole so valueFrom and friends work without the
chart enumerating them.
*/}}
{{- define "mq-gateway.env" -}}
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
Pod affinity: values.affinity, with the podAntiAffinity preset added to keep
replicas on different nodes. A preset beside an explicit
affinity.podAntiAffinity is refused rather than one silently replacing the
other.
*/}}
{{- define "mq-gateway.affinity" -}}
{{- $affinity := deepCopy (default dict .Values.affinity) }}
{{- with .Values.podAntiAffinity }}
  {{- if hasKey $affinity "podAntiAffinity" }}
    {{- fail "mq-gateway: podAntiAffinity and affinity.podAntiAffinity are both set; set podAntiAffinity to \"\" to use your own." }}
  {{- end }}
  {{- $term := dict "topologyKey" "kubernetes.io/hostname" "labelSelector" (dict "matchLabels" (include "mq-gateway.selectorLabels" $ | fromYaml)) }}
  {{- if eq . "soft" }}
    {{- $_ := set $affinity "podAntiAffinity" (dict "preferredDuringSchedulingIgnoredDuringExecution" (list (dict "weight" 100 "podAffinityTerm" $term))) }}
  {{- else if eq . "hard" }}
    {{- $_ := set $affinity "podAntiAffinity" (dict "requiredDuringSchedulingIgnoredDuringExecution" (list $term)) }}
  {{- else }}
    {{- fail (printf "mq-gateway: podAntiAffinity is %q; want soft, hard or \"\"." .) }}
  {{- end }}
{{- end }}
{{- toYaml $affinity }}
{{- end }}

{{/*
The pod's termination grace period: an explicit value, or the lame duck
duration plus 20s for the drain to finish and the gateway to exit. The
duration is read as Go parses it ("2m", "1m30s"); an opaque config Secret
cannot be read, so it is taken as the default.
*/}}
{{- define "mq-gateway.terminationGracePeriodSeconds" -}}
{{- $set := .Values.terminationGracePeriodSeconds }}
{{- if not (or (kindIs "invalid" $set) (kindIs "string" $set)) }}
{{- int $set }}
{{- else if $set }}
{{- fail (printf "mq-gateway: terminationGracePeriodSeconds is %q; want a number of seconds, or empty to follow ldm.duration." $set) }}
{{- else }}
{{- $duration := "2m" }}
{{- if not .Values.configSecret.existingSecret }}
  {{- with .Values.config.ldm }}
    {{- with get . "duration" }}{{ $duration = toString . }}{{ end }}
  {{- end }}
{{- end }}
{{- $number := "([0-9]+(\\.[0-9]*)?|\\.[0-9]+)" }}
{{- $unit := printf "%s(h|ms|m|s|us|µs|ns)" $number }}
{{- if not (regexMatch (printf "^\\+?(%s)+$" $unit) $duration) }}
  {{- fail (printf "mq-gateway: config.ldm.duration %q is not a duration such as 2m or 90s." $duration) }}
{{- end }}
{{- $seconds := dict "h" 3600.0 "m" 60.0 "s" 1.0 "ms" 0.001 "us" 0.000001 "µs" 0.000001 "ns" 0.000000001 }}
{{- $total := 0.0 }}
{{- range regexFindAll $unit $duration -1 }}
  {{- $n := regexFind (printf "^%s" $number) . }}
  {{- $total = addf $total (mulf (float64 $n) (get $seconds (trimPrefix $n .))) }}
{{- end }}
{{- add (int (ceil $total)) 20 }}
{{- end }}
{{- end }}
