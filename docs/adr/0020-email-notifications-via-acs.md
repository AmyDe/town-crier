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
