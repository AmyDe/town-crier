# Offer Codes

Date: 2026-04-18

## Problem

ADR 0010 (Subscription Entitlement Flow) explicitly defers "offer codes / promotional offers" to a future phase. Town Crier now has a web app, a .NET backend, and an iOS app in flight, with Android planned. The operator (solo dev) wants to seed early adopters via Reddit and Discord by handing out single-use codes that grant a time-bounded elevation to Personal or Pro. The codes must work on any client (web today, iOS on launch, Android later) from a single generation step.

This spec defines a minimal, platform-neutral offer-code system: bulk-generated single-use codes, redeemed by authenticated users in iOS or web settings, granting a time-bounded subscription via the existing `UserProfile.ActivateSubscription` path.

## Scope

**In scope**

- Domain aggregate `OfferCode` with single-use redemption.
- Admin-only bulk generation endpoint and matching `tc` CLI command.
- Authenticated user redemption endpoint.
- Redemption UI in iOS Settings and web Settings.
- Cosmos container for code storage.
- Reuse of existing Auth0 metadata sync + client token refresh mechanism.

**Out of scope**

- Apple StoreKit offer codes (see "Considered but rejected" below).
- Google Play promo codes.
- Campaign metadata / grouping / labels.
- Code expiry (campaign deadline) — codes remain valid until redeemed.
- Code revocation endpoint.
- Usage analytics beyond `redeemedAt` / `redeemedByUserId` on the row.
- Rate limiting on the redeem endpoint (60 bits of entropy makes brute force impractical; add later if abused).

## Design

### Domain Model (`TownCrier.Domain.OfferCodes`)

```csharp
public sealed class OfferCode
{
    public string Code { get; }                    // canonical, 12 chars, no separators, uppercase
    public SubscriptionTier Tier { get; }          // Personal | Pro (Free rejected at construction)
    public int DurationDays { get; }               // 1..365
    public DateTimeOffset CreatedAt { get; }
    public string? RedeemedByUserId { get; private set; }
    public DateTimeOffset? RedeemedAt { get; private set; }

    public bool IsRedeemed => this.RedeemedByUserId is not null;

    public void Redeem(string userId, DateTimeOffset now);   // throws OfferCodeAlreadyRedeemedException
}
```

**Invariants**

- `Tier` ≠ `Free` — `Free` codes have no meaning; enforced at construction.
- `DurationDays` ∈ [1, 365] — enforced at construction.
- `Code` is 12 chars from the offer-code alphabet — enforced at construction.
- `Redeem` is idempotent in the sense that calling it on an already-redeemed code throws `OfferCodeAlreadyRedeemedException`; the first redemption wins.

### Code Format

Alphabet: standard Crockford base32 — `0123456789ABCDEFGHJKMNPQRSTVWXYZ` (32 characters; omits `I`, `L`, `O`, `U` by design to avoid digit/letter confusion).

- 12 canonical chars × 5 bits = 60 bits of entropy (~1.15 × 10^18 possibilities).
- **Display format:** `XXXX-XXXX-XXXX` (three groups of four, hyphen-separated).
- **Canonical format:** uppercase, no separators — what's stored and compared.
- **Normalization at the edge:** strip whitespace and `-`, uppercase. Reject if the result isn't 12 chars from the alphabet.

**Generation:** `RandomNumberGenerator.GetBytes(8)` → truncate to 60 bits → encode to 12 chars. Collision with an existing code → generate a new one (vanishingly unlikely but handled).

### Cosmos Container

New container `offer-codes`:

- **Partition key:** `/code` (point reads by code on every redemption).
- **Document id:** equals `code` (canonical form).
- **Schema:**

```json
{
  "id": "A7KMZQR3FNXP",
  "code": "A7KMZQR3FNXP",
  "tier": "Pro",
  "durationDays": 30,
  "createdAt": "2026-04-18T12:00:00Z",
  "redeemedByUserId": null,
  "redeemedAt": null
}
```

Follows the existing Cosmos repository pattern (`IOfferCodeRepository` in application layer, `CosmosOfferCodeRepository` in infrastructure, `InMemoryOfferCodeRepository` for tests).

### Application Handlers

**`GenerateOfferCodesCommand` / `GenerateOfferCodesCommandHandler`**

```csharp
public sealed record GenerateOfferCodesCommand(int Count, SubscriptionTier Tier, int DurationDays);

public sealed record GenerateOfferCodesResult(IReadOnlyList<string> Codes);
```

