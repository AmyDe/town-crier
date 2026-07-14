#!/usr/bin/env bash
#
# prod-read.sh — read-only query against town_crier_prod.
#
# Exists so that an agent session can verify a deploy against production data
# without being handed a general-purpose psql shell. Three things make it safe
# enough to sit behind a permission allowlist entry:
#
#   1. The connection sets default_transaction_read_only=on, so the SERVER
#      rejects any write regardless of what SQL is passed in.
#   2. The statement is checked against a SELECT/WITH-only allowlist before it
#      is sent, so a write attempt fails loudly here rather than silently
#      being a no-op.
#   3. Auth is the caller's own Entra token (az account get-access-token), so
#      access is exactly the caller's, and expires. No stored password.
#
# Usage:
#   scripts/wf/prod-read.sh "SELECT count(*) FROM applications;"
#   scripts/wf/prod-read.sh -f query.sql
#   scripts/wf/prod-read.sh --csv "SELECT ..."
#
set -euo pipefail

readonly PGHOST_PROD="psql-town-crier-shared.postgres.database.azure.com"
readonly PGDB_PROD="town_crier_prod"

die() { printf 'prod-read: %s\n' "$*" >&2; exit 1; }

fmt_args=(-P pager=off)
sql=""

while [ $# -gt 0 ]; do
  case "$1" in
    --csv)  fmt_args+=(--csv); shift ;;
    --tsv)  fmt_args+=(-tAF $'\t'); shift ;;
    -f)     [ $# -ge 2 ] || die "-f needs a file"
            [ -r "$2" ] || die "cannot read $2"
            sql="$(cat "$2")"; shift 2 ;;
    -h|--help)
            sed -n '3,20p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *)      sql="$1"; shift ;;
  esac
done

[ -n "$sql" ] || die "no SQL given (pass a string or -f file)"

# Reject anything that isn't a read. Strip comments and leading whitespace
# first so "-- hi\nDELETE" can't sneak past.
stripped="$(printf '%s\n' "$sql" \
  | sed -e 's/--.*$//' \
  | tr '\n' ' ' \
  | sed -e 's/^[[:space:]]*//')"
case "$(printf '%s' "$stripped" | tr '[:upper:]' '[:lower:]')" in
  select*|with*|explain*|table*|show*) : ;;
  *) die "read-only: statement must begin with SELECT/WITH/EXPLAIN/TABLE/SHOW" ;;
esac
case "$(printf '%s' "$stripped" | tr '[:upper:]' '[:lower:]')" in
  *insert\ *|*update\ *|*delete\ *|*drop\ *|*alter\ *|*truncate\ *|*create\ *|*grant\ *|*copy\ *)
    die "read-only: statement contains a write keyword" ;;
esac

command -v psql >/dev/null || die "psql not on PATH"
command -v az   >/dev/null || die "az not on PATH"

upn="$(az ad signed-in-user show --query userPrincipalName -o tsv 2>/dev/null)" \
  || die "not signed in to az (run: az login)"
token="$(az account get-access-token --resource-type oss-rdbms --query accessToken -o tsv 2>/dev/null)" \
  || die "could not mint an oss-rdbms token"

# default_transaction_read_only=on is the real guard — the checks above are
# there to fail fast and legibly, this is what makes a write impossible.
PGOPTIONS="-c default_transaction_read_only=on" PGPASSWORD="$token" exec psql \
  "host=${PGHOST_PROD} port=5432 dbname=${PGDB_PROD} user=${upn} sslmode=require connect_timeout=15" \
  "${fmt_args[@]}" -v ON_ERROR_STOP=1 -c "$sql"
