#!/usr/bin/env bash
# Verify the iOS bundle's legal JSON files are byte-identical to the canonical
# API copies. Run by .github/workflows/legal-drift-check.yml on every PR.
#
# If this fails, run `scripts/sync-legal.sh` and commit the result.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

API_DIR="$REPO_ROOT/api/src/town-crier.application/Legal/Resources"
IOS_DIR="$REPO_ROOT/mobile/ios/packages/town-crier-presentation/Sources/Resources/legal"

failed=0

for dir in "$API_DIR" "$IOS_DIR"; do
    if [[ ! -d "$dir" ]]; then
        echo "error: missing directory: $dir" >&2
        exit 1
    fi
done

shopt -s nullglob
api_files=("$API_DIR"/*.json)
ios_files=("$IOS_DIR"/*.json)

if [[ ${#api_files[@]} -eq 0 ]]; then
    echo "error: no JSON files found in $API_DIR" >&2
    exit 1
fi

if [[ ${#api_files[@]} -ne ${#ios_files[@]} ]]; then
    echo "error: file count differs — API has ${#api_files[@]}, iOS has ${#ios_files[@]}" >&2
    echo "  API: $API_DIR" >&2
    echo "  iOS: $IOS_DIR" >&2
    failed=1
fi

for src in "${api_files[@]}"; do
    name="$(basename "$src")"
    target="$IOS_DIR/$name"
    if [[ ! -f "$target" ]]; then
        echo "error: missing iOS mirror for $name at $target" >&2
        failed=1
        continue
    fi
    if ! diff -q "$src" "$target" >/dev/null; then
        echo "error: legal docs drifted between API and iOS — $name" >&2
        echo "  API: $src" >&2
        echo "  iOS: $target" >&2
        echo "  fix: run 'scripts/sync-legal.sh' and commit the result" >&2
        failed=1
    fi
done

if [[ $failed -ne 0 ]]; then
    exit 1
fi

echo "legal docs in sync (${#api_files[@]} file(s))"
