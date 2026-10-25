#!/usr/bin/env bash
# Resolves the release-<TAG> manifests of the API and worker images to digest
# refs. Fails before any output is written unless both exist.
# Env: TAG ACR_SERVER REPO_API REPO_WORKER; writes api-image and worker-image
# to GITHUB_OUTPUT.
set -euo pipefail

: "${TAG:?}" "${ACR_SERVER:?}" "${REPO_API:?}" "${REPO_WORKER:?}" "${GITHUB_OUTPUT:?}"

if ! [[ "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "::error::Tag '$TAG' must match vMAJOR.MINOR.PATCH"
  exit 1
fi

ACR_NAME="${ACR_SERVER%%.*}"

digest_for() {
  az acr repository show --name "$ACR_NAME" --image "$1:release-${TAG}" \
    --query digest -o tsv 2>/dev/null || true
}

API_DIGEST="$(digest_for "$REPO_API")"
WORKER_DIGEST="$(digest_for "$REPO_WORKER")"

missing=0
if [ -z "$API_DIGEST" ]; then
  echo "::error::No ${REPO_API}:release-${TAG} manifest in ${ACR_NAME}"
  missing=1
fi
if [ -z "$WORKER_DIGEST" ]; then
  echo "::error::No ${REPO_WORKER}:release-${TAG} manifest in ${ACR_NAME}"
  missing=1
fi
[ "$missing" -eq 0 ] || exit 1

{
  echo "api-image=${ACR_SERVER}/${REPO_API}@${API_DIGEST}"
  echo "worker-image=${ACR_SERVER}/${REPO_WORKER}@${WORKER_DIGEST}"
} >> "$GITHUB_OUTPUT"
