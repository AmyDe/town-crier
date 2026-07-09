#!/usr/bin/env node
/**
 * Design-token generator (issue #849, ADR 0040).
 *
 * `design/tokens.json` is the single source of truth for every design token.
 * This script reads it and (re)writes the committed, generated outputs listed
 * in OUTPUTS below. It is dependency-free (Node >= 18 stdlib only) so CI and a
 * plain `node` invocation both work with no `npm install`.
 *
 *   node scripts/design-tokens/generate.mjs           regenerate every output
 *   node scripts/design-tokens/generate.mjs --check    fail (exit 1) if any
 *                                                       committed output is stale
 *
 * Generated files are committed to git; builds never invoke this generator (the
 * SEO renderer reads the committed lib, mobile builds must not depend on Node).
 * The `--check` mode mirrors scripts/check-legal-drift.sh semantics: a PR that
 * edits tokens.json (or a generated file) without regenerating fails the gate.
 *
 * T1 (#849) emitted the two web surfaces; T2 (#850) adds the two Go surfaces
 * (the shared internal/designtokens package + the share-page tokens.gohtml);
 * T3 (#851) adds the two mobile colour files (iOS Swift, Android Kotlin) and
 * regenerates the value tables in the design-language skill doc. Every emitter
 * reads the SAME tokens.json and the SAME statusBuckets map, which is why the
 * generator lives at the repo root rather than under web/. Mobile generation is
 * colours only — spacing/radius/typography stay hand-maintained on each platform
 * (ADR 0040).
 */

import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join, relative } from 'node:path';
import { parseArgs } from 'node:util';

const HERE = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = join(HERE, '..', '..');

export const TOKENS_PATH = join(REPO_ROOT, 'design', 'tokens.json');
export const THEMES = ['light', 'dark', 'oled'];

const IOS_COLORS_PATH = join(
  REPO_ROOT,
  'mobile', 'ios', 'packages', 'town-crier-presentation',
  'Sources', 'DesignSystem', 'Colors', 'Color+TownCrier.swift',
);
const ANDROID_COLORS_PATH = join(
  REPO_ROOT,
  'mobile', 'android', 'presentation', 'src', 'main', 'kotlin',
  'uk', 'towncrierapp', 'presentation', 'designsystem', 'Color.kt',
);
const SKILL_DOC_PATH = join(
  REPO_ROOT,
  '.claude', 'skills', 'design-language', 'references', 'tokens.md',
);

// --------------------------------------------------------------------------
// Loading + resolution
// --------------------------------------------------------------------------

/** @returns {any} the parsed tokens.json */
export function loadTokens(path = TOKENS_PATH) {
  return JSON.parse(readFileSync(path, 'utf8'));
}

/**
 * @param {string} hex a `#RRGGBB` string
 * @returns {[number, number, number]}
 */
function hexToRgb(hex) {
  const match = /^#([0-9a-fA-F]{6})$/.exec(hex);
  if (!match) {
    throw new Error(`expected a 6-digit #RRGGBB colour, got: ${hex}`);
  }
  const n = parseInt(match[1], 16);
  return [(n >> 16) & 0xff, (n >> 8) & 0xff, n & 0xff];
}

/**
 * Resolve a colour token to its concrete value for one theme.
 *
 * Three entry shapes are supported:
 *   - direct:    { light, dark, oled }                -> the per-theme hex
 *   - base ref:  { base: "amber" }                    -> that token's hex (per theme)
 *   - alpha ref: { base: "amber", alpha: 0.15 }       -> rgba(r, g, b, 0.15)
 *                { base: "#000000",
 *                  alpha: { light: 0.4, dark: 0.5 } } -> per-theme alpha
 * `base` may name another colour token or be a literal `#RRGGBB`.
 *
 * @param {Record<string, any>} colors the tokens.color map
 * @param {string} name
 * @param {string} theme one of THEMES
 * @returns {string}
 */
export function resolveColor(colors, name, theme) {
  const entry = colors[name];
  if (!entry) {
    throw new Error(`unknown colour token: ${name}`);
  }
  if ('base' in entry) {
    const baseHex = entry.base.startsWith('#')
      ? entry.base
      : resolveColor(colors, entry.base, theme);
    if ('alpha' in entry) {
      const alpha = typeof entry.alpha === 'object' ? entry.alpha[theme] : entry.alpha;
      const [r, g, b] = hexToRgb(baseHex);
      return `rgba(${r}, ${g}, ${b}, ${alpha})`;
    }
    return baseHex;
  }
  const value = entry[theme];
  if (typeof value !== 'string') {
    throw new Error(`colour token ${name} is missing a ${theme} value`);
  }
  return value;
}

// --------------------------------------------------------------------------
// Small formatting helpers
// --------------------------------------------------------------------------

/** A numeric spacing/radius token -> a `px` string. */
function px(n) {
  return `${n}px`;
}

/** Prepend `spaces` to every non-empty line of `text`. */
function indentLines(text, spaces) {
  const pad = ' '.repeat(spaces);
  return text
    .split('\n')
    .map((line) => (line.length > 0 ? pad + line : line))
    .join('\n');
}

/** PascalCase a hyphenated token name: "text-on-accent" -> "TextOnAccent". */
function pascalCase(name) {
  return name
    .split('-')
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join('');
}

/** Two uppercase hex digits for a 0..255 byte, e.g. 10 -> "0A". */
function hexByte(n) {
  return n.toString(16).toUpperCase().padStart(2, '0');
}

/**
 * A "solid" colour token resolves to a `#RRGGBB` hex in every theme; the alpha
 * tokens (amber-muted, overlay) resolve to `rgba(...)` and are not solid.
 *
 * @param {Record<string, any>} colors
 * @param {string} name
 * @returns {boolean}
 */
function isSolidColor(colors, name) {
  return resolveColor(colors, name, 'light').startsWith('#');
}

/** A compact, space-free `rgba()` matching the share-page CSS style. */
function compactRgba(hex, alpha) {
  const [r, g, b] = hexToRgb(hex);
  return `rgba(${r},${g},${b},${alpha})`;
}

