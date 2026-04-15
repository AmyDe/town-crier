#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${HOME}/.local/bin"
PROJECT="cli/src/tc/tc.csproj"
RUNTIME="osx-arm64"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Building tc (Native AOT, ${RUNTIME})..."
dotnet publish "${SCRIPT_DIR}/${PROJECT}" \
  -r "${RUNTIME}" \
  -c Release \
  --nologo \
  -v quiet

PUBLISH_DIR="${SCRIPT_DIR}/cli/src/tc/bin/Release/net10.0/${RUNTIME}/publish"
if [[ ! -f "${PUBLISH_DIR}/tc" ]]; then
  echo "Error: build did not produce ${PUBLISH_DIR}/tc" >&2
  exit 1
fi

mkdir -p "${INSTALL_DIR}"
cp -f "${PUBLISH_DIR}/tc" "${INSTALL_DIR}/tc"
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
