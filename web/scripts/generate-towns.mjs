/**
 * generate-towns.mjs — one-time / occasional generator for the committed town
 * gazetteer `web/src/data/towns.json` (`[{ slug, name, lat, lng, authorityId }]`).
 *
 * THIS SCRIPT IS NOT PART OF THE BUILD. It is run by a human, by hand, when the
 * gazetteer needs (re)generating. The build (`prerender-planning.mjs`) only ever
 * READS the committed JSON — it never downloads OS data.
 *
 * ── Data source ────────────────────────────────────────────────────────────
 * OS Open Names (Ordnance Survey OpenData, Open Government Licence v3, no API
 * key). It lists every named place in GB with a settlement classification
 * (`LOCAL_TYPE`) and a British National Grid coordinate.
 *
 *   https://osdatahub.os.uk/downloads/open/OpenNames
 *
 * We keep only `LOCAL_TYPE in (City, Town)` — villages and hamlets are dropped
 * to control page count and thin-content risk (issue #570 Phase 2 addendum).
 *
 * ── How to regenerate (manual) ─────────────────────────────────────────────
 *   1. Download "OS Open Names" (CSV) and unzip it. The payload is a `DATA/`
 *      directory of grid-square CSV files (no header row) plus a
 *      `DOC/OS_Open_Names_Header.csv` listing the column order.
 *   2. Cross-check `OS_OPEN_NAMES_COLUMNS` below against that header file — OS
 *      occasionally revises the schema. The indices here follow the long-stable
 *      Open Names CSV layout.
 *   3. Run:
 *        node scripts/generate-towns.mjs /path/to/OS_Open_Names/DATA
 *      It filters to City/Town, converts each BNG easting/northing to WGS84
 *      lat/lng, resolves the parent LPA from the OS DISTRICT_BOROUGH /
 *      COUNTY_UNITARY name against `api-go/internal/geocoding/authority-mapping.json`,
 *      de-duplicates, sorts, and overwrites `web/src/data/towns.json`.
 *   4. Review the diff, run `npx vitest run`, and commit.
 *
 * ── Town → LPA resolution ──────────────────────────────────────────────────
 * Primary path: the OS DISTRICT_BOROUGH (metropolitan/London/two-tier districts)
 * or COUNTY_UNITARY (unitary authorities) name, looked up in the committed
 * district-name → authority-id map. Towns whose OS administrative name does not
 * match an authority in our list are skipped (logged), never guessed.
 *
 * Fallback (not automated here): for stubborn unmatched towns, reverse-geocode
 * the OS POSTCODE_DISTRICT via postcodes.io `/outcodes/{outcode}` — it returns
 * `admin_district` and a WGS84 centroid directly — then map that district name
 * the same way. Left as a manual step because it makes one network call per
 * town and must never run in CI.
 *
 * ── Coordinate conversion ──────────────────────────────────────────────────
 * OS Open Names coordinates are British National Grid (OSGB36, EPSG:27700)
 * eastings/northings. `bngToLatLon` does the standard inverse Transverse
 * Mercator (Airy 1830) plus a Helmert datum shift to WGS84 — accurate to a few
 * metres, far finer than the 4-dp (~11 m) coordinates we store.
 */

import { readFile, readdir, writeFile } from 'node:fs/promises';
import { join, dirname } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { slugify } from './lib/slug.mjs';

const SCRIPT_DIR = dirname(fileURLToPath(import.meta.url));

/** Output gazetteer path (the file the build reads). */
const TOWNS_OUT = join(SCRIPT_DIR, '..', 'src', 'data', 'towns.json');

/** District-name → authority-id map, the same one the Go geocoder uses. */
const AUTHORITY_MAPPING_FILE = join(
  SCRIPT_DIR,
  '..',
  '..',
  'api-go',
  'internal',
  'geocoding',
  'authority-mapping.json',
);

/**
 * Zero-based column indices in the OS Open Names CSV (DATA rows have no header;
 * this mirrors DOC/OS_Open_Names_Header.csv). Re-verify against the shipped
 * header file when regenerating — see the script header.
 * @type {Record<string, number>}
 */
