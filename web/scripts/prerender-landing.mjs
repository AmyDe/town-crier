import { readdirSync, readFileSync, writeFileSync } from 'node:fs';

const MARKER = '<div id="root"></div>';

const dist = new URL('../dist/', import.meta.url);
const indexUrl = new URL('index.html', dist);

function fail(message) {
  console.error(`prerender-landing: ${message}`);
  process.exit(1);
}

const shell = readFileSync(indexUrl, 'utf-8');
writeFileSync(new URL('app.html', dist), shell);

if (!shell.includes(MARKER)) {
  fail(`dist/index.html does not contain ${MARKER}`);
}

const { renderLanding } = await import('../dist-ssr/landing.js');
const { html, jsonLd } = renderLanding();

if (!html) {
  fail('the landing render is empty');
}
if (!html.includes('Pricing')) {
  fail('the landing render does not contain "Pricing"');
}

const assets = new URL('assets/', dist);
const css = readdirSync(assets)
  .filter((file) => file.endsWith('.css'))
  .map((file) => readFileSync(new URL(file, assets), 'utf-8'))
  .join('\n');

const classNames = new Set(
  [...html.matchAll(/\sclass="([^"]*)"/g)]
    .flatMap((match) => match[1].split(/\s+/))
    .filter(Boolean),
);
if (classNames.size === 0) {
  fail('the landing render has no CSS classes');
}

const missing = [...classNames].filter((className) => {
  const escaped = className.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return !new RegExp(`\\.${escaped}(?![\\w-])`).test(css);
});
if (missing.length > 0) {
  fail(`classes missing from dist/assets/*.css: ${missing.join(', ')}`);
}

const script = `<script type="application/ld+json">${jsonLd.replaceAll('<', '\\u003c')}</script>`;

// Function replacements stop "$" in the content from being read as a replacement pattern.
const page = shell
  .replace(MARKER, () => `<div id="root">${html}</div>`)
  .replace('</head>', () => `${script}</head>`);

writeFileSync(indexUrl, page);
