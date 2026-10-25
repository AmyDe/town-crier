#!/usr/bin/env bash
# Tests revision-health-gate.sh with stubbed `az`, `curl` and `sleep`.
set -uo pipefail

SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/revision-health-gate.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin"

cat > "$WORK/bin/az" <<'STUB'
#!/usr/bin/env bash
echo "$*" >> "$FAKE_LOG"
case "$*" in
  *"revision show"*runningState*) echo "$FAKE_RUNNING" ;;
  *"revision show"*healthState*) echo "$FAKE_HEALTH" ;;
  *"containerapp show"*latestReadyRevisionName*) echo "$FAKE_READY" ;;
esac
exit 0
STUB
cat > "$WORK/bin/curl" <<'STUB'
#!/usr/bin/env bash
echo "curl $*" >> "$FAKE_LOG"
exit "${FAKE_CURL_RC:-0}"
STUB
printf '#!/usr/bin/env bash\nexit 0\n' > "$WORK/bin/sleep"
chmod +x "$WORK/bin/"*

fail=0
check() {
  if [ "$2" != "$3" ]; then echo "FAIL: $1 (got '$2', want '$3')"; fail=1; else echo "ok: $1"; fi
}
run() {
  : > "$WORK/log"
  env FAKE_LOG="$WORK/log" PATH="$WORK/bin:$PATH" APP=app RG=rg NEW_REVISION=app--r1 \
    HEALTH_URL="https://api.example.test/health" TIMEOUT_SECONDS=3 INTERVAL_SECONDS=1 \
    "$@" bash "$SCRIPT" > "$WORK/out" 2>&1
}

HEALTHY=(FAKE_RUNNING=Running FAKE_HEALTH=Healthy FAKE_READY=app--r1)

run MODE_ARG=check "${HEALTHY[@]}"; rc=$?
check "healthy: exit" "$rc" 0
check "healthy: curl called" "$(grep -c '^curl .*https://api.example.test/health' "$WORK/log")" 1

run MODE_ARG=check FAKE_RUNNING=RunningAtMaxScale FAKE_HEALTH=Healthy FAKE_READY=app--r1; rc=$?
check "RunningAtMaxScale accepted: exit" "$rc" 0

run MODE_ARG=check FAKE_RUNNING=Failed FAKE_HEALTH=Unhealthy FAKE_READY=app--r0; rc=$?
check "failed revision: exit" "$rc" 1
check "failed revision: no curl" "$(grep -c '^curl' "$WORK/log")" 0

run MODE_ARG=check FAKE_RUNNING=Running FAKE_HEALTH=Healthy FAKE_READY=app--r0; rc=$?
check "old revision still ready: exit" "$rc" 1
check "old revision still ready: no curl" "$(grep -c '^curl' "$WORK/log")" 0

run MODE_ARG=check "${HEALTHY[@]}" FAKE_CURL_RC=22; rc=$?
check "health endpoint fails: exit" "$rc" 1

run MODE_ARG=check "${HEALTHY[@]}" HEALTH_URL=; rc=$?
check "no health url: exit" "$rc" 0
check "no health url: no curl" "$(grep -c '^curl' "$WORK/log")" 0

run MODE_ARG=rollback REVISION_MODE=Multiple PREVIOUS_REVISION=app--r0 PREVIOUS_IMAGE=reg/app@sha256:aaa; rc=$?
check "multiple rollback: exit" "$rc" 0
check "multiple rollback: traffic set" "$(grep -c 'ingress traffic set .*--revision-weight app--r0=100' "$WORK/log")" 1
check "multiple rollback: no image update" "$(grep -c 'containerapp update' "$WORK/log")" 0

run MODE_ARG=rollback REVISION_MODE=Single PREVIOUS_REVISION=app--r0 PREVIOUS_IMAGE=reg/app@sha256:aaa; rc=$?
check "single rollback: exit" "$rc" 0
check "single rollback: previous image redeployed" "$(grep -c 'containerapp update .*--image reg/app@sha256:aaa' "$WORK/log")" 1
check "single rollback: no traffic set" "$(grep -c 'traffic set' "$WORK/log")" 0

run MODE_ARG=rollback REVISION_MODE=Single PREVIOUS_REVISION= PREVIOUS_IMAGE=; rc=$?
check "nothing to roll back to: exit" "$rc" 1

exit "$fail"
