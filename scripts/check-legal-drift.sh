#!/usr/bin/env bash
# Verify the downstream legal JSON mirrors are byte-identical to the canonical
# API copies. Run by .github/workflows/legal-drift-check.yml on every PR.
#
# Mirrors guarded:
#   - iOS bundle (mobile/ios .../Resources/legal)
#   - Go API embedded copy (api-go/internal/legal/resources) — the Go module
#     cannot go:embed files outside its own tree, so it carries its own copy.
#
# If this fails, run `scripts/sync-legal.sh` and commit the result.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

API_DIR="$REPO_ROOT/api/src/town-crier.application/Legal/Resources"
IOS_DIR="$REPO_ROOT/mobile/ios/packages/town-crier-presentation/Sources/Resources/legal"
GO_DIR="$REPO_ROOT/api-go/internal/legal/resources"

# Each mirror is "label:path"; the canonical API_DIR is checked against every one.
MIRRORS=(
    "iOS:$IOS_DIR"
    "Go:$GO_DIR"
)

failed=0

if [[ ! -d "$API_DIR" ]]; then
    echo "error: missing directory: $API_DIR" >&2
    exit 1
fi

shopt -s nullglob
api_files=("$API_DIR"/*.json)

if [[ ${#api_files[@]} -eq 0 ]]; then
    echo "error: no JSON files found in $API_DIR" >&2
    exit 1
fi

for mirror in "${MIRRORS[@]}"; do
    label="${mirror%%:*}"
    dir="${mirror#*:}"

    if [[ ! -d "$dir" ]]; then
        echo "error: missing $label directory: $dir" >&2
        failed=1
        continue
    fi

    mirror_files=("$dir"/*.json)
    if [[ ${#api_files[@]} -ne ${#mirror_files[@]} ]]; then
        echo "error: file count differs — API has ${#api_files[@]}, $label has ${#mirror_files[@]}" >&2
        echo "  API: $API_DIR" >&2
        echo "  $label: $dir" >&2
        failed=1
    fi

    for src in "${api_files[@]}"; do
        name="$(basename "$src")"
        target="$dir/$name"
        if [[ ! -f "$target" ]]; then
            echo "error: missing $label mirror for $name at $target" >&2
            failed=1
            continue
        fi
        if ! diff -q "$src" "$target" >/dev/null; then
            echo "error: legal docs drifted between API and $label — $name" >&2
            echo "  API: $src" >&2
            echo "  $label: $target" >&2
            echo "  fix: run 'scripts/sync-legal.sh' and commit the result" >&2
            failed=1
        fi
    done
done

if [[ $failed -ne 0 ]]; then
    exit 1
fi

echo "legal docs in sync (${#api_files[@]} file(s) × ${#MIRRORS[@]} mirror(s))"
