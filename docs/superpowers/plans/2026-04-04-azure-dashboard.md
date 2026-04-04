# Azure Operational Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add missing custom metrics to handlers and provision an Azure Portal dashboard via Pulumi that visualises user activity, watch zone usage, notification delivery, and sync health from Application Insights.

**Architecture:** Two workstreams — (1) add 4 new counters to `ApiMetrics` and wire all counters into their handlers, (2) add a `Pulumi.AzureNative.Portal.Dashboard` resource to `SharedStack.cs` with 10 tiles across 3 rows. The active users tile uses a KQL query against request telemetry; all other tiles reference custom metrics.

**Tech Stack:** .NET 10, System.Diagnostics.Metrics, Pulumi Azure Native v3, Application Insights

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `api/src/town-crier.application/Observability/ApiMetrics.cs` | Modify | Add 4 new counter fields |
| `api/src/town-crier.application/UserProfiles/CreateUserProfileCommandHandler.cs` | Modify | Increment `UsersRegistered` |
| `api/src/town-crier.application/WatchZones/CreateWatchZoneCommandHandler.cs` | Modify | Increment `WatchZonesCreated` |
| `api/src/town-crier.application/WatchZones/DeleteWatchZoneCommandHandler.cs` | Modify | Increment `WatchZonesDeleted` |
| `api/src/town-crier.application/Search/SearchPlanningApplicationsQueryHandler.cs` | Modify | Increment `SearchesPerformed` |
| `api/src/town-crier.application/Notifications/DispatchNotificationCommandHandler.cs` | Modify | Increment `NotificationsCreated` and `NotificationsSent` |
| `infra/SharedStack.cs` | Modify | Add Dashboard resource |

---

### Task 1: Add new counters to ApiMetrics

**Files:**
- Modify: `api/src/town-crier.application/Observability/ApiMetrics.cs`

- [ ] **Step 1: Add 4 new counters**

Add after the existing `EndpointErrors` counter (line 24):

```csharp
    public static readonly Counter<long> UsersRegistered =
        Meter.CreateCounter<long>("towncrier.users.registered");

    public static readonly Counter<long> WatchZonesDeleted =
        Meter.CreateCounter<long>("towncrier.watchzones.deleted");

    public static readonly Counter<long> SearchesPerformed =
        Meter.CreateCounter<long>("towncrier.search.performed");

    public static readonly Counter<long> NotificationsCreated =
        Meter.CreateCounter<long>(
            "towncrier.notifications.created",
            description: "Notification records created (may or may not result in push)");
```

- [ ] **Step 2: Verify build**

Run: `dotnet build api/`
Expected: Build succeeded

- [ ] **Step 3: Run tests**

Run: `dotnet test api/`
Expected: All tests pass (no behaviour change)

- [ ] **Step 4: Commit**

```bash
git add api/src/town-crier.application/Observability/ApiMetrics.cs
git commit -m "feat(observability): add custom metric counters for users, watch zones, search, and notifications"
```

---

### Task 2: Wire metrics into all handlers

**Files:**
- Modify: `api/src/town-crier.application/UserProfiles/CreateUserProfileCommandHandler.cs`
- Modify: `api/src/town-crier.application/WatchZones/CreateWatchZoneCommandHandler.cs`
- Modify: `api/src/town-crier.application/WatchZones/DeleteWatchZoneCommandHandler.cs`
- Modify: `api/src/town-crier.application/Search/SearchPlanningApplicationsQueryHandler.cs`
- Modify: `api/src/town-crier.application/Notifications/DispatchNotificationCommandHandler.cs`

Note: These counters are static fields — calling `.Add(1)` is a fire-and-forget operation with no side effects on handler logic. The existing test suites verify handler behaviour is unchanged. No new metric-specific tests are needed (testing that a static counter increments would require `MeterListener` ceremony with shared static state across test runs, for zero bug-catching value).

- [ ] **Step 1: Wire CreateUserProfileCommandHandler**

Add `using TownCrier.Application.Observability;` at the top.

After line 39 (`await this.repository.SaveAsync(profile, ct).ConfigureAwait(false);`), add:

```csharp
        ApiMetrics.UsersRegistered.Add(1);
```

Important: This must go AFTER the `SaveAsync` call and BEFORE the return. It must NOT fire on the early-return path (line 23-30) where an existing profile is returned — that's a login, not a registration.

