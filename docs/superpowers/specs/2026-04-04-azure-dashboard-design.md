# Azure Operational Dashboard

Date: 2026-04-04

## Overview

Add an Azure Portal dashboard provisioned via Pulumi that visualises key Town Crier metrics from Application Insights. The dashboard provides a real-time operational view (default 24h) of user activity, watch zone usage, notification delivery, and sync health.

## Workstream 1 — New Custom Metrics

### New counters in `ApiMetrics.cs`

| Metric name | Counter name | Incremented in |
|-------------|-------------|----------------|
| Users registered | `towncrier.users.registered` | `CreateUserProfileCommandHandler` (on successful profile creation) |
| Watch zones deleted | `towncrier.watchzones.deleted` | `DeleteWatchZoneCommandHandler` (on successful deletion) |
| Searches performed | `towncrier.search.performed` | `SearchPlanningApplicationsQueryHandler` (on successful search) |
| Notifications created | `towncrier.notifications.created` | `DispatchNotificationCommandHandler` (on notification record creation, distinct from push delivery) |

### Existing counters (verify wiring)

| Metric | Counter name | Expected handler |
|--------|-------------|-----------------|
| Watch zones created | `towncrier.watchzones.created` | `CreateWatchZoneCommandHandler` |
| Notifications sent | `towncrier.notifications.sent` | `DispatchNotificationCommandHandler` (push delivery path) |

### Active users (no code change)

Active users are derived from App Insights request telemetry via KQL — count distinct authenticated users hitting `GET /v1/me` per hour. No new counter needed.

## Workstream 2 — Pulumi Dashboard Resource

### Resource

`Azure.Native.Portal.Dashboard` added to `SharedStack.cs`, referencing the existing Application Insights resource.

Dashboard name: `towncrier-operational`

### Layout — 3 rows, 10 tiles

**Row 1 — Users & Engagement** (line charts)

| Tile | Source |
|------|--------|
| Active Users | KQL: `requests \| where name == "GET /v1/me" \| summarize dcount(user_AuthenticatedId) by bin(timestamp, 1h) \| render timechart` |
| Registrations | Custom metric: `towncrier.users.registered` |
| Searches | Custom metric: `towncrier.search.performed` |

**Row 2 — Watch Zones & Notifications** (line charts)

| Tile | Source |
|------|--------|
| Watch Zones Created | Custom metric: `towncrier.watchzones.created` |
| Watch Zones Deleted | Custom metric: `towncrier.watchzones.deleted` |
| Notifications Sent | Custom metric: `towncrier.notifications.sent` |

**Row 3 — Sync & Infrastructure Health** (line charts)

| Tile | Source |
|------|--------|
| Sync Success vs Failure | Stacked: `towncrier.polling.authorities_polled` vs `towncrier.polling.failures` |
| Applications Ingested | Custom metric: `towncrier.polling.applications_ingested` |
| Cosmos RU Consumption | Custom metric: `towncrier.cosmos.request_charge` |
| API Errors | Custom metric: `towncrier.api.errors` |

### Tile implementation

- **Custom metric tiles**: `MetricsExplorerBladePinnedPart` referencing the App Insights resource ID and metric name
- **KQL tile (Active Users)**: `LogsBladePinnedPart` with the KQL query above
- **Time range**: 24 hours default, user-adjustable in the portal

## Scope boundaries

- No new API endpoints
- No new dependencies
- No behavioural changes to existing functionality
- No second dashboard (business/monthly view) — can be added later
- `notifications.created` is distinct from `notifications.sent` — created tracks record creation, sent tracks push delivery
