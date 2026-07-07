/**
 * Build-time QR code rendering for the static planning pages.
 *
 * Desktop visitors who click an App Store link land on Apple's web listing and
 * have to remember to find the app on their phone later — intent usually dies
 * there. A QR code lets them scan straight to the store instead. The code is
 * generated at prerender time and inlined as SVG, keeping the pages
 * self-contained (no runtime JS, no image request, no third-party call).
 *
 * Uses the `qrcode` package's synchronous low-level `create()` API (the
 * high-level `toString()` is Promise-based, and every page renderer here is
 * sync) and draws the module matrix as a single `<path>`.
 */

import QRCode from 'qrcode';
import { escapeHtml } from './format.mjs';

/** Quiet-zone border around the module matrix, in modules. The QR spec's
 * minimum for reliable scanning is 4. */
const QUIET_ZONE = 4;

/**
 * Render `text` as a self-contained SVG QR code.
 *
 * Colours are deliberately hard-coded rather than theme tokens: the tile must
 * stay dark-on-light in dark mode too, or older phone cameras refuse to read
 * it. The page's stylesheet handles sizing and rounding; the SVG itself only
 * carries the viewBox, a white tile and the black module path.
 *
 * @param {string} text        the URL (or any payload) to encode
 * @param {string} ariaLabel   accessible name for the graphic
 * @returns {string} an inline `<svg>` element
 */
export function qrSvg(text, ariaLabel) {
  const qr = QRCode.create(text, { errorCorrectionLevel: 'M' });
  const size = qr.modules.size;
  const boxSize = size + QUIET_ZONE * 2;

  const segments = [];
  for (let row = 0; row < size; row += 1) {
    for (let col = 0; col < size; col += 1) {
      if (qr.modules.get(row, col)) {
        segments.push(`M${col + QUIET_ZONE} ${row + QUIET_ZONE}h1v1h-1z`);
      }
    }
  }

  return (
    `<svg class="qr" role="img" aria-label="${escapeHtml(ariaLabel)}" ` +
    `viewBox="0 0 ${boxSize} ${boxSize}" shape-rendering="crispEdges" xmlns="http://www.w3.org/2000/svg">` +
    `<rect width="${boxSize}" height="${boxSize}" fill="#FFFFFF"/>` +
    `<path d="${segments.join('')}" fill="#000000"/>` +
    `</svg>`
  );
}
