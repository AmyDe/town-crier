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
 * file) PLUS London (tc-2avw.7), composed on top via a second population CSV,
 * PLUS Scotland (tc-2avw.8) from NRS settlements via parseNrsRow/buildScotland
 * — a different source schema with its OWN centroid CSV. See the "Scotland
 * (NRS)" note (next to NRS_POPULATION_COLUMNS) and the EXTENSION SEAMS note at
 * the foot of main().
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
 * ── London population (tc-2avw.7) ───────────────────────────────────────────
 * The Census 2021 BUA workbook (above) explicitly EXCLUDES London: in Greater
 * London the BUA method follows borough boundaries rather than recognising
 * individual settlements, so ONS removed the ~33 borough-shaped London BUAs
 * from Tables 1c/1d. London therefore needs a SEPARATE population CSV, in the
 * same `bua_code,bua_name,lad_name,population` contract, built from genuine ONS
 * sources:
 *   - usual residents per London BUA = SUM of Census 2021 TS001 OA usual
 *     residents over the OAs the ONS OA21->BUA22->LAD22 lookup assigns to that
 *     BUA (both sources cover London; sum on BUA22CD).
 *   - lad_name = the London borough holding the plurality of the BUA's OA
 *     population (>=98.9% for every one — the BUAs are borough-shaped), which
 *     resolves the multi-LAD "Part" rows the BUA->LAD lookup carries.
 * The centroid CSV is GB-wide (BUA_2022_GB already includes London), so the
 * SAME centroid file feeds both the E&W and the London buildGazetteer calls.
 *
 * ── How to regenerate (manual) ──────────────────────────────────────────────
 *   1. Download the two ONS files above and produce two CSVs that match the
 *      column contracts (you may need to rename/reorder columns; a spatial
 *      pre-join supplies lad_name if the Census export lacks it). Build the
 *      separate London population CSV per the "London population" note above.
 *   2. Run:
 *        node scripts/generate-towns.mjs <ew-population.csv> <centroid.csv> \
 *          [<london-population.csv>] [<scotland-population.csv> <scotland-centroids.csv>]
 *      It parses each population CSV, joins on the settlement code against the
 *      matching centroid CSV (the shared GB BUA file for E&W + London; the NRS
 *      centroid file for Scotland), drops settlements < 5,000, resolves the LAD/
 *      council name -> authorityId against authority-mapping.json, skips+logs
 *      any settlement with no centroid or no resolvable authority, then
 *      de-duplicates the COMBINED set by authorityId/slug, sorts, and overwrites
 *      web/src/data/towns.json.
 *   3. Scotland (tc-2avw.8): build the NRS population CSV from the "Population
 *      estimates for settlements and localities in Scotland, mid-2020" workbook
 *      (Table 2.1 settlement All-ages population, joined to Table 1.1 for the
 *      council area); the ~6 settlements that straddle >1 council are resolved
 *      to the single council containing the settlement centroid (point-in-
 *      polygon against the ScotGov ScottishLocalAuthorities boundaries). Build
 *      the NRS centroid CSV from the NRS:SettlementCentroids WFS layer. Both
 *      key on the NRS settlement code (S2000....). Pass them as the last two
 *      arguments. See the "Scotland (NRS)" note and EXTENSION SEAMS.
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

// ── Scotland (NRS) extension ────────────────────────────────────────────────
// Scotland is NOT a BUA region — it comes from National Records of Scotland
// "settlements", a different source schema, so it gets its own parse/build path
// (`parseNrsRow` + `buildScotland`) rather than reusing the BUA contracts above.
// Once parsed, an NRS settlement emits the SAME gazetteer record shape and runs
// through the SAME `joinBua` join/skip/dedupe/sort as the BUA regions.
//
// Sources (joined on the NRS settlement code, e.g. `S20001877`):
//   1. POPULATION + COUNCIL — NRS "Population estimates for settlements and
//      localities in Scotland, mid-2020", Table 2.1 (settlement populations,
//      the All-ages row) joined to Table 1.1 (settlement -> council area). The
//      6 settlements that straddle >1 council are resolved to the single council
//      containing the settlement centroid (point-in-polygon against the ScotGov
//      ScottishLocalAuthorities boundaries) before the CSV is produced — the
//      generator never guesses; an unresolved council is left blank and skipped.
//   2. CENTROID (lat/lng) — NRS Settlement Centroids (WFS `NRS:SettlementCentroids`),
//      carrying the settlement code, a WGS84 lat/lng, and a BNG easting/northing.

