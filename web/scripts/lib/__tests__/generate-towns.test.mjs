import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { describe, it, expect } from 'vitest';
import {
  bngToLatLon,
  resolveAuthorityId,
  parseCsvLine,
  parsePopulationRow,
  parseCentroidRow,
  joinBua,
  buildGazetteer,
  combineRecords,
  POPULATION_COLUMNS,
  CENTROID_COLUMNS,
} from '../../generate-towns.mjs';

const FIXTURES_DIR = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'fixtures');

describe('bngToLatLon (OSGB36 British National Grid -> WGS84)', () => {
  it('converts a known central-London point to within ~100m', () => {
    // Trafalgar Square ~ TQ 30034 80381 -> WGS84 ~ 51.5080, -0.1281.
    const { lat, lng } = bngToLatLon(530034, 180381);
    expect(lat).toBeCloseTo(51.508, 2);
    expect(lng).toBeCloseTo(-0.128, 2);
  });

  it('returns coordinates inside the UK bounding box for a northern point', () => {
    // Edinburgh ~ NT 25731 73721 -> easting 325731, northing 673721.
    const { lat, lng } = bngToLatLon(325731, 673721);
    expect(lat).toBeGreaterThan(55.8);
    expect(lat).toBeLessThan(56.1);
    expect(lng).toBeGreaterThan(-3.3);
    expect(lng).toBeLessThan(-3.0);
  });
});

describe('resolveAuthorityId (LAD name -> authority id)', () => {
  const mapping = { Cornwall: 52, Stockport: 320, 'Staffordshire Moorlands': 81 };

  it('resolves an exact LAD name to its authority id', () => {
    expect(resolveAuthorityId('Cornwall', mapping)).toBe(52);
  });

  it('trims surrounding whitespace before matching', () => {
    expect(resolveAuthorityId('  Stockport  ', mapping)).toBe(320);
  });

  it('returns null when the LAD name is not in the authority mapping', () => {
    expect(resolveAuthorityId('Neverland', mapping)).toBeNull();
  });

  it('returns null for an empty LAD name', () => {
    expect(resolveAuthorityId('', mapping)).toBeNull();
  });
});

describe('parsePopulationRow (ONS Census 2021 BUA population file)', () => {
  /** Build a synthetic population CSV row in the documented column order. */
  function row(values) {
    const fields = [];
    for (const [key, index] of Object.entries(POPULATION_COLUMNS)) {
      fields[index] = values[key] ?? '';
    }
    return fields;
  }

  it('parses a well-formed row into {code, name, ladName, population}', () => {
    const parsed = parsePopulationRow(
      row({ BUA_CODE: 'E63001234', BUA_NAME: 'Truro', LAD_NAME: 'Cornwall', POPULATION: '18766' }),
    );
    expect(parsed).toEqual({
      code: 'E63001234',
      name: 'Truro',
      ladName: 'Cornwall',
      population: 18766,
    });
  });

  it('preserves a parenthetical-LAD disambiguated name verbatim', () => {
    const parsed = parsePopulationRow(
      row({
        BUA_CODE: 'E63005678',
        BUA_NAME: 'Cheadle (Stockport)',
        LAD_NAME: 'Stockport',
        POPULATION: '14000',
      }),
    );
    expect(parsed.name).toBe('Cheadle (Stockport)');
  });

  it('returns null when the BUA code is missing', () => {
    expect(
      parsePopulationRow(row({ BUA_NAME: 'Nameless', LAD_NAME: 'Cornwall', POPULATION: '9000' })),
    ).toBeNull();
  });

  it('returns null when the population is not a finite number', () => {
    expect(
      parsePopulationRow(
        row({ BUA_CODE: 'E63009999', BUA_NAME: 'Bad', LAD_NAME: 'Cornwall', POPULATION: 'N/A' }),
      ),
    ).toBeNull();
  });
});

