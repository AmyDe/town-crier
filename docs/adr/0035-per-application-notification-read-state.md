# 0035. Per-application notification read state

Date: 2026-07-01

## Status

Accepted

Supersedes the single per-user last-read watermark read-state design (the read-state
assumptions of [0033](0033-server-authoritative-watch-zone-querying.md)). Builds on
[0032](0032-consolidate-datastore-on-postgres-postgis.md). Graduates memo
[0012](../memo/0012-notification-read-state-per-application.md).

## Context

Notification read state was a single per-user watermark:
`notification_state(user_id PK, last_read_at timestamptz, version int)`, one row per
user, with a notification unread iff `notifications.created_at > last_read_at`. The
watermark was chosen to dodge Cosmos per-write economics — one timestamp move instead
of N per-item writes. Three SQL sites computed the unread rule (the total unread badge
count, `GetLatestUnreadByApplications` for the per-application badge, and the zonepage
recent-activity / unread-filter subquery, which INNER JOINed `notification_state` so a
user with no watermark row contributed no unread rows). Read state also moved forward
implicitly as the user scrolled, via a `POST .../advance` endpoint.

Two problems motivated a change:

- **The product wants genuine tap-to-read.** Opening an application should clear only
  that application's unread state, which a single monotonic watermark cannot express —
  a watermark can only say "everything before instant T is read".
- **The cost constraint is gone.** [0032](0032-consolidate-datastore-on-postgres-postgis.md)
  consolidated all data on a fixed-price Azure Postgres Burstable `Standard_B1ms`. There
  is no per-operation billing, so per-row read writes are effectively free at our scale
  (memo [0012](../memo/0012-notification-read-state-per-application.md) analyses this and
  concludes the change is net-neutral to net-negative on DB load: a self-pruning partial
  index replaces a growing `created_at > last_read_at` scan, and retiring scroll-to-clear
  for tap-to-read reduces write frequency).

The client contract exposes no notification IDs — clients operate on applications keyed
by `application_uid` — so per-notification-row read state (with a new list endpoint and
ID exposure) was rejected in favour of per-application granularity (memo 0012, option B).

## Decision

Track read state per application on the `notifications` table via a nullable
`read_at timestamptz`. A notification is unread iff `read_at IS NULL`.

- **Migration 0015** (shipped separately, ahead of the API) adds the column, backfills
  it so the unread set is identical at the flip (`read_at = last_read_at` where
  `created_at <= last_read_at`; `read_at = created_at` for users with no watermark row,
  who read as all-read today), and adds the partial index
  `idx_notifications_unread (user_id, application_uid, created_at) WHERE read_at IS NULL`.
- **New endpoint** `POST /v1/me/applications/mark-read` with body
  `{"applications": [{"applicationUid", "authorityId"}]}` clears the caller's unread rows
  for those applications and returns `204` (idempotent). The array is forward-compatible
  (clients send one today); an empty array marks nothing — never "all". It is scoped by
  the composite `(application_uid, authority_id)`: `application_uid` is the bare
  per-council PlanIt ref and is **not** unique across authorities, so matching on the uid
  alone (user-scoped) would clear the wrong council's rows. The endpoint uses a fixed path
  (no path parameter) because application UIDs contain slashes that Go's `ServeMux` cannot
  route.
- **Mark-all-read** (`POST /v1/me/notification-state/mark-all-read`) clears every unread
  row for the user.
- The three unread SQL sites are repointed to `read_at IS NULL`. The zonepage subqueries
  drop the `notification_state` JOIN but keep `GROUP BY application_uid, authority_id` and
  the outer join on both keys, so they stay authority-safe. The "untouched user has no
  unread" behaviour is preserved by the backfill (existing no-watermark users' history is
  marked read), while a genuinely new notification correctly reads as unread.
- **`notification_state` is retained** (table and `version`) as an opaque **change token**:
  `version` bumps on any read-state mutation — unconditionally on mark-all, and on
  mark-read only when it actually cleared a row — so the client's BadgeSync still detects
  change. The mutation and its version bump run atomically in a single data-modifying CTE.
  `last_read_at` is vestigial (kept only for GET DTO shape stability) and no longer drives
  unread.
- **`GET /v1/me/notification-state` no longer seeds a first-touch watermark row** (it must
  not write): for a user with a row it returns that row's `last_read_at` + `version`; for a
  user with none it returns `version 0` and `lastReadAt` computed at `now` (not persisted).
  The DTO shape `{ lastReadAt, version, totalUnreadCount }` is unchanged; `totalUnreadCount`
  is now the `read_at IS NULL` tally.
- **`advance` (scroll-to-clear) is removed** — endpoint, handler, and `State.AdvanceTo`.
  Unread now clears only via tap-to-read or mark-all, giving one source of truth.

## Consequences

- Tap-to-read is expressible: opening an application clears exactly that application's
  unread notifications, not everything older than a timestamp.
- The hot unread paths (badge count, unread filter) scan the self-pruning partial index
  instead of comparing `created_at > last_read_at` over a growing set, so the read path is
  slightly cheaper; per-row read writes are single-row, indexed, and human-paced.
- Cross-authority safety is now explicit in the mutation path (composite scoping). The
  **display** path (`GetLatestUnreadByApplications` and the watch-zone nearby handler) still
  correlates the per-row unread badge by bare `application_uid` alone, a pre-existing
  collision left unchanged here and tracked as a follow-up.
- Because GET no longer seeds a watermark, brand-new users' incoming notifications read as
  unread immediately (the intended behaviour). The watch-zone nearby handler still gates its
  per-row unread badge on the presence of a `notification_state` row, so a brand-new user's
  per-row badges there lag until a mark-read/mark-all creates the row — a known follow-up,
  out of scope for this change.
- Clients (iOS, web) must add `markApplicationRead` and delete `advance`; these ship as
  separate PRs. Until they do, the `advance` route returns 404.
- `notification_state`/`version` can be dropped in a later cleanup once `version` is
  confirmed unused; GDPR erasure is unchanged (`read_at` is a column on `notifications`
  rows already removed by erasure).
