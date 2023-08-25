#!/bin/bash

## Tweaks upstream NATS chart to suit Synadia Server deployment

VALUES_YAML="values.yaml"
TEMP_FILE="tmp.yaml"

IMAGE="synadia-server"
REGISTRY="registry.helix-dev.synadia.io"

# Versions
. $(dirname "$0")/VERSIONS

SED_I="sed -i"
if [[ $(uname) == "Darwin" ]]; then
  SED_I='sed -i.macos.bak'
fi

## Control Plane Values
IFS= read -r -d '' OPTS << 'EOF'
################################################################################
# Control Plane options
################################################################################
controlPlane:
  # external URL for Synadia Control Plane
  url:
  # system registration token
  token:
  tokenSecret:
    # merge or patch the context secret
    # https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secret-v1-core
    merge: {}
    patch: []
    # defaults to "{{ include "nats.fullname" $ }}-control-plane-token"
    name:
    # set secretName in order to mount an existing secret to dir
    secretName:

EOF

## Token Secret Name Helper
IFS= read -r -d '' HELPER << 'EOF'

{{- define "controlPlane.secretName" -}}
{{- $name := include "nats.fullname" . }}
{{- $secretName := "" }}
{{- if .Values.controlPlane.token }}
  {{- if not .Values.controlPlane.tokenSecret.name }}
    {{- $secretName = printf "%s-control-plane-token" $name }}
  {{- else }}
    {{- $secretName = .Values.controlPlane.tokenSecret.name }}
  {{- end }}
{{- else if .Values.controlPlane.tokenSecret.secretName }}
  {{- $secretName = .Values.controlPlane.tokenSecret.secretName }}
{{- end }}
{{- $secretName }}
{{- end }}
EOF

## Token Secret Template
IFS= read -r -d '' SECRET_TMPL << 'EOF'
  # control-plane token
  {{- if $name := (include "controlPlane.secretName" $) }}
  - name: token
    secret:
      secretName: {{ $name | quote }}
  {{- end }}
EOF

## Synadia Server Extra Args
IFS= read -r -d '' ARGS << 'EOF'
{{- if .Values.controlPlane.url }}
- --url
- {{ .Values.controlPlane.url }}
{{- end }}
{{- if (include "controlPlane.secretName" $) }}
- --token-file
- /etc/synadia-server/token
{{- end }}
EOF

## Synadia Server Lame Duck Signal
IFS= read -r -d '' LDM << 'EOF'
      - synadia-server
      - signal
      - -P /var/run/nats/nats.pid
      - ldm
EOF

## Token Volume Mount
IFS= read -r -d '' MOUNT << 'EOF'
# control plane token
{{- if (include "controlPlane.secretName" $) }}
- name: token
  mountPath: /etc/synadia-server
{{- end }}
EOF

# Set after read statements due to expected 1 return codes
set -e

########################################
########################################
## Add Control Plane Options
FILE="${VALUES_YAML}"
MATCH_TEXT='NATS Stateful Set'
MATCH_LINE=$(grep -m 1 -n "${MATCH_TEXT}" "${FILE}" | cut -d ':' -f 1)
INSERT_ABOVE_LINE=$(( ${MATCH_LINE} - 2 ))

if [[ -n "${INSERT_ABOVE_LINE}" ]]; then
  { head -n ${INSERT_ABOVE_LINE} "${FILE}"; printf "%s" "${OPTS}"; tail -n +$(( ${INSERT_ABOVE_LINE} + 1 )) "${FILE}"; } > "${TEMP_FILE}"
  mv "${TEMP_FILE}" "${FILE}"
fi

########################################
########################################
## Disable Reloader
FILE="${VALUES_YAML}"
MATCH_TEXT="reloader container"
cp "${FILE}" "${TEMP_FILE}"

BLOCK_START_LINE=$(awk -v match_text="${MATCH_TEXT}" '/^[#]+$/ { delimiter = NR; getline; if ($0 ~ match_text "$") { getline; if (/^[#]+$/) line = delimiter } } END { print line }' "${TEMP_FILE}")

if [[ -n "${BLOCK_START_LINE}" ]]; then
  FOLLOWING_ENABLE_LINE=$(awk -v start_line="${BLOCK_START_LINE}" 'NR > start_line && /enabled:.*/ { print NR; exit }' "${TEMP_FILE}")

  ${SED_I} "${FOLLOWING_ENABLE_LINE}s/enabled:.*/enabled: false/" "${TEMP_FILE}"
  mv "${TEMP_FILE}" "${FILE}"
fi

########################################
########################################
## Disable Prometheus Exporter
FILE="${VALUES_YAML}"
MATCH_TEXT="prom-exporter container"
cp "${FILE}" "${TEMP_FILE}"

BLOCK_START_LINE=$(awk -v match_text="${MATCH_TEXT}" '/^[#]+$/ { delimiter = NR; getline; if ($0 ~ match_text "$") { getline; if (/^[#]+$/) line = delimiter } } END { print line }' "${TEMP_FILE}")

