# 0015. Serve SEO pages live from the Go API via a Cloudflare Worker proxy

Date: 2026-07-13

## Status

Open

## Question

The programmatic SEO pages (`/planning/<authority>/`, `/planning/<authority>/<town>/`,
`sitemap.xml`) are statically prerendered from a daily blob snapshot
([ADR 0031](../adr/0031-decouple-seo-rendering-via-blob-snapshot.md)) and deployed to
the Static Web App on `towncrierapp.uk`. Separately, the share pages are rendered live
by the Go API (`internal/sharepage`) on `share.towncrierapp.uk` — a model that is
simpler, always fresh, and keeps rendering in one codebase.

Can we serve the SEO pages the same way — live Go-rendered HTML — **on the existing
`towncrierapp.uk` URLs**, so the established indexing is untouched, at near-zero cost
on the Cloudflare Free plan? And if free-plan limits ever bite, what does the paid
escape hatch actually cost?

## Analysis

### Why the ADR 0031 Phase-0 NO-GO does not apply here

The Phase 0 spike (2026-06-21, GH #598) rejected Cloudflare path-routing because the
design routed to a **second Static Web App**. SWA routes strictly by `Host`, so the
proxy hop needed an Origin Rules `host_header` / `origin.host` override — and both are
entitlement-blocked on the Free plan (Enterprise-only, see cost section below). The
remaining mechanism, a Worker, was declined *in that context* because it added edge
compute and a request ceiling **on top of a second SWA**.

Both objections dissolve when the routed-to origin is the Go API:

- A Worker needs no host-override entitlement. It proxies by `fetch()`ing a real
  hostname (e.g. `api.towncrierapp.uk` or a dedicated SEO host on the container app),
  so the `Host` header is correct by construction.
- There is no second SWA. The origin is the container app we already run at
  min-replica 1, rendering Go templates exactly as `internal/sharepage` does today.

### Proposed design

1. **`internal/seopage`** — a sibling of `internal/sharepage` serving the authority
   page, town page, and `sitemap.xml` as Go templates over Postgres. The page set,
   URLs, canonicals, titles, and H1s stay byte-comparable to the prerendered output
   (markup may change; head-metadata parity avoids ranking wobble).
2. **Orange-cloud the apex and `www`** so Cloudflare proxies them (today they are
   DNS-only to the SWA). Proxying SWA under Full (strict) is already proven:
   `dev.towncrierapp.uk` ran proxied during the Phase 0 spike and served fine, and
   `api`/`api-dev` run proxied today. Side benefit: edge caching and DoS absorption
   for the whole site.
3. **A small Worker** on routes `towncrierapp.uk/planning/*`,
   `www.towncrierapp.uk/planning/*`, and `/sitemap.xml`, which fetches the matching
   Go endpoint and serves the response with edge caching (`s-maxage` of an hour or
   so), so crawler bursts never reach Postgres. All other paths continue to the SWA
   untouched.
4. **Decommission the snapshot pipeline** once cut over: the daily `seo-refresh.yml`
   job, the `seo-snapshot` blob containers and storage accounts, the 1,674-line
   `prerender-planning.mjs`, and the `render-mode` plumbing in `build-web` /
   `cd-dev` / `cd-prod` all go away.

### What this buys

- **Indexing continuity is total** — same host, same paths, same canonicals.
- **Freshness moves from ~1 day to same-minute.** A same-day planning application is
  on its town page as soon as polling stores it.
- **One rendering codebase.** SEO pages join share pages as Go templates; the
  fetch/render split, snapshot seeding rules, and "build fails on missing blob"
  failure mode all disappear.
- **Unlocks per-application SEO pages.** Millions of application URLs can never be
  statically prerendered; a live renderer (which `sharepage` essentially already is)
  can serve them. Static generation caps that play permanently.

### Free-plan limits and failure modes

- **Workers Free: 100,000 requests/day per account**, resetting 00:00 UTC. The
  corpus is ~1,950 pages; crawler plus current organic traffic is two orders of
  magnitude below the ceiling. Edge cache hits still count as Worker invocations, so
  caching protects the origin, not the quota.
- **On exhaustion** Cloudflare returns error 1027 (5XX) for Worker routes until the
  daily reset — but the route can be configured to **fail open**, passing requests
  through to the origin (the SWA). During transition we can leave the last static
  corpus deployed on the SWA so a quota blowout degrades to day-stale pages instead
  of errors. Long-term the answer is the $5/month Workers Paid plan (below), and
  100k/day of organic traffic would be a good problem to have.
- **Cloudflare config stays outside Pulumi.** DNS, proxy status, Worker, and routes
  are managed via scoped API token, same as the existing `api`/`api-dev` proxying.

### Cost of leaving the free tier (researched 2026-07-13)

The important structural finding: **no affordable zone-plan upgrade ever unlocks the
Origin Rules path.** `host_header`, `origin.host`, and SNI overrides are
Enterprise-only ([Origin Rules features](https://developers.cloudflare.com/rules/origin-rules/features/)),
and Enterprise is sales-led (typically thousands per month). Pro and Business do not
include them. The Worker is therefore the right mechanism at every price point, and
the only upgrade this design could ever need is **Workers Paid**.

| Plan / product | Price (USD, 2026-07) | ~GBP | What it adds for this design |
|---|---|---|---|
| Free zone + Workers Free | $0 | £0 | Everything needed: proxy, routes, Worker, 100k req/day |
| **Workers Paid** | **$5/mo (account-wide)** | **~£4/mo** | Removes the daily ceiling: 10M req/mo + 30M CPU-ms included, then $0.30/M req + $0.02/M CPU-ms; no egress charges |
| Pro zone | $20/mo annual ($25 monthly) | ~£16–20/mo | Nothing this design needs. Includes Snippets (25 snippets, 2 subrequests, 5 ms CPU, 32 KB) which could do the proxy without a request ceiling — but strictly worse value than Workers Paid |
| Business zone | $200/mo annual ($250 monthly) | ~£160–200/mo | Still no host-header override; irrelevant |
| Enterprise | Sales-led | — | Unlocks Origin Rules host override, removing the Worker entirely — never economic for this |

Scale check on Workers Paid: sustained ceiling-level traffic (100k/day ≈ 3M/mo) fits
inside the included 10M requests, so the realistic worst case is a **flat $5/month**.
The proxy Worker uses well under 1 ms CPU per request, so CPU overage is not a
factor. Sources: [Workers pricing](https://developers.cloudflare.com/workers/platform/pricing/),
[Workers limits](https://developers.cloudflare.com/workers/platform/limits/),
[Snippets](https://developers.cloudflare.com/rules/snippets/),
[zone plans](https://www.cloudflare.com/plans/).

## Options Considered

- **A. Worker proxy on the apex path (this memo).** Free today, $5/mo at worst,
  keeps all URLs, deletes the snapshot pipeline, unlocks per-application pages.
- **B. Go API as the site's front door.** Apex points at the container app; Go
  serves SEO pages and proxies/embeds the SPA. No Cloudflare ceiling, but couples
  web deploys to API deploys, puts all marketing traffic on a min-1 single-region
  app, and loses the SWA CDN. Too heavy.
- **C. SWA Standard linked backend.** ~$9/month per environment to link the
  container app and rewrite routes into it. Fails the near-zero bar; the paid
  fallback if Cloudflare proxying ever became untenable.
- **D. Keep static serving, move rendering into Go.** The same `internal/seopage`
  endpoints, but the daily job snapshots their rendered HTML to blob instead of
  JSON, retiring the node prerender. No Cloudflare changes, no ceilings — but
  freshness stays daily, the pipeline machinery survives, and per-application pages
  remain impossible. The sane fallback if the spike hits another entitlement
  surprise.
- **E. Move SEO pages to a subdomain with 301s.** Transfers most equity over weeks
  but gambles established rankings for no saving over A. Rejected.

## Recommendation

Run a Phase-0-style spike first, given the last free-plan entitlement surprise: a
trivial Worker on `dev.towncrierapp.uk/planning/*` fetching from `api-dev`,
confirming Worker routes + Full (strict) + SWA coexistence on the Free plan end to
end (~an hour). If it passes, build in this order: `internal/seopage` handlers
(porting page content from `prerender-planning.mjs`), Worker + apex/`www`
orange-cloud cutover with the static corpus left as fail-open fallback, then
decommission the snapshot pipeline. Budget trigger: upgrade to Workers Paid
($5/month) if sustained traffic approaches ~70k requests/day, before the ceiling
bites.