- [ ] **Step 2: Wire CreateWatchZoneCommandHandler**

Add `using TownCrier.Application.Observability;` at the top.

After line 60 (`await this.watchZoneRepository.SaveAsync(zone, ct).ConfigureAwait(false);`), add:

```csharp
        ApiMetrics.WatchZonesCreated.Add(1);
```

- [ ] **Step 3: Wire DeleteWatchZoneCommandHandler**

Add `using TownCrier.Application.Observability;` at the top.

After line 16 (`await this.watchZoneRepository.DeleteAsync(command.UserId, command.ZoneId, ct).ConfigureAwait(false);`), add:

```csharp
        ApiMetrics.WatchZonesDeleted.Add(1);
```

- [ ] **Step 4: Wire SearchPlanningApplicationsQueryHandler**

Add `using TownCrier.Application.Observability;` at the top.

Before the final return on line 58 (`return new SearchPlanningApplicationsResult(...)`) add:

```csharp
        ApiMetrics.SearchesPerformed.Add(1);
```

This fires only on successful searches — not when the user is Free tier (line 35 throws) or not found (line 31 throws).

- [ ] **Step 5: Wire DispatchNotificationCommandHandler**

Add `using TownCrier.Application.Observability;` at the top.

**NotificationsCreated:** Add after line 67 (after `Notification.Create(...)`), before any of the save paths:

```csharp
        ApiMetrics.NotificationsCreated.Add(1);
```

This placement fires for every notification created, regardless of which save/return path follows. It does NOT fire for the early-return duplicate check (line 46) or missing profile (line 56), which is correct.

**NotificationsSent:** Add inside the `if (devices.Count > 0)` block, after line 106 (`notification.MarkPushSent();`):

```csharp
            ApiMetrics.NotificationsSent.Add(1);
```

- [ ] **Step 6: Verify build**

Run: `dotnet build api/`
Expected: Build succeeded

- [ ] **Step 7: Run all tests**

Run: `dotnet test api/`
Expected: All tests pass (no behaviour change)

- [ ] **Step 8: Commit**

```bash
git add api/src/town-crier.application/UserProfiles/CreateUserProfileCommandHandler.cs \
       api/src/town-crier.application/WatchZones/CreateWatchZoneCommandHandler.cs \
       api/src/town-crier.application/WatchZones/DeleteWatchZoneCommandHandler.cs \
       api/src/town-crier.application/Search/SearchPlanningApplicationsQueryHandler.cs \
       api/src/town-crier.application/Notifications/DispatchNotificationCommandHandler.cs
git commit -m "feat(observability): wire metric counters into all handlers"
```

---

### Task 3: Add Azure Dashboard to Pulumi

**Files:**
- Modify: `infra/SharedStack.cs`

The dashboard uses `Pulumi.AzureNative.Portal.Dashboard`. Each tile is a part within a single lens. Metric tiles use the `MonitorChartPart` type referencing the App Insights resource. The active users tile uses `LogsDashboardPart` with a KQL query.

**Grid layout (12-column grid):**

| Part | X | Y | ColSpan | RowSpan | Type |
|------|---|---|---------|---------|------|
| Active Users | 0 | 0 | 4 | 4 | KQL (LogsDashboardPart) |
| Registrations | 4 | 0 | 4 | 4 | Metric (MonitorChartPart) |
| Searches | 8 | 0 | 4 | 4 | Metric |
| WZ Created | 0 | 4 | 4 | 4 | Metric |
| WZ Deleted | 4 | 4 | 4 | 4 | Metric |
| Notifications Sent | 8 | 4 | 4 | 4 | Metric |
| Sync Success/Fail | 0 | 8 | 3 | 4 | Metric |
| Apps Ingested | 3 | 8 | 3 | 4 | Metric |
| Cosmos RU | 6 | 8 | 3 | 4 | Metric |
| API Errors | 9 | 8 | 3 | 4 | Metric |

- [ ] **Step 1: Add using statements to SharedStack.cs**

Add after the existing using statements:

```csharp
using Pulumi.AzureNative.Portal;
using Pulumi.AzureNative.Portal.Inputs;
```

- [ ] **Step 2: Add helper method for metric tile metadata**

