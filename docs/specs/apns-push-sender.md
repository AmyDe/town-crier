# APNs Push Notification Sender

## Context

TestFlight push notifications don't work. The dispatch pipeline (PlanIt poll → `Notification` persisted → `DispatchNotificationCommandHandler` → `IPushNotificationSender.SendAsync`) is real except for the final hop: `api/src/town-crier.worker/Program.cs:117` binds `IPushNotificationSender` to `NoOpPushNotificationSender`, which returns `Task.CompletedTask` for both `SendAsync` and `SendDigestAsync`.

The iOS side already captures the device token and POSTs it to the API via `APINotificationService`. The API persists it via `RegisterDeviceTokenCommandHandler` into a Cosmos `DeviceRegistrations` container. So tokens flow in correctly — they're just never used.

This spec covers replacing the no-op with a real APNs sender. See ADR-0026 for the decision (direct APNs HTTP/2 over Notification Hubs / FCM).

## Design

### Approach: direct APNs HTTP/2 with JWT (ES256)

- **Transport:** HTTP/2 to `api.push.apple.com` (prod) / `api.sandbox.push.apple.com` (sandbox).
- **Auth:** Provider JWT signed ES256 with the team's `.p8` auth key. Carried in `authorization: bearer <jwt>` on every request.
- **No SDK.** `System.Net.Http` + `System.Security.Cryptography.ECDsa` (`ImportFromPem`, `SignData` with SHA-256). Native AOT-friendly: no reflection, no source generators beyond the JSON ones already in use.
- **Bundle ID:** `uk.towncrierapp.mobile` (from `mobile/ios/project.yml:23`). Goes in `apns-topic` header.

### JWT lifecycle

Apple's window: a JWT older than 60 minutes returns `403 ExpiredProviderToken`; regenerating more than once per 20 minutes returns `429 TooManyProviderTokenUpdates`.

**Refresh strategy:** mint one JWT per process, cached in memory, regenerated at ~50 minutes. Single instance, lock-guarded, no persistence, no Key Vault round-trip.

```csharp
sealed class ApnsJwtProvider {
    readonly ECDsa key;             // loaded once from .p8 PEM in config
    readonly string keyId, teamId;
    readonly Lock gate = new();
    string? cached;
    DateTimeOffset mintedAt;

    public string Current() {
        lock (gate) {
            if (cached is null || DateTimeOffset.UtcNow - mintedAt > TimeSpan.FromMinutes(50))
                (cached, mintedAt) = (Mint(), DateTimeOffset.UtcNow);
            return cached;
        }
    }
}
```

JWT shape:
- Header: `{ "alg": "ES256", "kid": "<keyId>" }`
- Payload: `{ "iss": "<teamId>", "iat": <unix seconds> }`
- Signature: ES256 over `base64url(header) + "." + base64url(payload)`

### Sender contract change

The sender must report which device tokens APNs rejected so the handler can prune them. Today's contract returns `Task` — change to return a result type:

```csharp
public interface IPushNotificationSender
{
    Task<PushSendResult> SendAsync(Notification notification, IReadOnlyList<DeviceRegistration> devices, CancellationToken ct);
    Task<PushSendResult> SendDigestAsync(int applicationCount, IReadOnlyList<DeviceRegistration> devices, CancellationToken ct);
}

public sealed record PushSendResult(IReadOnlyList<string> InvalidTokens);
```

`InvalidTokens` is empty when nothing was rejected. The sender knows APNs status codes; the handler owns Cosmos cleanup. Clean separation.

### Sender implementation

`ApnsPushNotificationSender : IPushNotificationSender` in `api/src/town-crier.infrastructure/Notifications/`.

**HttpClient:** singleton, configured for HTTP/2 (`DefaultRequestVersion = HttpVersion.Version20`, `DefaultVersionPolicy = HttpVersionPolicy.RequestVersionExact`). Base address selected from `Apns:UseSandbox` config.

