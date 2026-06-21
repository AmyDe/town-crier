import { describe, it, expect } from 'vitest';
import { readFile } from 'node:fs/promises';
import {
  loadTownsFromFile,
  TOWNS_FILE,
  loadAuthoritiesFromFile,
  AUTHORITIES_FILE,
} from '../../prerender-planning.mjs';

const SLUG_PATTERN = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

describe('loadTownsFromFile', () => {
  const FAKE_PATH = '/fake/towns.json';

  it('loads and validates a well-formed gazetteer', async () => {
    const readImpl = async () =>
      JSON.stringify([
        { slug: 'truro', name: 'Truro', lat: 50.2632, lng: -5.051, authorityId: 52 },
      ]);
    const towns = await loadTownsFromFile(FAKE_PATH, readImpl);
    expect(towns).toHaveLength(1);
    expect(towns[0]).toEqual({
      slug: 'truro',
      name: 'Truro',
      lat: 50.2632,
      lng: -5.051,
      authorityId: 52,
    });
  });

  it('throws when the file is missing', async () => {
    const readImpl = async () => {
      throw new Error('ENOENT: no such file or directory');
    };
    await expect(loadTownsFromFile(FAKE_PATH, readImpl)).rejects.toThrow();
  });

  it('throws on invalid JSON', async () => {
    const readImpl = async () => 'not json';
    await expect(loadTownsFromFile(FAKE_PATH, readImpl)).rejects.toThrow();
  });

  it('throws on a non-array payload', async () => {
    const readImpl = async () => JSON.stringify({ towns: [] });
    await expect(loadTownsFromFile(FAKE_PATH, readImpl)).rejects.toThrow();
  });

  it('throws on a malformed town row (non-numeric coordinate)', async () => {
    const readImpl = async () =>
      JSON.stringify([
        { slug: 'x', name: 'X', lat: 'nope', lng: 0, authorityId: 1 },
      ]);
    await expect(loadTownsFromFile(FAKE_PATH, readImpl)).rejects.toThrow();
  });

  it('throws on a malformed town row (missing authorityId)', async () => {
    const readImpl = async () =>
      JSON.stringify([{ slug: 'x', name: 'X', lat: 0, lng: 0 }]);
    await expect(loadTownsFromFile(FAKE_PATH, readImpl)).rejects.toThrow();
  });
});

describe('committed towns.json gazetteer', () => {
  it('parses, is an array, and every row has slug/name/numeric lat-lng/authorityId', async () => {
    const towns = await loadTownsFromFile(TOWNS_FILE, readFile);
    expect(Array.isArray(towns)).toBe(true);
    for (const t of towns) {
      expect(t.slug).toMatch(SLUG_PATTERN);
      expect(typeof t.name).toBe('string');
      expect(t.name.length).toBeGreaterThan(0);
      expect(Number.isFinite(t.lat)).toBe(true);
      expect(Number.isFinite(t.lng)).toBe(true);
      expect(Number.isFinite(t.authorityId)).toBe(true);
    }
  });

  it('has unique slugs within each authority (no nested-path collisions)', async () => {
    const towns = await loadTownsFromFile(TOWNS_FILE, readFile);
    const seen = new Set();
    for (const t of towns) {
      const key = `${t.authorityId}/${t.slug}`;
      expect(seen.has(key)).toBe(false);
      seen.add(key);
    }
  });

  it('every town authorityId exists in the committed authority list (drift guard)', async () => {
    const towns = await loadTownsFromFile(TOWNS_FILE, readFile);
    const authorities = await loadAuthoritiesFromFile(AUTHORITIES_FILE, readFile);
    const ids = new Set(authorities.map((a) => a.id));
    for (const t of towns) {
      expect(ids.has(t.authorityId)).toBe(true);
    }
  });
});
