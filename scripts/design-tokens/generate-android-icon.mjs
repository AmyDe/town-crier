#!/usr/bin/env node
/**
 * Android adaptive launcher-icon foreground generator (issue #860, R7).
 *
 * The Public Notice rebrand KEEPS the existing app icon — the navy line-art
 * house-with-hanging-bell on paper. The product owner reviewed and rejected
 * every new-mark candidate, so there is deliberately no new artwork here. The
 * single master asset is the iOS app icon raster:
 *
 *   mobile/ios/town-crier-app/Resources/Assets.xcassets/AppIcon.appiconset/AppIcon.png
 *
 * There is no vector source for that artwork, so the Android adaptive icon's
 * foreground layer is a density-bucketed BITMAP generated from that master
 * (hand-vectorising it would risk visual drift from the approved icon). This
 * script reads the master once and (re)writes the five density foregrounds:
 *
 *   mobile/android/app/src/main/res/mipmap-{mdpi,hdpi,xhdpi,xxhdpi,xxxhdpi}/ic_launcher_foreground.png
 *
 * sized to the adaptive-icon foreground of 108dp at each density bucket.
 *
 *   node scripts/design-tokens/generate-android-icon.mjs           regenerate every foreground
 *   node scripts/design-tokens/generate-android-icon.mjs --check   fail (exit 1) if any is stale
 *
 * DEPENDENCY-FREE by design — Node >= 18 stdlib only (zlib for the PNG DEFLATE
 * stream), matching generate.mjs. That is a hard requirement, not a preference:
 * the design-token drift gate runs on CI with no `npm install`, so sharp / sips
 * / ImageMagick are all unavailable there. A tiny self-contained PNG codec is
 * the only thing that runs identically on a dev Mac and on the Linux CI runner.
 *
 * `--check` compares decoded PIXELS (not raw file bytes) so it is robust to
 * DEFLATE output differing between Node/zlib versions: the committed PNG is
 * decoded and its pixels are compared against a fresh downscale of the master.
 * Same pixels -> in sync, regardless of which runtime wrote the file.
 */

import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { deflateSync, inflateSync } from 'node:zlib';
import { fileURLToPath } from 'node:url';
import { dirname, join, relative } from 'node:path';
import { parseArgs } from 'node:util';

const HERE = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = join(HERE, '..', '..');

export const SOURCE_PATH = join(
  REPO_ROOT,
  'mobile',
  'ios',
  'town-crier-app',
  'Resources',
  'Assets.xcassets',
  'AppIcon.appiconset',
  'AppIcon.png',
);

const ANDROID_RES = join(REPO_ROOT, 'mobile', 'android', 'app', 'src', 'main', 'res');

/**
 * Adaptive-icon foreground is 108dp; these are its pixel sizes per density
 * bucket (mdpi = 1x = 160dpi). Standard Android launcher-icon densities.
 */
export const DENSITIES = [
  { dir: 'mipmap-mdpi', size: 108 },
  { dir: 'mipmap-hdpi', size: 162 },
  { dir: 'mipmap-xhdpi', size: 216 },
  { dir: 'mipmap-xxhdpi', size: 324 },
  { dir: 'mipmap-xxxhdpi', size: 432 },
];

const FOREGROUND_FILE = 'ic_launcher_foreground.png';

const PNG_SIGNATURE = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);

// --------------------------------------------------------------------------
// CRC-32 (PNG chunk checksum) — precomputed table, IEEE polynomial.
// --------------------------------------------------------------------------

const CRC_TABLE = (() => {
  const table = new Uint32Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) {
      c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    }
    table[n] = c >>> 0;
  }
  return table;
})();

function crc32(buf) {
  let c = 0xffffffff;
  for (let i = 0; i < buf.length; i++) {
    c = CRC_TABLE[(c ^ buf[i]) & 0xff] ^ (c >>> 8);
  }
  return (c ^ 0xffffffff) >>> 0;
}

function paethPredictor(a, b, c) {
  const p = a + b - c;
  const pa = Math.abs(p - a);
  const pb = Math.abs(p - b);
  const pc = Math.abs(p - c);
  if (pa <= pb && pa <= pc) return a;
  if (pb <= pc) return b;
  return c;
}

// --------------------------------------------------------------------------
// PNG decode (8-bit, non-interlaced, truecolour RGB or RGBA)
// --------------------------------------------------------------------------

/**
 * @param {Buffer} buffer
 * @returns {{ width: number, height: number, channels: number, data: Buffer }}
 */