/**
 * Lay out `[name, value]` rows the way gofmt aligns a Go const/var block: one
 * tab of indent, every `=` in the same column (names padded with spaces to the
 * widest name), one space either side of `=`. Emitting pre-aligned rows keeps
 * the generated Go file gofmt-clean without shelling out to gofmt.
 *
 * @param {Array<[string, string]>} rows
 * @returns {string}
 */
function alignGoAssignments(rows) {
  const width = Math.max(...rows.map(([name]) => name.length));
  return rows.map(([name, value]) => `\t${name.padEnd(width)} = ${value}`).join('\n');
}

// --------------------------------------------------------------------------
// Emitter: web/src/styles/tokens.css  (the SPA custom-property sheet)
// --------------------------------------------------------------------------

/**
 * Emit the theme-varying custom properties (brand, surfaces, text, status,
 * utility, shadows) that every SPA theme block redefines. `comments` lets each
 * block carry its own section headers ("Brand" vs "Brand — same as dark", etc.)
 * so the generated file matches the hand-written original byte-for-byte.
 *
 * @param {any} tokens
 * @param {string} theme
 * @param {{ brand: string, text: string, status: string, shadow: string }} comments
 * @returns {string} declaration lines at 2-space indent (no trailing newline)
 */
function spaThemedVars(tokens, theme, comments) {
  const c = tokens.color;
  const color = (name) => resolveColor(c, name, theme);
  return [
    `  /* ${comments.brand} */`,
    `  --tc-amber: ${color('amber')};`,
    `  --tc-amber-muted: ${color('amber-muted')};`,
    `  --tc-amber-hover: ${color('amber-hover')};`,
    ``,
    `  /* Surfaces */`,
    `  --tc-background: ${color('background')};`,
    `  --tc-surface: ${color('surface')};`,
    `  --tc-surface-elevated: ${color('surface-elevated')};`,
    ``,
    `  /* ${comments.text} */`,
    `  --tc-text-primary: ${color('text-primary')};`,
    `  --tc-text-secondary: ${color('text-secondary')};`,
    `  --tc-text-tertiary: ${color('text-tertiary')};`,
    `  --tc-text-on-accent: ${color('text-on-accent')};`,
    ``,
    `  /* ${comments.status} */`,
    `  --tc-status-permitted: ${color('status-permitted')};`,
    `  --tc-status-conditions: ${color('status-conditions')};`,
    `  --tc-status-rejected: ${color('status-rejected')};`,
    `  --tc-status-pending: ${color('status-pending')};`,
    `  --tc-status-withdrawn: ${color('status-withdrawn')};`,
    `  --tc-status-appealed: ${color('status-appealed')};`,
    ``,
    `  /* Utility */`,
    `  --tc-border: ${color('border')};`,
    `  --tc-border-focused: ${color('border-focused')};`,
    `  --tc-overlay: ${color('overlay')};`,
    ``,
    `  /* ${comments.shadow} */`,
    `  --tc-shadow-card: ${tokens.shadow.card[theme]};`,
    `  --tc-shadow-elevated: ${tokens.shadow.elevated[theme]};`,
  ].join('\n');
}

/**
 * Emit the theme-invariant custom properties (spacing, radius, typography,
 * animation, layout). Only the default dark `:root` block carries these — the
 * other theme blocks inherit them — so they live in one function appended to
 * the dark block alone.
 *
 * @param {any} tokens
 * @returns {string} declaration lines at 2-space indent (leading blank line)
 */
function spaInvariantVars(tokens) {
  const { spacing, radius, typography, duration, layout } = tokens;
  const t = typography;
  return [
    ``,
    ``,
    `  /* Spacing */`,
    `  --tc-space-xs: ${px(spacing.xs)};`,
    `  --tc-space-sm: ${px(spacing.sm)};`,
    `  --tc-space-md: ${px(spacing.md)};`,
    `  --tc-space-lg: ${px(spacing.lg)};`,
    `  --tc-space-xl: ${px(spacing.xl)};`,
    `  --tc-space-xxl: ${px(spacing.xxl)};`,
    ``,
    `  /* Corner radius */`,
    `  --tc-radius-sm: ${px(radius.sm)};`,
    `  --tc-radius-md: ${px(radius.md)};`,
    `  --tc-radius-lg: ${px(radius.lg)};`,
    `  --tc-radius-full: ${px(radius.full)};`,
    ``,
    `  /* Typography — Inter via Google Fonts */`,
    `  --tc-font-family: ${t['font-family']};`,
    `  --tc-font-display: ${t['font-display']};`,
    `  --tc-font-mono: ${t['font-mono']};`,
    ``,
    `  --tc-text-hero: ${t.sizes.hero};`,
    `  --tc-text-display-large: ${t.sizes['display-large']};`,
    `  --tc-text-display-small: ${t.sizes['display-small']};`,
    `  --tc-text-h3: ${t.sizes.h3};`,
    `  --tc-text-headline: ${t.sizes.headline};`,
    `  --tc-text-body: ${t.sizes.body};`,
    `  --tc-text-caption: ${t.sizes.caption};`,
    ``,
    `  --tc-weight-regular: ${t.weights.regular};`,
    `  --tc-weight-medium: ${t.weights.medium};`,
    `  --tc-weight-semibold: ${t.weights.semibold};`,
    `  --tc-weight-bold: ${t.weights.bold};`,
    ``,
    `  --tc-leading-tight: ${t.leadings.tight};`,
    `  --tc-leading-snug: ${t.leadings.snug};`,
    `  --tc-leading-normal: ${t.leadings.normal};`,
    `  --tc-leading-relaxed: ${t.leadings.relaxed};`,
    ``,
    `  /* Animation */`,
    `  --tc-duration-fast: ${duration.fast};`,
    `  --tc-duration-normal: ${duration.normal};`,
    `  --tc-duration-slow: ${duration.slow};`,
    ``,
    `  /* Layout */`,
    `  --tc-content-max-width: ${layout['content-max-width']};`,
  ].join('\n');
}

