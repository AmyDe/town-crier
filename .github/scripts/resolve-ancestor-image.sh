#!/usr/bin/env bash
# Resolves the image digest for TAG_SHA, or for its nearest ancestor commit
# (up to 50 commits) that has an image in ACR. Writes image=<server>/<repo>@<digest>
# to $GITHUB_OUTPUT. Required env: TAG_SHA, ACR_SERVER, REPO, DISPLAY_NAME.
set -euo pipefail

ACR_NAME="${ACR_SERVER%.azurecr.io}"

# The go category gates the image builds in cd-dev, so its paths are the image sources.
# shellcheck source=../actions/detect-changes/category-map.sh
source "$(dirname "${BASH_SOURCE[0]}")/../actions/detect-changes/category-map.sh"
read -ra SOURCE_PATHS <<< "${CATEGORY_PREFIXES[go]}"

digest_for_tag() {
  az acr repository show \
    --name "$ACR_NAME" \
    --image "${REPO}:$1" \
    --query digest -o tsv 2>/dev/null
}

# `|| true` keeps `bash -e` from aborting on a SHA that has no image.
while read -r sha; do
  DIGEST="$(digest_for_tag "$sha" || true)"
  if [ -n "$DIGEST" ]; then
    if [ "$sha" = "$TAG_SHA" ]; then
      echo "Using exact image for SHA $sha ($DIGEST)"
    else
      echo "No image for SHA $TAG_SHA, using nearest ancestor SHA $sha ($DIGEST)"
    fi
    if [ "$sha" != "$TAG_SHA" ] && ! git diff --quiet "$sha" "$TAG_SHA" -- "${SOURCE_PATHS[@]}"; then
      echo "::error::The ${DISPLAY_NAME} image source changed after the newest built image ($sha), so $TAG_SHA has no matching image"
      exit 1
    fi
    echo "image=${ACR_SERVER}/${REPO}@${DIGEST}" >> "$GITHUB_OUTPUT"
    exit 0
  fi
done < <(git rev-list --max-count=50 "$TAG_SHA")

echo "::error::No ${DISPLAY_NAME} image found in the 50 commits up to $TAG_SHA"
exit 1
