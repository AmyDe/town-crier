# 0040. Single-source design tokens (design/tokens.json + generator + CI drift gate)

Date: 2026-07-09

## Status

Accepted

## Context

Town Crier's design tokens — colours, spacing, typography, radii, shadows,
durations — were hand-maintained in five separate places with no shared source:
the SPA sheet (`web/src/styles/tokens.css`), the SEO/share-page block inside
`web/scripts/lib/render-shared.mjs` (`pageStyles()`), and the iOS, Android and
Go copies. With no single source they drifted. The web `--tc-status-conditions`
value (`#A85A0A` light / `#FF9500` dark) had diverged from the iOS and Android
`statusConditions` (`#B85C00` / `#FF9F0A`), so the same "Conditions" status
rendered a different amber on the website than in the apps.

The SPA sheet also duplicates the light theme twice (once under
`[data-theme="light"]`, once under the `prefers-color-scheme: light` auto-detect
block); keeping those two copies in step by hand is exactly the kind of
error-prone bookkeeping a generator removes.

This is the first slice (T1) of the "Public Notice" rebrand epic (#848). It
establishes the single source and converts the two web surfaces. Mobile
(Swift/Kotlin) and Go emitters are deliberately deferred to follow-up slices
(T2/T3), which is why the generator lives at the repo root, not under `web/`.

## Decision

Adopt a single source of truth plus a committed generator and a CI drift gate,
following the same shape as the legal-docs sync (`scripts/check-legal-drift.sh`)
and the SEO blob-snapshot decoupling (ADR 0031):

- **`design/tokens.json`** is the single, hand-ordered source of truth for every
  design token. It supports base references (`{ "base": "amber" }`), scalar and
  per-theme alpha (`{ "base": "amber", "alpha": 0.15 }`), and a `statusBuckets`
  map recording the three-bucket status-chip vocabulary (decision 4, #794) for
  the SEO/share/mobile emitters.
- **`scripts/design-tokens/generate.mjs`** is a dependency-free Node script
  (stdlib only) that regenerates the committed outputs idempotently. `--check`
  exits non-zero if any committed output is stale.
- **Generated files are committed to git.** Builds never invoke the generator:
  the SEO renderer reads the committed lib during CD, and mobile builds must not
  depend on a Node toolchain.
- **A CI drift gate** (`scripts/check-design-token-drift.sh` +
  `.github/workflows/design-token-drift-check.yml`) fails any PR that edits the
  tokens or a generated file without regenerating.

In this slice the generator emits the two web surfaces only:
`web/src/styles/tokens.css` and a new `web/scripts/lib/tokens.generated.mjs`
(consumed by `pageStyles()`). The conversion is zero visual change except one
intentional reconciliation: web `status-conditions` moves to the iOS+Android
canonical `#B85C00` / `#FF9F0A` (two of three platforms already agreed; the web
values were the drift).

### Rejected alternatives

- **Style Dictionary (or another token framework).** It adds an npm dependency
  and its own configuration DSL to drive what are six small, bespoke emitters.
  The hand-rolled generator is a single readable file with zero dependencies,
  easier to audit and to extend with the Go/Swift/Kotlin emitters in T2/T3.
- **Runtime token loading** (shipping `tokens.json` and resolving at runtime).
  The SEO and share pages must remain self-contained static HTML with no runtime
  JS dependency for styling (Core Web Vitals, reader-mode/print robustness — see
  `pageStyles()` and ADR 0031). Committed, pre-generated output keeps the pages
  self-contained and the mobile builds Node-free.

## Consequences

- One edit to `design/tokens.json` plus a regenerate keeps the SPA and SEO
  surfaces in step; the two hand-maintained light-theme copies can no longer
  drift from each other.
- A PR that changes a token but forgets to regenerate fails the drift gate
  loudly, the same way a legal-docs edit does — no silent drift.
- The `status-conditions` reconciliation is the only visual change; every other
  custom-property value is byte-identical before and after.
- The generator is the seam for T2/T3: the same `tokens.json` and `statusBuckets`
  map will drive Go, Swift and Kotlin emitters without a second source.
- One caveat carried forward for T2: the SEO neutral status chip currently
  resolves to `text-secondary` (its shipped value), whereas `statusBuckets.neutral`
  records `status-withdrawn`. Keeping the SEO neutral on `text-secondary` is what
  makes this slice zero-visual-change; reconciling the two is future work for the
  mobile/share emitters, not this slice.