describe('parseCentroidRow (ONS Open Geography BUA 2022 GB BGG file)', () => {
  /** Build a synthetic centroid CSV row in the documented column order. */
  function row(values) {
    const fields = [];
    for (const [key, index] of Object.entries(CENTROID_COLUMNS)) {
      fields[index] = values[key] ?? '';
    }
    return fields;
  }

  it('parses a well-formed row, using the provided lat/lng directly', () => {
    const parsed = parseCentroidRow(
      row({
        BUA_CODE: 'E63001234',
        BUA_NAME: 'Truro',
        LATITUDE: '50.2632',
        LONGITUDE: '-5.0510',
        BNG_EASTING: '182500',
        BNG_NORTHING: '44900',
      }),
    );
    expect(parsed.code).toBe('E63001234');
    expect(parsed.lat).toBeCloseTo(50.2632, 4);
    expect(parsed.lng).toBeCloseTo(-5.051, 4);
  });

  it('falls back to BNG conversion when lat/lng are absent but easting/northing are present', () => {
    // Trafalgar Square BNG -> WGS84 ~ 51.508, -0.128.
    const parsed = parseCentroidRow(
      row({
        BUA_CODE: 'E63000001',
        BUA_NAME: 'Somewhere',
        LATITUDE: '',
        LONGITUDE: '',
        BNG_EASTING: '530034',
        BNG_NORTHING: '180381',
      }),
    );
    expect(parsed.lat).toBeCloseTo(51.508, 2);
    expect(parsed.lng).toBeCloseTo(-0.128, 2);
  });

  it('returns null when neither lat/lng nor BNG coordinates are usable', () => {
    expect(
      parseCentroidRow(
        row({
          BUA_CODE: 'E63000002',
          BUA_NAME: 'Nowhere',
          LATITUDE: '',
          LONGITUDE: '',
          BNG_EASTING: '',
          BNG_NORTHING: '',
        }),
      ),
    ).toBeNull();
  });

  it('returns null when the BUA code is missing', () => {
    expect(
      parseCentroidRow(
        row({ BUA_NAME: 'Truro', LATITUDE: '50.2632', LONGITUDE: '-5.051' }),
      ),
    ).toBeNull();
  });
});

describe('joinBua (population x centroid on BUA code)', () => {
  const mapping = { Cornwall: 52, Stockport: 320 };

  it('emits a {slug,name,lat,lng,authorityId,population} record when both sources match', () => {
    const populations = [
      { code: 'E63001234', name: 'Truro', ladName: 'Cornwall', population: 18766 },
    ];
    const centroids = [{ code: 'E63001234', lat: 50.2632, lng: -5.051 }];

    const { records, skipped } = joinBua(populations, centroids, mapping);

    expect(skipped).toEqual([]);
    expect(records).toEqual([
      { slug: 'truro', name: 'Truro', lat: 50.2632, lng: -5.051, authorityId: 52, population: 18766 },
    ]);
  });

  it('derives a unique slug from a parenthetical-LAD name', () => {
    const populations = [
      { code: 'E63005678', name: 'Cheadle (Stockport)', ladName: 'Stockport', population: 14000 },
    ];
    const centroids = [{ code: 'E63005678', lat: 53.39, lng: -2.21 }];

    const { records } = joinBua(populations, centroids, mapping);

    expect(records).toHaveLength(1);
    expect(records[0].slug).toBe('cheadle-stockport');
    expect(records[0].name).toBe('Cheadle (Stockport)');
  });

  it('emits population as a finite number on every record', () => {
    const populations = [
      { code: 'E63001234', name: 'Truro', ladName: 'Cornwall', population: 18766 },
    ];
    const centroids = [{ code: 'E63001234', lat: 50.2632, lng: -5.051 }];

    const { records } = joinBua(populations, centroids, mapping);

    expect(Number.isFinite(records[0].population)).toBe(true);
  });

  it('skips and records a BUA with no matching centroid', () => {
    const populations = [
      { code: 'E63001234', name: 'Truro', ladName: 'Cornwall', population: 18766 },
      { code: 'E63009999', name: 'Lonely', ladName: 'Cornwall', population: 9000 },
    ];
    const centroids = [{ code: 'E63001234', lat: 50.2632, lng: -5.051 }];

    const { records, skipped } = joinBua(populations, centroids, mapping);

    expect(records).toHaveLength(1);
    expect(skipped).toEqual([{ code: 'E63009999', name: 'Lonely', reason: 'no-centroid' }]);
  });

  it('skips and records a BUA whose LAD does not resolve to an authority', () => {
    const populations = [
      { code: 'E63007777', name: 'Orphan', ladName: 'Unmapped District', population: 30000 },
    ];
    const centroids = [{ code: 'E63007777', lat: 52.0, lng: -1.0 }];

    const { records, skipped } = joinBua(populations, centroids, mapping);

    expect(records).toEqual([]);
    expect(skipped).toEqual([{ code: 'E63007777', name: 'Orphan', reason: 'unmatched-authority' }]);
  });

  it('sorts records by authorityId then slug for a stable diff', () => {
    const populations = [
      { code: 'B', name: 'Stockport Town', ladName: 'Stockport', population: 20000 },
      { code: 'A', name: 'Cheadle (Stockport)', ladName: 'Stockport', population: 14000 },
      { code: 'C', name: 'Truro', ladName: 'Cornwall', population: 18766 },
    ];
    const centroids = [
      { code: 'A', lat: 53.39, lng: -2.21 },
      { code: 'B', lat: 53.41, lng: -2.16 },
      { code: 'C', lat: 50.2632, lng: -5.051 },
    ];

    const { records } = joinBua(populations, centroids, mapping);

    expect(records.map((r) => `${r.authorityId}/${r.slug}`)).toEqual([
      '52/truro',
      '320/cheadle-stockport',
      '320/stockport-town',
    ]);
  });

  it('de-duplicates on authorityId/slug, keeping a stable single record', () => {
    const populations = [
      { code: 'X1', name: 'Truro', ladName: 'Cornwall', population: 18766 },
      { code: 'X2', name: 'Truro', ladName: 'Cornwall', population: 18766 },
    ];
    const centroids = [
      { code: 'X1', lat: 50.2632, lng: -5.051 },
      { code: 'X2', lat: 50.2632, lng: -5.051 },
    ];

    const { records } = joinBua(populations, centroids, mapping);

    expect(records).toHaveLength(1);
  });
});

