/**
 * generate-towns.mjs — one-time / occasional generator for the committed town
 * gazetteer `web/src/data/towns.json`
 * (`[{ slug, name, lat, lng, authorityId, population }]`).
 *
 * THIS SCRIPT IS NOT PART OF THE BUILD AND NEVER RUNS IN CI. It is run by a
 * human, by hand, when the gazetteer needs (re)generating. The build
 * (`prerender-planning.mjs`) only ever READS the committed JSON — it never
 * downloads ONS data. Keep it that way: no network calls belong in here.
 *
 * ── What this emits ─────────────────────────────────────────────────────────
 * Every England-&-Wales (excluding London) Built-Up Area (BUA) with a
 * population >= 5,000, as `{ slug, name, lat, lng, authorityId, population }`.
 * The gazetteer carries ALL BUAs >= 5k; the build-time threshold filter
 * (SEO_TOWN_MIN_POPULATION, default 20000, in prerender-planning.mjs) does the
 * >= 20k selection later. Lowering the published threshold never needs a regen.
 *
 * Scope for THIS generator: England-ex-London + Wales (ONS Census 2021 BUA
 * file). London BUAs (tc-2avw.7) and Scotland settlements (tc-2avw.8) are
 * separate sources composed on top — see the EXTENSION SEAMS note at the foot
 * of this file. They are deliberately NOT implemented here.
 *
 * ── Data sources (TWO files, joined on BUA code) ────────────────────────────
 *
 * 1. POPULATION + NAME — ONS Census 2021 Built-Up Areas
 *    (~1,396 BUAs, England-ex-London + Wales, all >= 5k). Names are globally
 *    unique because ONS disambiguates collisions with a parenthetical LAD,
 *    e.g. `Cheadle (Stockport)` vs `Cheadle (Staffordshire Moorlands)`.
 *
 *    EXPECTED CSV CONTRACT (header row required; column ORDER is what matters,
 *    the header names are documentation — see POPULATION_COLUMNS below):
 *
 *        bua_code,bua_name,lad_name,population
 *        E63001234,Truro,Cornwall,18766
 *        E63005678,Cheadle (Stockport),Stockport,14000
 *
 *      - bua_code   : ONS BUA 2022 code (e.g. E63......). The JOIN KEY.
 *      - bua_name   : disambiguated BUA name, kept VERBATIM as `name`.
 *      - lad_name   : the BUA's Local Authority District name, matched against
 *                     `authority-mapping.json` to get our authorityId. This is
 *                     the OFFICIAL BUA->LAD column; prefer it. If the ONS file
 *                     you download lacks a LAD column, the orchestrator must
 *                     pre-join the BUA centroid into LAD boundaries (a manual
 *                     spatial join) and produce this column before running the
 *                     generator. We never guess the authority.
 *      - population : Census 2021 usual-resident count, an integer.
 *
 * 2. CENTROID (lat/lng) — ONS Open Geography "Built Up Areas (2022) GB BGG"
 *    (a centroid per BUA in BOTH BNG and lat/lng, custom convex-hull method).
 *    We use the provided lat/lng directly; BNG columns are a fallback for any
 *    grid-only export (handled by the retained `bngToLatLon`).
 *
 *    EXPECTED CSV CONTRACT (header row required; see CENTROID_COLUMNS below):
 *
 *        bua_code,bua_name,latitude,longitude,bng_easting,bng_northing
 *        E63001234,Truro,50.2632,-5.0510,182500,44900
 *
 *      - bua_code     : ONS BUA 2022 code. The JOIN KEY (same domain as file 1).
 *      - bua_name     : informational only; the population file is authoritative
 *                       for the display name.
 *      - latitude     : WGS84 decimal degrees. Used directly when present.
 *      - longitude    : WGS84 decimal degrees. Used directly when present.
 *      - bng_easting  : OSGB36 BNG easting (metres). FALLBACK only — used via
 *      - bng_northing : OSGB36 BNG northing (metres). `bngToLatLon` when lat/lng
 *                       are blank.
 *
 *    Dataset: https://geoportal.statistics.gov.uk/datasets/ons::built-up-areas-2022-gb-bgg/about
 *    FAQ:     https://onsgeo.github.io/Built_Up_Areas/
 *
 * ── Why join on BUA code, not name ──────────────────────────────────────────
 * The BUA code is the stable ONS identifier and is shared by both files. Names
 * carry the parenthetical-LAD disambiguation and could drift between the Census
 * spreadsheet and the geography export, so code is the safe key. If a future
 * source genuinely lacks codes, fall back to an exact name match and justify it
 * at the call site — do NOT silently fuzzy-match.
 *
 * ── How to regenerate (manual) ──────────────────────────────────────────────
 *   1. Download the two ONS files above and produce two CSVs that match the
 *      column contracts (you may need to rename/reorder columns; a spatial
 *      pre-join supplies lad_name if the Census export lacks it).
 *   2. Run:
 *        node scripts/generate-towns.mjs <population.csv> <centroid.csv>
 *      It parses both, joins on bua_code, drops BUAs < 5,000, resolves
 *      lad_name -> authorityId against authority-mapping.json, skips+logs any
 *      BUA with no centroid or no resolvable authority, de-duplicates by
 *      authorityId/slug, sorts, and overwrites web/src/data/towns.json.
 *   3. (When tc-2avw.7 / tc-2avw.8 land) append the London and Scotland record
 *      sets before writing — see EXTENSION SEAMS.
 *   4. Review the diff, run `npx vitest run`, and commit.
 *
 * ── Coordinate conversion (fallback) ────────────────────────────────────────
 * `bngToLatLon` does the standard inverse Transverse Mercator (Airy 1830) plus
 * a Helmert datum shift to WGS84. Retained as the fallback for any grid-only
 * centroid source; the BUA-2022 GB BGG file already carries lat/lng so the fast
 * path uses those directly.
 */