export const OS_OPEN_NAMES_COLUMNS = {
  NAME1: 2,
  LOCAL_TYPE: 7,
  GEOMETRY_X: 8,
  GEOMETRY_Y: 9,
  DISTRICT_BOROUGH: 21,
  COUNTY_UNITARY: 24,
};

/** OS Open Names settlement classes we publish a page for. */
const QUALIFYING_LOCAL_TYPES = new Set(['City', 'Town']);

/**
 * @param {string} localType OS `LOCAL_TYPE` value
 * @returns {boolean} whether the place is a City or Town
 */
export function localTypeQualifies(localType) {
  return QUALIFYING_LOCAL_TYPES.has(localType);
}

/**
 * Resolve an OS administrative name to one of our authority ids. Prefers the
 * district/borough name (covers metropolitan, London and two-tier districts);
 * falls back to the county/unitary name (covers unitary authorities). Returns
 * null when neither matches — the caller then skips the town.
 *
 * @param {string} districtBorough OS `DISTRICT_BOROUGH`
 * @param {string} countyUnitary   OS `COUNTY_UNITARY`
 * @param {Record<string, number>} mapping district-name -> authority-id
 * @returns {number | null}
 */
export function resolveAuthorityId(districtBorough, countyUnitary, mapping) {
  const district = (districtBorough || '').trim();
  if (district && Object.prototype.hasOwnProperty.call(mapping, district)) {
    return mapping[district];
  }
  const county = (countyUnitary || '').trim();
  if (county && Object.prototype.hasOwnProperty.call(mapping, county)) {
    return mapping[county];
  }
  return null;
}

// ── OSGB36 British National Grid -> WGS84 ──────────────────────────────────
// Standard inverse Transverse Mercator on the Airy 1830 ellipsoid, then a
// Helmert transformation from the OSGB36 datum to WGS84. Constants are the
// published OS / EPSG values.

const DEG = 180 / Math.PI;

/**
 * Convert a British National Grid easting/northing (OSGB36, metres) to a WGS84
 * latitude/longitude in decimal degrees, rounded to 4 dp.
 *
 * @param {number} easting
 * @param {number} northing
 * @returns {{ lat: number, lng: number }}
 */
export function bngToLatLon(easting, northing) {
  // Airy 1830 ellipsoid (the OSGB36 figure of the Earth).
  const a = 6377563.396;
  const b = 6356256.909;
  const F0 = 0.9996012717; // central meridian scale factor
  const lat0 = (49 * Math.PI) / 180; // true origin latitude
  const lon0 = (-2 * Math.PI) / 180; // true origin longitude
  const N0 = -100000;
  const E0 = 400000;
  const e2 = 1 - (b * b) / (a * a);
  const n = (a - b) / (a + b);
  const n2 = n * n;
  const n3 = n * n * n;

  // Iteratively solve for the footpoint latitude.
  let lat = lat0;
  let M = 0;
  do {
    lat = (northing - N0 - M) / (a * F0) + lat;
    const Ma = (1 + n + (5 / 4) * n2 + (5 / 4) * n3) * (lat - lat0);
    const Mb =
      (3 * n + 3 * n2 + (21 / 8) * n3) *
      Math.sin(lat - lat0) *
      Math.cos(lat + lat0);
    const Mc =
      ((15 / 8) * n2 + (15 / 8) * n3) *
      Math.sin(2 * (lat - lat0)) *
      Math.cos(2 * (lat + lat0));
    const Md = (35 / 24) * n3 * Math.sin(3 * (lat - lat0)) * Math.cos(3 * (lat + lat0));
    M = b * F0 * (Ma - Mb + Mc - Md);
  } while (Math.abs(northing - N0 - M) >= 0.00001);

  const sinLat = Math.sin(lat);
  const cosLat = Math.cos(lat);
  const tanLat = Math.tan(lat);
  const nu = (a * F0) / Math.sqrt(1 - e2 * sinLat * sinLat);
  const rho = (a * F0 * (1 - e2)) / Math.pow(1 - e2 * sinLat * sinLat, 1.5);
  const eta2 = nu / rho - 1;

  const tan2 = tanLat * tanLat;
  const tan4 = tan2 * tan2;
  const sec = 1 / cosLat;

  const VII = tanLat / (2 * rho * nu);
  const VIII = (tanLat / (24 * rho * Math.pow(nu, 3))) * (5 + 3 * tan2 + eta2 - 9 * tan2 * eta2);
  const IX = (tanLat / (720 * rho * Math.pow(nu, 5))) * (61 + 90 * tan2 + 45 * tan4);
  const X = sec / nu;
  const XI = (sec / (6 * Math.pow(nu, 3))) * (nu / rho + 2 * tan2);
  const XII = (sec / (120 * Math.pow(nu, 5))) * (5 + 28 * tan2 + 24 * tan4);
  const XIIA =
    (sec / (5040 * Math.pow(nu, 7))) * (61 + 662 * tan2 + 1320 * tan4 + 720 * tan4 * tan2);

  const dE = easting - E0;
  const dE2 = dE * dE;
  const latAiry = lat - VII * dE2 + VIII * dE2 * dE2 - IX * dE2 * dE2 * dE2;
  const lonAiry =
    lon0 + X * dE - XI * dE * dE2 + XII * dE * dE2 * dE2 - XIIA * dE * dE2 * dE2 * dE2;

  return helmertAiryToWgs84(latAiry, lonAiry, a, b);
}