/**
 * Emit `web/src/styles/tokens.css`: the four-block structure (dark `:root`
 * default, `[data-theme="light"]`, `[data-theme="oled-dark"]`, and the
 * `prefers-color-scheme: light` auto-detect block) reproduced from one value
 * map. Section comments and property order match the hand-written original so
 * the first generated diff is limited to the header comment plus the intended
 * status-conditions reconciliation.
 *
 * @param {any} tokens
 * @returns {string}
 */
export function emitSpaTokensCss(tokens) {
  const dark =
    spaThemedVars(tokens, 'dark', {
      brand: 'Brand',
      text: 'Text',
      status: 'Status (PlanIt vocabulary: Permitted / Conditions / Rejected)',
      shadow: 'Shadows (none in dark mode)',
    }) + spaInvariantVars(tokens);

  const light = spaThemedVars(tokens, 'light', {
    brand: 'Brand',
    text: 'Text',
    status: 'Status (PlanIt vocabulary: Permitted / Conditions / Rejected)',
    shadow: 'Shadows (light mode only)',
  });

  const oled = spaThemedVars(tokens, 'oled', {
    brand: 'Brand — same as dark',
    text: 'Text — same as dark',
    status: 'Status — same as dark (PlanIt vocabulary)',
    shadow: 'Shadows (none in OLED mode)',
  });

  const media = spaThemedVars(tokens, 'light', {
    brand: 'Brand',
    text: 'Text',
    status: 'Status (PlanIt vocabulary)',
    shadow: 'Shadows (light mode only)',
  });

  return `/* GENERATED FILE — edit design/tokens.json and run scripts/design-tokens/generate.mjs */

/* ==========================================================================
   Town Crier Design Tokens
   Dark theme is the default. Light theme activates via data-theme attribute
   or prefers-color-scheme media query for first-visit auto-detection.
   ========================================================================== */

/* ---------- Dark theme (default) ---------- */
:root {
${dark}
}

/* ---------- Light theme ---------- */
[data-theme="light"] {
${light}
}

/* ---------- OLED Dark theme ---------- */
[data-theme="oled-dark"] {
${oled}
}

/* ---------- First-visit auto-detection ---------- */
@media (prefers-color-scheme: light) {
  :root:not([data-theme]) {
${indentLines(media, 2)}
  }
}
`;
}

// --------------------------------------------------------------------------
// Emitter: web/scripts/lib/tokens.generated.mjs  (the SEO/share-page block)
// --------------------------------------------------------------------------

/**
 * Emit the SEO custom-property block for one theme. This is a *subset* of the
 * SPA tokens (only what the static pages' pageStyles() uses) and uses the
 * three-bucket status chip vocabulary (decision 4, #794) rather than the full
 * PlanIt palette.
 *
 * Status chips: `granted`/`refused` are taken from tokens.statusBuckets (which
 * agree here: granted -> status-permitted, refused -> status-rejected). The
 * `neutral` chip intentionally resolves to `text-secondary` — the value the
 * shipped SEO pages already use for the undecided/long-tail catch-all — NOT
 * statusBuckets.neutral (status-withdrawn). statusBuckets records the decision
 * for the mobile/share emitters (T2); the SEO neutral chip keeping text-
 * secondary is what makes this slice a zero-visual-change conversion. The `-bg`
 * fills are the chip colour at 15% opacity, emitted as an 8-digit `#RRGGBB26`.
 *
 * @param {any} tokens
 * @param {string} theme
 * @param {number} indent base indent (spaces) for each declaration
 * @param {boolean} withComments whether to include the two prose section
 *   comments (present only on the light `:root` in the original)
 * @returns {string}
 */
function seoThemedVars(tokens, theme, indent, withComments) {
  const c = tokens.color;
  const color = (name) => resolveColor(c, name, theme);
  const buckets = tokens.statusBuckets;
  const granted = color(buckets.granted);
  const refused = color(buckets.refused);
  const neutral = color('text-secondary');

  const bgComment = `      /* Shared status chip vocabulary (decision 4, punch-list #794): three
         canonical buckets — granted (green), refused (red), neutral (the
         undecided/long-tail catch-all) — converged with the design-language
         tcStatusApproved/tcStatusRefused/tcTextSecondary tokens, replacing the
         previous five-way ad-hoc per-appState palette. The *-bg tokens are the
         foreground colour at 15% opacity, for the filled badge style. */`;
  const bgIntro = `      /* Background scale converged with the share-page family (see
         api-go/internal/sharepage/templates/styles.gohtml): a warm paper field
         (--tc-background) behind a warmer off-white card (--tc-surface) in light
         mode, so SEO pages and share pages read as one product rather than two
         differently-themed properties. */`;

  const lines = [];
  if (withComments) lines.push(bgIntro);
  lines.push(
    `      --tc-amber: ${color('amber')};`,
    `      --tc-amber-hover: ${color('amber-hover')};`,
    `      --tc-background: ${color('background')};`,
    `      --tc-surface: ${color('surface')};`,
    `      --tc-text-primary: ${color('text-primary')};`,
    `      --tc-text-secondary: ${color('text-secondary')};`,
    `      --tc-text-on-accent: ${color('text-on-accent')};`,
  );
  if (withComments) lines.push(bgComment);
  lines.push(
    `      --tc-status-granted: ${granted};`,
    `      --tc-status-granted-bg: ${granted}26;`,
    `      --tc-status-refused: ${refused};`,
    `      --tc-status-refused-bg: ${refused}26;`,
    `      --tc-status-neutral: ${neutral};`,
    `      --tc-status-neutral-bg: ${neutral}26;`,
    `      --tc-border: ${color('border')};`,
  );
  if (withComments) {
    // Layout / spacing / typography properties are theme-invariant, so they
    // appear only on the light-default `:root`, matching the original.
    lines.push(
      `      --tc-radius-md: ${px(tokens.radius.md)};`,
      `      --tc-radius-full: ${px(tokens.radius.full)};`,
      `      --tc-space-sm: ${px(tokens.spacing.sm)};`,
      `      --tc-space-md: ${px(tokens.spacing.md)};`,
      `      --tc-space-lg: ${px(tokens.spacing.lg)};`,
      `      --tc-space-xl: ${px(tokens.spacing.xl)};`,
      `      --tc-space-xxl: ${px(tokens.spacing.xxl)};`,
      `      --tc-font-family: ${tokens.typography['font-family']};`,
      `      --tc-font-display: ${tokens.typography['font-display']};`,
      `      --tc-font-mono: ${tokens.typography['font-mono']};`,
      // SEO reading width — narrower than the SPA's --tc-content-max-width
      // (1120px); a page-local literal, not a shared token.
      `      --tc-content-max-width: 760px;`,
    );
  }
  const body = lines.join('\n');
  // The base block sits at 4-space indent; the dark block nests two deeper.
  return indent === 4 ? body : indentLines(body, indent - 4);
}

