# 0050. Serve the SEO pages live from Go via an apex Cloudflare Worker

Date: 2026-10-08

## Status

Accepted. Supersedes [memo 0015](../memo/0015-seo-pages-go-ssr-via-cloudflare-worker-proxy.md) and the snapshot pipeline of [ADR 0031](0031-decouple-seo-rendering-via-blob-snapshot.md).

## Context

The programmatic SEO pages (`/planning`, `/planning/<authority>`, `/planning/<authority>/<town>`, `/planning/towns`, `/sitemap.xml`) are static HTML. A daily job (`seo-refresh.yml`) calls two build-key endpoints about 1,968 times, writes a blob snapshot, renders it with `web/scripts/prerender-planning.mjs` and deploys it to the Static Web App on `towncrierapp.uk`.

That pipeline has stopped being viable:

- The prod fetch grew from 16.7 min (2026-09-04) to 37-44 min (2026-10-07) as the applications table grew. The 45 min job timeout cancelled the prod job on 2026-10-03, 10-06 and 10-07, so prod pages went stale.
- Server time on `GET /v1/applications/near` alone doubled in a month (15.4 to 30.4 min per run): the nearest-town query cost scales with authority size times town count, and the job repeats it for every town.
- Pages are at best a day old, and per-application SEO pages can never be built statically.

The URLs are established in search indexes and must not change.

Two facts constrain how traffic reaches Go on the apex:

- Cloudflare Origin Rules cannot override the Host header or SNI below Enterprise, which killed the second-SWA design (ADR 0031 Phase 0). A Worker needs no override: it fetches a real hostname.
- An Azure managed certificate cannot renew behind the Cloudflare proxy (ADR 0048). This is already live: on 2026-10-08 `dev.towncrierapp.uk` (proxied since 2026-07-21) returns 526 because its SWA origin no longer serves a `dev.towncrierapp.uk` certificate. The prod apex certificate (expires 2027-02-06) would fail the same way once the apex is proxied, and Static Web Apps cannot bind an Origin CA certificate.

Traffic is small. Prod SWA `SiteHits` across all paths peaked at 8,978/day over the 30 days to 2026-10-08, under 9% of the Workers Free daily quota.

## Decision

1. **Go renders the SEO pages live.** A new `internal/seopage` package (sibling of `internal/sharepage`) serves every `/planning/*` page and `/sitemap.xml` as `html/template` output from Postgres. URLs, canonicals, titles, H1s, meta descriptions, JSON-LD and sitemap `<loc>` values stay identical to the static output.
2. **A precomputed catalog decides the page set.** A worker mode recomputes, daily, which authorities and towns publish (coverage gate, population floor, same-name 301s, duplicate slugs), their status breakdowns, neighbour links and sitemap `lastmod`. Each town's applications are assigned to it once and stored, by an hourly incremental worker mode, so a town page is an indexed read. Request-time work is one catalog lookup plus one bounded application read.
3. **One Worker fronts the whole apex.** It routes `towncrierapp.uk/*`: `/planning`, `/planning/*` and `/sitemap.xml` go to the Go API with the build key as a Worker secret; every other path goes to the SWA's default `*.azurestaticapps.net` hostname. The apex is orange-clouded. No origin depends on an Azure managed certificate behind the proxy.
4. **Workers Free plan only.** No paid Cloudflare product. A free rate-limiting rule on `/planning*` (verified bots excluded) protects the daily quota from a single abusive client.
5. **Fail closed, no permanent static fallback.** The route fails closed; a soft-404 SPA shell is worse for SEO than a short error. Resilience comes from the Worker's edge cache with stale-if-error.
6. **Build and prove side by side, then cut over in one step.** The complete stack (Worker, catalog, renderer) first serves only preview hostnames (`preview.towncrierapp.uk`, `preview-dev.towncrierapp.uk`) marked `noindex`, and a parity script compares every live URL with its preview twin. Cutover adds the real-host Worker routes and orange-clouds the apex; the Worker carries no feature flag. Rollback for one week is grey-clouding the apex, which returns the still-deployed static pages; after that the snapshot pipeline is deleted.
7. **Cloudflare config ships through CI.** The Worker source and routes live in the repo and deploy through GitHub Actions with a scoped Cloudflare API token, never by hand. DNS proxy status stays outside Pulumi, as for the other hosts.

## Consequences

- SEO pages are fresh: authority pages are live; town pages lag new applications by at most an hour.
- The snapshot pipeline goes away: `seo-refresh.yml`, the `seo-snapshot` blob containers, the `render-mode` plumbing in `build-web`, `prerender-planning.mjs` and its render libraries, and the two build-key JSON endpoints.
- Page rendering moves from JavaScript to Go. Templates, design tokens and the QR block must be ported once, with a parity check before cutover.
- The whole website now depends on the Worker. A Worker fault or quota exhaustion takes down every apex path, not only the SEO pages. The free quota has more than ten times headroom today; the rate rule and Cloudflare usage analytics guard it.
- The dev site 526 is fixed by the same Worker on `dev.towncrierapp.uk` at cutover; dev stays down until then.
- The prod pages keep coming from the failing snapshot job until cutover.
- Unknown `/planning/*` paths return a real 404 instead of today's 200 SPA shell.
- Per-application SEO pages become possible later without new infrastructure.
