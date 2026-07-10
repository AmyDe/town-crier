/**
 * Tests for the Android launcher-icon foreground generator (issue #860, R7).
 * Run with:
 *   node --test scripts/design-tokens/
 *
 * Stdlib node:test only — no npm dependencies, matching the generator itself.
 * These assert the two properties the drift gate depends on: the downscale is
 * deterministic (idempotent), and the PNG codec round-trips losslessly, so a
 * committed foreground can be pixel-compared against a fresh downscale.
 */

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

import {
  SOURCE_PATH,
  DENSITIES,
  decodePng,
  encodePng,
  resize,
  buildForegrounds,
  run,
} from './generate-android-icon.mjs';

const master = decodePng(readFileSync(SOURCE_PATH));

test('master icon decodes to a square RGB(A) raster', () => {
  assert.equal(master.width, master.height, 'app icon must be square');
  assert.ok(master.width >= 432, 'master must be at least the xxxhdpi foreground size');
  assert.ok(master.channels === 3 || master.channels === 4, 'expected truecolour RGB or RGBA');
});

test('resize is deterministic — same pixels every run', () => {
  const a = resize(master, 216, 216);
  const b = resize(master, 216, 216);
  assert.deepEqual(a.width, b.width);
  assert.deepEqual(a.height, b.height);
  assert.ok(a.data.equals(b.data), 'two downscales of the same master must be byte-identical');
});

test('resize produces the requested dimensions for every density bucket', () => {
  for (const { size } of DENSITIES) {
    const img = resize(master, size, size);
    assert.equal(img.width, size);
    assert.equal(img.height, size);
    assert.equal(img.data.length, size * size * img.channels);
  }
});

test('PNG encode/decode round-trips losslessly', () => {
  const small = resize(master, 108, 108);
  const roundTripped = decodePng(encodePng(small));
  assert.equal(roundTripped.width, small.width);
  assert.equal(roundTripped.height, small.height);
  assert.equal(roundTripped.channels, small.channels);
  assert.ok(roundTripped.data.equals(small.data), 'decoded pixels must equal the source pixels (lossless)');
});

test('encode is byte-stable within a runtime (idempotent output)', () => {
  const small = resize(master, 162, 162);
  assert.ok(encodePng(small).equals(encodePng(small)), 'encoding the same image twice must be byte-identical');
});

test('committed foregrounds are in sync with the master (--check would pass)', () => {
  // buildForegrounds returns the fresh in-memory downscales; run({check:true})
  // decodes the committed PNGs and pixel-compares. Zero stale == in sync.
  assert.equal(buildForegrounds().length, DENSITIES.length);
  const { stale } = run({ check: true });
  assert.deepEqual(stale, [], `stale foregrounds: ${stale.join(', ')}`);
});