**Send loop:** APNs is request-per-token. Iterate devices with a parallelism cap (`SemaphoreSlim`, e.g. 10) to avoid floods. Collect rejected tokens into a list returned in `PushSendResult`.

**Per-request headers:**
- `authorization: bearer <jwt>` — from `ApnsJwtProvider`
- `apns-topic: uk.towncrierapp.mobile`
- `apns-push-type: alert` (for alerts) / `background` (for silent — not used yet)
- `apns-priority: 10` (immediate delivery for alerts)
- `apns-expiration: 0` (don't store-and-forward)

**Payload (alert, instant):**
```json
{
  "aps": {
    "alert": { "title": "<watch zone name>", "body": "<address — decision>" },
    "sound": "default",
    "badge": 1
  },
  "notificationId": "<guid>",
  "applicationRef": "<planit-name>"
}
```

**Payload (digest):**
```json
{
  "aps": {
    "alert": { "title": "Town Crier", "body": "<n> new applications this week" },
    "sound": "default",
    "badge": "<n>"
  }
}
```

Both serialized via existing `[JsonSerializable]` source generators — add the payload types to `CosmosJsonSerializerContext` or a new `ApnsJsonSerializerContext`.

### Response handling

| Status | Meaning | Action |
|--------|---------|--------|
| 200 | Delivered | continue |
| 400 `BadDeviceToken` | malformed token | add to `InvalidTokens` |
| 403 `ExpiredProviderToken` | JWT expired/clock skew | invalidate cache, retry once with fresh JWT |
| 410 `Unregistered` | app uninstalled / token rotated | add to `InvalidTokens` |
| 429 `TooManyProviderTokenUpdates` | regenerated JWT too fast | log + brief backoff |
| 5xx | APNs transient | log + retry with exponential backoff (max 3 attempts) |

The 403 retry uses the freshly-minted JWT; if the second attempt also fails, log and skip the device (don't add to InvalidTokens — the token may still be valid).

The body of error responses is JSON `{ "reason": "<code>" }` per Apple's spec — parse it to choose the action above.

### Token pruning

`DispatchNotificationCommandHandler` already takes `IPushNotificationSender`. After `SendAsync`, iterate `result.InvalidTokens` and dispatch `RemoveInvalidDeviceTokenCommand` for each (handler already exists at `api/src/town-crier.application/DeviceRegistrations/RemoveInvalidDeviceTokenCommandHandler.cs`).

Same for `GenerateWeeklyDigestsCommandHandler` and any other call-site of `IPushNotificationSender` (grep `IPushNotificationSender` in `api/src/`).

### Configuration

```
Apns:Enabled    = true|false   (feature flag — false in local dev so missing key isn't fatal)
Apns:AuthKey    = <PEM contents of .p8 — secret>
Apns:KeyId      = <10-char Apple Key ID>
Apns:TeamId     = <10-char Apple Team ID>
Apns:BundleId   = uk.towncrierapp.mobile
Apns:UseSandbox = true (dev) | false (prod)
```

Bound via `ApnsOptions` class with `IOptions<T>`-style validation at startup. When `Apns:Enabled = false`, DI registers `NoOpPushNotificationSender` (keep the class). When `true`, registers `ApnsPushNotificationSender` and validates that `AuthKey/KeyId/TeamId/BundleId` are non-empty.

### Infrastructure wiring

In `infra/EnvironmentStack.cs`:

1. **Three new pulumi config keys:**
   - `town-crier:apnsAuthKey` (secret, .p8 PEM contents)
   - `town-crier:apnsKeyId` (plain, 10 chars)
   - `town-crier:apnsTeamId` (plain, 10 chars)

   `apnsBundleId` is hard-coded as `"uk.towncrierapp.mobile"` — not user-configurable per-stack.

2. **ACA secrets array:** add `new SecretArgs { Name = "apns-auth-key", Value = apnsAuthKey }` to both the API container app and any worker job that runs notification dispatch.

3. **Env vars on relevant containers:**
   ```csharp
   new EnvironmentVarArgs { Name = "Apns__Enabled",    Value = "true" },
   new EnvironmentVarArgs { Name = "Apns__AuthKey",    SecretRef = "apns-auth-key" },
   new EnvironmentVarArgs { Name = "Apns__KeyId",      Value = apnsKeyId },
   new EnvironmentVarArgs { Name = "Apns__TeamId",     Value = apnsTeamId },
   new EnvironmentVarArgs { Name = "Apns__BundleId",   Value = "uk.towncrierapp.mobile" },
   new EnvironmentVarArgs { Name = "Apns__UseSandbox", Value = env == "dev" ? "true" : "false" },
   ```

4. **Which containers need this?** Any process that calls `IPushNotificationSender`. Today that's:
   - `digest` and `digest-hourly` worker jobs (call `GenerateWeeklyDigestsCommandHandler`).
   - The container that runs `DispatchNotificationCommandHandler` (instant-push path). Verify whether this is the API web container (Cosmos change feed processor) or a worker job — grep `DispatchNotificationCommandHandler` in `api/src/town-crier.web` and `api/src/town-crier.worker` to confirm before wiring.

   Implementer must wire APNs env vars into every container that resolves `IPushNotificationSender`. The simplest approach is to project them into `CreateWorkerJob`'s shared env block plus the API container's env block.

### Native AOT compatibility

- No reflection: `ECDsa.ImportFromPem`, `SignData(ReadOnlySpan<byte>, HashAlgorithmName.SHA256)` are AOT-clean.
- JSON serialization via `[JsonSerializable]` source generators only — add APNs payload types to a serializer context. No anonymous-object serialization, no `JsonSerializer.Serialize<object>`.
- HTTP/2 client uses `System.Net.Http.SocketsHttpHandler` (default on modern .NET) — AOT-clean.

## Scope

### In
- New `ApnsPushNotificationSender` + `ApnsJwtProvider` in infrastructure.
- `PushSendResult` return type + handler pruning.
- `ApnsOptions` config binding + DI swap behind `Apns:Enabled` flag.
- Pulumi config + ACA env wiring across API and relevant worker containers.
- ADR-0026 documenting the choice.
- Apple Developer Portal: generate `.p8` auth key (operational task).
- TestFlight end-to-end verification.

### Out
- Notification Hubs (rejected — see ADR-0026).
- FCM / Android push (no Android client yet).
- Silent push / background fetch (`apns-push-type: background`) — not needed for current pipeline.
- Push payload localisation / per-locale strings — present payload uses notification copy from the existing domain model.
- Rich media attachments / notification service extensions on iOS — out of scope.
- Per-user APNs delivery analytics dashboard — `notifications.sent` metric already exists in App Insights; no new dashboards.

## Steps

### ADR-0026 — Direct APNs HTTP/2 over Notification Hubs

`docs/adr/0026-apns-direct-http2.md`. Captures: chose direct HTTP/2 + JWT because (a) iOS-only today, no multi-platform need, (b) avoids new Azure resource, (c) keeps minimal-dependency stance from `dotnet-coding-standards`, (d) AOT-clean. Trade-off: re-do work if Android lands and we want unified FCM+APNs via Notification Hubs — accepted, the `IPushNotificationSender` seam makes it a swap.

### PushSendResult contract

Change `IPushNotificationSender.SendAsync` and `SendDigestAsync` to return `Task<PushSendResult>`. Add `PushSendResult` record. Update `NoOpPushNotificationSender` (return empty result). Update all call sites — `DispatchNotificationCommandHandler`, `GenerateWeeklyDigestsCommandHandler`, `DispatchDecisionEventCommandHandler` — to consume the result (initially: discard `InvalidTokens`, the pruning step lands separately). Tests: existing handler tests update to `await result;` shape.

### ApnsJwtProvider

New class in `api/src/town-crier.infrastructure/Notifications/`. Loads PEM-encoded EC private key once from `ApnsOptions.AuthKey`. `Current()` returns cached JWT, mints fresh when older than 50 minutes, lock-guarded. ES256 signing with `ECDsa.SignData(..., HashAlgorithmName.SHA256)`. Base64url encoding (no `+`, `/`, padding). Tests: cache reuse, refresh boundary at 50 min (use injectable `TimeProvider`), thread-safety smoke test, header/payload structure.

### ApnsPushNotificationSender

New class in `api/src/town-crier.infrastructure/Notifications/`. Implements `IPushNotificationSender`. Takes `HttpClient`, `ApnsJwtProvider`, `ApnsOptions`, `ILogger`, `TimeProvider`. Builds payload, posts per device with parallelism cap, parses APNs error reasons, populates `PushSendResult.InvalidTokens` from 410/400-`BadDeviceToken` responses. 403 → invalidate JWT, retry once. 5xx → exponential backoff up to 3 attempts. Tests: fake `HttpMessageHandler`, assert HTTP/2 request building, JWT header presence, 200/403/410/429/5xx paths, JWT cache reuse on 403 retry, parallelism cap honoured.

### DispatchNotification handler invalid-token pruning

Update `DispatchNotificationCommandHandler` and `GenerateWeeklyDigestsCommandHandler` (and `DispatchDecisionEventCommandHandler` if it calls the sender) to dispatch `RemoveInvalidDeviceTokenCommand` for each token in `result.InvalidTokens`. Tests: spy on `RemoveInvalidDeviceTokenCommandHandler`, assert one call per invalid token, assert idempotent (no removal when `InvalidTokens` empty).

### ApnsOptions + DI registration

`ApnsOptions` class in `api/src/town-crier.infrastructure/Notifications/`. Bound from `IConfiguration` section `"Apns"`. Validation at startup: when `Enabled == true`, all of `AuthKey/KeyId/TeamId/BundleId` non-empty, `KeyId` and `TeamId` are 10 chars. Update `api/src/town-crier.worker/Program.cs:117` and the API web's DI to: register `NoOpPushNotificationSender` when `Apns:Enabled = false`, register `ApnsPushNotificationSender` (+ `ApnsJwtProvider` singleton + named `HttpClient` configured for HTTP/2) when `true`. Tests: DI registration tests under both flag values, options validation rejects missing key.

### Infra wiring

Update `infra/EnvironmentStack.cs`:
- Add three `config.Require[Secret]` reads near line 52.
- Add `apns-auth-key` to the API container app's `Secrets` array (line 246) plus every relevant worker job's secrets.
- Add the six `Apns__*` env vars to those containers' env arrays. Project `Apns__UseSandbox` based on the stack environment.
- Verify `CreateWorkerJob` shared env block is the right injection point so `digest`, `digest-hourly`, and any future dispatch workers all get the keys.
- Set the three pulumi config values for `dev` and `prod` stacks (the user runs `pulumi config set --secret town-crier:apnsAuthKey "$(cat AuthKey_XXXX.p8)"` etc.).

### Apple Developer Portal — generate APNs Auth Key

Operational. User logs into developer.apple.com, Certificates → Keys → "+", enables Apple Push Notifications service (APNs), downloads `AuthKey_<KEYID>.p8`. Captures Key ID (10 chars on the page), Team ID (top-right of portal). Hands the three values to whoever is running `pulumi config set`. Apple allows two active auth keys per team — useful for zero-downtime rotation later.

### TestFlight end-to-end verification

After deploy: trigger a real notification (either wait for PlanIt to surface a hit in a watched zone, or invoke `DispatchNotificationCommand` from an admin endpoint / one-off worker invocation). Confirm push lands on the TestFlight device. Then uninstall the app, trigger another notification, and confirm the device's token is removed from the `DeviceRegistrations` container in Cosmos (410 path). Watch App Insights `notifications.sent` and any APNs error logs.
