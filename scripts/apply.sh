#!/bin/bash

UPSTREAM_CHART_REPO="https://github.com/nats-io/k8s.git"
UPSTREAM_CHART_DIR="helm/charts/nats"
CHART_NAME="synadia-server"

cd $(dirname "$0")
scripts=$(pwd)

cd "${scripts}"/..
rm -rf charts/"${CHART_NAME}"
git clone "${UPSTREAM_CHART_REPO}" upstream
mv "upstream/${UPSTREAM_CHART_DIR}" "charts/${CHART_NAME}"
rm -rf upstream
cd charts/${CHART_NAME}
${scripts}/nats_to_syn_server.sh
