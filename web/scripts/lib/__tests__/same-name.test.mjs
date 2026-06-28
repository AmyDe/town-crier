import { describe, it, expect } from 'vitest';
import { isSameNameAsAuthority } from '../same-name.mjs';

// tc-77ll: a town whose normalized slug equals its authority's slug duplicates
// the stronger authority page with an identical <title>, so it must be
// suppressed and 301-redirected. The helper decides "is this town the same place
// as its authority?" on the NORMALIZED slug, never the raw display name.
describe('isSameNameAsAuthority', () => {
  describe('strict slug equality (catches all current collisions)', () => {
    it('matches when the authority name slugifies to the town slug', () => {
      expect(isSameNameAsAuthority('Wrexham', 'wrexham')).toBe(true);
      expect(isSameNameAsAuthority('Birmingham', 'birmingham')).toBe(true);
      expect(isSameNameAsAuthority('York', 'york')).toBe(true);
    });

    it('matches hyphenated names on their slug, not the display string', () => {
      expect(isSameNameAsAuthority('Stockton-on-Tees', 'stockton-on-tees')).toBe(
        true,
      );
    });

    it('matches names with ampersands/apostrophes via the shared slugify', () => {
      // slugify expands & -> "and" and strips apostrophes, so a same-place town
      // page would carry the expanded slug.
      expect(
        isSameNameAsAuthority("King's Lynn and West Norfolk", 'kings-lynn-and-west-norfolk'),
      ).toBe(true);
    });
  });

  describe('forward-looking normalization (no-op on current plain data)', () => {
    // The committed authorities.json carries PLAIN names today ("Bristol",
    // "Herefordshire", "Wrexham"), so these synthetic raw-PlanIt forms prove the
    // normalization works if a future authorities.json ever carries them.
    it('strips a trailing ", City of" administrative suffix', () => {
      expect(isSameNameAsAuthority('Bristol, City of', 'bristol')).toBe(true);
    });

    it('strips a trailing ", County of" administrative suffix', () => {
      expect(
        isSameNameAsAuthority('Herefordshire, County of', 'herefordshire'),
      ).toBe(true);
    });

    it('strips a multi-word base name with a ", City of" suffix', () => {
      expect(
        isSameNameAsAuthority('Kingston upon Hull, City of', 'kingston-upon-hull'),
      ).toBe(true);
    });

    it('strips a bilingual slash variant down to its first segment', () => {
      expect(isSameNameAsAuthority('Wrexham / Wrecsam', 'wrexham')).toBe(true);
      expect(isSameNameAsAuthority('Anglesey / Ynys Môn', 'anglesey')).toBe(true);
    });

    it('strips a bilingual trailing parenthetical', () => {
      expect(isSameNameAsAuthority('Wrexham (Wrecsam)', 'wrexham')).toBe(true);
    });
  });

  describe('does NOT over-match distinct nearby places', () => {
    it('"Hull" does not suppress town "Kingston upon Hull"', () => {
      expect(isSameNameAsAuthority('Hull', 'kingston-upon-hull')).toBe(false);
    });

    it('"Herefordshire" does not suppress town "Hereford"', () => {
      expect(isSameNameAsAuthority('Herefordshire', 'hereford')).toBe(false);
    });

    it('an authority does not suppress a genuinely different town in it', () => {
      expect(isSameNameAsAuthority('Cornwall', 'truro')).toBe(false);
    });

    it('a disambiguating "(District)" parenthetical does not suppress a sibling town', () => {
      // "New Forest (District)" normalizes to "new-forest"; its towns (Lymington,
      // Totton, ...) are NOT slugged "new-forest", so none is wrongly suppressed.
      expect(isSameNameAsAuthority('New Forest (District)', 'lymington')).toBe(
        false,
      );
      expect(isSameNameAsAuthority('Northumberland (County)', 'morpeth')).toBe(
        false,
      );
    });
  });
});