/**
 * Helmert datum shift: OSGB36 (Airy) lat/lon -> WGS84 lat/lon. Goes via
 * geocentric cartesian coordinates.
 *
 * @param {number} lat OSGB36 latitude (radians)
 * @param {number} lon OSGB36 longitude (radians)
 * @param {number} a   Airy semi-major axis
 * @param {number} b   Airy semi-minor axis
 * @returns {{ lat: number, lng: number }} WGS84 lat/lng in degrees, 4 dp
 */
function helmertAiryToWgs84(lat, lon, a, b) {
  const e2 = 1 - (b * b) / (a * a);
  const sinLat = Math.sin(lat);
  const cosLat = Math.cos(lat);
  const sinLon = Math.sin(lon);
  const cosLon = Math.cos(lon);
  const nu = a / Math.sqrt(1 - e2 * sinLat * sinLat);
  const H = 0; // settlement points carry no height

  const x1 = (nu + H) * cosLat * cosLon;
  const y1 = (nu + H) * cosLat * sinLon;
  const z1 = ((1 - e2) * nu + H) * sinLat;

  // OSGB36 -> WGS84 Helmert parameters (negated WGS84 -> OSGB36 set).
  const tx = 446.448;
  const ty = -125.157;
  const tz = 542.06;
  const s = 20.4894e-6;
  const rx = (0.1502 / 3600) * (Math.PI / 180);
  const ry = (0.247 / 3600) * (Math.PI / 180);
  const rz = (0.8421 / 3600) * (Math.PI / 180);

  const x2 = tx + (1 + s) * x1 - rz * y1 + ry * z1;
  const y2 = ty + rz * x1 + (1 + s) * y1 - rx * z1;
  const z2 = tz - ry * x1 + rx * y1 + (1 + s) * z1;

  // WGS84 ellipsoid.
  const aW = 6378137.0;
  const bW = 6356752.3142;
  const e2W = 1 - (bW * bW) / (aW * aW);
  const p = Math.sqrt(x2 * x2 + y2 * y2);
  let latW = Math.atan2(z2, p * (1 - e2W));
  let latPrev;
  let nuW;
  do {
    latPrev = latW;
    nuW = aW / Math.sqrt(1 - e2W * Math.sin(latW) * Math.sin(latW));
    latW = Math.atan2(z2 + e2W * nuW * Math.sin(latW), p);
  } while (Math.abs(latW - latPrev) >= 1e-12);
  const lonW = Math.atan2(y2, x2);

  return {
    lat: round4(latW * DEG),
    lng: round4(lonW * DEG),
  };
}