export function decodePng(buffer) {
  if (!buffer.subarray(0, 8).equals(PNG_SIGNATURE)) {
    throw new Error('not a PNG (bad signature)');
  }
  let offset = 8;
  let ihdr = null;
  const idatParts = [];
  while (offset < buffer.length) {
    const length = buffer.readUInt32BE(offset);
    const type = buffer.toString('ascii', offset + 4, offset + 8);
    const data = buffer.subarray(offset + 8, offset + 8 + length);
    if (type === 'IHDR') ihdr = data;
    else if (type === 'IDAT') idatParts.push(Buffer.from(data));
    else if (type === 'IEND') break;
    offset += 12 + length; // length(4) + type(4) + data + crc(4)
  }
  if (!ihdr) throw new Error('PNG missing IHDR');

  const width = ihdr.readUInt32BE(0);
  const height = ihdr.readUInt32BE(4);
  const bitDepth = ihdr[8];
  const colorType = ihdr[9];
  const interlace = ihdr[12];
  if (bitDepth !== 8) throw new Error(`unsupported PNG bit depth ${bitDepth} (need 8)`);
  if (interlace !== 0) throw new Error('unsupported interlaced PNG');
  const channels = colorType === 2 ? 3 : colorType === 6 ? 4 : 0;
  if (channels === 0) throw new Error(`unsupported PNG colour type ${colorType} (need 2 RGB or 6 RGBA)`);

  const raw = inflateSync(Buffer.concat(idatParts));
  const stride = width * channels;
  const out = Buffer.alloc(height * stride);
  for (let y = 0; y < height; y++) {
    const filter = raw[y * (stride + 1)];
    const lineStart = y * (stride + 1) + 1;
    for (let i = 0; i < stride; i++) {
      const x = raw[lineStart + i];
      const a = i >= channels ? out[y * stride + i - channels] : 0;
      const b = y > 0 ? out[(y - 1) * stride + i] : 0;
      const c = y > 0 && i >= channels ? out[(y - 1) * stride + i - channels] : 0;
      let recon;
      switch (filter) {
        case 0:
          recon = x;
          break;
        case 1:
          recon = x + a;
          break;
        case 2:
          recon = x + b;
          break;
        case 3:
          recon = x + ((a + b) >> 1);
          break;
        case 4:
          recon = x + paethPredictor(a, b, c);
          break;
        default:
          throw new Error(`unsupported PNG row filter ${filter}`);
      }
      out[y * stride + i] = recon & 0xff;
    }
  }
  return { width, height, channels, data: out };
}

// --------------------------------------------------------------------------
// Box-filter downscale — deterministic integer maths (so --check can compare
// pixels exactly). Averages the source box covering each destination pixel.
// --------------------------------------------------------------------------

/**
 * @param {{ width: number, height: number, channels: number, data: Buffer }} img
 * @param {number} dstW
 * @param {number} dstH
 */
export function resize(img, dstW, dstH) {
  const { width: srcW, height: srcH, channels, data } = img;
  const dst = Buffer.alloc(dstW * dstH * channels);
  const scaleX = srcW / dstW;
  const scaleY = srcH / dstH;
  for (let dy = 0; dy < dstH; dy++) {
    let sy0 = Math.floor(dy * scaleY);
    let sy1 = Math.floor((dy + 1) * scaleY);
    if (sy1 <= sy0) sy1 = sy0 + 1;
    if (sy1 > srcH) sy1 = srcH;
    for (let dx = 0; dx < dstW; dx++) {
      let sx0 = Math.floor(dx * scaleX);
      let sx1 = Math.floor((dx + 1) * scaleX);
      if (sx1 <= sx0) sx1 = sx0 + 1;
      if (sx1 > srcW) sx1 = srcW;
      const count = (sx1 - sx0) * (sy1 - sy0);
      for (let ch = 0; ch < channels; ch++) {
        let sum = 0;
        for (let sy = sy0; sy < sy1; sy++) {
          const rowBase = (sy * srcW + sx0) * channels + ch;
          for (let sx = 0, p = rowBase; sx < sx1 - sx0; sx++, p += channels) {
            sum += data[p];
          }
        }
        dst[(dy * dstW + dx) * channels + ch] = Math.round(sum / count);
      }
    }
  }
  return { width: dstW, height: dstH, channels, data: dst };
}

// --------------------------------------------------------------------------
// PNG encode (8-bit, non-interlaced) with adaptive per-row filtering.
// --------------------------------------------------------------------------

function chunk(type, data) {
  const length = Buffer.alloc(4);
  length.writeUInt32BE(data.length, 0);
  const typeBuf = Buffer.from(type, 'ascii');
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(Buffer.concat([typeBuf, data])), 0);
  return Buffer.concat([length, typeBuf, data, crc]);
}

