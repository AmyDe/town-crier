#!/usr/bin/env bash
# Tests resolve-ancestor-image.sh with stubbed `git` and `az`.
set -uo pipefail

SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/resolve-ancestor-image.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin"

cat > "$WORK/bin/git" <<'STUB'
#!/usr/bin/env bash
if [ "$1" = "diff" ]; then
  echo "$*" >> "$FAKE_GIT_LOG"
  [ "${FAKE_SOURCE_CHANGED:-0}" = 1 ] && exit 1
  exit 0
fi
max="${2#--max-count=}"
seq 1 60 | head -n "$max" | sed "s/^/sha/"
STUB
cat > "$WORK/bin/az" <<'STUB'
#!/usr/bin/env bash
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--image" ]; then tag="${2#*:}"; fi
  shift
done
for t in $FAKE_IMAGES; do
  if [ "$t" = "$tag" ]; then echo "sha256:digest-$tag"; exit 0; fi
done
exit 3
STUB
chmod +x "$WORK/bin/git" "$WORK/bin/az"

fail=0
run() {
  : > "$WORK/out"
  : > "$WORK/gitlog"
  FAKE_GIT_LOG="$WORK/gitlog" FAKE_SOURCE_CHANGED="${2:-0}" FAKE_IMAGES="$1" PATH="$WORK/bin:$PATH" GITHUB_OUTPUT="$WORK/out" TAG_SHA=sha1 \
    ACR_SERVER=reg.azurecr.io REPO=repo DISPLAY_NAME=worker \
    bash "$SCRIPT" > "$WORK/log" 2>&1
}
check() {
  if [ "$2" != "$3" ]; then echo "FAIL: $1 (got '$2', want '$3')"; fail=1; else echo "ok: $1"; fi
}

run "sha1 sha2"; rc=$?
check "exact SHA wins: exit" "$rc" 0
check "exact SHA wins: output" "$(cat "$WORK/out")" "image=reg.azurecr.io/repo@sha256:digest-sha1"

run "sha4 sha5"; rc=$?
check "gap skipped: exit" "$rc" 0
check "gap skipped: first ancestor wins" "$(cat "$WORK/out")" "image=reg.azurecr.io/repo@sha256:digest-sha4"
check "gap skipped: logs SHA used" "$(grep -c 'ancestor SHA sha4' "$WORK/log")" 1

run "sha51"; rc=$?
check "none in 50: exit" "$rc" 1
check "none in 50: error line" "$(grep -c '^::error::' "$WORK/log")" 1
check "none in 50: no output" "$(wc -c < "$WORK/out" | tr -d ' ')" 0

run "sha4"; rc=$?
check "ancestor, sources unchanged: exit" "$rc" 0
check "ancestor, sources unchanged: diffs api-go" "$(grep -c -- '--quiet sha4 sha1 -- api-go' "$WORK/gitlog")" 1

run "sha4" 1; rc=$?
check "ancestor, sources changed: exit" "$rc" 1
check "ancestor, sources changed: error line" "$(grep -c '^::error::.*sha4.*sha1' "$WORK/log")" 1
check "ancestor, sources changed: no output" "$(wc -c < "$WORK/out" | tr -d ' ')" 0

run "sha1" 1; rc=$?
check "exact SHA skips source diff: exit" "$rc" 0
check "exact SHA skips source diff: no diff call" "$(wc -c < "$WORK/gitlog" | tr -d ' ')" 0

exit "$fail"
