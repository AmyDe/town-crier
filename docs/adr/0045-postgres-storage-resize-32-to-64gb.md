# 0045. Resize shared Postgres storage from 32 GiB to 64 GiB

Date: 2026-07-23

## Status

Accepted

## Context

`psql-town-crier-shared` (Standard_B1ms, Burstable, UK South) is provisioned at 32 GiB
(`infra/shared.go:308-311`, `Storage.StorageSizeGB: pulumi.Int(32)`, `StorageType_Premium_LRS`).
It hosts both `town_crier_prod` and `town_crier_dev` on one disk with storage auto-grow
**disabled** — hitting 100% makes Postgres read-only, an outage for a paying customer.

Lane D (ADR 0042, historical backward backfill) has no storage-aware stop condition — it
only self-terminates after 12 consecutive empty 90-day windows (~3 years of national
silence), which won't happen; PlanIt holds data back to 1977. Left running, it will
eventually try to hold the full national dataset, estimated at roughly 20 million
application rows.

Measured 2026-07-23 (live queries against prod):

- `applications` currently holds 921,906 rows at 1,484.6 bytes/row (heap + all 10
  indexes + toast) — confirmed by direct measurement, not extrapolation.
- Extrapolated to 20,000,000 rows at the same per-row rate: **~27.65 GB** for that table
  alone.
- Non-`applications` baseline (server `storage_used` metric minus the `applications`
  table's own bytes) is **~4.9 GB** — `town_crier_dev` (709 MB), prod's other tables,
  WAL, and platform overhead not attributable to any specific database content (no
  stuck replication slots; WAL settings are modest, `max_wal_size=2GB`,
  `wal_keep_size=400MB`).
- Projected total at 20M rows: **~32.56 GB — over the current 32 GB ceiling**, even
  before adding any safety margin.

Three cost-cutting alternatives were researched before choosing to resize:

1. **Reduce backup retention/geo-redundancy.** Dead end for this goal — confirmed
   against Microsoft's own documentation that Azure Postgres Flexible Server backup
   storage is billed and tracked separately from provisioned data storage. It never
   counts against the 32 GB ceiling that triggers read-only mode, so cutting it saves
   nothing here (retention is already a lean 7 days, geo-redundancy already disabled).
2. **Drop `town_crier_dev`.** Frees 709 MB. Real but small next to the ~4.9 GB
   non-`applications` baseline, and negligible next to the ~27.65 GB table itself.
3. **Prune non-core indexes on `applications`.** Indexes are 702 MB of the table's
   1,305 MB (54%). Dropping the three search-feature indexes (`applications_address_trgm`
   162 MB, `applications_description_fts` 71 MB, `applications_uid_lower_pattern` 38 MB
   — backing GH #821 search) and two SEO indexes (`applications_authority_real_date`
   69 MB, `applications_authority_recent` 56 MB — backing GH #819 authority pages)
   would bring the projection to ~23.2 GB, fitting within 32 GB. But this is a real
   product trade-off, not free cleanup: at 20M rows, losing the backing index doesn't
   make those endpoints slower, it makes them full sequential scans on every request —
   a functional break for shipped, live features, not a graceful degradation.

Storage on this SKU (`ManagedDisk`/Premium SSD, confirmed live via the Azure
capabilities API for `Standard_B1ms` in `uksouth`) only scales in fixed tier jumps —
32 → 64 → 128 → 256 GiB — not arbitrary GB amounts, so the minimum possible increase
is a doubling to 64 GiB.

**Cost.** The Postgres free-tier benefit (12 months free from subscription creation,
expires 2027-03-21 — `project_postgres_free_tier_billing` memory, confirmed via two
independent methods against Cost Management and the billing account's `createdAt`) is a
fixed 32 GB storage quota, already fully consumed by the current provisioning. It is not
an all-or-nothing cliff: the original 32 GiB stays on the zero-rated "Storage Data
Stored - Free" meter exactly as today, and only the incremental 32 GiB above it bills at
the standard rate. Compute (`B1MS Compute - Free`) is a separate meter, unaffected by
storage size, and stays free regardless.

Live rate, confirmed just now via the Azure Retail Prices API for `uksouth` (GBP,
`Az DB for PostgreSQL Flexible Server Storage` / `Storage Data Stored` meter) and
matching the closed-out cost-forecast measurement from 12 days prior: **£0.1008/GB/month**.

- Extra 32 GiB × £0.1008 = **£3.23/month**, starting immediately on resize.
- Post-2027-03-21 (once the whole free tier lapses regardless of this decision): 64 GiB
  total would cost £10.51 (B1MS compute) + 64 × £0.1008 = £6.45 (storage) = **£16.96/month**,
  versus £13.74/month had we stayed at 32 GiB. The incremental cost of this decision at
  that point is the same £3.23/month, just no longer partially free.

At 64 GiB, the 20M-row projection (~32.56 GB) uses roughly half the new ceiling, with
headroom to spare — without touching the search or SEO indexes, and without dropping
`town_crier_dev`.

## Decision

Resize `psql-town-crier-shared` storage from 32 GiB to 64 GiB via Pulumi
(`infra/shared.go:309`, `Storage.StorageSizeGB: pulumi.Int(32)` → `pulumi.Int(64)`).
Resize is online (`onlineResizeSupported: Enabled` per the Azure capabilities API) — no
downtime expected. Ship through the normal PR → CI → deploy path per this repo's
PR-only deployment policy; do not resize by direct `az`/`pulumi` CLI call against prod.

Do not also pursue the index-pruning or dev-removal options as a substitute — they were
evaluated and rejected as the primary lever *for this specific goal* because they trade
away shipped product features (search, SEO authority pages) for savings that the £3.23/month
resize makes unnecessary. Dev removal and index hygiene may still be worth doing later on
separate merits (tidiness, query performance), just not as a precondition for fitting
20M rows.

## Consequences

- **Irreversible direction.** Azure Postgres Flexible Server storage only grows, never
  shrinks. This is a one-way commitment to at least 64 GiB going forward.
- **Immediate small cost.** £3.23/month starts accruing on resize, on top of the
  otherwise-still-free 32 GiB and B1MS compute.
- **Buys headroom, not a permanent fix.** ~32.56 GB projected need at 20M rows leaves
  real margin inside 64 GiB, but Lane D still has no storage-aware stop condition — if
  the eventual national row count is materially higher than the ~20M estimate, this
  ceiling will need revisiting too. Re-measure the per-row rate and baseline overhead
  periodically as Lane D progresses rather than treating this projection as final.
- **No change to backup cost or retention** — confirmed orthogonal to this decision.
- **IOPS**: the P4→P6 tier jump raises baseline IOPS from 120 to 240 as part of the
  storage tier itself; this is included in the storage price above and does not trigger
  the separate "IOPS Scaling" meter as long as provisioned IOPS are left at each tier's
  default.
