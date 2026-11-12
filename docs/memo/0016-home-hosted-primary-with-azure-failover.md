# 0016. Home-hosted primary with Azure pilot-light failover

Date: 2026-11-12

## Status

Open

## Question

Can Town Crier cut its hosting bill by running the production stack (API, workers, Postgres) on owner-operated hardware, with Azure kept only as a failover target? The aim is a "nearly free" steady state that still survives a home power, network or hardware failure without manual recovery.

The question is timely because the Azure Database for PostgreSQL free-tier meters end on **2027-03-21**, which adds about £13.74/month to a bill that is otherwise flat.

## Analysis

### Current spend

Azure Cost Management, October 2026 actuals, GBP:

| Item | Now /month | After 2027-03-21 /month | Notes |
|---|---:|---:|---|
| Azure Monitor | 10.10 | 10.10 | 15 log alert rules £7.30, 2 web tests £2.80 |
| Container Apps, prod | ~8.90 | ~8.90 | Always-on API replica, poll job |
| Container Apps, dev | ~1.30 | ~1.30 | Scales to zero |
| Container Registry Basic | 3.80 | 3.80 | Flat rate |
| Storage | 0.90 | 0.90 | `share-cards` container |
| Email (ACS) | 0.10 | 0.10 | |
| PostgreSQL B1ms, 32 GB | 0.00 | 13.74 | Free-tier meters end |
| **Total** | **~25** | **~39** | |

Observations:

- Compute is not the largest line. Alerts and web tests cost more than the prod API.
- Log Analytics and App Insights ingestion and retention bill £0. A self-hosted telemetry stack (Grafana, Loki, Tempo) cannot save on storage; it can only replace the alert rules and web tests.
- Web tests already fell from about £25/month (August, September) to £2.81 in October.
- `share-cards` holds about 148,000 blobs (78.8 GB) with no lifecycle policy. It grew 22 GB in 30 days (about 1,400 cards/day), so it costs about £1.25/month now and will reach about £5 to £6/month in a year if nothing changes.

### What owner hardware changes

Bandwidth is irrelevant. Prod serves 33,000 to 47,000 requests/day, which needs a few Mbit/s at peak. Uptime and latency of the link matter; its speed does not.

The real gain is storage performance. The current server is B1ms (1 burstable vCore, 2 GiB RAM, P4 disk with 120 baseline IOPS) against a 13.8 GB database. Measured in November: cold reads at about 1,100 blocks/s at queue depth 1, worst large-zone map 5.6 s after the thin table, 30 s timeouts before it. A Mac mini class machine (16 GB or more RAM, NVMe) keeps most or all of the database in memory, so the cold-read problem largely disappears. This is a performance argument as much as a cost argument.

### Hybrid shapes rejected

- **Postgres at home, API in Azure.** Every query round-trips UK South to home. A home outage is still a full outage. Worst of both.
- **Workers at home, API and Postgres in Azure.** Saves only job cost (£1 to £2/month). A home outage stops polling and digests, and Postgres must accept a home IP.
- **Hot standby on Flexible Server.** Flexible Server cannot be a physical replica of an external primary. Logical replication does not carry DDL or sequences, needs an inbound path to the home server, and still costs £13.74/month, which is the whole saving.

### Proposed architecture

```
                 Cloudflare (api.towncrierapp.uk, proxied CNAME)
                    |  normal                         |  failover
                    v                                 v
  HOME (active)                          AZURE UK South (standby)
  Owner hardware                         ACA API app, min=0, image from GHCR
   └ Linux arm64 VM                      ACA worker jobs, disabled until failover
      ├ Postgres 16 + PostGIS            Pilot-light VM (arm64), deallocated
      ├ api + worker (systemd timers)      └ same Postgres image, own disk
      ├ cloudflared                      Controller job (ACA, every minute)
      └ WAL-G → Azure Blob ──────────────► Blob: WAL archive, base backups,
                                                leader lease, share-cards
                                         App Insights, ACS email (unchanged)
```

