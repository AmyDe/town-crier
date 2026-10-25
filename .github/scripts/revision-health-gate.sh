#!/usr/bin/env bash
# Post-deploy gate for a Container App revision.
#   MODE_ARG=check    wait until NEW_REVISION is running, healthy and the latest ready
#                     revision, then GET HEALTH_URL (skipped when empty).
#   MODE_ARG=rollback restore the previous revision: shift traffic in Multiple mode,
#                     redeploy PREVIOUS_IMAGE in Single mode.
# Env: APP RG NEW_REVISION HEALTH_URL TIMEOUT_SECONDS INTERVAL_SECONDS
#      REVISION_MODE PREVIOUS_REVISION PREVIOUS_IMAGE
set -euo pipefail

: "${APP:?}" "${RG:?}" "${MODE_ARG:?}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-300}"
INTERVAL_SECONDS="${INTERVAL_SECONDS:-10}"

check() {
  : "${NEW_REVISION:?}"
  local deadline=$((SECONDS + TIMEOUT_SECONDS)) state health ready
  while :; do
    state="$(az containerapp revision show --name "$APP" --resource-group "$RG" \
      --revision "$NEW_REVISION" --query "properties.runningState" -o tsv || true)"
    health="$(az containerapp revision show --name "$APP" --resource-group "$RG" \
      --revision "$NEW_REVISION" --query "properties.healthState" -o tsv || true)"
    ready="$(az containerapp show --name "$APP" --resource-group "$RG" \
      --query "properties.latestReadyRevisionName" -o tsv || true)"
    echo "revision=$NEW_REVISION runningState=$state healthState=$health latestReady=$ready"

    if [ "$state" = "Failed" ]; then
      echo "::error::Revision $NEW_REVISION failed to start"
      return 1
    fi
    if [[ "$state" == Running* ]] && [ "$health" = "Healthy" ] && [ "$ready" = "$NEW_REVISION" ]; then
      if [ -z "${HEALTH_URL:-}" ]; then
        echo "::notice::No health URL configured, revision state check only"
        return 0
      fi
      if curl -fsS --max-time 10 "$HEALTH_URL" > /dev/null; then
        echo "Health check passed: $HEALTH_URL"
        return 0
      fi
    fi
    if [ "$SECONDS" -ge "$deadline" ]; then
      echo "::error::Revision $NEW_REVISION not healthy within ${TIMEOUT_SECONDS}s"
      return 1
    fi
    sleep "$INTERVAL_SECONDS"
  done
}

rollback() {
  if [ "${REVISION_MODE,,}" = "multiple" ] && [ -n "${PREVIOUS_REVISION:-}" ]; then
    az containerapp ingress traffic set --name "$APP" --resource-group "$RG" \
      --revision-weight "${PREVIOUS_REVISION}=100"
    echo "Traffic shifted back to $PREVIOUS_REVISION"
  elif [ -n "${PREVIOUS_IMAGE:-}" ]; then
    az containerapp update --name "$APP" --resource-group "$RG" \
      --image "$PREVIOUS_IMAGE" \
      --revision-suffix "rb-${GITHUB_RUN_ID:-0}-${GITHUB_RUN_ATTEMPT:-0}"
    echo "Redeployed previous image $PREVIOUS_IMAGE"
  else
    echo "::error::No previous revision or image recorded, cannot roll back"
    return 1
  fi
}

case "$MODE_ARG" in
  check) check ;;
  rollback) rollback ;;
  *) echo "unknown MODE_ARG $MODE_ARG" >&2; exit 2 ;;
esac
