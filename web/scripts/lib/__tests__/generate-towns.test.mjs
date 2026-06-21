import { describe, it, expect } from 'vitest';
import {
  bngToLatLon,
  localTypeQualifies,
  resolveAuthorityId,
  townRecordFromRow,
  OS_OPEN_NAMES_COLUMNS,
} from '../../generate-towns.mjs';

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

describe('localTypeQualifies', () => {
  it('accepts only City and Town LOCAL_TYPE values', () => {
    expect(localTypeQualifies('City')).toBe(true);
    expect(localTypeQualifies('Town')).toBe(true);
    expect(localTypeQualifies('Village')).toBe(false);
    expect(localTypeQualifies('Hamlet')).toBe(false);
    expect(localTypeQualifies('Other Settlement')).toBe(false);
    expect(localTypeQualifies('Suburban Area')).toBe(false);
  });
});

describe('resolveAuthorityId', () => {
  const mapping = { Cornwall: 52, Croydon: 301, Brent: 298 };

  it('resolves via the district/borough name', () => {
    expect(resolveAuthorityId('Croydon', '', mapping)).toBe(301);
  });

  it('falls back to the county/unitary name when no district match', () => {
    expect(resolveAuthorityId('', 'Cornwall', mapping)).toBe(52);
  });

  it('returns null when neither name is in the authority mapping', () => {
    expect(resolveAuthorityId('Nowhere', 'Neverland', mapping)).toBeNull();
  });
});

describe('townRecordFromRow', () => {
  const mapping = { Croydon: 301 };

  /** Build a synthetic OS Open Names field array from a partial map. */
  function row(values) {
    const fields = [];
    for (const [name, index] of Object.entries(OS_OPEN_NAMES_COLUMNS)) {
      fields[index] = values[name] ?? '';
    }
    return fields;
  }

  it('builds a {slug,name,lat,lng,authorityId} record for a qualifying town', () => {
    const fields = row({
      NAME1: 'Croydon',
      LOCAL_TYPE: 'Town',
      GEOMETRY_X: '532504',
      GEOMETRY_Y: '165522',
      DISTRICT_BOROUGH: 'Croydon',
    });
    const record = townRecordFromRow(fields, mapping);
    expect(record).not.toBeNull();
    expect(record.slug).toBe('croydon');
    expect(record.name).toBe('Croydon');
    expect(record.authorityId).toBe(301);
    expect(Number.isFinite(record.lat)).toBe(true);
    expect(Number.isFinite(record.lng)).toBe(true);
    // coordinates are rounded to 4 decimal places
    expect(String(record.lat).split('.')[1]?.length ?? 0).toBeLessThanOrEqual(4);
  });

  it('skips a non-qualifying LOCAL_TYPE', () => {
    const fields = row({
      NAME1: 'Tiny',
      LOCAL_TYPE: 'Village',
      GEOMETRY_X: '530000',
      GEOMETRY_Y: '180000',
      DISTRICT_BOROUGH: 'Croydon',
    });
    expect(townRecordFromRow(fields, mapping)).toBeNull();
  });

  it('skips a town whose authority cannot be resolved', () => {
    const fields = row({
      NAME1: 'Orphan',
      LOCAL_TYPE: 'Town',
      GEOMETRY_X: '530000',
      GEOMETRY_Y: '180000',
      DISTRICT_BOROUGH: 'Unmapped District',
    });
    expect(townRecordFromRow(fields, mapping)).toBeNull();
  });
});