import { readFile, writeFile } from 'node:fs/promises';
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

/** Minimum BUA population the gazetteer carries. The build filter selects above this. */
export const POPULATION_FLOOR = 5000;

/**
 * Zero-based column indices in the POPULATION CSV (ONS Census 2021 BUA file).
 * The expected header is `bua_code,bua_name,lad_name,population`. Re-verify the
 * order against the file you download — see the script header contract.
 * @type {Record<string, number>}
 */
export const POPULATION_COLUMNS = {
  BUA_CODE: 0,
  BUA_NAME: 1,
  LAD_NAME: 2,
  POPULATION: 3,
};

/**
 * Zero-based column indices in the CENTROID CSV (ONS Open Geography BUA 2022 GB
 * BGG file). Expected header
 * `bua_code,bua_name,latitude,longitude,bng_easting,bng_northing`.
 * @type {Record<string, number>}
 */
export const CENTROID_COLUMNS = {
  BUA_CODE: 0,
  BUA_NAME: 1,
  LATITUDE: 2,
  LONGITUDE: 3,
  BNG_EASTING: 4,
  BNG_NORTHING: 5,
};

/**
 * Resolve an ONS Local Authority District name to one of our authority ids by
 * exact match against the committed district-name → authority-id map. Returns
 * null when the name is empty or unmatched — the caller then skips the BUA
 * (never guesses).
 *
 * @param {string} ladName ONS `lad_name`
 * @param {Record<string, number>} mapping district-name -> authority-id
 * @returns {number | null}
 */
export function resolveAuthorityId(ladName, mapping) {
  const name = (ladName || '').trim();
  if (name && Object.prototype.hasOwnProperty.call(mapping, name)) {
    return mapping[name];
  }
  return null;
}

/**
 * Parse one POPULATION CSV row into a normalised record, or null when the row
 * is unusable (no code, or a non-finite population).
 *
 * @param {string[]} fields CSV row split on commas
 * @returns {{ code: string, name: string, ladName: string, population: number } | null}
 */
export function parsePopulationRow(fields) {
  const code = (fields[POPULATION_COLUMNS.BUA_CODE] || '').trim();
  if (!code) {
    return null;
  }
  const name = (fields[POPULATION_COLUMNS.BUA_NAME] || '').trim();
  if (!name) {
    return null;
  }
  const population = toNumberOrNull(fields[POPULATION_COLUMNS.POPULATION]);
  if (population === null) {
    return null;
  }
  const ladName = (fields[POPULATION_COLUMNS.LAD_NAME] || '').trim();
  return { code, name, ladName, population };
}

/**
 * Parse one CENTROID CSV row into `{ code, lat, lng }`, using the provided
 * lat/lng directly and falling back to BNG conversion when they are blank.
 * Returns null when the code is missing or no usable coordinate exists.
 *
 * @param {string[]} fields CSV row split on commas
 * @returns {{ code: string, lat: number, lng: number } | null}
 */
