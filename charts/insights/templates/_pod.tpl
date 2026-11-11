{{/*
Pod template shared by the serve StatefulSet and the Deployment that runs a
stateless subcommand. Only serve owns a data directory, so only it mounts the
data volume.
*/}}
{{- define "insights.podTemplate" -}}
{{- $serve := eq .Values.subcommand "serve" -}}
metadata:
  labels:
    {{- include "insights.selectorLabels" . | nindent 4 }}
  {{- if or (not .Values.configSecret.existingSecret) .Values.podAnnotations }}
  annotations:
    {{- if not .Values.configSecret.existingSecret }}
    checksum/config: {{ include (print $.Template.BasePath "/secret.yaml") . | sha256sum }}
    {{- end }}
    {{- with .Values.podAnnotations }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
  {{- end }}
spec:
  serviceAccountName: {{ include "insights.serviceAccountName" . }}
  automountServiceAccountToken: false
  enableServiceLinks: false
  {{- with .Values.image.pullSecrets }}
  imagePullSecrets:
    {{- toYaml . | nindent 4 }}
  {{- end }}
  {{- with .Values.podSecurityContext }}
  securityContext:
    {{- toYaml . | nindent 4 }}
  {{- end }}
  containers:
    - name: insights
      image: {{ include "insights.image" . | quote }}
      imagePullPolicy: {{ .Values.image.pullPolicy }}
      args:
        - {{ .Values.subcommand }}
        - --config
        - /etc/insights/config.yaml
        {{- range .Values.extraArgs }}
        - {{ . | quote }}
        {{- end }}
      {{- if or .Values.sessionSeed.existingSecret .Values.extraEnv }}
      env:
        {{- with .Values.sessionSeed.existingSecret }}
        - name: INSIGHTS_WEB_SESSION_SEED
          valueFrom:
            secretKeyRef:
              name: {{ . }}
              key: {{ $.Values.sessionSeed.key }}
        {{- end }}
        {{- with .Values.extraEnv }}
        {{- include "insights.env" . | trim | nindent 8 }}
        {{- end }}
      {{- end }}
      {{- with .Values.securityContext }}
      securityContext:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      ports:
        - name: http
          containerPort: {{ include "insights.port" . }}
          protocol: TCP
      {{- if or (not $serve) (eq (include "insights.webEnabled" .) "true") }}
      {{- range $name := list "startupProbe" "livenessProbe" "readinessProbe" }}
      {{- with get $.Values $name }}
      {{ $name }}:
        {{- include "insights.probe" (dict "probe" . "subcommand" $.Values.subcommand) | nindent 8 }}
      {{- end }}
      {{- end }}
      {{- end }}
      {{- with .Values.resources }}
      resources:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      volumeMounts:
        - name: config
          mountPath: /etc/insights
          readOnly: true
        {{- if $serve }}
        - name: data
          mountPath: {{ include "insights.dataDir" . | quote }}
        {{- end }}
        {{- with .Values.extraVolumeMounts }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
  volumes:
    - name: config
      secret:
        secretName: {{ include "insights.configSecretName" . }}
        items:
          - key: {{ .Values.configSecret.key | quote }}
            path: config.yaml
    {{- if $serve }}
    {{- if .Values.persistence.existingClaim }}
    - name: data
      persistentVolumeClaim:
        claimName: {{ .Values.persistence.existingClaim }}
    {{- else if not .Values.persistence.enabled }}
    - name: data
      emptyDir: {}
    {{- end }}
    {{- end }}
    {{- with .Values.extraVolumes }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
{{- end }}