describe('parseCsvLine (RFC-4180 quoting)', () => {
  it('splits an unquoted line on every comma', () => {
    expect(parseCsvLine('E63001234,Truro,Cornwall,18766')).toEqual([
      'E63001234',
      'Truro',
      'Cornwall',
      '18766',
    ]);
  });

  it('keeps a comma inside a quoted field (official LAD name)', () => {
    // ONS names "Bristol, City of" / "Kingston upon Hull, City of" carry a comma
    // that must survive so lad_name matches authority-mapping.json verbatim.
    expect(parseCsvLine('E63005057,Bristol,"Bristol, City of",425215')).toEqual([
      'E63005057',
      'Bristol',
      'Bristol, City of',
      '425215',
    ]);
  });

  it('handles a quoted comma in both the BUA name and the LAD name', () => {
    expect(
      parseCsvLine(
        'E63006696,"Christchurch (Bournemouth, Christchurch and Poole)","Bournemouth, Christchurch and Poole",48985',
      ),
    ).toEqual([
      'E63006696',
      'Christchurch (Bournemouth, Christchurch and Poole)',
      'Bournemouth, Christchurch and Poole',
      '48985',
    ]);
  });

  it('unescapes a doubled quote inside a quoted field', () => {
    expect(parseCsvLine('A,"say ""hi""",B')).toEqual(['A', 'say "hi"', 'B']);
  });
});

