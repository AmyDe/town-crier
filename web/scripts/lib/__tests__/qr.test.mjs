import { describe, it, expect } from 'vitest';
import { qrSvg } from '../qr.mjs';

const URL = 'https://apps.apple.com/gb/app/town-crier-planning-alerts/id6764095657?pt=128810278&ct=seo-lpa-qr&mt=8';

describe('qrSvg', () => {
  it('renders a self-contained inline SVG with an accessible name', () => {
    const svg = qrSvg(URL, 'Town Crier on the App Store');
    expect(svg.startsWith('<svg ')).toBe(true);
    expect(svg.endsWith('</svg>')).toBe(true);
    expect(svg).toContain('role="img"');
    expect(svg).toContain('aria-label="Town Crier on the App Store"');
    // Self-contained: no external references of any kind (the xmlns namespace
    // identifier is the only URL-shaped string allowed).
    expect(svg).not.toContain('href');
    expect(svg).not.toContain('url(');
    expect(svg.match(/https?:/g)).toEqual(['http:']);
  });

  it('draws a white tile with black modules regardless of theme, so cameras can always read it', () => {
    const svg = qrSvg(URL, 'label');
    expect(svg).toContain('fill="#FFFFFF"');
    expect(svg).toContain('fill="#000000"');
    expect(svg).not.toContain('var(--');
  });

  it('includes the QR quiet zone in the viewBox', () => {
    const svg = qrSvg(URL, 'label');
    const viewBox = svg.match(/viewBox="0 0 (\d+) \1"/);
    expect(viewBox).not.toBeNull();
    // Smallest QR version is 21 modules; plus a 4-module quiet zone per side.
    expect(Number(viewBox[1])).toBeGreaterThanOrEqual(21 + 8);
  });

  it('is deterministic for the same payload and differs across payloads', () => {
    expect(qrSvg(URL, 'label')).toBe(qrSvg(URL, 'label'));
    expect(qrSvg(URL, 'label')).not.toBe(qrSvg(`${URL}x`, 'label'));
  });

  it('escapes HTML in the aria label', () => {
    const svg = qrSvg(URL, 'a "quoted" <label> & more');
    expect(svg).toContain('aria-label="a &quot;quoted&quot; &lt;label&gt; &amp; more"');
    expect(svg).not.toContain('<label>');
  });
});