- Validates `Count ∈ [1, 1000]`, `Tier ∈ {Personal, Pro}`, `DurationDays ∈ [1, 365]`.
- Loops `Count` times: generate random code → `IOfferCodeRepository.CreateAsync` → retry on collision (extremely rare).
- Returns the list of codes in canonical form (the web layer applies display formatting on output).

**`RedeemOfferCodeCommand` / `RedeemOfferCodeCommandHandler`**

```csharp
public sealed record RedeemOfferCodeCommand(string UserId, string Code);

public sealed record RedeemOfferCodeResult(SubscriptionTier Tier, DateTimeOffset ExpiresAt);
```

Flow:

1. Normalize input `Code` (strip separators, uppercase, validate length + alphabet) — throws `InvalidOfferCodeFormatException` on bad shape.
2. `IOfferCodeRepository.GetAsync(canonicalCode, ct)` — throws `OfferCodeNotFoundException` if null.
3. If `code.IsRedeemed` — throws `OfferCodeAlreadyRedeemedException`.
4. `IUserProfileRepository.GetByUserIdAsync(command.UserId, ct)` — throws `UserProfileNotFoundException` if null.
5. If `profile.Tier != SubscriptionTier.Free` — throws `AlreadySubscribedException`.
6. `code.Redeem(command.UserId, now)` — mutates aggregate.
7. `profile.ActivateSubscription(code.Tier, now + TimeSpan.FromDays(code.DurationDays))` — reuses existing domain op.
8. Persist both (`IOfferCodeRepository.SaveAsync`, `IUserProfileRepository.SaveAsync`) and `IAuth0ManagementClient.UpdateSubscriptionTierAsync` — same pattern as `GrantSubscriptionCommandHandler`.
9. Return `RedeemOfferCodeResult(profile.Tier, profile.SubscriptionExpiry!.Value)`.

**Race-condition handling:** `CosmosOfferCodeRepository` uses Cosmos ETag optimistic concurrency — `GetAsync` returns both the aggregate and its `_etag`; `SaveAsync` passes `ItemRequestOptions { IfMatchEtag = etag }`. A concurrent redemption of the same code triggers a `412 Precondition Failed`, which the repository surfaces as `OfferCodeConcurrencyException`; the handler catches it, re-reads, and either returns `code_already_redeemed` (the common case — somebody else won) or retries once (unlikely, but the repository contract allows it).

This is the first Cosmos repository in the codebase to use ETag concurrency. Existing repositories (`CosmosUserProfileRepository` etc.) rely on last-writer-wins, which is acceptable there because a user only writes their own profile. Offer codes are the first aggregate where multiple requesters race for the same row, so the pattern is new — not a convention to inherit.

### Web Layer — Endpoints

**Admin generate** — `POST /v1/admin/offer-codes`

- Auth: `AdminApiKeyFilter` (existing `X-Admin-Key` header).
- Request body:

  ```json
  { "count": 50, "tier": "Pro", "durationDays": 30 }
  ```

- Response `200 OK`, content-type `text/plain`, one display-formatted code per line:

  ```
  A7KM-ZQR3-FNXP
  X2V9-PK4T-DHWR
  ...
  ```

  (Text output is deliberate — paste-friendly into a Reddit post or a text file.)

- Validation errors → `400 Bad Request` with the standard `ProblemDetails` body.

**User redeem** — `POST /v1/offer-codes/redeem`

- Auth: user JWT (standard authenticated endpoint).
- Request body:

  ```json
  { "code": "A7KM-ZQR3-FNXP" }
  ```

  Accepts with or without separators; case-insensitive.

- Success `200 OK`:

  ```json
  { "tier": "Pro", "expiresAt": "2026-05-18T12:00:00Z" }
  ```

- Error responses (structured body with `error` code for the clients to map):

  | HTTP | `error` | Condition |
  |------|---------|-----------|
  | 400 | `invalid_code_format` | Input can't be normalized to 12 alphabet chars |
  | 404 | `invalid_code` | Code not found |
  | 409 | `code_already_redeemed` | `RedeemedByUserId != null` |
  | 409 | `already_subscribed` | `profile.Tier != Free` |

### CLI — `tc generate-offer-codes`

Flat command (matches existing `grant-subscription`, `list-users`).

```
tc generate-offer-codes --count <N> --tier <Personal|Pro> --duration-days <D>
```