/**
 * @param {number} v
 * @returns {number} v rounded to 4 decimal places
 */
function round4(v) {
  return Math.round(v * 1e4) / 1e4;
}

/**
 * Build a gazetteer record from one parsed OS Open Names CSV row, or null when
 * the place is not a City/Town or its authority can't be resolved.
 *
 * @param {string[]} fields  CSV row split on commas
 * @param {Record<string, number>} mapping district-name -> authority-id
 * @returns {{ slug: string, name: string, lat: number, lng: number, authorityId: number } | null}
 */
export function townRecordFromRow(fields, mapping) {
  const localType = fields[OS_OPEN_NAMES_COLUMNS.LOCAL_TYPE];
  if (!localTypeQualifies(localType)) {
    return null;
  }
  const name = (fields[OS_OPEN_NAMES_COLUMNS.NAME1] || '').trim();
  if (!name) {
    return null;
  }
  const authorityId = resolveAuthorityId(
    fields[OS_OPEN_NAMES_COLUMNS.DISTRICT_BOROUGH],
    fields[OS_OPEN_NAMES_COLUMNS.COUNTY_UNITARY],
    mapping,
  );
  if (authorityId === null) {
    return null;
  }
  const easting = Number(fields[OS_OPEN_NAMES_COLUMNS.GEOMETRY_X]);
  const northing = Number(fields[OS_OPEN_NAMES_COLUMNS.GEOMETRY_Y]);
  if (!Number.isFinite(easting) || !Number.isFinite(northing)) {
    return null;
  }
  const { lat, lng } = bngToLatLon(easting, northing);
  return { slug: slugify(name), name, lat, lng, authorityId };
}

/**
 * Split a CSV line on commas, stripping a single layer of double quotes. OS Open
 * Names place fields do not contain embedded commas, so a full RFC-4180 parser
 * is unnecessary.
 *
 * @param {string} line
 * @returns {string[]}
 */
function splitCsvLine(line) {
  return line.split(',').map((f) => f.replace(/^"|"$/g, ''));
}

/**
 * CLI entry: read every CSV under `dataDir`, build the gazetteer, write
 * towns.json. Deduplicates by `authorityId/slug` and sorts for a stable diff.
 *
 * @param {string} dataDir OS Open Names DATA directory
 * @returns {Promise<void>}
 */
async function main(dataDir) {
  if (!dataDir) {
    console.error(
      'usage: node scripts/generate-towns.mjs <OS_Open_Names_DATA_dir>\n' +
        'See the script header for the full manual regeneration steps.',
    );
    process.exitCode = 1;
    return;
  }

  const mapping = JSON.parse(await readFile(AUTHORITY_MAPPING_FILE, 'utf-8'));
  const files = (await readdir(dataDir)).filter((f) => f.toLowerCase().endsWith('.csv'));

  /** @type {Map<string, { slug: string, name: string, lat: number, lng: number, authorityId: number }>} */
  const byKey = new Map();
  for (const file of files) {
    const text = await readFile(join(dataDir, file), 'utf-8');
    for (const line of text.split(/\r?\n/)) {
      if (!line) continue;
      const record = townRecordFromRow(splitCsvLine(line), mapping);
      if (record) {
        byKey.set(`${record.authorityId}/${record.slug}`, record);
      }
    }
  }

  const towns = [...byKey.values()].sort(
    (x, y) =>
      x.authorityId - y.authorityId || x.slug.localeCompare(y.slug),
  );

  await writeFile(TOWNS_OUT, JSON.stringify(towns, null, 2) + '\n', 'utf-8');
  console.log(
    `[generate-towns] wrote ${towns.length} town(s) to ${TOWNS_OUT} ` +
      `from ${files.length} OS Open Names file(s)`,
  );
}

// Run main() only when invoked directly, not when imported by tests.
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main(process.argv[2]);
}
