#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${HOME}/.local/bin"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Building tc (Go, static binary)..."
mkdir -p "${INSTALL_DIR}"
CGO_ENABLED=0 go build -C "${SCRIPT_DIR}/cli" \
  -trimpath -ldflags="-s -w" \
  -o "${INSTALL_DIR}/tc" \
  ./cmd/tc

if [[ ! -f "${INSTALL_DIR}/tc" ]]; then
  echo "Error: build did not produce ${INSTALL_DIR}/tc" >&2
  exit 1
fi

chmod +x "${INSTALL_DIR}/tc"
echo "Installed tc to ${INSTALL_DIR}/tc"

if ! echo "${PATH}" | tr ':' '\n' | grep -qx "${INSTALL_DIR}"; then
  SHELL_RC="${HOME}/.zshrc"
  EXPORT_LINE="export PATH=\"\${HOME}/.local/bin:\${PATH}\""
  if ! grep -qF '.local/bin' "${SHELL_RC}" 2>/dev/null; then
    echo "" >> "${SHELL_RC}"
    echo "# tc CLI" >> "${SHELL_RC}"
    echo "${EXPORT_LINE}" >> "${SHELL_RC}"
    echo "Added ${INSTALL_DIR} to PATH in ${SHELL_RC} — restart your shell or run:"
    echo "  source ${SHELL_RC}"
  else
    echo "${INSTALL_DIR} is referenced in ${SHELL_RC} but not on the current PATH."
    echo "Restart your shell or run: source ${SHELL_RC}"
  fi
else
  echo "tc is ready — run 'tc' to get started."
fi