if [[ -n "${BLOCK_START_LINE}" ]]; then
  FOLLOWING_ENABLE_LINE=$(awk -v start_line="${BLOCK_START_LINE}" 'NR > start_line && /enabled:.*/ { print NR; exit }' "${TEMP_FILE}")

  ${SED_I} "${FOLLOWING_ENABLE_LINE}s/enabled:.*/enabled: false/" "${TEMP_FILE}"
  mv "${TEMP_FILE}" "${FILE}"
fi

########################################
########################################
## Replace NATS image
FILE="${VALUES_YAML}"
MATCH_TEXT="repository: nats"
cp "${FILE}" "${TEMP_FILE}"

MATCH_LINE=$(grep -m 1 -n "${MATCH_TEXT}" "${TEMP_FILE}" | cut -d ':' -f 1)
FOLLOWING_TAG_LINE=$(awk -v start_line="${MATCH_LINE}" 'NR > start_line && /tag:.*/ { print NR; exit }' "${TEMP_FILE}")
FOLLOWING_REGISTRY_LINE=$(awk -v start_line="${MATCH_LINE}" 'NR > start_line && /registry:.*/ { print NR; exit }' "${TEMP_FILE}")

${SED_I} "${MATCH_LINE}s/repository.*/repository: ${IMAGE}/" "${TEMP_FILE}"
${SED_I} "${FOLLOWING_TAG_LINE}s/tag:.*/tag: ${TAG}/" "${TEMP_FILE}"
${SED_I} "${FOLLOWING_REGISTRY_LINE}s/registry:.*/registry: ${REGISTRY}/" "${TEMP_FILE}"
mv "${TEMP_FILE}" "${VALUES_YAML}"

########################################
########################################
## Add Control Plane Helper Function
FILE="templates/_helpers.tpl"
printf "%s" "${HELPER}" >> ${FILE}

########################################
########################################
## Add Control Plane Secret to Pod Template
FILE="files/stateful-set/pod-template.yaml"
MATCH_TEXT="volumes:"
MATCH_LINE=$(grep -m 1 -n "${MATCH_TEXT}" "${FILE}" | cut -d ':' -f 1)

if [[ -n "${MATCH_LINE}" ]]; then
  { head -n ${MATCH_LINE} "${FILE}"; printf "%s" "${SECRET_TMPL}"; tail -n +$(( ${MATCH_LINE} + 1 )) "${FILE}"; } > "${TEMP_FILE}"
  mv "${TEMP_FILE}" "${FILE}"
fi

########################################
########################################
## Add Synadia Server Extra Args
FILE="files/stateful-set/nats-container.yaml"
MATCH_TEXT="args:"
MATCH_LINE=$(grep -m 1 -n "${MATCH_TEXT}" "${FILE}" | cut -d ':' -f 1)

if [[ -n "${MATCH_LINE}" ]]; then
  { head -n ${MATCH_LINE} "${FILE}"; printf "%s" "${ARGS}"; tail -n +$(( ${MATCH_LINE} + 1 )) "${FILE}"; } > "${TEMP_FILE}"
  mv "${TEMP_FILE}" "${FILE}"
fi

########################################
########################################
## Replace Lame Duck Mode Signal
FILE="files/stateful-set/nats-container.yaml"
MATCH_TEXT="preStop"
MATCH_LINE=$(grep -m 1 -n "${MATCH_TEXT}" "${FILE}" | cut -d ':' -f 1)
FOLLOWING_CMD_LINE=$(awk -v start_line="${MATCH_LINE}" 'NR > start_line && /command:/ { print NR; exit }' "${FILE}")

if [[ -n "${FOLLOWING_CMD_LINE}" ]]; then
  { head -n $(( ${FOLLOWING_CMD_LINE} )) "${FILE}"; printf "%s" "${LDM}"; tail -n +$(( ${FOLLOWING_CMD_LINE} + 3 )) "${FILE}"; } > "${TEMP_FILE}"
  mv "${TEMP_FILE}" "${FILE}"
fi

########################################
########################################
## Add Token Volume Mount
FILE="files/stateful-set/nats-container.yaml"
MATCH_TEXT="volumeMounts:"
MATCH_LINE=$(grep -m 1 -n "${MATCH_TEXT}" "${FILE}" | cut -d ':' -f 1)

if [[ -n "${MATCH_LINE}" ]]; then
  { head -n ${MATCH_LINE} "${FILE}"; printf "%s" "${MOUNT}"; tail -n +$(( ${MATCH_LINE} + 1 )) "${FILE}"; } > "${TEMP_FILE}"
  mv "${TEMP_FILE}" "${FILE}"
fi

########################################
########################################
## Copy Overlay Files
cp -r $(dirname "$0")/overlay/* .

########################################
########################################
## Set Versions
${SED_I} "s/_CHART_VERSION_/${CHART_VERSION}/" Chart.yaml
${SED_I} "s/_APP_VERSION_/${APP_VERSION}/" Chart.yaml

########################################
########################################
## Remove backup files if on macOS
rm -f *.macos.bak
