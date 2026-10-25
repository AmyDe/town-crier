#!/usr/bin/env bash
# Fails unless every Container App Job named job-tc-<name>-<SUFFIX> runs the
# digest of IMAGE (a digest ref or an ACR tag ref).
# Env: JOBS (space-separated names) SUFFIX RG IMAGE
set -euo pipefail

: "${JOBS:?}" "${SUFFIX:?}" "${RG:?}" "${IMAGE:?}"

digest_of() {
  local ref="$1" server registry repo_ref
  case "$ref" in
    *@sha256:*) printf '%s' "${ref##*@}" ;;
    *)
      server="${ref%%/*}"
      registry="${server%%.*}"
      repo_ref="${ref#*/}"
      az acr repository show --name "$registry" --image "$repo_ref" --query digest -o tsv
      ;;
  esac
}

WANT="$(digest_of "$IMAGE")"
if [ -z "$WANT" ]; then
  echo "::error::Could not resolve digest of $IMAGE"
  exit 1
fi

bad=0
for JOB in $JOBS; do
  NAME="job-tc-${JOB}-${SUFFIX}"
  GOT_REF="$(az containerapp job show --name "$NAME" --resource-group "$RG" \
    --query "properties.template.containers[0].image" -o tsv)"
  GOT=""
  [ -n "$GOT_REF" ] && GOT="$(digest_of "$GOT_REF")"
  if [ "$GOT" != "$WANT" ]; then
    echo "::error::$NAME runs '$GOT' but we deployed '$WANT'"
    bad=1
  else
    echo "Verified: $NAME runs $WANT"
  fi
done
exit "$bad"
