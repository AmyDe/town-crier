import { describe, it, expect } from 'vitest';
import { renderApplicationsList } from '../render-shared.mjs';

const PLANIT_CAPTION = 'View full record on PlanIt';
const COUNCIL_CAPTION = 'View on the council website';

const PLANIT_HREF = 'https://planit.org.uk/planapplic/26-0001-FUL';
const COUNCIL_HREF = 'https://www.basingstoke.gov.uk/planning/26-0001-FUL';

/**
 * @param {Partial<import('../render-shared.mjs').PlanningApplicationItem>} [overrides]
 * @returns {import('../render-shared.mjs').PlanningApplicationItem}
 */
function application(overrides = {}) {
  return {
    uid: 'BDB/2026/0001',
    name: '26/0001/FUL',
    address: '12 Mill Road, Basingstoke, RG21 1AA',
    description: 'Erection of two-storey rear extension',
    appState: 'Permitted',
    startDate: '2026-01-15',
    lastDifferent: '2026-06-12T09:30:00+00:00',
    link: PLANIT_HREF,
    url: COUNCIL_HREF,
    ...overrides,
  };
}

/**
 * Extract the caption (anchor text) for the `appLink` whose href matches `href`.
 *
 * @param {string} html
 * @param {string} href
 * @returns {string | null}
 */
function captionForHref(html, href) {
  const escaped = href.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = html.match(
    new RegExp(`<a class="appLink" href="${escaped}"[^>]*>([^<]*)</a>`),
  );
  return match ? match[1] : null;
}

describe('renderApplicationsList per-application links', () => {
  it('captions the PlanIt permalink and the council website honestly when both are present', () => {
    const html = renderApplicationsList([application()]);

    // The PlanIt permalink (app.link) is captioned for PlanIt.
    expect(captionForHref(html, PLANIT_HREF)).toBe(PLANIT_CAPTION);
    // The council website (app.url) is captioned for the council site.
    expect(captionForHref(html, COUNCIL_HREF)).toBe(COUNCIL_CAPTION);

    // And the now-retired dishonest captions are gone.
    expect(html).not.toContain('View on the council portal');
    expect(html).not.toContain('Application details');
  });

  it('emits only the PlanIt caption when only link is present', () => {
    const html = renderApplicationsList([application({ url: null })]);

    expect(html).toContain(PLANIT_CAPTION);
    expect(captionForHref(html, PLANIT_HREF)).toBe(PLANIT_CAPTION);
    expect(html).not.toContain(COUNCIL_CAPTION);
  });

  it('emits only the council caption when only url is present', () => {
    const html = renderApplicationsList([application({ link: null })]);

    expect(html).toContain(COUNCIL_CAPTION);
    expect(captionForHref(html, COUNCIL_HREF)).toBe(COUNCIL_CAPTION);
    expect(html).not.toContain(PLANIT_CAPTION);
  });

  it('emits no appLink anchors when neither link nor url is present', () => {
    const html = renderApplicationsList([
      application({ link: null, url: null }),
    ]);

    expect(html).not.toContain('class="appLink"');
    expect(html).not.toContain(PLANIT_CAPTION);
    expect(html).not.toContain(COUNCIL_CAPTION);
  });

  it('keeps the link safety attributes on each anchor', () => {
    const html = renderApplicationsList([application()]);
    const anchors = html.match(/<a class="appLink"[^>]*>/g) ?? [];
    expect(anchors).toHaveLength(2);
    for (const anchor of anchors) {
      expect(anchor).toContain('rel="nofollow noopener"');
      expect(anchor).toContain('target="_blank"');
    }
  });
});
