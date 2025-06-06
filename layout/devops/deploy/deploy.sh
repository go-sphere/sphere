#!/bin/bash
set -euo pipefail


readonly REMOTE_HOST="orb"
readonly REMOTE_ARCH="arm64"
readonly PROJECT_NAME="sphere"

readonly SERVICE_NAME="${PROJECT_NAME}.service"
readonly REMOTE_DIR="/opt/${PROJECT_NAME}"
readonly LOCAL_BINARY="./build/linux_${REMOTE_ARCH}/app"

build() {
    make build/assets || return 1
    make "build/linux/${REMOTE_ARCH}"
}

install() {
    local service_file="./devops/deploy/${SERVICE_NAME}"
    scp "${service_file}" "${REMOTE_HOST}":/tmp/ || return 1
    ssh -t "${REMOTE_HOST}" \
        "sudo mv /tmp/${SERVICE_NAME} /etc/systemd/system/ && \
         sudo mkdir -p '${REMOTE_DIR}' && \
         sudo systemctl daemon-reload && \
         sudo systemctl enable '${SERVICE_NAME}'
         "
}

deploy() {
    local version
    build || return 1
    version=$(git rev-parse --short HEAD) || return 1
    binary_name="app-${version}-$(date +%Y%m%d%H%M%S)-${REMOTE_ARCH}"
    echo "Deploying ${binary_name} to ${REMOTE_HOST}..."

    scp "${LOCAL_BINARY}" "${REMOTE_HOST}:/tmp/${binary_name}" || return 1
    ssh -t "${REMOTE_HOST}" \
        "sudo mv /tmp/${binary_name} '${REMOTE_DIR}/' && \
         sudo chmod +x '${REMOTE_DIR}/${binary_name}' && \
         sudo systemctl stop '${SERVICE_NAME}' && \
         sudo rm -f '${REMOTE_DIR}/app' && \
         sudo ln -sf '${REMOTE_DIR}/${binary_name}' '${REMOTE_DIR}/app' && \
         sudo systemctl restart '${SERVICE_NAME}' && \
         sudo systemctl status '${SERVICE_NAME}'
         "
}

stop() {
    ssh -t "${REMOTE_HOST}" "sudo systemctl stop '${SERVICE_NAME}'"
}

start() {
    ssh -t "${REMOTE_HOST}" "sudo systemctl start '${SERVICE_NAME}'"
}

case "${1:-}" in
    build|install|deploy|stop|start)
        "$1"
        ;;
    *)
        echo "Usage: $0 {build|install|deploy|stop|start}"
        ;;
esac