/**
 * Emit `web/scripts/lib/tokens.generated.mjs`: the light-first SEO token block
 * consumed by pageStyles() in render-shared.mjs. Light values live on `:root`;
 * dark values move into `@media (prefers-color-scheme: dark)`. The leading
 * provenance comment is the only change the generated block introduces to the
 * rendered static pages.
 *
 * @param {any} tokens
 * @returns {string}
 */
export function emitSeoTokensMjs(tokens) {
  const light = seoThemedVars(tokens, 'light', 4, true);
  const dark = seoThemedVars(tokens, 'dark', 6, false);

  const css = `    /* GENERATED — design tokens from design/tokens.json; run scripts/design-tokens/generate.mjs */
    :root {
${light}
    }
    @media (prefers-color-scheme: dark) {
      :root {
${dark}
      }
    }`;

  return `// GENERATED FILE — edit design/tokens.json and run scripts/design-tokens/generate.mjs
// Consumed by web/scripts/lib/render-shared.mjs pageStyles(). See ADR 0040.

export const SEO_TOKEN_CSS = \`${css}\`;
`;
}

// --------------------------------------------------------------------------
// Emitter: api-go/internal/designtokens/tokens_gen.go  (Go palette projection)
// --------------------------------------------------------------------------

/**
 * Emit `api-go/internal/designtokens/tokens_gen.go`: the Go projection of the
 * palette. Every *solid* colour token becomes a `#RRGGBB` hex-string const (for
 * HTML/CSS template consumers) and a `color.RGBA` var (for image renderers such
 * as the share-page OG card). Names are `<Token><Theme>` — e.g. `AmberLightHex`
 * / `AmberLight`. Only light+dark are emitted: no Go-rendered surface implements
 * the OLED theme. The alpha tokens (amber-muted, overlay) resolve to rgba() and
 * are omitted; a Go consumer needing a translucent fill composes it explicitly.
 *
 * @param {any} tokens
 * @returns {string}
 */
export function emitGoTokens(tokens) {
  const colors = tokens.color;
  const solid = Object.keys(colors).filter((name) => isSolidColor(colors, name));

  const constRows = [];
  const varRows = [];
  for (const name of solid) {
    const base = pascalCase(name);
    for (const theme of ['light', 'dark']) {
      const themePascal = theme === 'light' ? 'Light' : 'Dark';
      const hex = resolveColor(colors, name, theme);
      const [r, g, b] = hexToRgb(hex);
      constRows.push([`${base}${themePascal}Hex`, `"${hex}"`]);
      varRows.push([
        `${base}${themePascal}`,
        `color.RGBA{R: 0x${hexByte(r)}, G: 0x${hexByte(g)}, B: 0x${hexByte(b)}, A: 0xFF}`,
      ]);
    }
  }

  return `// Code generated by scripts/design-tokens/generate.mjs. DO NOT EDIT.

// Package designtokens is the Go projection of design/tokens.json (ADR 0040):
// the single source of truth for Town Crier's colour palette. Each solid colour
// token is exposed as a hex-string const (for HTML/CSS template consumers) and
// as a color.RGBA var (for image renderers such as the share-page OG card).
//
// Only light and dark themes are emitted: no Go-rendered surface implements the
// OLED theme. The alpha tokens (amber-muted, overlay) resolve to rgba() rather
// than a solid hex and are intentionally omitted; a Go consumer needing a
// translucent fill composes it explicitly.
package designtokens

import "image/color"

// Hex strings per theme (leading '#'), for HTML/CSS template consumers.
const (
${alignGoAssignments(constRows)}
)

// RGBA values for image renderers. color.RGBA is alpha-premultiplied; every
// brand colour here is fully opaque (A: 0xFF), so R/G/B are the raw channels.
var (
${alignGoAssignments(varRows)}
)

// White is pure #FFFFFF — an image-renderer primitive (pin outline, on-accent
// text) that is white in every theme. It is not a design token.
var White = color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
`;
}

// --------------------------------------------------------------------------
// Emitter: api-go/internal/sharepage/templates/tokens.gohtml  (share-page CSS)
// --------------------------------------------------------------------------

/**
 * Emit the theme-varying custom-property declarations for the share page, in
 * the exact hand-formatted line grouping the page has always used (multiple
 * properties per line, specific wraps). Values are token-sourced so a change in
 * tokens.json flows through, but the layout is share-page-specific and fixed.
 *
 * The status chips use the canonical statusBuckets map (granted -> permitted,
 * refused -> rejected, neutral -> withdrawn); unlike the SEO block, the share
 * page uses the *true* neutral bucket. The `-bg` fills are the chip colour at
 * 15% opacity as a compact `rgba(...)`. Layout invariants (radius, spacing) and
 * the vocabulary comment live only on the light `:root`, matching the original.
 *
 * @param {any} tokens
 * @param {string} theme
 * @param {number} indent base indent (spaces) for each declaration line
 * @param {boolean} withInvariants include the radius/spacing lines + comment
 * @returns {string}
 */
