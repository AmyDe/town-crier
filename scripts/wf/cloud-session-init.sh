#!/bin/bash
# Per-session init for Claude Code cloud (web) sessions. Runs from the
# SessionStart hook in .claude/settings.json; exits immediately on local
# machines, so it is a committed no-op for Mac development.
#
# Why this exists (and why none of it can live in the environment setup
# script): cloud environments snapshot the filesystem after the setup script
# runs ONCE, and later sessions reuse the snapshot with setup skipped
# entirely. Environment variables (including the DOLT_CLOUD_CREDS_* secrets)
# are injected per-session and are NOT present while the setup script runs.
# The git proxy port changes every session, so a remote rewrite done at
# snapshot time would bake in a dead port. And a hydrated .beads/dolt in the
# snapshot would serve week-stale backlog data. So the setup script installs
# binaries only (bd, dolt), and everything session-specific happens here.
#
# Three jobs, all idempotent so resumed sessions re-run this safely:
#   1. Install DoltHub credentials from DOLT_CLOUD_CREDS_ID/_B64. Without
#      them, dolt pushes anonymously: reads succeed (the DB is public) but
#      bd dolt push fails with rpc PermissionDenied. Must happen before the
#      first bd command — the dolt sql-server reads ~/.dolt/creds at startup.
#   2. Repo fingerprint: bd doctor hashes the configured origin URL against
#      the repo_id stored in the shared (DoltHub-synced) metadata table
#      (fb4ae834 = sha256 of "github.com-amyde/AmyDe/town-crier", minted
#      from the Mac's SSH-alias remote). Set origin to that same URL string
#      and route actual git traffic through the live session proxy with an
#      insteadOf rewrite. Never "fix" this with bd migrate --update-repo-id
#      from a cloud session — repo_id syncs via DoltHub and rewriting it
#      would break the Mac on its next pull.
#   3. Hydrate the Dolt-backed issue DB from the DoltHub remote (fresh
#      clone), or pull to freshen one restored from the environment cache.

[ "${CLAUDE_CODE_REMOTE:-}" = "true" ] || exit 0
set -euo pipefail

repo_root="${CLAUDE_PROJECT_DIR:-/home/user/town-crier}"

# 1. DoltHub credentials
if [ -z "${DOLT_CLOUD_CREDS_ID:-}" ] || [ -z "${DOLT_CLOUD_CREDS_B64:-}" ]; then
  echo "cloud-session-init FATAL: DOLT_CLOUD_CREDS_ID/_B64 not set in this session." >&2
  echo "bd dolt push will run anonymously and DoltHub will deny it (PermissionDenied)." >&2
  echo "Check the cloud environment's variable configuration. Do NOT work around this" >&2
  echo "by skipping pushes — unpushed bd mutations vanish with this container." >&2
  exit 1
fi
if [ ! -f "$HOME/.dolt/creds/$DOLT_CLOUD_CREDS_ID.jwk" ]; then
  mkdir -p "$HOME/.dolt/creds"
  printf '%s' "$DOLT_CLOUD_CREDS_B64" | base64 -d > "$HOME/.dolt/creds/$DOLT_CLOUD_CREDS_ID.jwk"
  chmod 600 "$HOME/.dolt/creds/$DOLT_CLOUD_CREDS_ID.jwk"
  # dolt creds ls will show a key ID derived from the JWK contents, not
  # $DOLT_CLOUD_CREDS_ID — that mismatch is normal.
  dolt config --global --add user.creds "$DOLT_CLOUD_CREDS_ID"
fi

# 2. Repo fingerprint / origin rewrite
canonical="git@github.com-amyde:AmyDe/town-crier.git"
current_origin="$(git -C "$repo_root" remote get-url origin)"
if [ "$current_origin" != "$canonical" ]; then
  proxy_prefix="${current_origin%AmyDe/town-crier*}"
  git -C "$repo_root" remote set-url origin "$canonical"
  git -C "$repo_root" config url."$proxy_prefix".insteadOf "git@github.com-amyde:"
fi

# 3. Beads DB: hydrate or freshen
chmod 700 "$repo_root/.beads"
cd "$repo_root"
if [ -d .beads/dolt/town_crier ]; then
  bd dolt pull
else
  # bd bootstrap's metadata-reconcile step reads the server port file, which
  # doesn't exist yet on a fresh clone — start the server first to write it.
  bd dolt start
  bd bootstrap
fi

echo "cloud-session-init: dolt creds installed, origin fingerprint set, beads DB ready"