describe('buildGazetteer (end-to-end CSV text -> records)', () => {
  const mapping = { Cornwall: 52, Stockport: 320 };

  const populationCsv = [
    'bua_code,bua_name,lad_name,population',
    'E63001234,Truro,Cornwall,18766',
    'E63005678,Cheadle (Stockport),Stockport,14000',
    'E63007777,Orphan,Unmapped District,30000',
    'E63004000,Tiny,Cornwall,4200',
    'E63009999,NoCentroid,Cornwall,9000',
  ].join('\n');

  const centroidCsv = [
    'bua_code,bua_name,latitude,longitude,bng_easting,bng_northing',
    'E63001234,Truro,50.2632,-5.0510,182500,44900',
    'E63005678,Cheadle (Stockport),53.3900,-2.2100,387000,388000',
    'E63007777,Orphan,52.0000,-1.0000,440000,250000',
    'E63004000,Tiny,50.1000,-5.1000,177000,33000',
  ].join('\n');

  it('emits every BUA >= 5,000 that joins and resolves, dropping the sub-5k floor', () => {
    const { records } = buildGazetteer(populationCsv, centroidCsv, mapping);
    const slugs = records.map((r) => r.slug);
    expect(slugs).toContain('truro');
    expect(slugs).toContain('cheadle-stockport');
    // Tiny is below the 5k floor.
    expect(slugs).not.toContain('tiny');
    // Orphan has no resolvable authority.
    expect(slugs).not.toContain('orphan');
    // NoCentroid has no centroid match.
    expect(slugs).not.toContain('nocentroid');
  });

  it('records the reason for every skipped BUA', () => {
    const { skipped } = buildGazetteer(populationCsv, centroidCsv, mapping);
    const byCode = Object.fromEntries(skipped.map((s) => [s.code, s.reason]));
    expect(byCode['E63004000']).toBe('below-floor');
    expect(byCode['E63007777']).toBe('unmatched-authority');
    expect(byCode['E63009999']).toBe('no-centroid');
  });

  it('every emitted record carries a finite numeric population', () => {
    const { records } = buildGazetteer(populationCsv, centroidCsv, mapping);
    for (const r of records) {
      expect(Number.isFinite(r.population)).toBe(true);
    }
  });

  it('resolves a BUA whose official LAD name contains a comma', () => {
    // Regression: "Bristol, City of" must round-trip through the quoted CSV and
    // match the authority mapping, otherwise Bristol is silently dropped.
    const popCsv = [
      'bua_code,bua_name,lad_name,population',
      'E63005057,Bristol,"Bristol, City of",425215',
    ].join('\n');
    const cenCsv = [
      'bua_code,bua_name,latitude,longitude,bng_easting,bng_northing',
      'E63005057,Bristol,51.4518,-2.5898,358000,173000',
    ].join('\n');
    const { records } = buildGazetteer(popCsv, cenCsv, { 'Bristol, City of': 23 });
    expect(records).toHaveLength(1);
    expect(records[0]).toMatchObject({
      slug: 'bristol',
      name: 'Bristol',
      authorityId: 23,
      population: 425215,
    });
  });
});

describe('buildGazetteer over the on-disk fixture CSVs (documented contract)', () => {
  // Proves the documented column contract round-trips from real files matching
  // the layout the orchestrator will produce for the real ONS regen.
  const mapping = { Cornwall: 52, Stockport: 320, 'Staffordshire Moorlands': 81 };
  const populationCsv = readFileSync(join(FIXTURES_DIR, 'sample-bua-population.csv'), 'utf-8');
  const centroidCsv = readFileSync(join(FIXTURES_DIR, 'sample-bua-centroids.csv'), 'utf-8');

  const { records, skipped } = buildGazetteer(populationCsv, centroidCsv, mapping);
  const bySlug = Object.fromEntries(records.map((r) => [`${r.authorityId}/${r.slug}`, r]));

  it('joins both fixture files on BUA code into {slug,name,lat,lng,authorityId,population}', () => {
    expect(bySlug['52/truro']).toEqual({
      slug: 'truro',
      name: 'Truro',
      lat: 50.2632,
      lng: -5.051,
      authorityId: 52,
      population: 18766,
    });
  });

  it('keeps both parenthetical-LAD Cheadles as distinct unique-slug records', () => {
    expect(bySlug['320/cheadle-stockport']).toBeDefined();
    expect(bySlug['81/cheadle-staffordshire-moorlands']).toBeDefined();
    expect(bySlug['320/cheadle-stockport'].name).toBe('Cheadle (Stockport)');
  });

  it('uses the BNG fallback for a fixture row with blank lat/lng (Newquay)', () => {
    const newquay = bySlug['52/newquay'];
    expect(newquay).toBeDefined();
    expect(newquay.lat).toBeCloseTo(50.41, 1);
    expect(newquay.lng).toBeCloseTo(-5.07, 1);
  });

  it('skips and logs the sub-5k, unmatched-authority, and no-centroid fixtures', () => {
    const byCode = Object.fromEntries(skipped.map((s) => [s.code, s.reason]));
    expect(byCode['E63004000']).toBe('below-floor');
    expect(byCode['E63007777']).toBe('unmatched-authority');
    expect(byCode['E63009999']).toBe('no-centroid');
  });
});

