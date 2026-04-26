#!/usr/bin/env bash
# Copy canonical legal JSON files from the API project into the iOS bundle resources.
#
# The API copy is the source of truth. The iOS copy is a byte-equal mirror, enforced
# by scripts/check-legal-drift.sh in CI. Edit the API copy, then run this script to
# refresh the iOS copy, then commit both.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

API_DIR="$REPO_ROOT/api/src/town-crier.application/Legal/Resources"
IOS_DIR="$REPO_ROOT/mobile/ios/packages/town-crier-presentation/Sources/Resources/legal"

if [[ ! -d "$API_DIR" ]]; then
    echo "error: canonical API legal docs directory missing: $API_DIR" >&2
    exit 1
fi

mkdir -p "$IOS_DIR"

shopt -s nullglob
copied=0
for src in "$API_DIR"/*.json; do
    name="$(basename "$src")"
    cp -f "$src" "$IOS_DIR/$name"
    echo "  $name"
    copied=$((copied + 1))
done

if [[ $copied -eq 0 ]]; then
    echo "error: no JSON files found in $API_DIR" >&2
    exit 1
fi

echo "synced $copied legal document(s) from API → iOS"