function sharepageThemeBlock(tokens, theme, indent, withInvariants) {
  const c = tokens.color;
  const color = (name) => resolveColor(c, name, theme);
  const buckets = tokens.statusBuckets;
  const muted = resolveColor(c, 'amber-muted', theme).replace(/,\s+/g, ',');
  const granted = color(buckets.granted);
  const refused = color(buckets.refused);
  const neutral = color(buckets.neutral);
  const bg = (hex) => compactRgba(hex, 0.15);
  const pad = ' '.repeat(indent);

  const lines = [
    `${pad}--bg:${color('background')};--surface:${color('surface')};--text-primary:${color('text-primary')};--text-secondary:${color('text-secondary')};`,
    `${pad}--text-tertiary:${color('text-tertiary')};--border:${color('border')};--amber:${color('amber')};--amber-hover:${color('amber-hover')};`,
    `${pad}--text-on-accent:${color('text-on-accent')};--amber-muted:${muted};`,
  ];
  if (withInvariants) {
    lines.push(
      `${pad}--radius-sm:${px(tokens.radius.sm)};--radius-md:${px(tokens.radius.md)};`,
      `${pad}--space-xs:${px(tokens.spacing.xs)};--space-sm:${px(tokens.spacing.sm)};--space-md:${px(tokens.spacing.md)};--space-lg:${px(tokens.spacing.lg)};--space-xl:${px(tokens.spacing.xl)};`,
      `${pad}--font-display:${tokens.typography['font-display']};--font-mono:${tokens.typography['font-mono']};`,
      `${pad}/* Shared status vocabulary (tc-r4n9 decision 4): three colour buckets, not a`,
      `${pad}   five-way traffic light — granted (green), refused (red), neutral for`,
      `${pad}   "Undecided" and every long-tail state. Mirrors web/scripts/lib/render-shared.mjs. */`,
    );
  }
  lines.push(
    `${pad}--status-granted:${granted};--status-granted-bg:${bg(granted)};`,
    `${pad}--status-refused:${refused};--status-refused-bg:${bg(refused)};`,
    `${pad}--status-neutral:${neutral};--status-neutral-bg:${bg(neutral)};`,
  );
  return lines.join('\n');
}

/**
 * Emit `api-go/internal/sharepage/templates/tokens.gohtml`: a `{{define
 * "tokenVars"}}` block holding the light `:root` + dark `@media` custom-property
 * declarations. styles.gohtml includes it via `{{template "tokenVars"}}` so the
 * rendered CSS is byte-identical to the previously hand-written block.
 *
 * @param {any} tokens
 * @returns {string}
 */
export function emitSharepageTokensGohtml(tokens) {
  const light = sharepageThemeBlock(tokens, 'light', 2, true);
  const dark = sharepageThemeBlock(tokens, 'dark', 4, false);

  // A Go-template comment (renders nothing) records provenance without altering
  // a single byte of the CSS the "tokenVars" block emits into styles.gohtml.
  return `{{/* GENERATED FILE — edit design/tokens.json and run scripts/design-tokens/generate.mjs. See ADR 0040. */}}
{{define "tokenVars"}}
:root{
${light}
}
@media (prefers-color-scheme:dark){
  :root{
${dark}
  }
}{{end}}
`;
}

// --------------------------------------------------------------------------
// Shared helpers for the mobile + skill-doc emitters (T3, issue #851)
// --------------------------------------------------------------------------

/** Every platform a token targets unless it declares a narrower `platforms`. */
const ALL_PLATFORMS = ['web', 'ios', 'android'];

/** @param {Record<string, any>} entry @returns {string[]} */
function tokenPlatforms(entry) {
  return Array.isArray(entry.platforms) ? entry.platforms : ALL_PLATFORMS;
}

