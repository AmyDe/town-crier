# 0039. Anonymous Per-IP Rate Limiting

Date: 2026-07-08

## Status

Accepted

## Context

Town Crier is adding an anonymous browse mode (GitHub issue #868): a fresh-install
user reaches the map screen with real planning applications near their postcode
*before* creating an account. This requires a new public, unauthenticated endpoint
(`GET /v1/applications/near-point`) that serves "applications near a point".

A public point+radius geo endpoint is an attractive scraping target. An abusive
caller can tile the UK with overlapping radius queries and dump the whole
applications table, and — worse for us — drive heavy load onto PlanIt
(planit.org.uk), the free, single-operator upstream that is our sole planning-data
provider. Hammering PlanIt is a non-negotiable red line for this project.

Today anonymous routes carry **no rate limiting at all**. `middleware.RateLimit`
keys on the authenticated subject and passes unauthenticated requests through
unmetered. To meter anonymous callers we need a stable per-caller key, and the only
one available is the client IP address: anonymous callers have no account, no API
key, and we collect no cookie or device identifier from them.

The `internal/clientip` package already resolves the genuine client IP behind
Cloudflare (trusting `CF-Connecting-IP` only when the TCP peer is a published
Cloudflare edge range, ignoring the spoofable `X-Forwarded-For`). It was built and
tested but left **deliberately unwired**: its package doc records that recording a
client IP anywhere requires a Privacy Policy update and a Legitimate Interests
Assessment (LIA) *first*, because the current policy discloses no IP handling and
the UK GDPR data-minimisation principle (Art. 5(1)(c)) forbids collecting personal
data ahead of a documented need. A client IP is personal data under UK GDPR. This
ADR is that assessment and decision; it unblocks Phase 1 (the per-IP limiter
itself) of issue #868.

### Legitimate Interests Assessment

The lawful basis for this processing is legitimate interests (UK GDPR
Art. 6(1)(f)), consistent with the "keep the service reliable and secure" basis
already stated in the Privacy Policy. The three-part assessment:

1. **Purpose test — is there a legitimate interest?** Yes. The purpose is abuse
   prevention for a public, unauthenticated API endpoint. A point+radius geo query
   is an attractive scraping target: tiling the UK with radius queries can enumerate
   the whole applications table and, in doing so, drive runaway load onto PlanIt,
   the free single-operator upstream we are bound not to hammer. Keeping the service
   available for genuine users, and protecting a third party's free service from
   abuse routed through us, are both legitimate interests — of Town Crier, of its
   other users, and of PlanIt.

2. **Necessity test — is the processing necessary?** Yes. There is no less-intrusive
   way to identify an abusive anonymous caller. Anonymous users have no account, no
   API key, and we deliberately collect no cookie or device identifier from them
   (consistent with the public "we do not track" stance). The client IP is the only
   stable per-caller signal available. Radius and result-limit clamps on the
   endpoint bound the cost of any single request but cannot distinguish one abusive
   caller making thousands of well-formed requests from ordinary traffic; that
   requires per-caller accounting, and the IP is the only key we have.

3. **Balancing test — do the individual's interests override the legitimate
   interest?** No, because the processing is minimal and proportionate. The resolved
   IP is held **transiently in memory only**, in a rate-limit accounting map. It is
   **never logged, never persisted** to disk or database, and **never exported**. It
   is used **solely** for rate-limit accounting — no profiling, no cross-request
   identity linking beyond the counting window, no enrichment, no sharing. Stale
   entries are **evicted** once their window expires, so an IP is discarded shortly
   after the caller's last request. The individual's privacy interest is protected
   by that transience and the narrow single purpose; there is no lasting record and
   nothing that could identify or track a person over time. Against that minimal,
   short-lived processing sits the interest of every other user in the service
   staying up, and of PlanIt not being abused via runaway scraping through our API.
   The latter outweighs the former.

This carve-out **preserves** the existing "no client IP logging" stance. It is a
rate-limiting-only, in-memory use of the resolved IP; it is not a reversal of the
policy that IP addresses are not written to logs, telemetry, or storage. Nothing in
this decision permits logging, persisting, or exporting a client IP.

### Prior art

- **#517** — earlier work on metering anonymous routes.
- **#518** (closed) — the per-subject rate-limit map once grew unbounded because
  keys were never evicted. The new per-IP limiter must evict from day one; that is a
  correctness requirement here, not only a memory-hygiene nicety, because an unbounded
  map keyed on client IP would also amount to retaining IP data indefinitely.
- `internal/clientip` package doc — records the deliberate GDPR gap this ADR closes.

## Decision

Adopt **per-IP rate limiting on anonymous (unauthenticated) API routes**, keyed on
the client IP resolved by `internal/clientip`, held transiently in memory only,
used solely for rate-limit accounting.

- The resolved client IP is used **only** as a rate-limit key in an in-memory
  accounting structure. It is never written to a log line, span, metric label,
  database, or export.
- **Stale entries are evicted** once their window expires (regression guard for
  #518). The accounting map is bounded; an IP leaves memory shortly after the
  caller stops sending requests.
- The limiter applies **only** to anonymous traffic (no authenticated subject on the
  request context). Authenticated traffic keeps the existing per-subject
  `middleware.RateLimit` and is never keyed on IP.
- The Privacy Policy is updated in the same phase to disclose this transient
  in-memory use of the client IP for rate limiting / abuse prevention (done
  alongside this ADR, per issue #868 Phase 0).

The limiter mechanism, limits, eviction implementation, and wiring are Phase 1 of
issue #868 and are specified there; this ADR records the decision and its lawful
basis, not the code.

## Consequences

### Easier

- The public `near-point` endpoint (Phase 2) can ship with a proportionate defence
  against whole-table scraping and against runaway load reaching PlanIt.
- `internal/clientip` moves from "built but unwired" to "wired for one documented,
  assessed purpose", with the LIA on record to cite in any future data-protection
  review.
- The same anonymous limiter unblocks other future anonymous surfaces (the embed
  widget, the redesigned public search in #863) without re-doing the privacy work.

### Harder

- Client IP is now processed, where previously it was resolved-but-never-used. The
  transient-in-memory, never-logged, never-persisted constraints are load-bearing
  for the lawful basis: any future change that logs, persists, or exports the IP,
  or reuses it for anything beyond rate-limit accounting, is outside this
  assessment and needs its own LIA and Privacy Policy update. The `clientip`
  package doc's prohibition on logging/storing/exporting the value still stands.
- Eviction is mandatory, not optional. A limiter that never evicts would both
  repeat the #518 unbounded-map bug and undermine the balancing test by retaining
  IP data indefinitely.
- Callers whose IP cannot be resolved as genuinely Cloudflare-forwarded fall into a
  shared conservative bucket (Phase 1 detail); a legitimate caller reaching us by an
  unusual path could be throttled alongside others in that bucket. Accepted as
  proportionate for launch.

## See also

- GitHub issue [#868](https://github.com/AmyDe/town-crier/issues/868) — anonymous
  browse mode (this is Phase 0; the limiter is Phase 1).
- `api-go/internal/clientip` package doc — the deliberate GDPR gap this ADR closes.
- Issues #517, #518 — prior anonymous-route metering and rate-limit-eviction history.
- The Privacy Policy disclosure shipped with this ADR
  (`api-go/internal/legal/resources/privacy.json`).
