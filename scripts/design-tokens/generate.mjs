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
 * This slice (T1) emits the two web surfaces only. Mobile (Swift/Kotlin) and Go
 * emitters land in follow-up slices (T2/T3); they will read the SAME tokens.json
 * and the SAME statusBuckets map, which is why the generator lives at the repo
 * root rather than under web/.
 */

import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join, relative } from 'node:path';
import { parseArgs } from 'node:util';

const HERE = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = join(HERE, '..', '..');

export const TOKENS_PATH = join(REPO_ROOT, 'design', 'tokens.json');
export const THEMES = ['light', 'dark', 'oled'];

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
         api-go/internal/sharepage/templates/styles.gohtml): a warm cream field
         (--tc-background: #FAF8F5) behind a white card (--tc-surface:
         #FFFFFF) in light mode, so SEO pages and share pages read as one
         product rather than two differently-themed properties. */`;

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