- `--count` required, 1..1000.
- `--tier` required, `Personal` or `Pro` (case-insensitive, normalized).
- `--duration-days` required, 1..365.
- Calls `POST /v1/admin/offer-codes` with `X-Admin-Key` from config.
- **stdout:** one display-formatted code per line (streaming the API response body directly).
- **stderr:** one-line summary, e.g. `Generated 50 codes: Pro tier, 30 days duration`.
- Exit codes: `0` success, `1` argument validation, `2` API error.

Operator workflow:

```bash
tc generate-offer-codes --count 50 --tier Pro --duration-days 30 > pro-30d.txt
# paste codes into a Reddit post, share in Discord, etc.
```

### iOS — Redemption in Settings

Standards: `ios-coding-standards` skill (MVVM-C, Swift Concurrency, XCTest, protocol services).

- **Settings screen** — new row "Redeem offer code" under the subscription section.
- Tap → push a `RedeemOfferCodeView` (or present as a sheet, consistent with existing Settings modality).
- **View:** monospaced text field with auto-uppercase, auto-format `XXXX-XXXX-XXXX` as the user types, primary CTA `Redeem`, loading state on the CTA while the request is in flight.
- **ViewModel:** `RedeemOfferCodeViewModel` with `redeem()` async method; depends on `OfferCodeService` protocol.
- **Service protocol:**

  ```swift
  protocol OfferCodeService {
      func redeem(code: String) async throws -> RedemptionResult
  }
  ```

- **On 200:**
  1. Show success alert: `"You're on Pro for 30 days. Enjoy!"` (tier + days taken from the response).
  2. Force `credentialsManager.renew()` so the next JWT has `subscription_tier = Pro`.
  3. Re-fetch `UserProfile` so the UI reflects the new tier.
  4. Dismiss back to Settings.
- **On 4xx:** inline error under the text field, mapped from the server's `error` code:
  - `invalid_code_format` → "Please check the code and try again."
  - `invalid_code` → "This code isn't valid."
  - `code_already_redeemed` → "This code has already been used."
  - `already_subscribed` → "You already have an active subscription. Offer codes are only for new subscribers."

### Web — Redemption in Settings

Standards: `react-coding-standards` skill (feature-sliced, hooks-as-ViewModels, CSS Modules).

