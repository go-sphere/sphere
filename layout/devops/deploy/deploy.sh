#!/bin/bash

# deploy.sh - A script to build and deploy the Sphere application.
#
# This script provides functions to build the binary, install the systemd service,
# and deploy new versions of the application to a remote server.

# --- Configuration ---
# Exit immediately if a command exits with a non-zero status.
set -euo pipefail

# --- Constants ---
readonly REMOTE_HOST="orb"
readonly REMOTE_ARCH="arm64"
readonly PROJECT_NAME="sphere"

readonly SERVICE_NAME="${PROJECT_NAME}.service"
readonly REMOTE_DIR="/opt/${PROJECT_NAME}"
readonly LOCAL_BINARY_PATH="./build/linux_${REMOTE_ARCH}/app"
readonly LOCAL_SERVICE_FILE="./devops/deploy/${SERVICE_NAME}"

# --- Colors ---
readonly COLOR_RESET='\033[0m'
readonly COLOR_RED='\033[0;31m'
readonly COLOR_GREEN='\033[0;32m'
readonly COLOR_YELLOW='\033[0;33m'
readonly COLOR_CYAN='\033[0;36m'

# --- Helper Functions ---

# Print an informational message.
#
# Arguments:
#   $1: The message to print.
log_info() {
    printf "${COLOR_CYAN}%s${COLOR_RESET}\n" "$1"
}

# Print a success message.
#
# Arguments:
#   $1: The message to print.
log_success() {
    printf "${COLOR_GREEN}%s${COLOR_RESET}\n" "$1"
}

# Print an error message and exit.
#
# Arguments:
#   $1: The message to print.
#   $2: The exit code (optional, defaults to 1).
log_error() {
    printf "${COLOR_RED}Error: %s${COLOR_RESET}\n" "$1" >&2
    exit "${2:-1}"
}

# Print a warning message.
#
# Arguments:
#   $1: The message to print.
log_warn() {
    printf "${COLOR_YELLOW}Warning: %s${COLOR_RESET}\n" "$1"
}

# Show usage information.
usage() {
    printf "Usage: %s {build|install|deploy|stop|start}\n" "$0"
    printf "Commands:\n"
    printf "  install  - Install the systemd service on the remote host.\n"
    printf "  deploy   - Build and deploy a new version of the application.\n"
    printf "  stop     - Stop the application service on the remote host.\n"
    printf "  start    - Start the application service on the remote host.\n"
    printf "  restart  - Restart the application service on the remote host.\n"
}

# Check for required command-line tools.
check_dependencies() {
    log_info "Checking for required tools..."
    local missing=0
    for cmd in ssh scp git make; do
        if ! command -v "${cmd}" &> /dev/null; then
            log_error "Required command '${cmd}' is not installed."
            # shellcheck disable=SC2317
            missing=1
        fi
    done
    if [[ ${missing} -eq 1 ]]; then
        log_error "Please install the missing dependencies and try again."
    fi
    log_success "All dependencies are present."
}

# Execute a command on the remote host via SSH.
#
# Arguments:
#   $@: The command and its arguments to execute.
run_remote() {
    # shellcheck disable=SC2145
    log_info "Executing on ${REMOTE_HOST}: $@"
    ssh -t "${REMOTE_HOST}" "$@"
}

# --- Core Functions ---

# Build the application binary and assets.
build_app() {
    log_info "Building assets..."
    if ! make build/assets; then
        log_error "Failed to build assets."
    fi

    log_info "Building binary for linux/${REMOTE_ARCH}..."
    if ! make "build/linux/${REMOTE_ARCH}"; then
        log_error "Failed to build binary."
    fi

    if [[ ! -f "${LOCAL_BINARY_PATH}" ]]; then
        log_error "Build succeeded, but binary not found at '${LOCAL_BINARY_PATH}'."
    fi
    log_success "Build completed successfully."
}

