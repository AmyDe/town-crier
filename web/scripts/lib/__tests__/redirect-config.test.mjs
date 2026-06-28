import { describe, it, expect } from 'vitest';
import { mergeRedirects } from '../redirect-config.mjs';

// tc-77ll: suppressed same-name town URLs are in the sitemap and likely indexed.
// Stopping emission alone leaves them to the SWA navigationFallback (200 + SPA
// shell = soft-404, worse than the dup). mergeRedirects folds a 301 for each
// suppressed URL into the hand-written base staticwebapp.config.json.
const baseConfig = Object.freeze({
  navigationFallback: {
    rewrite: '/index.html',
    exclude: ['*.{css,js,svg,png}'],
  },
  globalHeaders: { 'X-Frame-Options': 'DENY' },
  routes: [
    { route: '/assets/*', headers: { 'Cache-Control': 'public' } },
    {
      route: '/.well-known/apple-app-site-association',
      rewrite: '/.well-known/aasa.json',
    },
  ],
});

describe('mergeRedirects', () => {
  it('appends a 301 redirect route for each suppressed same-name town path', () => {
    const merged = mergeRedirects(baseConfig, ['wrexham/wrexham']);
    expect(merged.routes).toContainEqual({
      route: '/planning/wrexham/wrexham',
      redirect: '/planning/wrexham',
      statusCode: 301,
    });
  });

  it('keeps every hand-written base route intact (never drops them)', () => {
    const merged = mergeRedirects(baseConfig, ['birmingham/birmingham']);
    expect(merged.routes).toContainEqual({
      route: '/assets/*',
      headers: { 'Cache-Control': 'public' },
    });
    expect(merged.routes).toContainEqual({
      route: '/.well-known/apple-app-site-association',
      rewrite: '/.well-known/aasa.json',
    });
    // base routes come first, generated redirects appended after.
    expect(merged.routes).toHaveLength(baseConfig.routes.length + 1);
  });

  it('preserves the rest of the base config (navigationFallback, globalHeaders)', () => {
    const merged = mergeRedirects(baseConfig, ['york/york']);
    expect(merged.navigationFallback).toEqual(baseConfig.navigationFallback);
    expect(merged.globalHeaders).toEqual(baseConfig.globalHeaders);
  });

  it('derives the redirect target from the first (authority) path segment', () => {
    const merged = mergeRedirects(baseConfig, ['stockton-on-tees/stockton-on-tees']);
    const added = merged.routes.find((r) =>
      r.route === '/planning/stockton-on-tees/stockton-on-tees',
    );
    expect(added).toEqual({
      route: '/planning/stockton-on-tees/stockton-on-tees',
      redirect: '/planning/stockton-on-tees',
      statusCode: 301,
    });
  });

  it('returns the base routes unchanged when there are no redirects', () => {
    const merged = mergeRedirects(baseConfig, []);
    expect(merged.routes).toEqual(baseConfig.routes);
  });

  it('dedupes repeated paths into a single redirect route', () => {
    const merged = mergeRedirects(baseConfig, [
      'derby/derby',
      'derby/derby',
    ]);
    const derbyRoutes = merged.routes.filter(
      (r) => r.route === '/planning/derby/derby',
    );
    expect(derbyRoutes).toHaveLength(1);
  });

  it('does not mutate the base config', () => {
    const before = baseConfig.routes.length;
    mergeRedirects(baseConfig, ['leeds/leeds']);
    expect(baseConfig.routes).toHaveLength(before);
  });

  it('tolerates a base config with no routes array', () => {
    const merged = mergeRedirects(
      { navigationFallback: { rewrite: '/index.html' } },
      ['bath/bath'],
    );
    expect(merged.routes).toEqual([
      {
        route: '/planning/bath/bath',
        redirect: '/planning/bath',
        statusCode: 301,
      },
    ]);
  });
});