/**
 * Zero-based column indices in the NRS POPULATION CSV. Expected header
 * `settlement_code,settlement_name,council_area,population`. Distinct from the
 * BUA population contract: the join key is an NRS settlement code (`S2000....`)
 * and the authority column is a Scottish council-area name (matched verbatim
 * against authority-mapping.json — all 32 councils are present).
 * @type {Record<string, number>}
 */
export const NRS_POPULATION_COLUMNS = {
  SETTLEMENT_CODE: 0,
  SETTLEMENT_NAME: 1,
  COUNCIL_AREA: 2,
  POPULATION: 3,
};

/**
 * Zero-based column indices in the NRS CENTROID CSV. Expected header
 * `settlement_code,settlement_name,latitude,longitude,bng_easting,bng_northing`.
 * The NRS WFS emits a WGS84 lat/lng directly; the BNG easting/northing remain a
 * fallback (via `bngToLatLon`) for any grid-only export.
 * @type {Record<string, number>}
 */
export const NRS_CENTROID_COLUMNS = {
  SETTLEMENT_CODE: 0,
  SETTLEMENT_NAME: 1,
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
 * De-duplicate and stable-sort a COMBINED set of gazetteer records drawn from
 * more than one region (E&W + London, later + Scotland). Each region's
 * `buildGazetteer` already dedupes+sorts within itself, but concatenating two
 * sets can reintroduce an `authorityId/slug` collision across regions, so the
 * combine step re-applies the same last-wins dedupe and the same
 * authorityId-then-slug stable sort `joinBua` uses. Identical key/sort keeps
 * the committed diff stable regardless of the order regions are passed in.
 *
 * @param {Array<{ slug: string, name: string, lat: number, lng: number, authorityId: number, population: number }>} records
 * @returns {Array<{ slug: string, name: string, lat: number, lng: number, authorityId: number, population: number }>}
 */
export function combineRecords(records) {
  const byKey = new Map();
  for (const r of records) {
    byKey.set(`${r.authorityId}/${r.slug}`, r);
  }
  return [...byKey.values()].sort(
    (x, y) => x.authorityId - y.authorityId || x.slug.localeCompare(y.slug),
  );
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
 * Parse one NRS POPULATION CSV row into the same normalised shape `joinBua`
 * consumes (`{ code, name, ladName, population }`), or null when the row is
 * unusable (no settlement code, no name, or a non-finite population). The NRS
 * `council_area` becomes `ladName` so it resolves against the SAME
 * authority-mapping.json the BUA path uses; a blank council survives as an
 * empty `ladName` and is skipped downstream as 'unmatched-authority' (the
 * orchestrator leaves it blank only for the genuinely unresolved settlements).
 *
 * @param {string[]} fields CSV row split on commas
 * @returns {{ code: string, name: string, ladName: string, population: number } | null}
 */
export function parseNrsRow(fields) {
  const code = (fields[NRS_POPULATION_COLUMNS.SETTLEMENT_CODE] || '').trim();
  if (!code) {
    return null;
  }
  const name = (fields[NRS_POPULATION_COLUMNS.SETTLEMENT_NAME] || '').trim();
  if (!name) {
    return null;
  }
  const population = toNumberOrNull(fields[NRS_POPULATION_COLUMNS.POPULATION]);
  if (population === null) {
    return null;
  }
  const ladName = (fields[NRS_POPULATION_COLUMNS.COUNCIL_AREA] || '').trim();
  return { code, name, ladName, population };
}

/**
 * Parse one NRS CENTROID CSV row into `{ code, lat, lng }`, preferring the
 * provided WGS84 lat/lng and falling back to BNG conversion when they are blank.
 * Returns null when the settlement code is missing or no usable coordinate
 * exists. Mirrors `parseCentroidRow` but reads the NRS column layout.
 *
 * @param {string[]} fields CSV row split on commas
 * @returns {{ code: string, lat: number, lng: number } | null}
 */
export function parseNrsCentroidRow(fields) {
  const code = (fields[NRS_CENTROID_COLUMNS.SETTLEMENT_CODE] || '').trim();
  if (!code) {
    return null;
  }
  const lat = toNumberOrNull(fields[NRS_CENTROID_COLUMNS.LATITUDE]);
  const lng = toNumberOrNull(fields[NRS_CENTROID_COLUMNS.LONGITUDE]);
  if (lat !== null && lng !== null) {
    return { code, lat: round4(lat), lng: round4(lng) };
  }
  const easting = toNumberOrNull(fields[NRS_CENTROID_COLUMNS.BNG_EASTING]);
  const northing = toNumberOrNull(fields[NRS_CENTROID_COLUMNS.BNG_NORTHING]);
  if (easting !== null && northing !== null) {
    const converted = bngToLatLon(easting, northing);
    return { code, lat: converted.lat, lng: converted.lng };
  }
  return null;
}

/**
 * End-to-end Scotland build: parse the NRS population + centroid CSV texts, drop
 * settlements below the population floor, join on settlement code, and resolve
 * each council-area name to an authority id — the SAME `joinBua` tail the BUA
 * regions use, so the emitted record shape, skip reasons, dedupe, and sort are
 * identical. Below-floor settlements are reported distinctly (as in
 * `buildGazetteer`).
 *
 * @param {string} populationCsv raw NRS POPULATION CSV text (with header row)
 * @param {string} centroidCsv   raw NRS CENTROID CSV text (with header row)
 * @param {Record<string, number>} mapping council-name -> authority-id
 * @returns {{
 *   records: Array<{ slug: string, name: string, lat: number, lng: number, authorityId: number, population: number }>,
 *   skipped: Array<{ code: string, name: string, reason: string }>
 * }}
 */
export function buildScotland(populationCsv, centroidCsv, mapping) {
  /** @type {Array<{ code: string, name: string, ladName: string, population: number }>} */
  const populations = [];
  /** @type {Array<{ code: string, name: string, reason: string }>} */
  const belowFloor = [];
  for (const fields of parseCsvRows(populationCsv)) {
    const parsed = parseNrsRow(fields);
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
    const parsed = parseNrsCentroidRow(fields);
    if (parsed) {
      centroids.push(parsed);
    }
  }

  const { records, skipped } = joinBua(populations, centroids, mapping);
  return { records, skipped: [...belowFloor, ...skipped] };
}

/**
 * Split CSV text into field arrays, dropping the header row and blank lines.
 * A handful of official ONS names carry embedded commas — both BUA names
 * (`Longfield, New Ash Green and Hartley`) and, more importantly, LAD names
 * that must match `authority-mapping.json` verbatim (`Bristol, City of`,
 * `Kingston upon Hull, City of`, `Herefordshire, County of`, `Bournemouth,
 * Christchurch and Poole`). Those fields are double-quoted in the source CSVs,
 * so we honour RFC-4180 quoting: a comma inside a quoted field is part of the
 * value, and a doubled `""` is a literal quote.
 *
 * @param {string} text raw CSV text (first non-blank line is the header)
 * @returns {string[][]}
 */
function parseCsvRows(text) {
  const lines = text.split(/\r?\n/).filter((line) => line.trim() !== '');
  // Drop the header row.
  return lines.slice(1).map((line) => parseCsvLine(line));
}

/**
 * Split a single CSV line into fields, respecting RFC-4180 double-quoting so a
 * comma inside a quoted field is not treated as a delimiter. A doubled quote
 * (`""`) inside a quoted field is an escaped literal quote.
 *
 * @param {string} line one CSV record (no trailing newline)
 * @returns {string[]} the parsed fields, with surrounding quotes removed
 */
export function parseCsvLine(line) {
  const fields = [];
  let field = '';
  let inQuotes = false;
  for (let i = 0; i < line.length; i++) {
    const ch = line[i];
    if (inQuotes) {
      if (ch === '"') {
        if (line[i + 1] === '"') {
          field += '"';
          i++;
        } else {
          inQuotes = false;
        }
      } else {
        field += ch;
      }
    } else if (ch === '"') {
      inQuotes = true;
    } else if (ch === ',') {
      fields.push(field);
      field = '';
    } else {
      field += ch;
    }
  }
  fields.push(field);
  return fields;
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
 * CLI entry: read the source CSVs, build the gazetteer (E&W + optional London +
 * optional Scotland), write towns.json.
 *
 * @param {string} populationFile      path to the E&W POPULATION CSV (BUA contract)
 * @param {string} centroidFile        path to the GB-wide BUA CENTROID CSV
 * @param {string} [londonPopulationFile] optional path to the London POPULATION
 *   CSV (same `bua_code,bua_name,lad_name,population` contract). When supplied,
 *   its records are composed on top of E&W against the SAME centroid CSV.
 * @param {string} [scotlandPopulationFile] optional path to the NRS Scotland
 *   POPULATION CSV (`settlement_code,settlement_name,council_area,population`).
 * @param {string} [scotlandCentroidFile] optional path to the NRS Scotland
 *   CENTROID CSV (`settlement_code,settlement_name,latitude,longitude,
 *   bng_easting,bng_northing`). Required when a Scotland population CSV is given
 *   — Scotland centroids come from NRS, not the GB BUA centroid file.
 * @returns {Promise<void>}
 */
async function main(
  populationFile,
  centroidFile,
  londonPopulationFile,
  scotlandPopulationFile,
  scotlandCentroidFile,
) {
  if (!populationFile || !centroidFile) {
    console.error(
      'usage: node scripts/generate-towns.mjs <ew-population.csv> <centroid.csv> ' +
        '[<london-population.csv>] [<scotland-population.csv> <scotland-centroids.csv>]\n' +
        'See the script header for the full CSV contracts and regeneration steps.',
    );
    process.exitCode = 1;
    return;
  }
  if (scotlandPopulationFile && !scotlandCentroidFile) {
    console.error(
      'A Scotland population CSV requires its NRS centroid CSV as the next argument.',
    );
    process.exitCode = 1;
    return;
  }

  const mapping = JSON.parse(await readFile(AUTHORITY_MAPPING_FILE, 'utf-8'));
  const centroidCsv = await readFile(centroidFile, 'utf-8');

  // England-excluding-London + Wales (ONS Census 2021 Tables 1c + 1d).
  const ewPopulationCsv = await readFile(populationFile, 'utf-8');
  const ew = buildGazetteer(ewPopulationCsv, centroidCsv, mapping);

  // EXTENSION SEAMS — compose other regions here before writing:
  //   * London BUAs (tc-2avw.7): the Census 1c/1d tables EXCLUDE London, so
  //     London arrives as its own population CSV (see the "London population"
  //     header note) joined against the same GB centroid CSV. London borough
  //     LADs are already in authority-mapping.json.
  //   * Scotland settlements (tc-2avw.8): NRS settlement population + NRS
  //     settlement centroids — a DIFFERENT source schema, so it goes through
  //     parseNrsRow/buildScotland with its OWN centroid CSV (not the GB BUA
  //     one). Council-area names resolve against the same mapping.
  // Each region must keep emitting { slug, name, lat, lng, authorityId,
  // population } and resolve LAD/council -> authorityId against the same
  // mapping. Skipped rows stay skipped+logged, never guessed.
  const regions = [ew];
  let london = null;
  let scotland = null;
  if (londonPopulationFile) {
    const londonPopulationCsv = await readFile(londonPopulationFile, 'utf-8');
    london = buildGazetteer(londonPopulationCsv, centroidCsv, mapping);
    regions.push(london);
  }
  if (scotlandPopulationFile) {
    const scotlandPopulationCsv = await readFile(scotlandPopulationFile, 'utf-8');
    const scotlandCentroidCsv = await readFile(scotlandCentroidFile, 'utf-8');
    scotland = buildScotland(scotlandPopulationCsv, scotlandCentroidCsv, mapping);
    regions.push(scotland);
  }

  // Combine across regions: concat, then re-apply the dedupe-by-authorityId/slug
  // + stable sort across the whole set (an authorityId/slug can only collide
  // within a region in practice, but combineRecords keeps the invariant honest).
  const records = combineRecords(regions.flatMap((r) => r.records));
  const skipped = regions.flatMap((r) => r.skipped);

  await writeFile(TOWNS_OUT, JSON.stringify(records, null, 2) + '\n', 'utf-8');

  const byReason = skipped.reduce((acc, s) => {
    acc[s.reason] = (acc[s.reason] || 0) + 1;
    return acc;
  }, /** @type {Record<string, number>} */ ({}));
  console.log(
    `[generate-towns] wrote ${records.length} town(s) to ${TOWNS_OUT}` +
      ` (E&W ${ew.records.length}` +
      (london ? `, London ${london.records.length}` : '') +
      (scotland ? `, Scotland ${scotland.records.length}` : '') +
      ')',
  );
  console.log(
    `[generate-towns] skipped ${skipped.length} settlement(s): ` +
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
  await main(
    process.argv[2],
    process.argv[3],
    process.argv[4],
    process.argv[5],
    process.argv[6],
  );
}