export function parseCentroidRow(fields) {
  const code = (fields[CENTROID_COLUMNS.BUA_CODE] || '').trim();
  if (!code) {
    return null;
  }
  const lat = toNumberOrNull(fields[CENTROID_COLUMNS.LATITUDE]);
  const lng = toNumberOrNull(fields[CENTROID_COLUMNS.LONGITUDE]);
  if (lat !== null && lng !== null) {
    return { code, lat: round4(lat), lng: round4(lng) };
  }
  // Fallback: convert the British National Grid centroid to WGS84.
  const easting = toNumberOrNull(fields[CENTROID_COLUMNS.BNG_EASTING]);
  const northing = toNumberOrNull(fields[CENTROID_COLUMNS.BNG_NORTHING]);
  if (easting !== null && northing !== null) {
    const converted = bngToLatLon(easting, northing);
    return { code, lat: converted.lat, lng: converted.lng };
  }
  return null;
}

/**
 * Parse a CSV field to a finite number, treating a blank/whitespace field as
 * absent (null). `Number('')` is 0, which would silently produce a valid-looking
 * coordinate — this guards against that.
 *
 * @param {string | undefined} raw
 * @returns {number | null}
 */
function toNumberOrNull(raw) {
  const trimmed = (raw || '').trim();
  if (trimmed === '') {
    return null;
  }
  const value = Number(trimmed);
  return Number.isFinite(value) ? value : null;
}

/**
 * Join the parsed population and centroid sets on BUA code, resolving each
 * LAD name to an authority id. Emits sorted, de-duplicated gazetteer records
 * and a parallel list of skipped BUAs with reasons.
 *
 * Skip reasons: 'no-centroid' (population row had no matching centroid),
 * 'unmatched-authority' (LAD name not in our mapping).
 *
 * @param {Array<{ code: string, name: string, ladName: string, population: number }>} populations
 * @param {Array<{ code: string, lat: number, lng: number }>} centroids
 * @param {Record<string, number>} mapping district-name -> authority-id
 * @returns {{
 *   records: Array<{ slug: string, name: string, lat: number, lng: number, authorityId: number, population: number }>,
 *   skipped: Array<{ code: string, name: string, reason: string }>
 * }}
 */
export function joinBua(populations, centroids, mapping) {
  /** @type {Map<string, { lat: number, lng: number }>} */
  const centroidByCode = new Map();
  for (const c of centroids) {
    centroidByCode.set(c.code, { lat: c.lat, lng: c.lng });
  }

  /** @type {Map<string, { slug: string, name: string, lat: number, lng: number, authorityId: number, population: number }>} */
  const byKey = new Map();
  /** @type {Array<{ code: string, name: string, reason: string }>} */
  const skipped = [];

  for (const pop of populations) {
    const centroid = centroidByCode.get(pop.code);
    if (!centroid) {
      skipped.push({ code: pop.code, name: pop.name, reason: 'no-centroid' });
      continue;
    }
    const authorityId = resolveAuthorityId(pop.ladName, mapping);
    if (authorityId === null) {
      skipped.push({ code: pop.code, name: pop.name, reason: 'unmatched-authority' });
      continue;
    }
    const slug = slugify(pop.name);
    byKey.set(`${authorityId}/${slug}`, {
      slug,
      name: pop.name,
      lat: centroid.lat,
      lng: centroid.lng,
      authorityId,
      population: pop.population,
    });
  }

  const records = [...byKey.values()].sort(
    (x, y) => x.authorityId - y.authorityId || x.slug.localeCompare(y.slug),
  );

  return { records, skipped };
}

/**
 * End-to-end: parse both CSV texts, drop BUAs below the population floor, join,
 * and return records + skip log. The floor is applied here (not in `joinBua`)
 * so callers can inspect below-floor skips distinctly.
 *
 * @param {string} populationCsv raw POPULATION CSV text (with header row)
 * @param {string} centroidCsv   raw CENTROID CSV text (with header row)
 * @param {Record<string, number>} mapping district-name -> authority-id
 * @returns {{
 *   records: Array<{ slug: string, name: string, lat: number, lng: number, authorityId: number, population: number }>,
 *   skipped: Array<{ code: string, name: string, reason: string }>
 * }}
 */
