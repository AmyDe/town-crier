import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  compareFields,
  decodeEntities,
  diffPathSets,
  extractSeoFields,
  parseArgs,
  sitemapPaths,
  DEFAULT_LIVE,
  DEFAULT_PREVIEW,
} from './seo-parity.mjs';

const STATIC_HTML = `<!doctype html><html><head>
<title>Planning applications in King&#39;s Lynn | Town Crier</title>
<meta name="description" content="Recent planning applications in King&#39;s Lynn &amp; West Norfolk." />
<meta name="robots" content="index,follow" />
<link rel="canonical" href="https://towncrierapp.uk/planning/kings-lynn" />
<meta property="og:title" content="Planning applications in King's Lynn" />
<meta property="og:url" content="https://towncrierapp.uk/planning/kings-lynn" />
<script type="application/ld+json">[{"@context":"https://schema.org","@type":"ItemList","itemListElement":[{"@type":"ListItem"}]},{"@type":"BreadcrumbList"}]</script>
</head><body><h1>Planning applications in
  <span>King&#x27;s Lynn</span></h1></body></html>`;

const GO_HTML = `<html><head><meta content="Recent planning applications in King's Lynn &#38; West Norfolk." name=description>
<title>Planning applications in King's Lynn | Town Crier</title>
<meta name="robots" content="index,follow">
<link href="https://towncrierapp.uk/planning/kings-lynn" rel="canonical">
<meta property="og:title" content="Planning applications in King&#39;s Lynn">
<meta property="og:url" content="https://towncrierapp.uk/planning/kings-lynn">
<script type="application/ld+json">{"@context":"https://schema.org","@graph":[{"@type":"BreadcrumbList"},{"@type":"ItemList"}]}</script>
</head><body><h1>Planning applications in King's Lynn</h1></body></html>`;

test('extracts SEO fields with entities decoded', () => {
  assert.deepEqual(extractSeoFields(STATIC_HTML), {
    title: "Planning applications in King's Lynn | Town Crier",
    description: "Recent planning applications in King's Lynn & West Norfolk.",
    canonical: 'https://towncrierapp.uk/planning/kings-lynn',
    robots: 'index,follow',
    ogTitle: "Planning applications in King's Lynn",
    ogUrl: 'https://towncrierapp.uk/planning/kings-lynn',
    h1: "Planning applications in King's Lynn",
    jsonLdTypes: 'BreadcrumbList, ItemList',
  });
});

test('markup differences that do not change values compare equal', () => {
  const live = { status: 200, ...extractSeoFields(STATIC_HTML) };
  const preview = { status: 200, ...extractSeoFields(GO_HTML) };
  assert.deepEqual(compareFields(live, preview), []);
});

test('reports each differing field and status', () => {
  const live = { status: 200, ...extractSeoFields(STATIC_HTML) };
  const preview = { status: 404, ...extractSeoFields(GO_HTML.replace('index,follow', 'noindex')) };
  assert.deepEqual(
    compareFields(live, preview).map((d) => d.field),
    ['status', 'robots'],
  );
});

test('missing fields are null', () => {
  const fields = extractSeoFields('<html><head></head><body></body></html>');
  assert.equal(fields.title, null);
  assert.equal(fields.canonical, null);
  assert.equal(fields.h1, null);
  assert.equal(fields.jsonLdTypes, '');
});

test('sitemap paths are compared by path regardless of origin', () => {
  const live = sitemapPaths(
    '<urlset><url><loc>https://towncrierapp.uk/planning</loc><lastmod>2026-10-01</lastmod></url>' +
      '<url><loc>https://towncrierapp.uk/planning/croydon</loc></url></urlset>',
  );
  const preview = sitemapPaths(
    '<urlset><url><loc>https://preview.towncrierapp.uk/planning</loc></url>' +
      '<url><loc>https://preview.towncrierapp.uk/planning/towns</loc></url></urlset>',
  );
  assert.deepEqual(live, ['/planning', '/planning/croydon']);
  assert.deepEqual(diffPathSets(live, preview), {
    onlyLive: ['/planning/croydon'],
    onlyPreview: ['/planning/towns'],
    common: ['/planning'],
  });
});

test('decodes numeric and named entities', () => {
  assert.equal(decodeEntities('a &amp; b &#39;c&#x27; &lt;d&gt; &unknown;'), "a & b 'c' <d> &unknown;");
});

test('parses origins with defaults', () => {
  assert.deepEqual(parseArgs([]), { live: DEFAULT_LIVE, preview: DEFAULT_PREVIEW });
  assert.deepEqual(parseArgs(['--preview', 'https://preview-dev.towncrierapp.uk/', '--live', 'http://localhost:8080']), {
    live: 'http://localhost:8080',
    preview: 'https://preview-dev.towncrierapp.uk',
  });
  assert.throws(() => parseArgs(['--bogus']));
  assert.throws(() => parseArgs(['--live']));
});
