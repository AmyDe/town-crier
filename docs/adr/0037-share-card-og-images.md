# 0037. Bake share-page OG images server-side from OpenStreetMap tiles, cached once

Date: 2026-07-01

## Status

Accepted

## Context

The public share page (#738, Slice 1, ADR-adjacent) needs a rich social-unfurl
image (`og:image`) so a shared planning-application link renders as a large map
card on social media. With ~589k applications, per-application images cannot be
prerendered (the reason the page itself is dynamic, ADR 0031). Slice 1 shipped a
static branded placeholder as the `og:image`; Slice 2 replaces it with a real
map of the application's location.

We need a 1200×630 image per application, showing a map centred on the app's
`(latitude, longitude)` with a pin and the required OpenStreetMap attribution,
generated on demand. The web SPA already renders OpenStreetMap raster tiles in
the user's browser (ADR 0006 attribution requirements apply). The open question
was how to produce the baked image: a third-party static-map provider (e.g. a
hosted static-map API) versus compositing raw OSM tiles ourselves.

## Decision

The Go API bakes the card itself and serves it at `GET /og/{authoritySlug}/{ref...}.png`
(registered as `GET /og/{authoritySlug}/{ref...}`; the handler enforces and trims
the `.png` suffix, because a Go `ServeMux` trailing-wildcard segment cannot carry
literal text after it).

- **Raw OSM over a provider.** We composite raster tiles fetched directly from
  `tile.openstreetmap.org` with an identifying `User-Agent`
  (`TownCrier/1.0 (+https://towncrierapp.uk)`), using stdlib `image`/`image/draw`
  plus `golang.org/x/image` (`font/opentype` + `gofont`) for anti-aliased text.
  A pin is drawn at the app coordinates and "© OpenStreetMap contributors" is
  burned into the image. No new third-party sub-processor is introduced.
- **Cache-once.** The PNG is generated on first request and stored via a Get/Put
  storage seam; thereafter it is served from the store without re-fetching tiles.
  It is regenerated only when absent. A Cloudflare edge cache sits in front
  (Slice 3). This keeps OSM tile volume trivially small — a few tiles per
  *shared* application, once — well within the OSM Tile Usage Policy, and lighter
  than the SPA already is on OSM.
- **Swappable seam.** Tile fetching is behind a `tileClient` interface and
  storage behind an `imageStore` interface, so a static-map provider or a
  different cache backend can be substituted later without touching the handler.
- **Graceful degradation.** When the store is unwired (the Azure `share-cards`
  Blob container + managed-identity RBAC land in Slice 3), the seam regenerates
  on every request and logs. This let Slice 2 ship ahead of the infra slice.
- **Branded fallback.** An application with null coordinates yields a branded
  1200×630 fallback PNG (no tiles fetched), so `og:image` is always valid.

## Consequences

- Rich, map-based unfurls with no new commercial dependency or API key, and no
  new sub-processor beyond OpenStreetMap (already used by the web client).
- A **new server-side data flow**: our servers fetch OSM tiles when generating a
  share card (previously OSM was only contacted by the user's browser). No
  personal data is sent — only tile coordinates derived from a public
  application's location. The Privacy Policy is updated to disclose this.
- Adds `golang.org/x/image` as a dependency for font rasterisation.
- OSM load is bounded by cache-once + edge caching; if OSM volume or policy ever
  becomes a problem, the `tileClient` seam allows swapping to a provider without
  a rewrite.
- The full Blob-backed cache and the `share.towncrierapp.uk` edge cache rule are
  deferred to Slice 3; until then the card regenerates per request behind the
  API host, which is acceptable at pre-launch volume.
