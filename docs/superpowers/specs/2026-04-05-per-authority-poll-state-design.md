# Per-Authority Poll State

Date: 2026-04-05

## Problem

The polling worker stores a single global `lastPollTime` shared across all authorities. This causes two issues:

1. **New authorities start from "now"** — when a user creates a watch zone for a new authority, the next poll cycle uses the global timestamp (e.g. "5 minutes ago") instead of backfilling 30 days of history. The new authority appears empty until councils publish fresh changes.

2. **Rate-limited authorities lose progress** — if authority A succeeds and advances the global timestamp, then authority B gets rate-limited and skipped, B's next poll uses A's timestamp, potentially missing applications that changed between B's last real poll and A's.

## Design

### Approach: Per-authority documents in existing PollState container

Store one Cosmos document per authority instead of a single global document. No new containers, indexes, or migrations required.

### Interface change

`IPollStateStore` gains an `authorityId` parameter on both methods:

```csharp
Task<DateTimeOffset?> GetLastPollTimeAsync(int authorityId, CancellationToken ct);
Task SaveLastPollTimeAsync(int authorityId, DateTimeOffset pollTime, CancellationToken ct);
```

### Cosmos store

- Document ID changes from `"poll-state"` to `"poll-state-{authorityId}"` (e.g. `"poll-state-314"`)
- ID is used as both document ID and partition key (same pattern as current)
- `PollStateDocument` gains an `AuthorityId` property for debuggability when browsing Cosmos Data Explorer

### Handler

- `GetLastPollTimeAsync` moves inside the per-authority loop, called once per authority
- The 30-day fallback (`now.AddDays(-30)`) applies per authority — a brand-new authority automatically backfills
- `SaveLastPollTimeAsync` is called after each authority succeeds (already the case), now scoped to that authority ID
- A rate-limited or errored authority does not get its timestamp advanced

### Cleanup

Delete the orphaned global `"poll-state"` document from the PollState container. This is a one-time operation in the Cosmos store — on startup or as part of the first per-authority save cycle, attempt to delete `"poll-state"` and swallow NotFound. Simpler alternative: add a delete method to `ICosmosRestClient` (already exists as `DeleteDocumentAsync`) and call it once in the handler after the loop completes, guarded by a flag.

Chosen approach: add a `DeleteGlobalPollStateAsync` method to `IPollStateStore` and call it at the end of the poll cycle in the handler. The Cosmos implementation deletes doc `"poll-state"` with partition key `"poll-state"`, swallowing 404. This is idempotent — safe to call every cycle until the doc is gone.

### Test changes

- `FakePollStateStore` stores a `Dictionary<int, DateTimeOffset?>` instead of a single value
- Existing tests get mechanical `authorityId` parameter additions on Get/Save calls
- New test: "new authority uses 30-day lookback when other authorities already have poll state"
- New test: "rate-limited authority retains its own last poll time"
- New test: "global poll state document is cleaned up after cycle"

## Files changed

| File | Change |
|------|--------|
| `api/src/town-crier.application/Polling/IPollStateStore.cs` | Add `authorityId` param, add `DeleteGlobalPollStateAsync` |
| `api/src/town-crier.application/Polling/PollPlanItCommandHandler.cs` | Move poll state read inside loop, pass authority ID, call cleanup |
| `api/src/town-crier.infrastructure/Polling/CosmosPollStateStore.cs` | Per-authority doc IDs, implement cleanup |
| `api/src/town-crier.infrastructure/Polling/PollStateDocument.cs` | Add `AuthorityId` property |
| `api/tests/town-crier.application.tests/Polling/FakePollStateStore.cs` | Dictionary-based storage |
| `api/tests/town-crier.application.tests/Polling/PollPlanItCommandHandlerTests.cs` | Update existing + add new tests |

## What doesn't change

- Cosmos PollState container — no schema or index changes
- `PlanItClient` — already accepts `differentStart` per call
- Worker `Program.cs` — doesn't touch poll state directly
- `CosmosJsonSerializerContext` — `PollStateDocument` already registered
- No migration needed — old doc becomes orphaned then cleaned up
