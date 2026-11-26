export const PAYLOAD_EXAMPLE = `{
  "schemaVersion": 1,
  "eventId": "6f1d2c9e-8a7b-4c3d-9e0f-1a2b3c4d5e6f",
  "type": "application.new",
  "occurredAt": "2026-11-26T14:00:03Z",
  "watchZone": { "id": "c0ffee00-0000-4000-8000-000000000001", "name": "Home" },
  "sources": ["zone"],
  "application": {
    "reference": "26/01234/FUL",
    "authorityName": "Leeds",
    "authoritySlug": "leeds",
    "address": "1 High Street, Leeds",
    "postcode": "LS1 1AA",
    "description": "Single-storey rear extension",
    "appType": "Full",
    "appState": "Undecided",
    "appSize": "Small",
    "startDate": "2026-11-20",
    "decidedDate": null,
    "latitude": 53.7997,
    "longitude": -1.5492,
    "councilUrl": "https://publicaccess.leeds.gov.uk/...",
    "shareUrl": "https://share.towncrierapp.uk/a/leeds/26/01234/FUL"
  }
}`;

export const NODE_EXAMPLE = `import { createHmac, timingSafeEqual } from 'node:crypto';

const MAX_AGE_SECONDS = 5 * 60;

// rawBody is a Buffer holding the request body, before any JSON parsing.
export function isFromTownCrier(secret, headers, rawBody, now = Date.now()) {
  const timestamp = headers['x-towncrier-timestamp'] ?? '';
  const signature = headers['x-towncrier-signature'] ?? '';
  if (!/^\\d+$/.test(timestamp)) return false;
  if (Math.abs(now / 1000 - Number(timestamp)) > MAX_AGE_SECONDS) return false;

  const digest = createHmac('sha256', secret)
    .update(\`\${timestamp}.\`)
    .update(rawBody)
    .digest('hex');
  const expected = Buffer.from(\`v1=\${digest}\`);
  const received = Buffer.from(signature);
  return expected.length === received.length && timingSafeEqual(expected, received);
}`;

export const PYTHON_EXAMPLE = `import hashlib
import hmac
import time

MAX_AGE_SECONDS = 5 * 60


# headers is your framework's case-insensitive header mapping.
# raw_body is the request body as bytes, before any JSON parsing.
def is_from_town_crier(secret, headers, raw_body, now=None):
    timestamp = headers.get("X-TownCrier-Timestamp", "")
    signature = headers.get("X-TownCrier-Signature", "")
    if not (timestamp.isascii() and timestamp.isdigit()):
        return False
    now = time.time() if now is None else now
    if abs(now - int(timestamp)) > MAX_AGE_SECONDS:
        return False

    signed = timestamp.encode() + b"." + raw_body
    digest = hmac.new(secret.encode(), signed, hashlib.sha256).hexdigest()
    return hmac.compare_digest(f"v1={digest}".encode(), signature.encode())`;

export const HEADERS_EXAMPLE = `POST <your URL>
Content-Type: application/json
User-Agent: TownCrier-Webhooks/1.0 (+https://towncrierapp.uk/developers/webhooks)
X-TownCrier-Event-Id: <eventId>
X-TownCrier-Event-Type: <type>
X-TownCrier-Timestamp: <Unix time in seconds when we sent it>
X-TownCrier-Signature: v1=<lowercase hex HMAC-SHA256>`;

export const SLACK_EXAMPLE = `*Town Crier: 3 planning updates*
• New near Home: <shareUrl|26/01234/FUL>, 1 High Street, Leeds. Single-storey rear extension
• Decision (Permitted): <shareUrl|26/01001/HOU>, 4 Mill Lane, Leeds. Loft conversion
• New near Office: 26/09999/FUL, 9 Park Row, Leeds. Change of use to café`;
