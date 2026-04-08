# Legal Documents API

Date: 2026-04-08

## Summary

Serve privacy policy and terms of service from the API as structured JSON, providing a single source of truth across iOS, web, and any future clients.

## Endpoint

`GET /v1/legal/{documentType}` — anonymous, no authentication required.

- `documentType`: `privacy` or `terms`
- Returns 404 for unknown document types

## Response Shape

```json
{
  "documentType": "privacy",
  "title": "Privacy Policy",
  "lastUpdated": "2026-04-08",
  "sections": [
    {
      "heading": "What We Collect",
      "body": "We collect the following information..."
    }
  ]
}
```

## Architecture

Follows existing CQRS patterns in the API:

- **Endpoint**: `LegalEndpoints.cs` in `town-crier.web/Endpoints/` — maps `GET /v1/legal/{documentType}`, anonymous access via `.AllowAnonymous()`
- **Query + Handler**: `GetLegalDocumentQuery` / `GetLegalDocumentQueryHandler` in `town-crier.application/Legal/`
- **Domain model**: `LegalDocument` and `LegalDocumentSection` value objects, `LegalDocumentType` enum in `town-crier.domain/Legal/`
- **Content**: Hardcoded in a static content class — no Cosmos DB, no external files. Version-controlled, changes via PRs.

## Content Source

Port the existing privacy policy and terms of service text from the iOS app's `LegalDocumentViewModel.swift`.

## Client Migration Path

Once this endpoint ships:
- iOS app replaces hardcoded `LegalDocumentViewModel` content with an API call
- Web `LegalPage.tsx` fetches from this endpoint instead of showing "coming soon"

Client migration is out of scope for this spec — separate beads.

## Testing

- **Handler tests**: returns correct document for each type, returns error for unknown type
- **Endpoint integration**: correct status codes (200, 404), correct response shape, anonymous access works