**Normal operation.** The home server serves all traffic through a Cloudflare Tunnel (no open port, no static IP). WAL-G or pgBackRest ships WAL to Blob with `archive_timeout = 60` plus a nightly base backup. Each night the controller starts the pilot-light VM, replays WAL since its last run, runs a check query and stops it. That is a restore test every night for about £0.20/month of compute.

**Failover.** The controller sees home is down and takes the leader lease, starts the VM, replays the remaining WAL, promotes Postgres, enables the ACA jobs and repoints the proxied CNAME from the tunnel to the ACA hostname (seconds to take effect). Clients keep using `api.towncrierapp.uk`.

### Standby options

| Standby | Idle cost /month | RPO | RTO | Verdict |
|---|---:|---|---|---|
| Flexible Server logical subscriber | £13.74 | Seconds | Minutes | No (see above) |
| VM always-on streaming replica | ~£6 to £10 | < 1 s | 1 to 2 min | Viable if a hot standby is wanted |
| **Pilot-light VM + WAL archive** | **~£2 to £4** | **≤ 60 s** | **10 to 15 min** | **Recommended** |
| Nightly dump into a new Flexible Server | < £1 | ≤ 24 h | > 1 h | No |

VM prices are estimates and need checking. The 12-month free VM allowance also ends 2027-03-21.

### Design rules

1. **Same Postgres platform on both sides.** Physical replication and WAL restore need the same major version, CPU architecture and collation library. Text index order differs between the macOS C library and glibc, so a native macOS Postgres restored onto Linux can produce silently corrupt indexes. Run Postgres in a Linux arm64 VM at home and on an arm64 Azure VM, pinned to one image digest.
2. **One writer, enforced by an Azure Blob lease.** The home server holds a 60 s lease on a `leader` blob and renews it every 15 s. If it cannot renew for 40 s it fences itself: stops the API and workers and sets Postgres read-only. The controller promotes Azure only after home health has failed for 5 minutes **and** it has acquired the lease itself, which proves home stopped renewing. A returning home server that does not hold the lease stays fenced until it is re-seeded as a replica.
3. **One poller.** `POLLING.md` allows one polling environment at a time; two prod pollers means two pacers and breaks the 60 s spacing. Every worker checks the lease and `pg_is_in_recovery()` before any PlanIt call. Azure jobs stay disabled until failover.
4. **Failback is planned and manual.** At a quiet hour the home server becomes a replica of Azure, catches up, then switches over with zero data loss.
5. **Unplanned failover loses up to 60 s of writes.** That can mean one duplicate push or one lost notification row. The day-window poller re-reads its windows and still alerts on events up to 14 days old, so no planning events are missed.

### Failure cases

| Event | Behaviour | Data loss | User outage |
|---|---|---|---|
| Planned OS update | Planned switchover or authenticated restart | 0 | 0 to 2 min |
| Power cut within UPS runtime | None | 0 | 0 |
| Power cut beyond UPS runtime | Clean shutdown, lease expires, failover | ≤ 60 s | 10 to 15 min |
| Home internet down, power on | Self-fence at 40 s, failover | ≤ 60 s | 10 to 15 min |
| Home hardware failure | Failover, rebuild later | ≤ 60 s | 10 to 15 min |
| Azure outage while home is active | WAL archive, email, telemetry pause | 0 | 0 |
| Cloudflare outage | Both paths down (same as today) | 0 | Yes |

If Blob is unreachable, WAL accumulates on the home disk. Archive lag and free disk need alerts.

### Steady-state cost

| Item | /month |
|---|---:|
| Blob: WAL, base backups, lease | £0.50 to £1 |
| Share cards with a lifecycle rule | £0.50 to £1.50 |
| Pilot-light VM disk (deallocated) | £2 to £4 |
| Nightly restore test | ~£0.20 |
| ACA API and jobs at min=0, controller job | ~£0 (controller estimated inside the monthly ACA grant) |
| Alert rules after trimming | £3 to £5 |
| Email, App Insights ingestion | ~£0.10 |
| GHCR, Cloudflare Tunnel, Workers | £0 |
| **Azure total** | **~£6 to £11** |
| Electricity and UPS (home) | £3 to £4 |

