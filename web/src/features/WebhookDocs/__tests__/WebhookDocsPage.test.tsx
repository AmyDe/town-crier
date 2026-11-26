import { createHmac, timingSafeEqual } from 'node:crypto';
import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { MemoryRouter } from 'react-router';
import { WebhookDocsPage } from '../WebhookDocsPage';
import { NODE_EXAMPLE, PAYLOAD_EXAMPLE, PYTHON_EXAMPLE } from '../examples';

function renderPage() {
  return render(
    <MemoryRouter>
      <WebhookDocsPage />
    </MemoryRouter>,
  );
}

// The known value from the API's signature test.
const SECRET = 'tcwh_AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8';
const TIMESTAMP = 1795701600;
const BODY = Buffer.from('{"schemaVersion":1,"eventId":"e-1"}');
const SIGNATURE = 'v1=0fa9f5bf4ab8b38b3f02f989cf0d8791e11d14dfd8e2c2cef494bb486d58bd74';

type Verifier = (
  secret: string,
  headers: Record<string, string>,
  rawBody: Buffer,
  now: number,
) => boolean;

function publishedNodeVerifier(): Verifier {
  const source = NODE_EXAMPLE.replace(/^import .*$/m, '').replace('export function', 'function');
  const build = new Function(
    'createHmac',
    'timingSafeEqual',
    `${source}\nreturn isFromTownCrier;`,
  ) as (hmac: typeof createHmac, equal: typeof timingSafeEqual) => Verifier;
  return build(createHmac, timingSafeEqual);
}

describe('WebhookDocsPage', () => {
  it('renders each section heading', () => {
    renderPage();

    expect(screen.getByRole('heading', { level: 1, name: 'Webhooks' })).toBeInTheDocument();
    expect(screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent)).toEqual([
      'Set up a webhook',
      'What sends a request',
      'When requests are sent',
      'Which addresses work',
      'JSON payload',
      'Request headers',
      'Check the signature',
      'Retries and switching off',
      'Duplicates',
      'Slack format',
    ]);
  });

  it('documents the headers, limits and de-duplication', () => {
    const { container } = renderPage();
    const text = container.textContent ?? '';

    for (const expected of [
      'X-TownCrier-Event-Id',
      'X-TownCrier-Event-Type',
      'X-TownCrier-Timestamp',
      'X-TownCrier-Signature',
      'TownCrier-Webhooks/1.0 (+https://towncrierapp.uk/developers/webhooks)',
      '6 attempts',
      '24 failed runs in a row',
      '410',
      '5 minutes',
      'at least once',
      '…and N more in the Town Crier app',
    ]) {
      expect(text).toContain(expected);
    }
    expect(text).not.toMatch(/[–—]/);
  });

  it('shows the payload with exactly the contract keys', () => {
    const payload = JSON.parse(PAYLOAD_EXAMPLE) as { application: object };

    expect(Object.keys(payload)).toEqual([
      'schemaVersion',
      'eventId',
      'type',
      'occurredAt',
      'watchZone',
      'sources',
      'application',
    ]);
    expect(Object.keys(payload.application)).toEqual([
      'reference',
      'authorityName',
      'authoritySlug',
      'address',
      'postcode',
      'description',
      'appType',
      'appState',
      'appSize',
      'startDate',
      'decidedDate',
      'latitude',
      'longitude',
      'councilUrl',
      'shareUrl',
    ]);
  });

  it('shows both signature examples', () => {
    const { container } = renderPage();

    expect(container.textContent).toContain(NODE_EXAMPLE);
    expect(container.textContent).toContain(PYTHON_EXAMPLE);
    expect(PYTHON_EXAMPLE).toContain('hmac.compare_digest');
  });

  it('publishes a Node.js check that accepts a real signature', () => {
    const verify = publishedNodeVerifier();
    const headers = {
      'x-towncrier-timestamp': String(TIMESTAMP),
      'x-towncrier-signature': SIGNATURE,
    };
    const now = (TIMESTAMP + 60) * 1000;

    expect(verify(SECRET, headers, BODY, now)).toBe(true);
    expect(verify(SECRET, headers, Buffer.from('{}'), now)).toBe(false);
    expect(verify(`${SECRET}x`, headers, BODY, now)).toBe(false);
    expect(verify(SECRET, headers, BODY, (TIMESTAMP + 301) * 1000)).toBe(false);
    expect(verify(SECRET, {}, BODY, now)).toBe(false);
  });
});
