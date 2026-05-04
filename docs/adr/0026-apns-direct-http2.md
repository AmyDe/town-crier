# 0026. Direct APNs HTTP/2 push delivery over Notification Hubs

Date: 2026-05-04

## Status

Accepted

Refines [ADR 0009](0009-notification-delivery-architecture.md), which left the concrete push transport open. This ADR fills that gap for the iOS-only delivery path.

## Context

Push notifications are the core of the Town Crier product, but they don't work yet. The dispatch pipeline (PlanIt poll → `Notification` persisted → `DispatchNotificationCommandHandler` → `IPushNotificationSender.SendAsync`) is wired end-to-end except for the final hop: `town-crier.worker/Program.cs` binds `IPushNotificationSender` to `NoOpPushNotificationSender`, which returns `Task.CompletedTask`. iOS captures the device token and the API persists it via `RegisterDeviceTokenCommandHandler` into a Cosmos `DeviceRegistrations` container, so tokens flow in correctly — they're just never used.

Three transport options were on the table for replacing the no-op:

1. **Azure Notification Hubs** — Azure-managed pub/sub fan-out across APNs and FCM with tag-based addressing.
2. **Direct APNs HTTP/2 + JWT (ES256)** — single-protocol HTTP/2 client posting to `api.push.apple.com` with a process-cached provider token.
3. **Firebase Cloud Messaging as an APNs proxy** — Google-hosted relay, common on cross-platform stacks.

Today's product surface is iOS-only. There is no Android client, no web push, and no plan to add either inside the next product cycle. The dispatch path is also low-volume by design — one notification per matched application per registered device, capped by the polling cadence and watch-zone fan-out. This is not a 100k-RPS push problem.

A few project-specific constraints narrow the choice further:

- `dotnet-coding-standards` favours minimal dependencies and direct SDK calls over middleware libraries (cf. CQRS without MediatR, Cosmos DB SDK without an ORM).
- Native AOT compatibility rules out anything that relies on reflection or runtime code generation. Most third-party push SDKs assume reflection-based JSON.
- We already pay the operational cost of a `.p8` provider key for App Store Connect (TestFlight automation). Generating a second key for APNs reuses the same Apple Developer Portal flow.
- `IPushNotificationSender` is already a port; whichever transport we pick sits behind that seam, so a future swap is an adapter rewrite, not a domain change.

## Decision

Implement `ApnsPushNotificationSender` as a direct HTTP/2 client to APNs, signed with a process-cached ES256 JWT. No Notification Hubs, no FCM, no third-party push SDK.

Concretely:

- `System.Net.Http` configured for `HttpVersion.Version20` + `RequestVersionExact`, posting to `api.push.apple.com` (prod) or `api.sandbox.push.apple.com` (dev/TestFlight).
- `ApnsJwtProvider` mints one JWT per process, cached behind a `Lock`, regenerated at ~50 minutes (Apple expires at 60 and rate-limits regeneration to one per 20 minutes).
- ES256 signing via `ECDsa.ImportFromPem` + `SignData(..., HashAlgorithmName.SHA256)` — both AOT-clean.
- JSON payloads serialised via `[JsonSerializable]` source-generated contexts; no anonymous-object serialisation.
- Per-device request-per-token loop with a `SemaphoreSlim` parallelism cap.
- APNs status codes drive a small response-handling table: `400 BadDeviceToken` and `410 Unregistered` populate a new `PushSendResult.InvalidTokens` list which the dispatch handler uses to prune Cosmos via the existing `RemoveInvalidDeviceTokenCommandHandler`. `403 ExpiredProviderToken` invalidates the JWT cache and retries once. `5xx` exponentially backs off up to three attempts. `429 TooManyProviderTokenUpdates` logs and waits.
- Configuration lives under `Apns:*` (`Enabled`, `AuthKey`, `KeyId`, `TeamId`, `BundleId`, `UseSandbox`). DI registers `NoOpPushNotificationSender` when `Apns:Enabled = false` so local dev without a key still boots.
- Pulumi adds three secrets (`apnsAuthKey`, `apnsKeyId`, `apnsTeamId`) and projects them as `Apns__*` env vars onto every container that resolves `IPushNotificationSender` — the API web container plus the digest worker jobs. `Apns:UseSandbox` is `true` in dev, `false` in prod.

Notification Hubs is rejected. FCM is rejected. Both are revisitable if Android lands and we want unified fan-out — at that point the `IPushNotificationSender` seam absorbs the change and the work in this ADR becomes the iOS leg of a two-leg sender, not throwaway.

## Consequences

### Easier

- One Azure resource fewer. No Notification Hub namespace, no tag schema, no additional RBAC surface, no separate per-environment Pulumi component.
- Native AOT stays clean. Every dependency in the send path is in-box BCL (`HttpClient`, `ECDsa`, `System.Text.Json` source generators) — no SDK that pulls in reflection or `System.Reflection.Emit`.
- Failure semantics are debuggable. A 410 from APNs is one HTTP response away in App Insights; with Notification Hubs the same outcome is buried behind an async outcome callback.
- Token pruning is precise. The sender knows exactly which device APNs rejected and returns it in `PushSendResult.InvalidTokens`; the handler dispatches `RemoveInvalidDeviceTokenCommand` per token. No batch-fan-out outcome polling.
- Costs are essentially zero. APNs is free; the only cost is the ACA egress bytes per push.
- Provider-token model matches the App Store Connect `.p8` flow we already operate. Operationally it's "one more key in the developer portal."

### Harder

- Android (if/when) needs its own transport. FCM has its own JWT/HTTP v1 dance and its own error codes. The `IPushNotificationSender` port absorbs this — implementation will be a sibling adapter, not a rewrite of the dispatch handler — but it's strictly more code than a single Notification Hubs adapter would have been.
- Tag-based broadcast (e.g. "send to everyone subscribed to watch zone X") is not a primitive any more. Today's pipeline iterates explicit `DeviceRegistration` rows, so we don't need it; if it became a requirement we'd either build it on top of Cosmos queries or revisit Notification Hubs.
- The provider key is now a Town Crier-managed secret in Pulumi config and an ACA secret in every container that pushes. Rotation is a manual `pulumi config set --secret` plus a redeploy. Apple permits two active keys per team, so zero-downtime rotation is mechanically possible but operationally manual.
- We own the retry/backoff/parallelism/HTTP/2 connection-management code. Notification Hubs would have absorbed all of it. The code is small (the spec's table fits on one screen) but it is ours to maintain.

### Not addressed

- **Silent push (`apns-push-type: background`).** No use case yet. Adding it later is a payload variant on the existing sender, not a new path.
- **Rich media / notification service extensions on iOS.** Out of scope for this ADR; the iOS app would gain a Notification Service Extension target at the time it actually needs one.
- **Per-locale notification copy.** Existing domain-model copy is used as-is. Localisation is a domain-layer concern, not a transport concern.
- **Push delivery analytics.** `notifications.sent` already exists in App Insights. No new dashboards required. Per-device delivery analytics would need APNs feedback ingestion, which Apple does not offer outside the response code path the sender already handles.
- **FCM as an APNs proxy.** Rejected — adds a third-party dependency and a second JWT scheme for no upside on an iOS-only product.
- **Notification Hubs revisit trigger.** Not codified. The pragmatic trigger is "Android client lands and the iOS push code is mature enough that re-doing it via Hubs is cheaper than maintaining two adapters." Until then, leave it.
