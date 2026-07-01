# 0012. Per-application notification read state (replacing the last-read watermark)

Date: 2026-07-01

## Status

Superseded by ADR [0035](../adr/0035-per-application-notification-read-state.md)

## Question

Now that all data lives on Postgres + PostGIS (ADR [0032](../adr/0032-consolidate-datastore-on-postgres-postgis.md)) and we no longer pay per-operation Cosmos RUs, should we replace the single per-user "last read" watermark with per-notification read tracking, so that opening a notification marks just that item read? And does the extra write volume increase the cost of running the application, especially at scale?

## Analysis

### Current model

Read state today is a single per-user watermark, not a per-item flag:

- `notification_state(user_id PK, last_read_at timestamptz, version int)` — one row per user.
- A notification is unread iff `notifications.created_at > last_read_at`.
- The `notifications` table carries no read column (only `push_sent` / `email_sent` delivery booleans). Notification rows are already per-user (`user_id` on every row).
- Three endpoints: `GET /v1/me/notification-state`, `POST /v1/me/notification-state/mark-all-read`, `POST /v1/me/notification-state/advance` (moves the watermark forward as the user scrolls past items).
- The unread rule (`created_at > last_read_at`) is computed in three SQL sites: the total unread badge count, `GetLatestUnreadByApplications` (per-application badge), and the zonepage recent-activity subquery (which INNER JOINs `notification_state` to implement a first-touch clean slate).
- The client contract has no notification IDs. Clients operate on applications (keyed by `application_uid`), not individual notification rows. There is no list-notifications endpoint.

The watermark was almost certainly chosen to dodge Cosmos economics: one timestamp move instead of N per-item writes, and unread-count as a single query rather than a maintained counter. ADR 0032 removed that constraint.

### Cost analysis

The prod database is an Azure Postgres Flexible Server, **Burstable `Standard_B1ms`** (1 vCore, 2 GB RAM, 32 GB storage, HA disabled) — see `infra/shared.go`. This is a fixed monthly cost billed by the hour regardless of query volume. There is no per-operation billing.

Consequences for this change:

- **Writes are free until the tier saturates.** A mark-read is a single-row, indexed `UPDATE ... WHERE user_id = $1 AND application_uid = $2 AND read_at IS NULL` touching (usually) one row. It is human-paced — one write per app-open-with-unread, plus the occasional mark-all — not a poll, background job, or fan-out.
- **Headroom is large.** 1,000 daily-active users clearing 30 notifications/day is 30k writes/day, under 0.5 writes/sec average. A B1ms does not notice that. Write volume alone would not register until tens of thousands of concurrently active users.
- **The scaling ceiling is unchanged.** The workload that forces a tier bump is reads — the PostGIS zone queries, `/clusters` grid snapping, recent-activity sort — not mark-read. When "significant users" arrives, compute is scaled for reads regardless, and mark-read remains a rounding error on top.
- **This is net-neutral to net-negative on load.** We retire `advance` (a scroll-frequency write, potentially chatty) and replace it with tap-to-read (tap-frequency, less often). The partial index `WHERE read_at IS NULL` is self-pruning — read rows drop out of it — so the hot unread-count and unread-filter queries scan a small index instead of comparing `created_at > last_read_at` across a growing set. The read path gets slightly cheaper.
- **Storage delta is negligible.** One nullable `timestamptz` per row (~8 bytes), tens of MB at millions of rows, on £-per-GB storage.

Conclusion: the migration to per-application read state does not materially increase running cost and may slightly reduce DB load. The concern that motivated the watermark (per-write cost) no longer applies.

## Options Considered

**A. Keep the watermark.** Cheapest possible writes and O(1) mark-all, but cannot express "I read this one and not that one." Rejected: the product wants tap-to-read.

**B. Per-application `read_at` column on `notifications` (chosen).** Add `read_at timestamptz NULL`; unread iff `read_at IS NULL`. Mark-one is scoped to an application (`WHERE user_id, application_uid, read_at IS NULL`), matching the existing client contract, which keys everything on `application_uid`. Mark-all becomes a bulk UPDATE over the user's unread rows. No join table needed because notification rows are already per-user. A partial index keeps the hot paths fast. Requires a one-time backfill and rewriting the three unread SQL sites.

**C. True per-notification-row read state.** Full per-item granularity, but the client has no notification IDs today, so it needs a new list-notifications endpoint and an ID exposed to iOS/web — a larger contract change for no product benefit over (B), since the user reads at the application level. Rejected.

### Decisions taken with the owner

- **Granularity: per application.** Tapping into an application marks its unread notifications read. No notification IDs exposed to clients.
- **Explicit only.** Retire `advance` / scroll-to-clear. Unread clears on tap-to-read or mark-all. One source of truth (`read_at`).
- **Keep mark-all as its own endpoint.** Do not overload mark-one with "empty list means all" — that is a mass-mutation footgun.

## Recommendation

Proceed with option B, staged so the change is additive and back-compatible (relevant now we have a paying customer and rollback safety is back on the table — see CLAUDE.md Business Status):

1. **Migration first**: add `read_at`, backfill (`read_at = last_read_at` where `created_at <= last_read_at`; `read_at = created_at` for users with no watermark row; everything after the watermark stays NULL/unread), add the partial index. Old API ignores the column; watermark still works. Safe to deploy alone.
2. **API next**: add `POST /v1/me/applications/{applicationUid}/mark-read`; re-point `mark-all-read` and the three unread SQL sites to `read_at IS NULL`; keep `GET` returning `totalUnreadCount`; remove `advance`. The backfill guarantees the unread set is identical at the flip.
3. **Clients last** (iOS, web): add `markApplicationRead`, wire tap-to-read, delete `advance`. Retire the endpoint once nothing calls it.

Write the superseding ADR as part of step 2. Guardrail worth building in: the client only calls mark-read when the application actually shows an unread badge, avoiding pointless round-trips.
