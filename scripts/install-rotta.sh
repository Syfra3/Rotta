#!/usr/bin/env bash
set -euo pipefail

# Rotta Universal Installer
# Usage:
#   curl -sSL https://raw.githubusercontent.com/Syfra3/Rotta/main/scripts/install-rotta.sh | bash

VERSION="${ROTTA_VERSION:-latest}"
INSTALL_DIR="${ROTTA_INSTALL_DIR:-/usr/local/bin}"
INSTALL_BIN_NAME="${ROTTA_BINARY_NAME:-rotta}"
REPO="Syfra3/Rotta"
PACKAGE="rotta"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
  echo -e "${BLUE}==>${NC} $1"
}

log_success() {
  echo -e "${GREEN}OK${NC} $1"
}

log_warn() {
  echo -e "${YELLOW}!${NC} $1"
}

log_error() {
  echo -e "${RED}ERR${NC} $1"
}

detect_platform() {
  local os=""
  local arch=""

  case "$(uname -s)" in
    Linux*)
      os="linux"
      ;;
    Darwin*)
      os="darwin"
      ;;
    MINGW*|MSYS*|CYGWIN*)
      os="windows"
      ;;
    *)
      log_error "Unsupported operating system: $(uname -s)"
      exit 1
      ;;
  esac

  case "$(uname -m)" in
    x86_64|amd64)
      arch="amd64"
      ;;
    aarch64|arm64)
      arch="arm64"
      ;;
    *)
      log_error "Unsupported architecture: $(uname -m)"
      exit 1
      ;;
  esac

  echo "${os}-${arch}"
}

get_latest_version() {
  log_info "Fetching latest rotta version" >&2
  local latest
  latest=$(curl -s "https://api.github.com/repos/${REPO}/releases" | sed -n 's/.*"tag_name": "\(v[^\"]*\)".*/\1/p' | head -n1)
  if [ -z "${latest}" ]; then
    latest=$(curl -s "https://api.github.com/repos/${REPO}/releases" | sed -n 's/.*"tag_name": "\(rotta-v[^\"]*\)".*/\1/p' | head -n1)
  fi

  if [ -z "${latest}" ]; then
    log_error "Unable to detect latest v-prefixed rotta release from GitHub"
    exit 1
  fi

  normalize_release_tag "${latest}"
}

normalize_release_tag() {
  local input="$1"

  case "${input}" in
    rotta-v*) echo "${input}" ;;
    v*) echo "${input}" ;;
    *) echo "v${input}" ;;
  esac
}

version_number_from_tag() {
  local tag="$1"
  tag="${tag#rotta-v}"
  echo "${tag#v}"
}

verify_checksum() {
  local asset="$1"
  local archive_path="$2"

  local checksum_url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}.sha256"
  local expected
  expected=$(curl -fsSL "${checksum_url}" 2>/dev/null | awk '{print $1}' || true)

  if [[ ! "${expected}" =~ ^[a-fA-F0-9]{64}$ ]]; then
    log_warn "No checksum file found for ${asset}; skipping checksum check"
    return
  fi

  local actual=""

  if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "${archive_path}" | awk '{print $1}')
  else
    actual=$(shasum -a 256 "${archive_path}" | awk '{print $1}')
  fi

  if [ "${expected}" != "${actual}" ]; then
    log_error "Checksum mismatch for ${asset}"
    echo "Expected: ${expected}"
    echo "Actual:   ${actual}"
    exit 1
  fi
}

download_and_install() {
  local platform="$1"
  local version_number
  version_number=$(version_number_from_tag "${VERSION}")
  local archive="${PACKAGE}-${version_number}-${platform}.tar.gz"
  local url="https://github.com/${REPO}/releases/download/${VERSION}/${archive}"
  local tmp_dir="$(mktemp -d)"

  cleanup() {
    rm -rf "${tmp_dir}"
  }

  trap cleanup EXIT
  local archive_path="${tmp_dir}/${archive}"

  log_info "Downloading ${archive}"
  curl -L "${url}" -o "${archive_path}"

  verify_checksum "${archive}" "${archive_path}"

  log_info "Extracting ${archive}"
  tar -xzf "${archive_path}" -C "${tmp_dir}"

  local source_binary="${tmp_dir}/rotta"
  if [ "${platform}" = "windows-amd64" ]; then
    source_binary="${tmp_dir}/rotta.exe"
  fi

  if [ ! -f "${source_binary}" ]; then
    log_error "Downloaded archive did not contain expected binary"
    exit 1
  fi

  if [ ! -d "${INSTALL_DIR}" ]; then
    log_warn "Install directory does not exist. Creating ${INSTALL_DIR}"
    mkdir -p "${INSTALL_DIR}"
  fi

  log_info "Installing ${INSTALL_BIN_NAME} to ${INSTALL_DIR}"
  if ! mv "${source_binary}" "${INSTALL_DIR}/${INSTALL_BIN_NAME}"; then
    log_error "Install failed. Try setting ROTTA_INSTALL_DIR to a writable location."
    exit 1
  fi

  chmod +x "${INSTALL_DIR}/${INSTALL_BIN_NAME}"
  log_success "Installed ${PACKAGE} ${VERSION}"
}

verify_install() {
  if ! command -v "${INSTALL_BIN_NAME}" >/dev/null 2>&1; then
    log_warn "${INSTALL_BIN_NAME} installed but not in PATH"
    log_info "Add ${INSTALL_DIR} to PATH to use it directly"
    return
  fi

  local installed_version
  installed_version="$("${INSTALL_BIN_NAME}" --version 2>/dev/null || true)"
  log_success "Verified ${INSTALL_BIN_NAME} (${installed_version})"
}

main() {
  echo ""
  echo "----------------------------------------"
  echo -e "${BLUE}Rotta Installer${NC}"
  echo "----------------------------------------"
  echo ""

  for cmd in curl tar; do
    if ! command -v "${cmd}" >/dev/null 2>&1; then
      log_error "Missing required command: ${cmd}"
      exit 1
    fi
  done

  local platform
  platform=$(detect_platform)
  log_info "Detected platform ${platform}"

  if [ "${VERSION}" = "latest" ]; then
    VERSION=$(get_latest_version)
  else
    VERSION=$(normalize_release_tag "${VERSION}")
  fi

  if [ -z "${VERSION}" ]; then
    log_error "No version to install"
    exit 1
  fi

  log_info "Target version ${VERSION}"
  download_and_install "${platform}"
  verify_install

  echo ""
  echo "Run: ${INSTALL_BIN_NAME} --help"
  echo ""
}

main "$@"
