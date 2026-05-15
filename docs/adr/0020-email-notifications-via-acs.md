# 0020. Email Notifications via Azure Communication Services

Date: 2026-04-04

## Status

Accepted

## Context

Town Crier needs email as a notification channel — weekly digest emails for all subscription tiers and instant notification emails for Personal/Pro users. The memo (0002) analysed cost and vendor options.

## Decision

Use Azure Communication Services (ACS) Email with the `towncrierapp.uk` custom domain. ACS is provisioned via Pulumi alongside existing Azure resources — no new vendor. The sender address is `hello@towncrierapp.uk`.

The implementation extends the existing hexagonal notification architecture:
- New `IEmailSender` port with `AcsEmailSender` adapter
- `GenerateWeeklyDigestsCommandHandler` sends email digests to all tiers (grouped by watch zone, card-per-application layout)
- `DispatchNotificationCommandHandler` sends instant emails to Personal/Pro users with `EmailInstantEnabled`
- Email preferences (`EmailDigestEnabled`, `EmailInstantEnabled`) added to `NotificationPreferences`
- Email address read from user profile in Cosmos (set during onboarding)

Weekly digests are triggered by a daily Container Apps Job (shared with the future cron infrastructure). Instant emails fire inline with the existing change feed notification path.

## Consequences

- Near-zero marginal cost (~$0.75/month at 4K emails)
- Email becomes the primary engagement channel for free-tier users
- Custom domain requires DNS verification records in Cloudflare (SPF, DKIM, DMARC)
- ACS SDK dependency added to infrastructure layer — must verify Native AOT compatibility
- If ACS SDK has AOT issues, fallback to direct REST API

## Amendments

### 2026-05-15
- Corrected: the original Decision section says instant emails "fire inline with the existing change feed notification path." There is no change-feed processor (see [ADR 0009](0009-notification-delivery-architecture.md) 2026-04-21 amendment). Instant emails fire inline within the polling worker via `DispatchNotificationCommandHandler`.
- Added: **Hourly digest** as a paid-tier email cadence. The original ADR only contemplated weekly digests for all tiers; an intermediate hourly cadence has since been introduced as a tier-gated feature.
  - New handler: `GenerateHourlyDigestsCommandHandler` in the application layer.
  - New Container Apps Job: `digest-hourly`, cron `0 * * * *` (top of every hour, both dev and prod), `WORKER_MODE=hourly-digest`, 300s replica timeout. Wired in `infra/EnvironmentStack.cs` alongside the existing daily `digest` job (`0 7 * * *`).
  - New entitlement: `Entitlement.HourlyDigestEmails`. Mapped to **Personal** and **Pro** tiers only; **Free** tier is explicitly excluded (see `EntitlementMap` and its tests). Free-tier users continue to receive the weekly digest only.
  - Worker dispatcher in `town-crier.worker/Program.cs` adds `hourly-digest` to the `WORKER_MODE` switch (alongside `poll-sb`, `poll-bootstrap`, `digest`, `dormant-cleanup`); unknown modes fail fast.
- Cost impact: hourly cadence multiplies email volume for Personal/Pro digest subscribers by up to ~24x compared to weekly, but absolute volume is still well under ACS's free tier. No change to the SDK or wiring.
