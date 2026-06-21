import { describe, it, expect } from 'vitest';
import { resolveAuthority, townPagePath } from '../town-path.mjs';
import { slugify } from '../slug.mjs';

const AUTHORITIES = [
  { id: 52, name: 'Cornwall', areaType: 'English Unitary Authority' },
  { id: 301, name: 'Croydon', areaType: 'London Borough' },
  { id: 298, name: 'Brent', areaType: 'London Borough' },
];

describe('resolveAuthority', () => {
  it('maps an authorityId to its name and lowercase-hyphenated slug', () => {
    expect(resolveAuthority(52, AUTHORITIES)).toEqual({
      name: 'Cornwall',
      slug: 'cornwall',
    });
    expect(resolveAuthority(298, AUTHORITIES)).toEqual({
      name: 'Brent',
      slug: 'brent',
    });
  });

  it('throws loudly when the authorityId is not in the list (drift guard)', () => {
    expect(() => resolveAuthority(99999, AUTHORITIES)).toThrow();
  });
});

describe('townPagePath', () => {
  it('builds a nested <authority-slug>/<town-slug> path', () => {
    const { slug } = resolveAuthority(52, AUTHORITIES);
    expect(townPagePath(slug, 'truro')).toBe('cornwall/truro');
  });

  it('never collides with a bare authority page path (two segments, not one)', () => {
    // The authority page lives at /planning/<authority-slug>; a town page lives
    // one level deeper at /planning/<authority-slug>/<town-slug>. The nested path
    // must always carry a slash and split into exactly two segments, so it can
    // never equal the single-segment authority slug.
    const authoritySlug = slugify('Croydon'); // 'croydon'
    const townSlug = slugify('Croydon'); // a town also called Croydon
    const path = townPagePath(authoritySlug, townSlug);

    expect(path).toBe('croydon/croydon');
    expect(path).not.toBe(authoritySlug);
    expect(path.split('/')).toHaveLength(2);
  });
});
