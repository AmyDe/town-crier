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
  emitIosColors,
  emitAndroidColors,
  emitSkillDoc,
  THEMES,
} from './generate.mjs';

const HERE = dirname(fileURLToPath(new URL(import.meta.url)));
const GENERATE = join(HERE, 'generate.mjs');
const REPO_ROOT = join(HERE, '..', '..');
const tokens = loadTokens();

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
const SKILL_DOC_PATH = join(REPO_ROOT, '.claude', 'skills', 'design-language', 'references', 'tokens.md');
const SKILL_BEGIN = '<!-- tokens:generated:begin -->';
const SKILL_END = '<!-- tokens:generated:end -->';

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

// --------------------------------------------------------------------------
// Mobile colour emitters (T3, issue #851)
// --------------------------------------------------------------------------

test('emitIosColors reproduces the committed Color+TownCrier.swift byte-for-byte', () => {
  assert.equal(emitIosColors(tokens), readFileSync(IOS_COLORS_PATH, 'utf8'));
});

test('emitIosColors leads with the generated-file header', () => {
  const out = emitIosColors(tokens);
  assert.ok(out.startsWith('// Code generated by scripts/design-tokens/generate.mjs. DO NOT EDIT.\n\nimport SwiftUI\n'));
});

test('emitIosColors skips amber-hover (per-platform skip list)', () => {
  const out = emitIosColors(tokens);
  assert.ok(!out.includes('tcAmberHover'), 'iOS must not gain an amber-hover token');
  assert.ok(!out.includes('amber-hover'));
  assert.ok(out.includes('public static let tcAmber ='));
});

test('emitIosColors renders derived tokens in their iOS forms', () => {
  const out = emitIosColors(tokens);
  assert.ok(out.includes('public static let tcAmberMuted = tcAmber.opacity(0.15)'));
  assert.ok(out.includes('public static let tcBorderFocused = tcAmber\n'));
  // overlay is a flat scrim using the light-theme alpha, not a themed colour.
  assert.ok(out.includes('public static let tcOverlay = Color.black.opacity(0.4)'));
});

test('emitIosColors wraps status-rejected but keeps status-appealed inline (byte-exact wrapping)', () => {
  const out = emitIosColors(tokens);
  assert.ok(out.includes('public static let tcStatusRejected = Color.themed(\n    light: 0xC42B2B, dark: 0xFF453A, oled: 0xFF453A)'));
  assert.ok(out.includes('public static let tcStatusAppealed = Color.themed(light: 0x7C3AED, dark: 0xA78BFA, oled: 0xA78BFA)'));
});

test('emitAndroidColors reproduces the committed Color.kt byte-for-byte', () => {
  assert.equal(emitAndroidColors(tokens), readFileSync(ANDROID_COLORS_PATH, 'utf8'));
});

test('emitAndroidColors keeps amber-hover and formats overlay alpha per theme', () => {
  const out = emitAndroidColors(tokens);
  assert.ok(out.includes('amberHover = Color(0xFFB87A08)'));
  assert.ok(out.includes('overlay = Color.Black.copy(alpha = 0.40f)'), 'light overlay alpha');
  assert.ok(out.includes('overlay = Color.Black.copy(alpha = 0.50f)'), 'dark overlay alpha');
  assert.ok(out.includes('amberMuted = muted(Color(0xFFD4910A))'), 'amber-muted uses the muted() helper');
});

test('emitAndroidColors OledPalette overrides exactly the four differing fields', () => {
  const out = emitAndroidColors(tokens);
  const oledBlock = out.slice(out.indexOf('DarkPalette.copy('));
  const overridden = [...oledBlock.matchAll(/^ {8}(\w+) =/gm)].map((m) => m[1]);
  assert.deepEqual(overridden, ['background', 'surface', 'surfaceElevated', 'border']);
});

// --------------------------------------------------------------------------
// Skill-doc emitter (T3, issue #851)
// --------------------------------------------------------------------------

test('emitSkillDoc reproduces the committed tokens.md byte-for-byte', () => {
  assert.equal(emitSkillDoc(tokens), readFileSync(SKILL_DOC_PATH, 'utf8'));
});

test('emitSkillDoc regenerates only between the markers; outside prose survives', () => {
  const original = readFileSync(SKILL_DOC_PATH, 'utf8');
  try {
    // Add a prose line after the end marker and tamper inside the markers.
    const withProse = original.replace(SKILL_END, `${SKILL_END}\n\nHAND-WRITTEN SENTINEL`);
    const tampered = withProse.replace(SKILL_BEGIN, `${SKILL_BEGIN}\n<!-- junk -->`);
    writeFileSync(SKILL_DOC_PATH, tampered);

    const regenerated = emitSkillDoc(tokens);
    assert.ok(regenerated.includes('HAND-WRITTEN SENTINEL'), 'prose outside the markers must survive');
    assert.ok(!regenerated.includes('<!-- junk -->'), 'content between the markers is regenerated');
    assert.ok(regenerated.includes('`TcPalette.statusConditions`'), 'tables are present');
  } finally {
    writeFileSync(SKILL_DOC_PATH, original);
  }
});

test('emitSkillDoc uses real names, includes status-conditions, drops the drifted names', () => {
  const out = emitSkillDoc(tokens);
  const generated = out.slice(out.indexOf(SKILL_BEGIN), out.indexOf(SKILL_END));
  assert.ok(!out.includes('tcStatusApproved'), 'the drifted tcStatusApproved name is gone');
  assert.ok(!out.includes('tcStatusRefused'), 'the drifted tcStatusRefused name is gone');
  assert.ok(!out.includes('tcSpaceMD'), 'the drifted tcSpaceMD name is gone');
  assert.ok(generated.includes('`status-conditions`'), 'status-conditions is documented');
  assert.ok(generated.includes('`Color.tcStatusPermitted`') && generated.includes('`TCSpacing.medium`'));
  // web-only / web+android tokens are qualified by the platforms column.
  assert.ok(/`amber-hover` \|.*\| — \|.*\| Web, Android \|/.test(generated), 'amber-hover marked web+Android, no iOS');
  assert.ok(/`card` \|.*\| Web \|/.test(generated), 'shadows marked web-only');
});

// --------------------------------------------------------------------------
// Drift gate covers every output (T3)
// --------------------------------------------------------------------------

test('--check catches a tamper in each generated output', () => {
  const outputs = buildOutputs(tokens);
  assert.equal(outputs.length, 5, 'expected web(2) + iOS + Android + skill doc');

  for (const { path } of outputs) {
    const original = readFileSync(path, 'utf8');
    // Tamper inside the regenerated region: for the marker-based skill doc that
    // means just after the begin marker; every other output regenerates wholly.
    const tampered = original.includes(SKILL_BEGIN)
      ? original.replace(SKILL_BEGIN, `${SKILL_BEGIN}\n<!-- tampered -->`)
      : `${original}\n/* tampered */\n`;
    try {
      writeFileSync(path, tampered);
      let exitCode = 0;
      try {
        execFileSync('node', [GENERATE, '--check'], { stdio: 'pipe' });
      } catch (err) {
        exitCode = err.status;
      }
      assert.notEqual(exitCode, 0, `--check should flag a tampered ${path}`);
    } finally {
      writeFileSync(path, original);
    }
  }
  // Everything restored; the tree is clean again.
  execFileSync('node', [GENERATE, '--check'], { stdio: 'pipe' });
});