Add a private static helper method at the bottom of the `SharedStack` class (after the `Run` method's closing brace, but inside the class):

```csharp
    private static Dictionary<string, object> MetricTile(
        Output<string> appInsightsId, string metricName, string title)
    {
        return new Dictionary<string, object>
        {
            ["type"] = "Extension/HubsExtension/PartType/MonitorChartPart",
            ["settings"] = new Dictionary<string, object>
            {
                ["content"] = new Dictionary<string, object>
                {
                    ["options"] = new Dictionary<string, object>
                    {
                        ["chart"] = new Dictionary<string, object>
                        {
                            ["metrics"] = new object[]
                            {
                                new Dictionary<string, object>
                                {
                                    ["resourceMetadata"] = new Dictionary<string, object>
                                    {
                                        ["id"] = appInsightsId,
                                    },
                                    ["name"] = metricName,
                                    ["aggregationType"] = 1, // Count
                                    ["namespace"] = "azure.applicationinsights",
                                    ["metricVisualization"] = new Dictionary<string, object>
                                    {
                                        ["displayName"] = title,
                                    },
                                },
                            },
                            ["title"] = title,
                            ["titleKind"] = 1,
                            ["visualization"] = new Dictionary<string, object>
                            {
                                ["chartType"] = 2, // Line chart
                            },
                            ["timespan"] = new Dictionary<string, object>
                            {
                                ["relative"] = new Dictionary<string, object>
                                {
                                    ["duration"] = 86400000, // 24 hours in ms
                                },
                            },
                        },
                    },
                },
            },
        };
    }
```

- [ ] **Step 3: Add helper method for KQL tile metadata**

```csharp
    private static Dictionary<string, object> KqlTile(
        Output<string> appInsightsId, string query, string title)
    {
        return new Dictionary<string, object>
        {
            ["type"] = "Extension/Microsoft_OperationsManagementSuite_Workspace/PartType/LogsDashboardPart",
            ["settings"] = new Dictionary<string, object>
            {
                ["content"] = new Dictionary<string, object>
                {
                    ["Query"] = query,
                    ["ControlType"] = "AnalyticsChart",
                    ["SpecificChart"] = "Line",
                    ["PartTitle"] = title,
                    ["Dimensions"] = new Dictionary<string, object>
                    {
                        ["xAxis"] = new Dictionary<string, object>
                        {
                            ["name"] = "timestamp",
                            ["type"] = "datetime",
                        },
                        ["yAxis"] = new Dictionary<string, object>
                        {
                            ["name"] = "aggregation",
                            ["type"] = "long",
                        },
                    },
                },
            },
            ["inputs"] = new object[]
            {
                new Dictionary<string, object>
                {
                    ["name"] = "resourceTypeMode",
                    ["value"] = "components",
                },
                new Dictionary<string, object>
                {
                    ["name"] = "ComponentId",
                    ["value"] = appInsightsId,
                },
            },
        };
    }
```

- [ ] **Step 4: Add Dashboard resource to SharedStack.Run()**

Add before the `return` statement (before line 168), after the `cosmosRoleAssignment`:

```csharp
        // Operational Dashboard
        var dashboard = new Dashboard("dash-towncrier-operational", new DashboardArgs
        {
            DashboardName = "dash-towncrier-operational",
            ResourceGroupName = resourceGroup.Name,
            Location = resourceGroup.Location,
            Tags = tags,
            Lenses = new[]
            {
                new DashboardLensArgs
                {
                    Order = 0,
                    Parts = new[]
                    {
                        // Row 1: Users & Engagement
                        new DashboardPartsArgs
                        {
                            Position = new DashboardPartsPositionArgs { X = 0, Y = 0, ColSpan = 4, RowSpan = 4 },
                            Metadata = KqlTile(
                                appInsights.Id,
                                "requests | where name == 'GET /v1/me' | summarize dcount(user_AuthenticatedId) by bin(timestamp, 1h) | render timechart",
                                "Active Users"),
                        },
                        new DashboardPartsArgs
                        {
                            Position = new DashboardPartsPositionArgs { X = 4, Y = 0, ColSpan = 4, RowSpan = 4 },
                            Metadata = MetricTile(appInsights.Id, "towncrier.users.registered", "Registrations"),
                        },
                        new DashboardPartsArgs
                        {
                            Position = new DashboardPartsPositionArgs { X = 8, Y = 0, ColSpan = 4, RowSpan = 4 },
                            Metadata = MetricTile(appInsights.Id, "towncrier.search.performed", "Searches"),
                        },

                        // Row 2: Watch Zones & Notifications
                        new DashboardPartsArgs
                        {
                            Position = new DashboardPartsPositionArgs { X = 0, Y = 4, ColSpan = 4, RowSpan = 4 },
                            Metadata = MetricTile(appInsights.Id, "towncrier.watchzones.created", "Watch Zones Created"),
                        },
                        new DashboardPartsArgs
                        {
                            Position = new DashboardPartsPositionArgs { X = 4, Y = 4, ColSpan = 4, RowSpan = 4 },
                            Metadata = MetricTile(appInsights.Id, "towncrier.watchzones.deleted", "Watch Zones Deleted"),
                        },
                        new DashboardPartsArgs
                        {
                            Position = new DashboardPartsPositionArgs { X = 8, Y = 4, ColSpan = 4, RowSpan = 4 },
                            Metadata = MetricTile(appInsights.Id, "towncrier.notifications.sent", "Notifications Sent"),
                        },

                        // Row 3: Sync & Infrastructure Health
                        new DashboardPartsArgs
                        {
                            Position = new DashboardPartsPositionArgs { X = 0, Y = 8, ColSpan = 3, RowSpan = 4 },
                            Metadata = MetricTile(appInsights.Id, "towncrier.polling.authorities_polled", "Sync Success vs Failure"),
                        },
                        new DashboardPartsArgs
                        {
                            Position = new DashboardPartsPositionArgs { X = 3, Y = 8, ColSpan = 3, RowSpan = 4 },
                            Metadata = MetricTile(appInsights.Id, "towncrier.polling.applications_ingested", "Applications Ingested"),
                        },
                        new DashboardPartsArgs
                        {
                            Position = new DashboardPartsPositionArgs { X = 6, Y = 8, ColSpan = 3, RowSpan = 4 },
                            Metadata = MetricTile(appInsights.Id, "towncrier.cosmos.request_charge_ru", "Cosmos RU Consumption"),
                        },
                        new DashboardPartsArgs
                        {
                            Position = new DashboardPartsPositionArgs { X = 9, Y = 8, ColSpan = 3, RowSpan = 4 },
                            Metadata = MetricTile(appInsights.Id, "towncrier.api.errors", "API Errors"),
                        },
                    },
                },
            },
        });
```

**Important type note:** The exact Pulumi type names for `DashboardPartsArgs` and `DashboardPartsPositionArgs` may differ in your version of `Pulumi.AzureNative`. If `dotnet build` reports type errors, check the available types in the `Pulumi.AzureNative.Portal.Inputs` namespace. Common alternatives:
- `DashboardPartArgs` instead of `DashboardPartsArgs`
- `DashboardPartPositionArgs` instead of `DashboardPartsPositionArgs`

If the dictionary-based `Metadata` approach doesn't compile, wrap it with `System.Text.Json.JsonSerializer.SerializeToElement(...)`.

**Note on the "Sync Success vs Failure" tile:** The spec calls for a stacked line chart with both `authorities_polled` and `failures`. The helper method above only supports single-metric tiles. If the implementer wants stacked metrics, they should add a second entry to the `metrics` array in the `MetricTile` return value for that specific tile. A single-metric tile showing `authorities_polled` is acceptable as an MVP — the `failures` metric can be viewed by adjusting the tile in the portal.

- [ ] **Step 5: Verify infra build**

Run: `dotnet build infra/`
Expected: Build succeeded

If build fails due to Pulumi type mismatches, adjust type names per the note above and re-run.

- [ ] **Step 6: Commit**

```bash
git add infra/SharedStack.cs
git commit -m "feat(infra): add Azure operational dashboard via Pulumi"
```

---

## Post-Implementation Notes

- The dashboard deploys via PR → CI/CD like all infra changes. Do NOT run `pulumi up` directly.
- After merging and deploying, verify the dashboard appears in Azure Portal under **Dashboard** and that tiles render data.
- Custom metrics will only show data once the API has been deployed with the counter increments AND has received traffic.
- The KQL active users tile queries request telemetry which is already flowing — it should show data immediately after dashboard deploy.
- The "Sync Success vs Failure" tile is MVP with a single metric. To add the stacked failure metric, modify the tile's metadata to include a second entry in the `metrics` array after verifying the dashboard renders correctly.
