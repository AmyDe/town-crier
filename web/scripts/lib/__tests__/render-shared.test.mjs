import { describe, it, expect } from 'vitest';
import {
  renderApplicationsList,
  renderAttributionList,
  pageStyles,
} from '../render-shared.mjs';
import { ATTRIBUTION_LINES } from '../constants.mjs';

const PLANIT_CAPTION = 'View full record on PlanIt';
const COUNCIL_CAPTION = 'View on the council website';

const SLUG = 'basingstoke-and-deane';
// The share ref is the app's `name` (planit_name), NOT its uid; slashes are kept.
const SHARE_HREF = `https://share.towncrierapp.uk/a/${SLUG}/26/0001/FUL`;
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

  it('keeps the link safety attributes on the external anchors', () => {
    const html = renderApplicationsList([application()]);
    const anchors = html.match(/<a class="appLink"[^>]*>/g) ?? [];
    expect(anchors).toHaveLength(2);
    for (const anchor of anchors) {
      expect(anchor).toContain('rel="nofollow noopener"');
      expect(anchor).toContain('target="_blank"');
    }
  });
});

describe('renderApplicationsList share links', () => {
  it('links the reference heading to the share page built from the slug and ref', () => {
    const html = renderApplicationsList([application()], SLUG);
    // The <h3> reference heading itself is the share link; the ref (app.name /
    // planit_name) is used verbatim in the URL, slashes preserved.
    expect(html).toContain(
      `<h3 class="appCard__ref"><a class="appCard__refLink" href="${SHARE_HREF}">26/0001/FUL</a></h3>`,
    );
  });

  it('makes the heading link do-follow and same-tab (an internal Town Crier page)', () => {
    const html = renderApplicationsList([application()], SLUG);
    const anchor = html.match(
      new RegExp(`<a class="appCard__refLink" href="${SHARE_HREF.replace(/[.*+?^${}()|[\]\\/]/g, '\\$&')}"[^>]*>`),
    );
    expect(anchor).not.toBeNull();
    expect(anchor[0]).not.toContain('nofollow');
    expect(anchor[0]).not.toContain('target="_blank"');
  });

  it('leaves the meta row with only the two external links (no third action link)', () => {
    const html = renderApplicationsList([application()], SLUG);
    const anchors = html.match(/<a class="appLink"[^>]*>/g) ?? [];
    expect(anchors).toHaveLength(2);
    expect(html).not.toContain('View on Town Crier');
  });

  it('percent-encodes unsafe ref characters per segment while preserving slashes', () => {
    const html = renderApplicationsList(
      [application({ name: 'DC/2026/0001 A&B' })],
      SLUG,
    );
    expect(html).toContain(
      `href="https://share.towncrierapp.uk/a/${SLUG}/DC/2026/0001%20A%26B"`,
    );
  });

  it('renders the reference as plain heading text when no slug is supplied', () => {
    const html = renderApplicationsList([application()]);
    expect(html).toContain('<h3 class="appCard__ref">26/0001/FUL</h3>');
    expect(html).not.toContain('share.towncrierapp.uk');
    expect(html).not.toContain('appCard__refLink');
  });

  it('renders a plain heading when the app carries no ref', () => {
    const html = renderApplicationsList([application({ name: '' })], SLUG);
    expect(html).not.toContain('share.towncrierapp.uk');
    expect(html).not.toContain('appCard__refLink');
  });
});

describe('renderAttributionList', () => {
  it('defaults to the base ATTRIBUTION_LINES (one escaped <li> per line)', () => {
    const html = renderAttributionList();
    for (const line of ATTRIBUTION_LINES) {
      expect(html).toContain(`<li>${line}</li>`);
    }
    expect((html.match(/<li>/g) ?? []).length).toBe(ATTRIBUTION_LINES.length);
  });

  it('renders a caller-supplied list of lines, so a page can extend the base set', () => {
    const html = renderAttributionList([
      'Line one',
      'Line two & <b>bold</b>',
    ]);
    expect(html).toContain('<li>Line one</li>');
    // HTML in a line is escaped so data can never inject markup.
    expect(html).toContain('<li>Line two &amp; &lt;b&gt;bold&lt;/b&gt;</li>');
    expect((html.match(/<li>/g) ?? []).length).toBe(2);
  });
});

describe('pageStyles appCard__refLink', () => {
  it('styles the heading link to inherit the heading colour and reveal amber on hover', () => {
    const css = pageStyles();
    expect(css).toContain('.appCard__refLink');
    // Neutral by default (inherits the heading colour), amber on hover.
    expect(css).toMatch(/\.appCard__refLink \{[^}]*color: inherit/);
    expect(css).toMatch(/\.appCard__refLink:hover \{[^}]*var\(--tc-amber\)/);
  });
});

describe('pageStyles townLinks', () => {
  it('styles the .townLinks__list and its anchors with design tokens', () => {
    const css = pageStyles();
    expect(css).toContain('.townLinks__list');
    expect(css).toContain('.townLinks__list a');
    // Uses design tokens (var(--tc-*)), never hard-coded colours/spacing.
    expect(css).toMatch(/\.townLinks__list a \{[^}]*var\(--tc-/);
  });
});
