# 0009. Notification Delivery Architecture

Date: 2026-03-16

## Status

Accepted

## Context

Town Crier's core value proposition is push notifications when new planning applications appear near a user's watch zone. The system needs a mechanism to: detect new/changed applications after ingestion, match them against active watch zones, enforce per-tier notification limits, and dispatch APNs push notifications.

Two approaches were considered:

1. **Azure Queue Storage**: polling service enqueues application IDs after upsert, a separate consumer reads the queue and dispatches notifications. Adds infrastructure but provides explicit retry/dead-letter semantics.
2. **Cosmos DB Change Feed**: a change feed processor on the Applications container fires on every upsert, performs spatial matching, and dispatches notifications. No additional infrastructure — built into Cosmos DB with checkpointing and ordering guarantees.

## Decision

### Change Feed Processor

Use the **Cosmos DB change feed** on the Applications container as the trigger for notification dispatch. The processor runs as a background hosted service in the same Container App as the API.

**Flow:**

1. PlanIt polling service upserts applications into the Applications container
2. Change feed processor receives batches of changed documents
3. For each application, execute a spatial query against the WatchZones container: find all zones where ST_DISTANCE(application.location, zone.centre) <= zone.radius
4. For each matched zone, load the owning user's profile to check:
   - Subscription tier and entitlements
   - Notification preferences for this zone
   - Monthly notification count against free-tier cap
5. Create a Notification document in the Notifications container
6. Send APNs push notification

**Checkpointing:** A dedicated `Leases` container in the same Cosmos DB database stores change feed processor lease state. On failure, the processor resumes from the last checkpoint — no messages are lost.

**Failed notifications:** If APNs dispatch fails after retries, write to a `FailedNotifications` container for manual inspection rather than silently dropping.

### Hosting Model

The change feed processor runs as an `IHostedService` in the **same Container App as the API**. This keeps the deployment model simple (single container image, single revision) while the processor runs independently on a background thread.

If the notification workload grows to the point where it impacts API latency, the processor can be extracted into a separate Container App without architectural changes — it only depends on Cosmos DB and APNs, not on the API's HTTP pipeline.

### Free-Tier Notification Cap

- Free users receive a maximum of **5 notifications per calendar month**
- The counter resets on the 1st of each month at 00:00 UTC
- Tracked by counting Notification documents for the user within the current month's date range
- When the cap is reached, matched applications are still recorded (visible in-app if the user opens the app) but no push notification is sent
- Upgrade prompt included in the final (5th) notification: "You've reached your monthly limit — upgrade to Personal for unlimited notifications"

### APNs Token Lifecycle

- Device tokens stored on the User document as an array (supports multiple devices)
- Token re-registered on every app launch to capture rotation
- APNs feedback responses processed to remove invalid/expired tokens
- Batch APNs calls where multiple users match the same application

### Notification Content

Notifications include:
- Application address (primary line)
- Application type and description (secondary line)
- Authority code and application ID in the payload (for deep link routing to the detail screen)

## Consequences

- **No additional infrastructure** beyond Cosmos DB. No queues, no service bus, no extra billing. The change feed is included in Cosmos DB RU consumption.
- **Ordering guarantees** within a partition (authority code). Applications from the same authority are processed in order. Cross-authority ordering is not guaranteed but is not required.
- **Retry semantics are implicit** — a failed batch is re-processed from the last checkpoint. This is simpler than queue-based retry but means a single poison document could block the processor. Mitigation: try/catch per document within a batch, log and skip failures after retries.
- **Scaling ceiling**: a single change feed processor instance handles all notifications. At significant scale, the processor can be partitioned by lease assignment or extracted to a separate Container App. Not a concern for early growth.
- **Free-tier cap is calendar-month based**, which is simpler to implement and reason about than rolling windows. Users who sign up mid-month get a partial first month — acceptable since it's the free tier.
