/**
 * Tests for the design-token generator (issue #849). Run with:
 *   node --test scripts/design-tokens/
 *
 * Stdlib node:test only — no npm dependencies, matching the generator itself.
 */

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

import {
  loadTokens,
  resolveColor,
  buildOutputs,
  run,
  THEMES,
} from './generate.mjs';

const HERE = dirname(fileURLToPath(new URL(import.meta.url)));
const GENERATE = join(HERE, 'generate.mjs');
const REPO_ROOT = join(HERE, '..', '..');
const tokens = loadTokens();

test('resolveColor returns the direct per-theme value', () => {
  assert.equal(resolveColor(tokens.color, 'amber', 'light'), '#D4910A');
  assert.equal(resolveColor(tokens.color, 'amber', 'dark'), '#E9A620');
  assert.equal(resolveColor(tokens.color, 'amber', 'oled'), '#E9A620');
  assert.equal(resolveColor(tokens.color, 'background', 'oled'), '#000000');
});

test('resolveColor follows a base reference with no alpha (per theme)', () => {
  // border-focused: { base: "amber" } -> amber's hex, resolved per theme.
  assert.equal(
    resolveColor(tokens.color, 'border-focused', 'light'),
    resolveColor(tokens.color, 'amber', 'light'),
  );
  assert.equal(resolveColor(tokens.color, 'border-focused', 'dark'), '#E9A620');
});

test('resolveColor emits rgba() for a scalar alpha over a token base', () => {
  // amber-muted: { base: "amber", alpha: 0.15 } -> rgba of amber, 15%.
  assert.equal(resolveColor(tokens.color, 'amber-muted', 'light'), 'rgba(212, 145, 10, 0.15)');
  assert.equal(resolveColor(tokens.color, 'amber-muted', 'dark'), 'rgba(233, 166, 32, 0.15)');
  assert.equal(resolveColor(tokens.color, 'amber-muted', 'oled'), 'rgba(233, 166, 32, 0.15)');
});

test('resolveColor emits rgba() for a per-theme alpha over a literal base', () => {
  // overlay: { base: "#000000", alpha: { light: 0.4, dark: 0.5, oled: 0.5 } }
  assert.equal(resolveColor(tokens.color, 'overlay', 'light'), 'rgba(0, 0, 0, 0.4)');
  assert.equal(resolveColor(tokens.color, 'overlay', 'dark'), 'rgba(0, 0, 0, 0.5)');
  assert.equal(resolveColor(tokens.color, 'overlay', 'oled'), 'rgba(0, 0, 0, 0.5)');
});

test('resolveColor throws on an unknown token', () => {
  assert.throws(() => resolveColor(tokens.color, 'no-such-token', 'light'), /unknown colour token/);
});

test('every colour token resolves for all three themes (light/dark/oled)', () => {
  for (const name of Object.keys(tokens.color)) {
    for (const theme of THEMES) {
      const value = resolveColor(tokens.color, name, theme);
      assert.ok(
        typeof value === 'string' && value.length > 0,
        `colour token "${name}" produced no value for theme "${theme}"`,
      );
    }
  }
});

test('direct (non-ref) colour tokens declare all three themes explicitly', () => {
  for (const [name, entry] of Object.entries(tokens.color)) {
    if ('base' in entry) continue; // refs inherit their themes from the base
    for (const theme of THEMES) {
      assert.ok(theme in entry, `colour token "${name}" is missing the "${theme}" theme`);
    }
  }
});

test('generation is idempotent — buildOutputs is a pure function of tokens.json', () => {
  const a = buildOutputs(tokens);
  const b = buildOutputs(loadTokens());
  assert.deepEqual(
    a.map((o) => o.content),
    b.map((o) => o.content),
  );
});

test('--check exits 0 on a clean tree and non-zero once a generated file is tampered', () => {
  // Clean tree first (the committed files must already be in sync).
  execFileSync('node', [GENERATE, '--check'], { stdio: 'pipe' });

  const target = join(REPO_ROOT, 'web', 'src', 'styles', 'tokens.css');
  const original = readFileSync(target, 'utf8');
  try {
    writeFileSync(target, original + '\n/* tampered */\n');
    let exitCode = 0;
    try {
      execFileSync('node', [GENERATE, '--check'], { stdio: 'pipe' });
    } catch (err) {
      exitCode = err.status;
    }
    assert.notEqual(exitCode, 0, '--check should fail when a generated file is stale');
  } finally {
    // Restore the committed content regardless of assertion outcome.
    run(); // regenerate from tokens.json
    assert.equal(readFileSync(target, 'utf8'), original, 'generated file was not restored');
  }
});