- **Settings page** — new section "Redeem offer code" under the subscription section.
- Text input + submit button; same auto-formatting behaviour (`XXXX-XXXX-XXXX`) on input handler.
- **Hook:** `useRedeemOfferCode()` — encapsulates the fetch, states (`idle` / `loading` / `error` / `success`), and the token refresh.
- **On 200:**
  1. Toast: `"You're on Pro for 30 days."`
  2. `getAccessTokenSilently({ cacheMode: 'off' })` so the new claim is picked up.
  3. Re-fetch the profile query (if there's a query cache) so tier-gated UI updates.
- **On 4xx:** inline error with the same mapping as iOS.

### Native AOT

Per ADR 0010 / dotnet standards:

- New DTOs (`GenerateOfferCodesRequest`, `RedeemOfferCodeRequest`, `RedeemOfferCodeResponse`) added to `AppJsonSerializerContext` in `town-crier.web` and `TcJsonContext` in `cli/tc`.
- No reflection, no dynamic dispatch.
- Cosmos SDK used directly (`ItemResponse<OfferCode>`, no ORM).

### Auth0 Sync

On redemption, `RedeemOfferCodeCommandHandler` calls `IAuth0ManagementClient.UpdateSubscriptionTierAsync` — same path `GrantSubscriptionCommandHandler` already uses. No new Auth0 config required.

### Entitlement Enforcement After Redemption

No changes to the entitlement framework:

- `UserProfile.SubscriptionExpiry` is set to `now + durationDays` by `ActivateSubscription`.
- On the tier-gated endpoint filter, the JWT's `subscription_tier` claim is checked (already fail-safe to Free).
- On expiry, the existing "subscriptionExpiry in the past ⇒ treat as Free" rule kicks in — silent drop, same pattern as Apple-sourced expiry.
- On the next natural token refresh (or an explicit one triggered by an auth-dependent feature), the claim catches up with Cosmos.

Small caveat: a user whose promo expires while holding a live access token keeps frontend tier-gated UI visible until their token refreshes (typically ≤ 24h, depending on Auth0 token lifetime). This matches current behaviour for paid-subscription expiry and is acceptable at this stage.

## Testing

Per `dotnet-coding-standards`, `ios-coding-standards`, `react-coding-standards`.

**Domain (`town-crier.domain.tests`)**

- `OfferCodeTests.Constructor_RejectsFreeTier`
- `OfferCodeTests.Constructor_RejectsOutOfRangeDuration`
- `OfferCodeTests.Constructor_RejectsMalformedCode`
- `OfferCodeTests.Redeem_SetsRedeemedByAndRedeemedAt`
- `OfferCodeTests.Redeem_OnAlreadyRedeemed_Throws`

**Application (`town-crier.application.tests`)**

- `GenerateOfferCodesCommandHandlerTests` — validates arg ranges, generates N codes, tier+duration propagated, repo called N times.
- `RedeemOfferCodeCommandHandlerTests` — all four error paths + happy path; verifies `UserProfile.ActivateSubscription` invoked with correct expiry; verifies Auth0 sync called.
- `FakeOfferCodeRepository` — new hand-written fake following `FakeUserProfileRepository` pattern.

**Infrastructure (`town-crier.infrastructure.tests`)**

- `CosmosOfferCodeRepositoryTests` — create, get-by-code (point read), save (with ETag concurrency test), mirroring existing Cosmos repo tests.

**Web (`town-crier.web.tests`)**

- `GenerateOfferCodesEndpointTests` — admin auth required, arg validation, `text/plain` response format.
- `RedeemOfferCodeEndpointTests` — error-code mapping for each 4xx case, success response shape.

**CLI (`tc.tests`)**

- `GenerateOfferCodesCommandTests` — arg validation, exit codes, output routing (codes to stdout, summary to stderr).

**iOS (`town-crier-tests`)**

- `RedeemOfferCodeViewModelTests` — happy path, each error-code mapping, loading state transitions.
- `FakeOfferCodeService` fake following existing service-fake conventions.

**Web (`web/__tests__`)**

- `useRedeemOfferCode.test.ts` — state transitions, error mapping.
- Component test for the Settings section with a fake hook.

## Considered but rejected

### Apple StoreKit offer codes

StoreKit offer codes (generated in App Store Connect, redeemed via `AppStore.presentOfferCodeRedeemSheet` or `apps.apple.com/redeem`) would let us reuse the entire existing App Store verify + notifications pipeline with zero new backend code. They also enable auto-conversion from a free intro period to a paid subscription, which outperforms any custom build on conversion.

Rejected because:

1. **iOS-only.** The web app cannot redeem StoreKit codes; Reddit/Discord recipients will land on the website first. Distribution on social channels is the primary use case and web redemption is the primary flow.
2. **Android compatibility.** Google Play has a separate promo-code system. Maintaining two platform-specific flows forever is more operator burden than running one uniform system.
3. **Fixed durations.** Apple's intro-offer menu (3d / 1w / 2w / 1m / ...) doesn't allow arbitrary "14 days of Pro".
4. **Apple ID requirement.** Redeemer must have an Apple ID with a valid payment method on file — friction at the earliest-adopter stage.
5. **Quota.** 1,000 codes per product per quarter; adequate now, constraining later.

Future: StoreKit intro offers remain a good fit for *paid-trial conversion campaigns* ("get 3 months free, then £5.99/month"). Adding them alongside the custom system in a later phase is straightforward — they'd land via the existing notification webhook with no interaction with this spec's code.

### Per-user multi-use codes (e.g. "BETA50" used by anyone)

Not suitable for single-use-per-code semantics; collapses to a coupon-code system which has different fraud properties. Out of scope.

### Rate limiting on redeem

60 bits of entropy makes brute force infeasible. If abuse materialises (noise in logs, or actual attempts), add per-user or per-IP bucket as a follow-up.

## Migration Path

1. Add `OfferCode` aggregate + `OfferCodeAlreadyRedeemedException` to domain layer.
2. Add `IOfferCodeRepository`, `GenerateOfferCodesCommandHandler`, `RedeemOfferCodeCommandHandler` + exceptions to application layer.
3. Add `InMemoryOfferCodeRepository` (tests) and `CosmosOfferCodeRepository` (infrastructure).
4. Provision the `offer-codes` Cosmos container via Pulumi.
5. Add endpoints (`POST /v1/admin/offer-codes`, `POST /v1/offer-codes/redeem`) + DTOs + `AppJsonSerializerContext` entries.
6. Add `tc generate-offer-codes` CLI command + dispatch in `Program.cs` + help text.
7. Add iOS redemption view, ViewModel, service, and wire into Settings.
8. Add web redemption section, hook, and API client method.
9. End-to-end smoke test: generate 5 codes via CLI → redeem one in web → verify profile tier + expiry in Cosmos → redeem same code again (expect 409) → redeem different code (expect 409 already_subscribed).