export function buildGazetteer(populationCsv, centroidCsv, mapping) {
  /** @type {Array<{ code: string, name: string, ladName: string, population: number }>} */
  const populations = [];
  /** @type {Array<{ code: string, name: string, reason: string }>} */
  const belowFloor = [];
  for (const fields of parseCsvRows(populationCsv)) {
    const parsed = parsePopulationRow(fields);
    if (!parsed) {
      continue;
    }
    if (parsed.population < POPULATION_FLOOR) {
      belowFloor.push({ code: parsed.code, name: parsed.name, reason: 'below-floor' });
      continue;
    }
    populations.push(parsed);
  }

  /** @type {Array<{ code: string, lat: number, lng: number }>} */
  const centroids = [];
  for (const fields of parseCsvRows(centroidCsv)) {
    const parsed = parseCentroidRow(fields);
    if (parsed) {
      centroids.push(parsed);
    }
  }

  const { records, skipped } = joinBua(populations, centroids, mapping);
  return { records, skipped: [...belowFloor, ...skipped] };
}

/**
 * Split CSV text into field arrays, dropping the header row and blank lines.
 * ONS BUA names do not contain embedded commas, so a full RFC-4180 parser is
 * unnecessary; we strip a single layer of surrounding double quotes per field.
 *
 * @param {string} text raw CSV text (first non-blank line is the header)
 * @returns {string[][]}
 */
function parseCsvRows(text) {
  const lines = text.split(/\r?\n/).filter((line) => line.trim() !== '');
  // Drop the header row.
  return lines.slice(1).map((line) => line.split(',').map((f) => f.replace(/^"|"$/g, '')));
}

// ── OSGB36 British National Grid -> WGS84 ──────────────────────────────────
// Standard inverse Transverse Mercator on the Airy 1830 ellipsoid, then a
// Helmert transformation from the OSGB36 datum to WGS84. Constants are the
// published OS / EPSG values. Retained as a fallback for grid-only centroid
// sources — the BUA-2022 GB BGG file already carries lat/lng.

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
 * CLI entry: read the two ONS CSVs, build the gazetteer, write towns.json.
 *
 * @param {string} populationFile path to the POPULATION CSV
 * @param {string} centroidFile   path to the CENTROID CSV
 * @returns {Promise<void>}
 */
async function main(populationFile, centroidFile) {
  if (!populationFile || !centroidFile) {
    console.error(
      'usage: node scripts/generate-towns.mjs <population.csv> <centroid.csv>\n' +
        'See the script header for the full CSV contracts and regeneration steps.',
    );
    process.exitCode = 1;
    return;
  }

  const mapping = JSON.parse(await readFile(AUTHORITY_MAPPING_FILE, 'utf-8'));
  const populationCsv = await readFile(populationFile, 'utf-8');
  const centroidCsv = await readFile(centroidFile, 'utf-8');

  // EXTENSION SEAMS — compose other regions here before writing:
  //   * London BUAs (tc-2avw.7): BUA-2022 geometry + a London BUA population
  //     table. Produce records via buildGazetteer(londonPopCsv, centroidCsv,
  //     mapping) and concat. London LADs are already in authority-mapping.json.
  //   * Scotland settlements (tc-2avw.8): NRS settlement population + NRS
  //     settlement centroids. Different source schema — add a parseNrsRow /
  //     buildScotland helper and concat its records here.
  // Each region must keep emitting { slug, name, lat, lng, authorityId,
  // population } and resolve LAD->authorityId against the same mapping. Skipped
  // BUAs stay skipped+logged, never guessed.
  const { records, skipped } = buildGazetteer(populationCsv, centroidCsv, mapping);

  await writeFile(TOWNS_OUT, JSON.stringify(records, null, 2) + '\n', 'utf-8');

  const byReason = skipped.reduce((acc, s) => {
    acc[s.reason] = (acc[s.reason] || 0) + 1;
    return acc;
  }, /** @type {Record<string, number>} */ ({}));
  console.log(
    `[generate-towns] wrote ${records.length} town(s) to ${TOWNS_OUT}`,
  );
  console.log(
    `[generate-towns] skipped ${skipped.length} BUA(s): ` +
      Object.entries(byReason)
        .map(([reason, count]) => `${reason}=${count}`)
        .join(', '),
  );
  // Log each skip so the operator can audit unmatched authorities by hand.
  for (const s of skipped) {
    console.log(`[generate-towns] skip ${s.code} "${s.name}" (${s.reason})`);
  }
}

// Run main() only when invoked directly, not when imported by tests.
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main(process.argv[2], process.argv[3]);
}
