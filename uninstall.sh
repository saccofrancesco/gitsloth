#!/usr/bin/env sh
set -eu

BINARY_NAME="gitsloth"
INSTALL_DIR="${GITSLOTH_INSTALL_DIR:-}"

say() {
  printf "%s\n" "$*"
}

cleanup_profile_file() {
  file="$1"
  [ -f "${file}" ] || return 0

  tmp_file="${file}.gitsloth.tmp.$$"
  awk '
    BEGIN { skip_next = 0; changed = 0 }
    {
      if ($0 == "# Added by gitsloth installer") {
        skip_next = 1
        changed = 1
        next
      }

      if (skip_next == 1) {
        skip_next = 0
        changed = 1
        next
      }

      print
    }
  ' "${file}" > "${tmp_file}"

  if cmp -s "${file}" "${tmp_file}"; then
    rm -f "${tmp_file}"
  else
    mv "${tmp_file}" "${file}"
    say "Updated ${file}"
  fi
}

remove_binary_if_present() {
  path="$1"
  [ -n "${path}" ] || return 1

  if [ -f "${path}" ]; then
    rm -f "${path}"
    say "Removed ${path}"
    return 0
  fi

  return 1
}

resolve_current_binary_path() {
  candidate="$(command -v "${BINARY_NAME}" 2>/dev/null || true)"
  case "${candidate}" in
    /*)
      if [ -f "${candidate}" ]; then
        printf "%s" "${candidate}"
      fi
      ;;
  esac
}

main() {
  removed_any_binary=0

  current_binary="$(resolve_current_binary_path)"
  if remove_binary_if_present "${current_binary}"; then
    removed_any_binary=1
  fi

  if [ -n "${INSTALL_DIR}" ]; then
    if [ "${INSTALL_DIR%/}/${BINARY_NAME}" != "${current_binary}" ]; then
      if remove_binary_if_present "${INSTALL_DIR%/}/${BINARY_NAME}"; then
        removed_any_binary=1
      fi
    fi
  fi

  for dir in "/usr/local/bin" "${HOME}/.local/bin"; do
    candidate="${dir}/${BINARY_NAME}"
    if [ "${candidate}" = "${current_binary}" ]; then
      continue
    fi
    if remove_binary_if_present "${candidate}"; then
      removed_any_binary=1
    fi
  done

  cleanup_profile_file "${HOME}/.zshrc"
  cleanup_profile_file "${HOME}/.bashrc"
  cleanup_profile_file "${HOME}/.bash_profile"
  cleanup_profile_file "${HOME}/.profile"
  cleanup_profile_file "${HOME}/.config/fish/config.fish"

  if [ "${removed_any_binary}" -eq 0 ]; then
    say "No ${BINARY_NAME} binary found in known install locations."
  fi

  say "Uninstall complete."
}

main "$@"