Saving against the post-March baseline: about £25 to £30/month, £300 to £350/year.

### Constraints and open questions

- **Privacy policy.** `privacy.json` names Azure Database for PostgreSQL and "Azure's default encryption" for data at rest. Data stays in the UK, but the policy must change (via the `legal` skill) before cutover, and the home server needs documented UK GDPR Article 32 measures.
- **Identity.** `internal/platform/postgres/pool.go`, `internal/blobstore/store.go` and `internal/pgbackup/blobsink.go` use `azidentity`. The home stack needs password auth for Postgres and a service principal for Blob and ACS. Azure Arc (free tier) can give managed identity to a Linux server; arm64 support inside a VM is unverified.
- **No self-hosted GitHub runner.** The repo is public, so fork PRs could run code on the home network. Deploys to the home server must be pull-based.
- **PlanIt source IP.** Prod polling and local sessions would share one IP. `POLLING.md` keeps local traffic unaffiliated with prod so a block on one cannot stop the other; that separation weakens if PlanIt blocks by IP.
- **Unattended restart.** The host must boot and start services after a power cut without a person present. The disk-encryption and unlock approach needs a decision; with failover in place, an unlock delay no longer causes a user outage.
- **Flexible Server cannot ship physical WAL out.** The one-off migration is `pg_dump`/`pg_restore` with a short write freeze (night slot), or logical replication.
- **Container runtime.** Colima, Apple `container` and Docker Desktop (small-business licence) are free; OrbStack needs a paid commercial licence. Keep the Postgres data directory on the VM's own disk, not a host bind mount.
- **Side finding.** About 1,400 new share cards/day suggests crawlers bake most of them via `/og`, and each bake fetches OpenStreetMap tiles. This rate has not been checked against the OSM tile usage policy.

## Options Considered

1. **Stay on Azure and trim.** Move images to GHCR (£3.84), trim alert rules (£2 to £4), add a `share-cards` lifecycle rule. Saves £6 to £9/month with almost no risk. Leaves the £13.74 Postgres jump in March.
2. **Home primary, Azure pilot-light failover (this memo).** Saves £25 to £30/month after March and fixes cold-read performance. Costs a substantial build (lease fencing, controller, WAL shipping, portable auth) and ongoing host maintenance. RPO ≤ 60 s, RTO 10 to 15 min.
3. **Home primary, Azure hot standby VM.** Same as 2 with RPO under 1 s and RTO 1 to 2 min, for about £4 to £6/month more.
4. **Single cloud VPS with local NVMe** (UK or EU) running API, workers and Postgres. Similar savings and the same performance gain without home risks, but no failover unless a second site is added, and the privacy policy still changes.
5. **Home primary with no failover.** Cheapest, but any home outage is a full outage for paying users. Rejected.

## Recommendation

1. Do option 1 now regardless; it is independent of everything else.
2. Treat option 2 (pilot light) as the target, phased with gates:
   - **Phase 1:** make the code portable (password DSN, service principal path, lease and `pg_is_in_recovery()` guard in workers, a non-ACA scheduler).
   - **Phase 2:** run the home stack as **dev** for 4 weeks with nightly restore tests. Gate: every test passes and RTO is measured.
   - **Phase 3:** controller and fencing, tested on dev by pulling network, pulling power, stopping the VM and blocking Blob. Run in alert-only mode for 2 weeks before allowing automatic promotion.
   - **Phase 4:** prod cutover at night, keeping Flexible Server as the rollback. A stopped Flexible Server restarts itself after 7 days.
   - **Phase 5:** remove Flexible Server and the always-on ACA replica.
3. Aim to cut over by mid-February 2027 so the Flexible Server rollback path is still free during the soak.
4. Decide before Phase 1: automatic versus manual promotion (recommended: automatic behind the lease after the alert-only period), pilot light versus hot standby (recommended: pilot light), and the privacy policy wording.

If a decision lands, graduate this memo to an ADR.
