# 0023. Dormant Account Cleanup for UK GDPR Storage Limitation

Date: 2026-04-21

## Status

Accepted

## Context

UK GDPR Article 5(1)(e) — the **storage limitation** principle — requires personal data to be kept "for no longer than is necessary for the purposes for which the personal data are processed." Town Crier collects email, push-token, watch-zone geometry, and onboarding answers. When a user stops using the app entirely, retaining that data indefinitely is not lawful without a justification.

The platform also collects telemetry and usage data that accumulates whether or not the account is active. A user who installs the app once, grants a watch zone, and never returns has a live profile in Cosmos, a device-registration document, possibly saved applications, and an email address that will keep receiving digests forever unless we take action.

The Privacy Policy served by `GetLegalDocumentQueryHandler` commits to retaining personal data only as long as necessary to provide the service, and deleting it once the account is dormant. That promise was previously unimplemented, which is both a lawful-basis gap and a trust gap.

## Decision

Introduce a **dormant account cleanup** job that periodically identifies accounts with no activity for 12+ months and erases the user's personal data in a cascading delete.

### Activity signal

Every authenticated request flows through `RecordUserActivityMiddleware`, which writes `LastActiveAt` onto the user profile document. Any authenticated API call counts as activity — reading a digest email link does not, because digest links point to the iOS app / web bundle, which in turn hits the API.

### Cleanup job

`DormantAccountCleanupCommandHandler` runs as a Container Apps Job (worker mode `dormant-cleanup`) on a daily cron (`30 3 * * *`, 03:30 UTC). Steps:

1. Query `Users` for profiles where `LastActiveAt` is older than 12 months (configurable).
2. For each dormant user, delete in order:
   - `Notifications` (partitioned by `userId`)
   - `DecisionAlerts` (partitioned by `userId`)
   - `SavedApplications` (partitioned by `userId`)
   - `DeviceRegistrations` (partitioned by `userId`)
   - `WatchZones` (partitioned by `userId`)
   - `Users` (the profile itself)
3. Log the deletion with the user id redacted, and increment a cleanup counter metric.

Auth0 account deletion is triggered via the existing Auth0 M2M management client, which also has a `NoOpAuth0ManagementClient` fallback for environments where the M2M credentials are not configured.

Offer-code redemptions survive the cleanup as **anonymised** records — `OfferCodes` documents are kept with the redeeming user id cleared, so offer-code audit data (how many were redeemed, when, by batch) remains but the link to the deleted user is severed.

### Re-activation window

If a user returns between the last-active timestamp and the cleanup run, `LastActiveAt` is updated and they are no longer eligible for deletion. There is no soft-delete / grace period beyond that — once the cleanup job has deleted, the account is gone. Re-registration creates a fresh profile.

### Admin override

Deletion is reversible only from Auth0 logs (if M2M was wired) and Cosmos backup. There is no "restore my old account" flow. This is intentional — GDPR erasure is meant to be permanent.

## Consequences

### Easier

- Privacy Policy commitment on dormant data is implemented, not just asserted.
- Storage footprint shrinks over time instead of growing linearly with total historical signups.
- GDPR data-protection impact assessments have a concrete retention rule to cite.
- Reduces blast radius of a future data breach — a leak cannot expose data for users who stopped using the app a year ago.

### Harder

- Deletion is cascading across seven containers. Partial failure mid-cascade leaves orphan documents. Mitigation: each delete is idempotent (404-tolerant), and the cleanup job re-scans for the same user on the next run and finishes the cascade.
- Users who uninstall and return after 12+ months re-onboard from scratch. This is acceptable — the alternative is indefinite retention.
- The 12-month threshold is a policy choice, not a legal requirement. If the retention promise in the Privacy Policy changes, the threshold has to move with it.
- Email is the first signal a user notices — a user whose account is about to be deleted does not currently receive a warning digest. A future enhancement could send a "we're going to delete your account in 14 days" email at the 11.5-month mark; out of scope for the initial implementation.

## See also

- [ADR 0008 — Cosmos DB data model](0008-cosmos-db-data-model.md) — the containers cleared on cascade
- [ADR 0019 — Extract polling to Container Apps Job](0019-extract-polling-to-container-apps-job.md) — the worker + cron infrastructure this job reuses