/** `#RRGGBB` (or `RRGGBB`) -> upper-case `RRGGBB` (no `#`). */
function bareHex(hex) {
  return hex.replace(/^#/, '').toUpperCase();
}

/** kebab-case -> camelCase (`status-conditions` -> `statusConditions`). */
function camelCase(kebab) {
  const p = pascalCase(kebab);
  return p.charAt(0).toLowerCase() + p.slice(1);
}

/** iOS `Color` extension member for a colour token (`amber` -> `tcAmber`). */
function iosTokenName(name) {
  return `tc${pascalCase(name)}`;
}

// --------------------------------------------------------------------------
// Emitter: mobile/ios/.../Colors/Color+TownCrier.swift
// --------------------------------------------------------------------------

/**
 * iOS `Color` extension, section by section. These lists are the source of
 * truth for iOS token *order* and double as the per-platform skip list: iOS has
 * no `amber-hover`, so it simply never appears here. emitIosColors() asserts
 * these sections exactly cover the colour tokens whose `platforms` include
 * `ios`, so a token added to tokens.json can't silently go missing (nor a
 * skipped token silently reappear).
 */
const IOS_SECTIONS = [
  { title: 'Brand', tokens: ['amber', 'amber-muted'] },
  { title: 'Surfaces', tokens: ['background', 'surface', 'surface-elevated'] },
  { title: 'Text', tokens: ['text-primary', 'text-secondary', 'text-tertiary', 'text-on-accent'] },
  {
    title: 'Status',
    tokens: [
      'status-permitted',
      'status-conditions',
      'status-rejected',
      'status-pending',
      'status-withdrawn',
      'status-appealed',
    ],
  },
  { title: 'Utility', tokens: ['border', 'border-focused', 'overlay'] },
];

/** Column limit above which a `Color.themed(...)` declaration wraps. */
const IOS_MAX_COLUMNS = 100;

/**
 * `status-rejected` is hand-wrapped in the committed file at exactly 100
 * columns — one tighter than every other token follows. Preserving it keeps the
 * first generation byte-identical (issue #851 acceptance). Safe to drop from
 * this set if that declaration is ever reflowed onto a single line.
 */
const IOS_FORCE_WRAP = new Set(['status-rejected']);

/**
 * The Swift value expression for one colour token, and whether it's a
 * `Color.themed(...)` call eligible for line wrapping.
 *
 * @param {Record<string, any>} colors
 * @param {string} name
 * @returns {{ expr: string, wrappable: boolean }}
 */
function iosColorExpr(colors, name) {
  const entry = colors[name];
  if ('base' in entry) {
    if (entry.base.startsWith('#')) {
      // Literal base (overlay). iOS renders a single flat scrim, not a
      // theme-aware one, so it takes the light-theme alpha — the hand-written
      // value. `#000000` is Color.black; nothing else uses a literal base today.
      const baseExpr = entry.base === '#000000' ? 'Color.black' : `Color(hex: 0x${bareHex(entry.base)})`;
      const alpha = typeof entry.alpha === 'object' ? entry.alpha.light : entry.alpha;
      return { expr: `${baseExpr}.opacity(${alpha})`, wrappable: false };
    }
    // Token-reference base (amber-muted, border-focused) -> the iOS member.
    const baseName = iosTokenName(entry.base);
    if ('alpha' in entry) {
      // iOS applies a single flat .opacity() to a theme-aware base — there is no
      // per-theme-opacity helper — so a per-theme alpha collapses to the light
      // value, the same rule the overlay scrim already uses above.
      const alpha = typeof entry.alpha === 'object' ? entry.alpha.light : entry.alpha;
      return { expr: `${baseName}.opacity(${alpha})`, wrappable: false };
    }
    return { expr: baseName, wrappable: false };
  }
  const call = `Color.themed(light: 0x${bareHex(entry.light)}, dark: 0x${bareHex(entry.dark)}, oled: 0x${bareHex(entry.oled)})`;
  return { expr: call, wrappable: true };
}

/** The `public static let …` declaration line(s) for one iOS colour token. */
function iosDecl(colors, name) {
  const { expr, wrappable } = iosColorExpr(colors, name);
  const line = `  public static let ${iosTokenName(name)} = ${expr}`;
  if (wrappable && (line.length > IOS_MAX_COLUMNS || IOS_FORCE_WRAP.has(name))) {
    const args = expr.slice('Color.themed('.length); // "light: 0x…, dark: 0x…, oled: 0x…)"
    return `  public static let ${iosTokenName(name)} = Color.themed(\n    ${args}`;
  }
  return line;
}

/** Doc comment + declaration for one iOS colour token. */
function iosTokenBlock(colors, name) {
  const doc = colors[name].doc
    .split('\n')
    .map((l) => `  /// ${l}`)
    .join('\n');
  return `${doc}\n${iosDecl(colors, name)}`;
}

/**
 * Emit `mobile/ios/.../Colors/Color+TownCrier.swift`: a whole generated file of
 * `Color` extension sections. Colours only — TCSpacing/TCCornerRadius/
 * TCTypography stay hand-maintained (ADR 0040). Byte-identical to the
 * hand-written original apart from the generated-file header.
 *
 * @param {any} tokens
 * @returns {string}
 */
export function emitIosColors(tokens) {
  const colors = tokens.color;

  const emitted = new Set(IOS_SECTIONS.flatMap((s) => s.tokens));
  const expected = new Set(Object.keys(colors).filter((n) => tokenPlatforms(colors[n]).includes('ios')));
  for (const name of emitted) {
    if (!expected.has(name)) {
      throw new Error(`iOS sections list "${name}", which isn't an ios-platform colour token`);
    }
  }
  for (const name of expected) {
    if (!emitted.has(name)) {
      throw new Error(`ios-platform colour token "${name}" is missing from the iOS sections`);
    }
  }

  const sections = IOS_SECTIONS.map((section) => {
    const blocks = section.tokens.map((name) => iosTokenBlock(colors, name)).join('\n\n');
    return `// MARK: - ${section.title}\n\nextension Color {\n${blocks}\n}`;
  }).join('\n\n');

  return `// Code generated by scripts/design-tokens/generate.mjs. DO NOT EDIT.

import SwiftUI

${sections}
`;
}

// --------------------------------------------------------------------------
// Emitter: mobile/android/.../designsystem/Color.kt
// --------------------------------------------------------------------------

/**
 * Android palette field order. Differs from tokens.json order (amberMuted
 * precedes amberHover) and from iOS (Android keeps amberHover), so it's its own
 * list. emitAndroidColors() cross-checks it against the android-platform tokens.
 */
const ANDROID_ORDER = [
  'amber',
  'amber-muted',
  'amber-hover',
  'background',
  'surface',
  'surface-elevated',
  'text-primary',
  'text-secondary',
  'text-tertiary',
  'text-on-accent',
  'status-permitted',
  'status-conditions',
  'status-rejected',
  'status-pending',
  'status-withdrawn',
  'status-appealed',
  'border',
  'border-focused',
  'overlay',
];

/** Compose alpha literal: `0.4` -> `0.40f`. */
function kotlinAlpha(alpha) {
  return `${alpha.toFixed(2)}f`;
}

/**
 * The Kotlin `Color` expression for one token in one theme.
 *
 * @param {Record<string, any>} colors
 * @param {string} name
 * @param {string} theme one of THEMES
 * @returns {string}
 */
function androidColorExpr(colors, name, theme) {
  const entry = colors[name];
  if ('base' in entry) {
    if (entry.base.startsWith('#')) {
      // Literal base (overlay) -> Color.Black.copy(alpha = …) per theme.
      const baseExpr = entry.base === '#000000' ? 'Color.Black' : `Color(0xFF${bareHex(entry.base)})`;
      const alpha = typeof entry.alpha === 'object' ? entry.alpha[theme] : entry.alpha;
      return `${baseExpr}.copy(alpha = ${kotlinAlpha(alpha)})`;
    }
    // Token-reference base. A per-theme (or scalar) alpha applies .copy(alpha=)
    // per theme, matching the overlay scrim style; Android emits one field per
    // theme, so it carries the per-theme alpha natively (unlike iOS's single
    // flat value).
    const baseHex = bareHex(resolveColor(colors, entry.base, theme));
    if ('alpha' in entry) {
      const alpha = typeof entry.alpha === 'object' ? entry.alpha[theme] : entry.alpha;
      return `Color(0xFF${baseHex}).copy(alpha = ${kotlinAlpha(alpha)})`;
    }
    return `Color(0xFF${baseHex})`;
  }
  return `Color(0xFF${bareHex(entry[theme])})`;
}

/** One `field = expr,` line at 8-space indent inside a TcPalette(...) block. */
function androidField(colors, name, theme) {
  return `        ${camelCase(name)} = ${androidColorExpr(colors, name, theme)},`;
}

/**
 * Emit `mobile/android/.../designsystem/Color.kt`: a whole generated file. The
 * TcPalette data class, muted() helper and doc comments are template constants;
 * only the three palette `val` bodies come from tokens.json. OledPalette is a
 * DarkPalette.copy(...) overriding exactly the fields whose oled value differs
 * from dark. Byte-identical to the hand-written original apart from the header.
 *
 * @param {any} tokens
 * @returns {string}
 */
export function emitAndroidColors(tokens) {
  const colors = tokens.color;

  const expected = new Set(Object.keys(colors).filter((n) => tokenPlatforms(colors[n]).includes('android')));
  const listed = new Set(ANDROID_ORDER);
  for (const name of ANDROID_ORDER) {
    if (!expected.has(name)) {
      throw new Error(`ANDROID_ORDER lists "${name}", which isn't an android-platform colour token`);
    }
  }
  for (const name of expected) {
    if (!listed.has(name)) {
      throw new Error(`android-platform colour token "${name}" is missing from ANDROID_ORDER`);
    }
  }

  const fields = ANDROID_ORDER.map((name) => `    val ${camelCase(name)}: Color,`).join('\n');
  const light = ANDROID_ORDER.map((name) => androidField(colors, name, 'light')).join('\n');
  const dark = ANDROID_ORDER.map((name) => androidField(colors, name, 'dark')).join('\n');
  const oled = ANDROID_ORDER.filter(
    (name) => androidColorExpr(colors, name, 'oled') !== androidColorExpr(colors, name, 'dark'),
  )
    .map((name) => androidField(colors, name, 'oled'))
    .join('\n');

  return `// Code generated by scripts/design-tokens/generate.mjs. DO NOT EDIT.

// Color.kt is the bead-mandated file name for the whole token file (epic
// #770): TcPalette below is a supporting type alongside the Light/Dark/
// OledPalette vals that follow it, not the file's sole declaration in spirit.
@file:Suppress("MatchingDeclarationName")

package uk.towncrierapp.presentation.designsystem

import androidx.compose.ui.graphics.Color

/**
 * One resolved set of Town Crier color tokens — exact hex values from epic
 * #770 / the design-language skill's token table. [TcPalette] never appears
 * in feature code directly: [TownCrierTheme] maps it onto a Material 3
 * \`ColorScheme\` (see [colorScheme]) and the extended [TownCrierColors]
 * CompositionLocal (see [extendedColors]) for tokens with no Material role.
 */
internal data class TcPalette(
${fields}
)

internal val LightPalette =
    TcPalette(
${light}
    )

internal val DarkPalette =
    TcPalette(
${dark}
    )

// OLED is a dark sub-variant, not a fourth palette: every token matches Dark
// except the three surfaces (and border, which steps darker with them) that
// go true-black. See design-language skill / epic #770.
internal val OledPalette =
    DarkPalette.copy(
${oled}
    )
`;
}

// --------------------------------------------------------------------------
// Emitter: .claude/skills/design-language/references/tokens.md (value tables)
// --------------------------------------------------------------------------

const SKILL_BEGIN = '<!-- tokens:generated:begin -->';
const SKILL_END = '<!-- tokens:generated:end -->';

/** Human labels for the platforms column. */
const PLATFORM_LABEL = { web: 'Web', ios: 'iOS', android: 'Android' };

/** Colour-token doc groups, in doc order (includes web/android-only tokens). */
const DOC_COLOR_GROUPS = [
  { title: 'Brand', tokens: ['amber', 'amber-muted', 'amber-hover'] },
  { title: 'Surface', tokens: ['background', 'surface', 'surface-elevated'] },
  { title: 'Text', tokens: ['text-primary', 'text-secondary', 'text-tertiary', 'text-on-accent'] },
  {
    title: 'Status',
    tokens: [
      'status-permitted',
      'status-conditions',
      'status-rejected',
      'status-pending',
      'status-withdrawn',
      'status-appealed',
    ],
  },
  { title: 'Utility', tokens: ['border', 'border-focused', 'overlay'] },
];

/** iOS TCSpacing member per spacing key. */
const IOS_SPACING = {
  xs: 'extraSmall', sm: 'small', md: 'medium', lg: 'large', xl: 'extraLarge', xxl: 'extraExtraLarge',
};
/** iOS TCCornerRadius member per radius key (no `full` — iOS uses Capsule()). */
const IOS_RADIUS = { sm: 'small', md: 'medium', lg: 'large' };

/** Render a colour value for one theme as doc text (`#D4910A`, `#000000 @ 40%`). */
function describeColor(colors, name, theme) {
  const entry = colors[name];
  if ('base' in entry) {
    const baseHex = entry.base.startsWith('#') ? entry.base : resolveColor(colors, entry.base, theme);
    if ('alpha' in entry) {
      const alpha = typeof entry.alpha === 'object' ? entry.alpha[theme] : entry.alpha;
      return `${baseHex} @ ${Math.round(alpha * 100)}%`;
    }
    return baseHex;
  }
  return entry[theme];
}

/** "Web, iOS, Android" style platform list for a set of platform keys. */
function platformList(keys) {
  return keys.map((k) => PLATFORM_LABEL[k]).join(', ');
}

/** Markdown table from a header row and body rows (arrays of cell strings). */
function mdTable(header, rows) {
  const line = (cells) => `| ${cells.join(' | ')} |`;
  const sep = `| ${header.map(() => '---').join(' | ')} |`;
  return [line(header), sep, ...rows.map(line)].join('\n');
}

/** Generate the value-table block that lives between the tokens.md markers. */
function skillTables(tokens) {
  const colors = tokens.color;
  const parts = [];

  for (const group of DOC_COLOR_GROUPS) {
    const rows = group.tokens.map((name) => {
      const plats = tokenPlatforms(colors[name]);
      const ios = plats.includes('ios') ? `\`Color.${iosTokenName(name)}\`` : '—';
      return [
        `\`${name}\``,
        `\`--tc-${name}\``,
        ios,
        `\`TcPalette.${camelCase(name)}\``,
        `\`${describeColor(colors, name, 'light')}\``,
        `\`${describeColor(colors, name, 'dark')}\``,
        `\`${describeColor(colors, name, 'oled')}\``,
        platformList(plats),
      ];
    });
    parts.push(
      `### ${group.title}\n\n` +
        mdTable(['Token', 'Web', 'iOS', 'Android', 'Light', 'Dark', 'OLED', 'Platforms'], rows),
    );
  }

  const spacingRows = Object.keys(tokens.spacing).map((key) => [
    `\`${key}\``,
    `\`--tc-space-${key}\``,
    `\`TCSpacing.${IOS_SPACING[key]}\``,
    `\`TownCrierSpacing.${key}\``,
    `${tokens.spacing[key]}pt`,
    'Web, iOS, Android',
  ]);
  parts.push(`### Spacing\n\n` + mdTable(['Token', 'Web', 'iOS', 'Android', 'Value', 'Platforms'], spacingRows));

  const radiusRows = Object.keys(tokens.radius).map((key) => {
    const ios = IOS_RADIUS[key] ? `\`TCCornerRadius.${IOS_RADIUS[key]}\`` : '—';
    const value = key === 'full' ? 'capsule' : `${tokens.radius[key]}pt`;
    const plats = key === 'full' ? 'Web, Android' : 'Web, iOS, Android';
    return [`\`${key}\``, `\`--tc-radius-${key}\``, ios, `\`TownCrierRadius.${key}\``, value, plats];
  });
  parts.push(
    `### Corner radius\n\n` + mdTable(['Token', 'Web', 'iOS', 'Android', 'Value', 'Platforms'], radiusRows),
  );

  const shadowRows = Object.keys(tokens.shadow).map((key) => [
    `\`${key}\``,
    `\`--tc-shadow-${key}\``,
    `\`${tokens.shadow[key].light}\``,
    `\`${tokens.shadow[key].dark}\``,
    'Web',
  ]);
  parts.push(
    `### Shadows\n\nWeb only. Dark and OLED communicate elevation through surface stepping, not shadow.\n\n` +
      mdTable(['Token', 'Web', 'Light', 'Dark / OLED', 'Platforms'], shadowRows),
  );

  const durationRows = Object.keys(tokens.duration).map((key) => [
    `\`${key}\``,
    `\`--tc-duration-${key}\``,
    tokens.duration[key],
    'Web',
  ]);
  parts.push(`### Motion durations\n\nWeb only.\n\n` + mdTable(['Token', 'Web', 'Value', 'Platforms'], durationRows));

  return parts.join('\n\n');
}

/** Splice freshly-generated tables between the markers, preserving all prose. */
function spliceSkillDoc(current, generated) {
  const begin = current.indexOf(SKILL_BEGIN);
  const end = current.indexOf(SKILL_END);
  if (begin === -1 || end === -1 || end < begin) {
    throw new Error(`tokens.md is missing the ${SKILL_BEGIN} / ${SKILL_END} markers`);
  }
  const before = current.slice(0, begin + SKILL_BEGIN.length);
  const after = current.slice(end);
  return `${before}\n${generated}\n${after}`;
}

/**
 * Emit `.claude/skills/design-language/references/tokens.md`: regenerate only
 * the value tables between the markers; hand-written prose outside them
 * survives. Reads the current file for that prose, so it isn't a pure function
 * of tokens.json alone.
 *
 * @param {any} tokens
 * @param {string} path
 * @returns {string}
 */
export function emitSkillDoc(tokens, path = SKILL_DOC_PATH) {
  return spliceSkillDoc(readFileSync(path, 'utf8'), skillTables(tokens));
}

// --------------------------------------------------------------------------
// Output registry + write/check driver
// --------------------------------------------------------------------------

/**
 * @param {any} tokens
 * @returns {Array<{ path: string, content: string }>}
 */
export function buildOutputs(tokens) {
  return [
    { path: join(REPO_ROOT, 'web', 'src', 'styles', 'tokens.css'), content: emitSpaTokensCss(tokens) },
    { path: join(REPO_ROOT, 'web', 'scripts', 'lib', 'tokens.generated.mjs'), content: emitSeoTokensMjs(tokens) },
    { path: join(REPO_ROOT, 'api-go', 'internal', 'designtokens', 'tokens_gen.go'), content: emitGoTokens(tokens) },
    {
      path: join(REPO_ROOT, 'api-go', 'internal', 'sharepage', 'templates', 'tokens.gohtml'),
      content: emitSharepageTokensGohtml(tokens),
    },
    { path: IOS_COLORS_PATH, content: emitIosColors(tokens) },
    { path: ANDROID_COLORS_PATH, content: emitAndroidColors(tokens) },
    { path: SKILL_DOC_PATH, content: emitSkillDoc(tokens, SKILL_DOC_PATH) },
  ];
}

/**
 * Write every output (default) or, in check mode, report the first stale one.
 *
 * @param {{ check?: boolean, tokensPath?: string }} [opts]
 * @returns {{ stale: string[] }}
 */
export function run(opts = {}) {
  const tokens = loadTokens(opts.tokensPath);
  const outputs = buildOutputs(tokens);
  const stale = [];
  for (const { path, content } of outputs) {
    let current = null;
    try {
      current = readFileSync(path, 'utf8');
    } catch {
      current = null;
    }
    if (opts.check) {
      if (current !== content) {
        stale.push(relative(REPO_ROOT, path));
      }
    } else if (current !== content) {
      mkdirSync(dirname(path), { recursive: true });
      writeFileSync(path, content);
    }
  }
  return { stale };
}

function main() {
  const { values } = parseArgs({ options: { check: { type: 'boolean', default: false } } });
  const { stale } = run({ check: values.check });

  if (values.check) {
    if (stale.length > 0) {
      console.error('error: generated design-token files are stale:');
      for (const f of stale) console.error(`  ${f}`);
      console.error("  fix: run 'node scripts/design-tokens/generate.mjs' and commit the result");
      process.exit(1);
    }
    console.log(`design tokens in sync (${buildOutputs(loadTokens()).length} generated file(s))`);
    return;
  }

  console.log(`design tokens generated (${buildOutputs(loadTokens()).length} file(s))`);
}

// Run only when invoked directly (not when imported by the test file).
if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  main();
}
