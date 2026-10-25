#!/usr/bin/env bash
# Tests resolve-release-images.sh with a stubbed `az`.
set -uo pipefail

SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/resolve-release-images.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin"

cat > "$WORK/bin/az" <<'STUB'
#!/usr/bin/env bash
case "$*" in
  *"--image api:release-v1.2.3"*) echo "$FAKE_API" ;;
  *"--image worker:release-v1.2.3"*) echo "$FAKE_WORKER" ;;
esac
STUB
chmod +x "$WORK/bin/az"

fail=0
check() {
  if [ "$2" != "$3" ]; then echo "FAIL: $1 (got '$2', want '$3')"; fail=1; else echo "ok: $1"; fi
}
run() {
  : > "$WORK/gho"
  env PATH="$WORK/bin:$PATH" ACR_SERVER=reg.azurecr.io REPO_API=api REPO_WORKER=worker \
    GITHUB_OUTPUT="$WORK/gho" "$@" bash "$SCRIPT" > "$WORK/out" 2>&1
}

run TAG=v1.2.3 FAKE_API=sha256:aaa FAKE_WORKER=sha256:bbb; rc=$?
check "both present: exit" "$rc" 0
check "both present: api" "$(grep '^api-image=' "$WORK/gho")" "api-image=reg.azurecr.io/api@sha256:aaa"
check "both present: worker" "$(grep '^worker-image=' "$WORK/gho")" "worker-image=reg.azurecr.io/worker@sha256:bbb"

run TAG=v1.2.3 FAKE_API=sha256:aaa FAKE_WORKER=; rc=$?
check "worker missing: exit" "$rc" 1
check "worker missing: error" "$(grep -c '::error::No worker:release-v1.2.3' "$WORK/out")" 1
check "worker missing: no outputs" "$(wc -c < "$WORK/gho" | tr -d ' ')" 0

run TAG=v1.2.3 FAKE_API= FAKE_WORKER=; rc=$?
check "both missing: exit" "$rc" 1
check "both missing: errors" "$(grep -c '::error::' "$WORK/out")" 2

for bad in latest v1.2 1.2.3 'v1.2.3-rc1' 'v1.2.3;x'; do
  run TAG="$bad" FAKE_API=sha256:aaa FAKE_WORKER=sha256:bbb; rc=$?
  check "bad tag '$bad': exit" "$rc" 1
done

exit "$fail"
