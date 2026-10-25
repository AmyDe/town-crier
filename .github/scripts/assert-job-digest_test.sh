#!/usr/bin/env bash
# Tests assert-job-digest.sh with a stubbed `az`.
set -uo pipefail

SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/assert-job-digest.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin"

cat > "$WORK/bin/az" <<'STUB'
#!/usr/bin/env bash
case "$*" in
  *"acr repository show"*) echo "${FAKE_TAG_DIGEST:-sha256:tag}" ;;
  *"containerapp job show"*"job-tc-poll-dev"*) echo "$FAKE_POLL_IMAGE" ;;
  *"containerapp job show"*"job-tc-digest-dev"*) echo "$FAKE_DIGEST_IMAGE" ;;
esac
STUB
chmod +x "$WORK/bin/az"

fail=0
check() {
  if [ "$2" != "$3" ]; then echo "FAIL: $1 (got '$2', want '$3')"; fail=1; else echo "ok: $1"; fi
}
run() {
  env PATH="$WORK/bin:$PATH" RG=rg SUFFIX=dev JOBS="poll digest" "$@" bash "$SCRIPT" > "$WORK/out" 2>&1
}

run IMAGE=reg.azurecr.io/app@sha256:aaa FAKE_POLL_IMAGE=reg.azurecr.io/app@sha256:aaa FAKE_DIGEST_IMAGE=reg.azurecr.io/app@sha256:aaa; rc=$?
check "all match: exit" "$rc" 0

run IMAGE=reg.azurecr.io/app@sha256:aaa FAKE_POLL_IMAGE=reg.azurecr.io/app@sha256:aaa FAKE_DIGEST_IMAGE=reg.azurecr.io/app@sha256:bbb; rc=$?
check "one mismatch: exit" "$rc" 1
check "one mismatch: names job" "$(grep -c 'job-tc-digest-dev' "$WORK/out")" 1

run IMAGE=reg.azurecr.io/app:abc FAKE_TAG_DIGEST=sha256:tag FAKE_POLL_IMAGE=reg.azurecr.io/app@sha256:tag FAKE_DIGEST_IMAGE=reg.azurecr.io/app:abc; rc=$?
check "tag ref resolved: exit" "$rc" 0

run IMAGE=reg.azurecr.io/app@sha256:aaa FAKE_POLL_IMAGE= FAKE_DIGEST_IMAGE=reg.azurecr.io/app@sha256:aaa; rc=$?
check "empty job image: exit" "$rc" 1

exit "$fail"