/** Sum of magnitudes of a filtered row, interpreting bytes as signed 8-bit. */
function filteredCost(row) {
  let sum = 0;
  for (let i = 0; i < row.length; i++) {
    const v = row[i];
    sum += v < 128 ? v : 256 - v;
  }
  return sum;
}

/**
 * @param {{ width: number, height: number, channels: number, data: Buffer }} img
 * @returns {Buffer}
 */
export function encodePng(img) {
  const { width, height, channels, data } = img;
  const colorType = channels === 4 ? 6 : 2;
  const stride = width * channels;

  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0);
  ihdr.writeUInt32BE(height, 4);
  ihdr[8] = 8; // bit depth
  ihdr[9] = colorType;
  ihdr[10] = 0; // compression
  ihdr[11] = 0; // filter method
  ihdr[12] = 0; // interlace

  // Filter each scanline, choosing the cheapest of the five filter types
  // (minimum-sum-of-absolute-differences heuristic — deterministic).
  const filtered = Buffer.alloc(height * (stride + 1));
  const candidate = Buffer.alloc(stride);
  for (let y = 0; y < height; y++) {
    let best = null;
    let bestCost = Infinity;
    let bestType = 0;
    for (let ft = 0; ft <= 4; ft++) {
      for (let i = 0; i < stride; i++) {
        const x = data[y * stride + i];
        const a = i >= channels ? data[y * stride + i - channels] : 0;
        const b = y > 0 ? data[(y - 1) * stride + i] : 0;
        const c = y > 0 && i >= channels ? data[(y - 1) * stride + i - channels] : 0;
        let f;
        switch (ft) {
          case 0:
            f = x;
            break;
          case 1:
            f = x - a;
            break;
          case 2:
            f = x - b;
            break;
          case 3:
            f = x - ((a + b) >> 1);
            break;
          default:
            f = x - paethPredictor(a, b, c);
            break;
        }
        candidate[i] = f & 0xff;
      }
      const cost = filteredCost(candidate);
      if (cost < bestCost) {
        bestCost = cost;
        bestType = ft;
        best = Buffer.from(candidate);
      }
    }
    filtered[y * (stride + 1)] = bestType;
    best.copy(filtered, y * (stride + 1) + 1);
  }

  const idat = deflateSync(filtered, { level: 9 });
  return Buffer.concat([
    PNG_SIGNATURE,
    chunk('IHDR', ihdr),
    chunk('IDAT', idat),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

// --------------------------------------------------------------------------
// Driver
// --------------------------------------------------------------------------

/** The master decoded once, downscaled per density into an in-memory image. */
export function buildForegrounds(sourcePath = SOURCE_PATH) {
  const master = decodePng(readFileSync(sourcePath));
  return DENSITIES.map(({ dir, size }) => ({
    path: join(ANDROID_RES, dir, FOREGROUND_FILE),
    image: resize(master, size, size),
  }));
}

function pixelsMatch(a, b) {
  return a.width === b.width && a.height === b.height && a.channels === b.channels && a.data.equals(b.data);
}

/**
 * Write every foreground (default) or, in check mode, report stale ones by
 * decoding the committed PNG and comparing pixels to a fresh downscale.
 *
 * @param {{ check?: boolean, sourcePath?: string }} [opts]
 * @returns {{ stale: string[] }}
 */
export function run(opts = {}) {
  const outputs = buildForegrounds(opts.sourcePath);
  const stale = [];
  for (const { path, image } of outputs) {
    if (opts.check) {
      let committed = null;
      try {
        committed = decodePng(readFileSync(path));
      } catch {
        committed = null;
      }
      if (!committed || !pixelsMatch(committed, image)) {
        stale.push(relative(REPO_ROOT, path));
      }
    } else {
      mkdirSync(dirname(path), { recursive: true });
      writeFileSync(path, encodePng(image));
    }
  }
  return { stale };
}

function main() {
  const { values } = parseArgs({ options: { check: { type: 'boolean', default: false } } });
  const { stale } = run({ check: values.check });

  if (values.check) {
    if (stale.length > 0) {
      console.error('error: generated Android launcher-icon foregrounds are stale:');
      for (const f of stale) console.error(`  ${f}`);
      console.error("  fix: run 'node scripts/design-tokens/generate-android-icon.mjs' and commit the result");
      process.exit(1);
    }
    console.log(`Android launcher-icon foregrounds in sync (${DENSITIES.length} density bucket(s))`);
    return;
  }

  console.log(`Android launcher-icon foregrounds generated (${DENSITIES.length} density bucket(s))`);
}

// Run only when invoked directly (not when imported by the test file).
if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  main();
}