# Install the systemd service on the remote host.
install_service() {
    log_info "Installing systemd service '${SERVICE_NAME}' on ${REMOTE_HOST}..."

    if [[ ! -f "${LOCAL_SERVICE_FILE}" ]]; then
        log_error "Service file not found at '${LOCAL_SERVICE_FILE}'."
    fi

    local remote_tmp_service_file="/tmp/${SERVICE_NAME}"

    log_info "Uploading service file to ${REMOTE_HOST}:${remote_tmp_service_file}..."
    if ! scp "${LOCAL_SERVICE_FILE}" "${REMOTE_HOST}:${remote_tmp_service_file}"; then
        log_error "Failed to upload service file."
    fi

    log_info "Setting up service on remote host..."
    local install_commands="
        set -euo pipefail;
        echo 'Moving service file to /etc/systemd/system...';
        sudo mv '${remote_tmp_service_file}' /etc/systemd/system/;

        echo 'Creating remote directory ${REMOTE_DIR}...';
        sudo mkdir -p '${REMOTE_DIR}';

        echo 'Reloading systemd daemon...';
        sudo systemctl daemon-reload;

        echo 'Enabling service ${SERVICE_NAME}...';
        sudo systemctl enable '${SERVICE_NAME}';
    "
    if ! run_remote "${install_commands}"; then
        log_error "Failed to install service on remote host."
    fi

    log_success "Service '${SERVICE_NAME}' installed and enabled successfully."
}

# Deploy a new version of the application.
deploy_app() {
    log_info "Starting new deployment..."

    # Step 1: Build the application
    build_app

    # Step 2: Generate versioned binary name
    local version
    version=$(git rev-parse --short HEAD)
    # shellcheck disable=SC2155
    local binary_name="app-${version}-$(date +%Y%m%d%H%M%S)-${REMOTE_ARCH}"
    local remote_tmp_binary="/tmp/${binary_name}"

    log_info "Deploying version '${version}' as '${binary_name}' to ${REMOTE_HOST}..."

    # Step 3: Upload the binary
    log_info "Uploading binary to ${REMOTE_HOST}:${remote_tmp_binary}..."
    if ! scp "${LOCAL_BINARY_PATH}" "${REMOTE_HOST}:${remote_tmp_binary}"; then
        log_error "Failed to upload binary."
    fi

    # Step 4: Execute deployment steps on the remote server
    log_info "Performing deployment on remote host..."
    local deploy_commands="
        set -euo pipefail;
        echo 'Moving binary to ${REMOTE_DIR}...';
        sudo mv '${remote_tmp_binary}' '${REMOTE_DIR}/';

        echo 'Making binary executable...';
        sudo chmod +x '${REMOTE_DIR}/${binary_name}';

        echo 'Stopping service ${SERVICE_NAME}...';
        sudo systemctl stop '${SERVICE_NAME}';

        echo 'Updating symbolic link...';
        sudo rm -f '${REMOTE_DIR}/app';
        sudo ln -sf '${REMOTE_DIR}/${binary_name}' '${REMOTE_DIR}/app';

        echo 'Restarting service ${SERVICE_NAME}...';
        sudo systemctl restart '${SERVICE_NAME}';

        echo 'Checking service status...';
        sudo systemctl status '${SERVICE_NAME}';
    "
    if ! run_remote "${deploy_commands}"; then
        log_error "Deployment script failed on remote host."
    fi

    log_success "Deployment completed successfully."
}

# Stop the application service on the remote host.
stop_service() {
    log_info "Stopping service '${SERVICE_NAME}' on ${REMOTE_HOST}..."
    if ! run_remote "sudo systemctl stop '${SERVICE_NAME}'"; then
        log_error "Failed to stop the service."
    fi
    log_success "Service stopped."
}

# Start the application service on the remote host.
start_service() {
    log_info "Starting service '${SERVICE_NAME}' on ${REMOTE_HOST}..."
    if ! run_remote "sudo systemctl start '${SERVICE_NAME}'"; then
        log_error "Failed to start the service."
    fi
    run_remote "sudo systemctl status '${SERVICE_NAME}'"
    log_success "Service started."
}

# Restart the application service on the remote host.
restart_service() {
    log_info "Restarting service '${SERVICE_NAME}' on ${REMOTE_HOST}..."
    if ! run_remote "sudo systemctl restart '${SERVICE_NAME}'"; then
        log_error "Failed to restart the service."
    fi
    run_remote "sudo systemctl status '${SERVICE_NAME}'"
    log_success "Service restarted."
}

# --- Main Function ---
main() {
    check_dependencies

    local command
    if [[ $# -eq 0 ]]; then
        usage
        printf "\nPlease enter a command: "
        read -r command
    else
        command="$1"
    fi

    case "${command}" in
        install)
            install_service
            ;;
        deploy)
            deploy_app
            ;;
        stop)
            stop_service
            ;;
        start)
            start_service
            ;;
        restart)
            restart_service
            ;;
        *)
            usage
            log_error "Unknown command: ${command}" 1
            ;;
    esac
}

# --- Script Entrypoint ---
# Pass all script arguments to the main function.
main "$@"