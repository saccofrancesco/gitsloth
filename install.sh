#!/usr/bin/env sh
set -eu

BINARY_NAME="gitsloth"
REPO="${GITSLOTH_REPO:-saccofrancesco/gitsloth}"
VERSION="${GITSLOTH_VERSION:-latest}"
INSTALL_DIR="${GITSLOTH_INSTALL_DIR:-}"
TMP_DIR=""
PLATFORM=""
ARCHIVE_NAME=""

say() {
  printf "%s\n" "$*"
}

fail() {
  printf "Error: %s\n" "$*" >&2
  exit 1
}

cleanup() {
  if [ -n "${TMP_DIR}" ] && [ -d "${TMP_DIR}" ]; then
    rm -rf "${TMP_DIR}"
  fi
}

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "required command not found: $1"
  fi
}

download_file() {
  url="$1"
  out="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
    return 0
  fi

  if command -v wget >/dev/null 2>&1; then
    wget -qO "$out" "$url"
    return 0
  fi

  fail "curl or wget is required"
}

detect_platform() {
  os="$(uname -s 2>/dev/null || true)"
  arch="$(uname -m 2>/dev/null || true)"

  case "$os" in
    Linux)
      case "$arch" in
        x86_64|amd64)
          PLATFORM="linux-x86_64"
          ARCHIVE_NAME="${BINARY_NAME}-linux-x86_64.tar.gz"
          ;;
        *)
          fail "unsupported Linux architecture: ${arch} (supported: x86_64)"
          ;;
      esac
      ;;
    Darwin)
      case "$arch" in
        x86_64|amd64)
          PLATFORM="macos-x86_64"
          ARCHIVE_NAME="${BINARY_NAME}-macos-x86_64.tar.gz"
          ;;
        arm64|aarch64)
          PLATFORM="macos-aarch64"
          ARCHIVE_NAME="${BINARY_NAME}-macos-aarch64.tar.gz"
          ;;
        *)
          fail "unsupported macOS architecture: ${arch}"
          ;;
      esac
      ;;
    *)
      fail "unsupported OS: ${os}. This installer supports macOS and Linux."
      ;;
  esac
}

resolve_install_dir() {
  if [ -n "${INSTALL_DIR}" ]; then
    return 0
  fi

  if [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="${HOME}/.local/bin"
  fi
}

ensure_install_dir() {
  mkdir -p "${INSTALL_DIR}"
  if [ ! -w "${INSTALL_DIR}" ]; then
    fail "install directory is not writable: ${INSTALL_DIR} (set GITSLOTH_INSTALL_DIR to a writable path)"
  fi
}

path_contains_install_dir() {
  case ":${PATH}:" in
    *:"${INSTALL_DIR}":*)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

append_line_if_missing() {
  file="$1"
  line="$2"

  [ -f "${file}" ] || touch "${file}"
  if grep -F "${line}" "${file}" >/dev/null 2>&1; then
    return 0
  fi

  printf "\n# Added by gitsloth installer\n%s\n" "${line}" >> "${file}"
}

ensure_path() {
  if path_contains_install_dir; then
    return 0
  fi

  shell_name="$(basename "${SHELL:-}")"
  profile_file=""
  path_line=""

  case "${shell_name}" in
    zsh)
      profile_file="${HOME}/.zshrc"
      path_line="export PATH=\"${INSTALL_DIR}:\$PATH\""
      ;;
    bash)
      if [ "$(uname -s)" = "Darwin" ]; then
        profile_file="${HOME}/.bash_profile"
      else
        profile_file="${HOME}/.bashrc"
      fi
      path_line="export PATH=\"${INSTALL_DIR}:\$PATH\""
      ;;
    fish)
      profile_file="${HOME}/.config/fish/config.fish"
      mkdir -p "$(dirname "${profile_file}")"
      path_line="fish_add_path \"${INSTALL_DIR}\""
      ;;
    *)
      profile_file="${HOME}/.profile"
      path_line="export PATH=\"${INSTALL_DIR}:\$PATH\""
      ;;
  esac

  append_line_if_missing "${profile_file}" "${path_line}"
  say "Added ${INSTALL_DIR} to PATH in ${profile_file}"
  say "Open a new shell or run: export PATH=\"${INSTALL_DIR}:\$PATH\""
}

main() {
  trap cleanup EXIT INT TERM

  need_cmd uname
  need_cmd tar

  detect_platform
  resolve_install_dir
  ensure_install_dir

  TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t gitsloth)"
  archive_path="${TMP_DIR}/${ARCHIVE_NAME}"

  if [ "${VERSION}" = "latest" ]; then
    download_url="https://github.com/${REPO}/releases/latest/download/${ARCHIVE_NAME}"
  else
    download_url="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE_NAME}"
  fi

  say "Installing ${BINARY_NAME} (${PLATFORM}) from ${REPO} (${VERSION})"
  download_file "${download_url}" "${archive_path}" || fail "failed to download ${download_url}"

  tar -xzf "${archive_path}" -C "${TMP_DIR}"

  if [ ! -f "${TMP_DIR}/${BINARY_NAME}" ]; then
    fail "archive did not contain expected binary: ${BINARY_NAME}"
  fi

  cp "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
  chmod 0755 "${INSTALL_DIR}/${BINARY_NAME}"

  ensure_path
  say "Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"
  say "Verify with: ${BINARY_NAME} -h"
}

main "$@"