describe('London composition (tc-2avw.7)', () => {
  // London arrives as a SEPARATE population CSV (the Census 1c/1d tables exclude
  // London) joined against the SAME GB centroid CSV. These tests prove a
  // London-borough BUA row flows through buildGazetteer to a record with the
  // right authorityId + population, and an unmatched one skips — using the real
  // London-borough spellings/ids from authority-mapping.json.
  const mapping = { Camden: 300, 'City of London': 471, Westminster: 326 };

  // A GB centroid CSV carries E&W *and* London BUAs (BUA_2022_GB is GB-wide).
  const centroidCsv = [
    'bua_code,bua_name,latitude,longitude,bng_easting,bng_northing',
    'E63004858,Camden,51.5488,-0.1621,529000,183000',
    'E63004906,City and County of the City of London,51.5154,-0.0899,532500,181200',
    'E63004916,City of Westminster,51.5127,-0.1571,529300,180600',
    // Hackney has a centroid in the shared GB file, so a missing authority
    // mapping skips it as 'unmatched-authority' (not 'no-centroid').
    'E63004850,Hackney,51.5513,-0.0656,534000,185000',
    // An E&W centroid present in the same shared file, to prove the join keys on code.
    'E63001234,Truro,50.2632,-5.0510,182500,44900',
  ].join('\n');

  it('flows a London-borough BUA to a record with the right authorityId and population', () => {
    const londonPopCsv = [
      'bua_code,bua_name,lad_name,population',
      'E63004858,Camden,Camden,209342',
    ].join('\n');

    const { records, skipped } = buildGazetteer(londonPopCsv, centroidCsv, mapping);

    expect(skipped).toEqual([]);
    expect(records).toEqual([
      {
        slug: 'camden',
        name: 'Camden',
        lat: 51.5488,
        lng: -0.1621,
        authorityId: 300,
        population: 209342,
      },
    ]);
  });

  it('keeps the ONS parenthetical/administrative BUA name verbatim (City of London)', () => {
    const londonPopCsv = [
      'bua_code,bua_name,lad_name,population',
      'E63004906,City and County of the City of London,City of London,7820',
    ].join('\n');

    const { records } = buildGazetteer(londonPopCsv, centroidCsv, mapping);

    expect(records).toHaveLength(1);
    expect(records[0].name).toBe('City and County of the City of London');
    expect(records[0].slug).toBe('city-and-county-of-the-city-of-london');
    expect(records[0].authorityId).toBe(471);
  });

  it('skips and logs a London BUA whose borough is not in the mapping (never guessed)', () => {
    const londonPopCsv = [
      'bua_code,bua_name,lad_name,population',
      'E63004858,Camden,Camden,209342',
      // Hackney is a real London borough but absent from this test mapping.
      'E63004850,Hackney,Hackney,258464',
    ].join('\n');

    const { records, skipped } = buildGazetteer(londonPopCsv, centroidCsv, mapping);

    expect(records.map((r) => r.slug)).toEqual(['camden']);
    expect(skipped).toEqual([
      { code: 'E63004850', name: 'Hackney', reason: 'unmatched-authority' },
    ]);
  });
});

describe('combineRecords (cross-region dedupe + stable sort)', () => {
  // The composed E&W + London set is concatenated then deduped+sorted by
  // authorityId-then-slug. This mirrors the joinBua tail so the committed diff
  // stays stable regardless of region order.
  const ew = [
    { slug: 'truro', name: 'Truro', lat: 50.2632, lng: -5.051, authorityId: 52, population: 18766 },
  ];
  const london = [
    {
      slug: 'camden',
      name: 'Camden',
      lat: 51.5488,
      lng: -0.1621,
      authorityId: 300,
      population: 209342,
    },
    {
      slug: 'barnet',
      name: 'Barnet',
      lat: 51.6152,
      lng: -0.2091,
      authorityId: 296,
      population: 388572,
    },
  ];

  it('orders the combined set by authorityId then slug regardless of input order', () => {
    const combined = combineRecords([...london, ...ew]);
    expect(combined.map((r) => `${r.authorityId}/${r.slug}`)).toEqual([
      '52/truro',
      '296/barnet',
      '300/camden',
    ]);
  });

  it('keeps every region record and adds nothing when keys do not collide', () => {
    const combined = combineRecords([...ew, ...london]);
    expect(combined).toHaveLength(ew.length + london.length);
  });

  it('de-duplicates an authorityId/slug collision across regions, keeping the last', () => {
    const duplicateOfTruro = {
      slug: 'truro',
      name: 'Truro',
      lat: 99,
      lng: 99,
      authorityId: 52,
      population: 1,
    };
    const combined = combineRecords([...ew, duplicateOfTruro]);
    expect(combined).toHaveLength(1);
    // Last write wins, matching joinBua's Map-based dedupe.
    expect(combined[0]).toEqual(duplicateOfTruro);
  });
});
