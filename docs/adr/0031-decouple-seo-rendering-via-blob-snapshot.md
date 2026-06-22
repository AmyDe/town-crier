# 0031. Decouple SEO page rendering from the Go API via a weekly Blob snapshot

Date: 2026-06-22

## Status

Accepted

Supersedes the follow-up `tc-75qo` (web-deploy job approaching its timeout) and the
original Cloudflare-path-routing / dedicated-Static-Web-App approach to the SEO
decouple (epic `tc-w5w9` / [GH #598](https://github.com/AmyDe/town-crier/issues/598)).
Builds on the programmatic SEO pages introduced by SEO Phase 1 (epic `tc-nnte`) and
the per-town gazetteer of SEO Phase 2 (epic `tc-2avw` / GH #585).

## Context

The programmatic SEO pages — `/planning/<authority>/`,
`/planning/<authority>/<town>/`, and `sitemap.xml` — are statically prerendered by
`web/scripts/prerender-planning.mjs`, wired into `npm run build` through the
`build-web` composite action. That build runs in **three** places: `cd-dev.yml`
(every push to main), `cd-prod.yml` (every prod tag), and `seo-refresh.yml`
(weekly).

The prerender fetched its data from the **live, gated Go API at build time**: one
call per authority and one per population-eligible town, with each town costing two
Cosmos reads (`RecentNearby` + `CountNearby`). After SEO Phase 2 the gazetteer is
**1,550 GB towns** (~540 eligible at the default `SEO_TOWN_MIN_POPULATION` ≥ 20,000)
on top of ~410 authorities, so **every release fired ~1,900 serial Go API calls**,
and that number grows each time the population threshold is lowered to ramp
coverage.

This was wasted load on the user-facing API (which runs at min-replica 1), needless
Cosmos RU, and it pushed the `web-deploy` job toward its timeout (the open follow-up
`tc-75qo`). Crucially, the weekly cadence chosen in `tc-2avw.6` only governed the
**standalone** `seo-refresh` schedule — it never stopped `cd-dev`/`cd-prod` from
running the full prerender on every release, so the "rebuild the large static corpus
weekly, not on every release" intent was not actually met.

**Root cause:** the prerender coupled *data fetching* to *page rendering*, and the
build ran on every release. The fix is to decouple the two — fetch weekly against
the API, render every build offline.

### The original fix was a Phase 0 NO-GO

The first design split the SEO corpus onto a dedicated Static Web App fronted by
**Cloudflare path-routing**. The Phase 0 spike (resolved 2026-06-21) proved this is
not viable on the current plan:

- Cloudflare proxy of the web host already works — `dev.towncrierapp.uk` is proxied
  (orange) under Full(strict) and serves the app fine.
- But the zone is on the **Free** plan, and Origin Rules' `host_header` **and**
  `origin.host` overrides are both entitlement-blocked on Free (*"not entitled to
  use the HostHeader/Origin Host override"*). Azure SWA routes strictly by `Host`,
  so a second SWA cannot be path-routed to without those overrides.
- The only remaining Cloudflare mechanism — a Worker (Free 100k req/day) — adds edge
  compute, a request ceiling, and a second SWA, and was declined.

The Phase 0 verdict (see the GH #598 comment dated 2026-06-21) was to **pivot to the
blob-snapshot design** below, dropping the Cloudflare-split / dedicated-SWA approach
entirely.

## Decision

**Decouple data fetching from rendering via a weekly snapshot in Azure Blob
Storage.**

### Fetch / render split (web)

Split `web/scripts/prerender-planning.mjs` into two modes that share the existing,
unchanged page-generation logic:

- `--fetch` (snapshot mode): call the gated Go API for all authorities and all
  population-eligible towns and write a single `seo-snapshot.json` containing
  everything needed to render every page plus the sitemap. Emits no HTML.
- `--render` (offline mode): read `seo-snapshot.json` from disk and render all
  `/planning/*` pages + `sitemap.xml` with **zero** network calls.

Only *where the data comes from* changes (live API → local snapshot); URLs,
canonicals, and the page set are unchanged.

### Snapshot storage (infra)

A **per-environment Azure Storage account + `seo-snapshot` blob container** in
`rg-town-crier-{dev,prod}` (`sttowncrierdev` / `sttowncrierprod`), provisioned in
`/infra` (Pulumi Go) — the first storage-account resource type in the project, a
tiny Standard_LRS account. **Shared-key access is disabled**; all blob I/O is
AAD/RBAC via `--auth-mode login`. The CI OIDC identity holds **Storage Blob Data
Contributor** (weekly job writes) and **Storage Blob Data Reader** (builds read);
the assignment requires **User Access Administrator** on the resource group to
manage the role.

### Weekly job is the sole API caller

`seo-refresh.yml` (weekly, both envs) does: Azure OIDC login → run the prerender in
`--fetch` mode (passing `SITE_BUILD_KEY` + `SEO_TOWN_MIN_POPULATION`) → upload
`seo-snapshot.json` to the environment's blob → render + deploy. This is the **only**
thing that ever calls the Go API.

### Every CD build renders offline

`build-web` gained a `render-mode` input (default off, preserving today's
behaviour). When on, the caller runs `azure-login` first, then `build-web` builds the
SPA only (forcing `SITE_BUILD_KEY` empty so the build-time prerender skips),
downloads `seo-snapshot.json` from Blob, and renders `/planning/*` + `sitemap.xml`
offline. **A missing snapshot fails the build** — there is deliberately no
live-fetch fallback, because falling back would re-introduce the per-release Go API /
Cosmos load this change removes (GH #598 acceptance: "fails loudly if the snapshot is
missing").

### Single SWA, no Cloudflare changes

There remains **one** Static Web App per environment with unchanged URLs and
canonicals. No second SWA, no Cloudflare Worker, no request ceiling, no host/origin
overrides.

### Phased rollout

The cutover is staged behind the "seed before cutover" safety rule:

- **dev first (this ADR / bead `tc-w5w9.10`):** `cd-dev.yml`'s `web-deploy` job moves
  `azure-login` ahead of `build-web`, drops `site-build-key`, and sets
  `render-mode: 'true'` + `storage-account-name: sttowncrierdev`. The dev blob is
  seeded by a weekly-job run before cutover.
- **prod deferred (bead `tc-so05`):** `cd-prod.yml` is **not** touched here. Prod
  storage does not exist yet — it is created on the next `cd-prod` release — and the
  prod blob must be seeded before its cutover. Cutting prod over now would fail every
  release on a missing snapshot.

## Consequences

### Easier

- **Zero per-release Go API / Cosmos calls.** Once cut over, `cd-dev`/`cd-prod`
  builds render `/planning/*` from the local snapshot — the ~1,900 serial gated API
  calls per release are gone, removing the load on the min-replica-1 API and the
  Cosmos RU spend, and they no longer grow as the population threshold drops.
- **Supersedes `tc-75qo`.** Rendering from local JSON removes the API-bound latency
  that pushed `web-deploy` toward its timeout, so that follow-up is resolved by this
  design rather than by tuning the timeout.
- **The weekly-corpus intent is finally met.** Page data is at most ~1 week stale
  between weekly runs, which is exactly the "rebuild the large corpus weekly" cadence
  `tc-2avw.6` aimed for and is fine for SEO pages.
- **One SWA, no Cloudflare entanglement.** URLs and canonicals are unchanged; there
  is no second SWA, no edge Worker, and no Free-plan entitlement dependency.

### Harder

- **Net-new resource type.** The project now owns its first Azure Storage account(s),
  a small additional surface and cost, plus a per-environment blob to keep seeded.
- **CI principal needs blob-data RBAC.** The OIDC identity needs Storage Blob Data
  Contributor/Reader, and managing those assignments needs User Access Administrator
  on each resource group — extra IAM surface to maintain.
- **Builds depend on a fresh snapshot.** A build now fails loud if the blob is
  missing (by design). Bootstrapping a new environment requires seeding the blob
  (run the weekly job once) **before** flipping that environment's CD to render-mode;
  rollback for an unseeded/stale blob is to re-enable the build-time fetch
  (`SITE_BUILD_KEY`) until reseeded.
- **Two-phase cutover to track.** dev and prod cut over in separate beads; until
  `tc-so05` lands, prod still runs the build-time prerender on every tag.

## See also

- [GH #598](https://github.com/AmyDe/town-crier/issues/598) — epic `tc-w5w9`: the
  full blob-snapshot design and the Phase 0 Cloudflare NO-GO verdict.
- Epic `tc-2avw` / [GH #585](https://github.com/AmyDe/town-crier/issues/585) — SEO
  Phase 2 per-town gazetteer, which created the per-release load this ADR removes.
- `tc-75qo` — the web-deploy-timeout follow-up superseded by this decision.
</content>
</invoke>
