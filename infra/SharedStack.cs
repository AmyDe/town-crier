using Pulumi;
using Pulumi.AzureNative.Resources;
using Pulumi.AzureNative.ContainerRegistry;
using Pulumi.AzureNative.ManagedIdentity;
using Pulumi.AzureNative.Authorization;
using Pulumi.AzureNative.OperationalInsights;
using Pulumi.AzureNative.App;
using Pulumi.AzureNative.App.Inputs;
using Pulumi.AzureNative.CosmosDB;
using Pulumi.AzureNative.CosmosDB.Inputs;
using Pulumi.AzureNative.ApplicationInsights;
using Pulumi.AzureNative.Monitor;
using Pulumi.AzureNative.Monitor.Inputs;
using Pulumi.AzureNative.Portal;
using Pulumi.AzureNative.Portal.Inputs;
using Pulumi.AzureNative.Communication;
using Pulumi.AzureNative.Communication.Inputs;
using Pulumi.AzureNative.Consumption;
using Pulumi.AzureNative.Consumption.Inputs;

public static class SharedStack
{
    private const int DashboardTimespan24HoursMs = 86400000;

    public static Dictionary<string, object?> Run(Config config, InputMap<string> tags)
    {
        var ciServicePrincipalId = config.Require("ciServicePrincipalId");

        // Resource Group
        var resourceGroup = new ResourceGroup("rg-town-crier-shared", new ResourceGroupArgs
        {
            ResourceGroupName = "rg-town-crier-shared",
            Tags = tags,
        });

        // Azure Container Registry (shared across environments).
        //
        // RetentionPolicy is Premium-SKU only — Azure rejects any Policies block on Basic
        // ("Policies are only supported for managed registries in Premium SKU"), so
        // untagged-manifest cleanup must happen out-of-band. See tc-dq46.
        var containerRegistry = new Registry("acrtowncriershared", new RegistryArgs
        {
            RegistryName = "acrtowncriershared",
            ResourceGroupName = resourceGroup.Name,
            Sku = new Pulumi.AzureNative.ContainerRegistry.Inputs.SkuArgs
            {
                Name = SkuName.Basic,
            },
            AdminUserEnabled = false,
            Tags = tags,
        });

        // User-assigned managed identity for AcrPull
        var acrPullIdentity = new UserAssignedIdentity("id-town-crier-acr-pull", new UserAssignedIdentityArgs
        {
            ResourceName = "id-town-crier-acr-pull",
            ResourceGroupName = resourceGroup.Name,
            Tags = tags,
        });

        // Extract subscription ID from the ACR's resource ID
        // ACR ID format: /subscriptions/{subId}/resourceGroups/{rg}/providers/Microsoft.ContainerRegistry/registries/{name}
        var subscriptionId = containerRegistry.Id.Apply(id => id.Split('/')[2]);

        // AcrPull role assignment — managed identity can pull images from the ACR
        var acrPullRoleAssignment = new RoleAssignment("acr-pull-role", new RoleAssignmentArgs
        {
            Scope = containerRegistry.Id,
            RoleDefinitionId = subscriptionId.Apply(subId =>
                $"/subscriptions/{subId}/providers/Microsoft.Authorization/roleDefinitions/7f951dda-4ed3-4680-a7ca-43fe172d538d"),
            PrincipalId = acrPullIdentity.PrincipalId,
            PrincipalType = PrincipalType.ServicePrincipal,
        });

        // AcrPush role assignment — CI service principal can push images to the ACR
        var acrPushRoleAssignment = new RoleAssignment("acr-push-role", new RoleAssignmentArgs
        {
            Scope = containerRegistry.Id,
            RoleDefinitionId = subscriptionId.Apply(subId =>
                $"/subscriptions/{subId}/providers/Microsoft.Authorization/roleDefinitions/8311e382-0749-4cb8-b61a-304f252e45ec"),
            PrincipalId = ciServicePrincipalId,
            PrincipalType = PrincipalType.ServicePrincipal,
        });

        // Log Analytics Workspace (shared across environments)
        var logAnalytics = new Workspace("log-town-crier-shared", new WorkspaceArgs
        {
            WorkspaceName = "log-town-crier-shared",
            ResourceGroupName = resourceGroup.Name,
            Sku = new Pulumi.AzureNative.OperationalInsights.Inputs.WorkspaceSkuArgs
            {
                Name = WorkspaceSkuNameEnum.PerGB2018,
            },
            RetentionInDays = 30, // PerGB2018 SKU minimum is 30 days
            WorkspaceCapping = new Pulumi.AzureNative.OperationalInsights.Inputs.WorkspaceCappingArgs
            {
                DailyQuotaGb = 1.0,
            },
            Tags = tags,
        });

        // Application Insights (shared, backed by Log Analytics)
        var appInsights = new Component("appi-town-crier-shared", new ComponentArgs
        {
            ResourceName = "appi-town-crier-shared",
            ResourceGroupName = resourceGroup.Name,
            WorkspaceResourceId = logAnalytics.Id,
            ApplicationType = "web",
            Kind = "web",
            IngestionMode = IngestionMode.LogAnalytics,
            RetentionInDays = 30, // Minimum allowed by Azure (30, 60, 90, 120, etc.)
            Tags = tags,
        });

        // Container Apps Environment (shared across environments).
        // Destination "azure-monitor" routes logs through a Diagnostic Setting (DCR-based)
        // into the native ContainerAppConsoleLogs table — not the legacy ContainerAppConsoleLogs_CL
        // Classic Custom Log table written by the "log-analytics" destination. The DCR-backed
        // native table can then be moved to the Basic Logs plan (~80% cheaper than Analytics).
        var containerAppsEnv = new ManagedEnvironment("cae-town-crier-shared", new ManagedEnvironmentArgs
        {
            EnvironmentName = "cae-town-crier-shared",
            ResourceGroupName = resourceGroup.Name,
            AppLogsConfiguration = new AppLogsConfigurationArgs
            {
                Destination = "azure-monitor",
            },
            Tags = tags,
        });

        // Diagnostic Setting routes Container Apps Environment logs to the shared Log Analytics
        // workspace via the platform's DCR pipeline. The "allLogs" category group includes
        // ContainerAppConsoleLogs, ContainerAppSystemLogs, and any future categories Azure adds.
        _ = new DiagnosticSetting("diag-cae-town-crier-shared", new DiagnosticSettingArgs
        {
            Name = "diag-cae-town-crier-shared",
            ResourceUri = containerAppsEnv.Id,
            WorkspaceId = logAnalytics.Id,
            Logs = new[]
            {
                new LogSettingsArgs
                {
                    CategoryGroup = "allLogs",
                    Enabled = true,
                },
            },
        });

        // Set the native ContainerAppConsoleLogs table to the Basic plan.
        // Basic Logs is ~$0.50/GB vs Analytics ~$2.76/GB but limits queries to 8-day retention,
        // disallows summarize/joins, and charges $0.005/GB scanned on search. Acceptable for
        // application stdout/stderr where we mostly tail-read recent rows.
        //
        // Azure pre-creates this table's schema in the workspace when ManagedEnvironment
        // destination=azure-monitor is applied, so the native table already exists on the
        // default (Analytics) plan before Pulumi reaches this resource. Declaring the
        // resource with ImportId below causes the first `pulumi up` to adopt the existing
        // table into Pulumi state and then PATCH the plan to Basic. Subsequent updates
        // manage plan only.
        //
        // FOLLOW-UP: Remove the ImportId (and the armSubscriptionId lookup) once CD has
        // successfully imported the table into state — leaving ImportId in place is a no-op
        // on subsequent runs but is noise. Tracked under the tc-i103 follow-ups.
        //
        // ImportId must be a plain string (not Output<string>) because the Pulumi engine
        // needs it at planning time before any Output has resolved. We read the subscription
        // from the ARM_SUBSCRIPTION_ID env var that CI (and local `pulumi up`) sets via the
        // Azure login step; resource group and workspace names are static literals.
        var armSubscriptionId = Environment.GetEnvironmentVariable("ARM_SUBSCRIPTION_ID")
            ?? throw new InvalidOperationException("ARM_SUBSCRIPTION_ID must be set to import the ContainerAppConsoleLogs table.");
        var tableImportId = $"/subscriptions/{armSubscriptionId}/resourceGroups/rg-town-crier-shared/providers/Microsoft.OperationalInsights/workspaces/log-town-crier-shared/tables/ContainerAppConsoleLogs";
        _ = new Table("table-containerappconsolelogs-basic", new TableArgs
        {
            ResourceGroupName = resourceGroup.Name,
            WorkspaceName = logAnalytics.Name,
            TableName = "ContainerAppConsoleLogs",
            Plan = TablePlanEnum.Basic,
        }, new CustomResourceOptions
        {
            ImportId = tableImportId,
        });

        // Move the AppTraces table to the Basic Logs plan for the same ~80% ingestion cost
        // saving. AppTraces holds OpenTelemetry verbose traces and is the highest-volume
        // App* table in this workspace. It is NOT queried by any dashboard tile (those use
        // `requests` and `customMetrics`) and no log-search alerts depend on it, so the
        // Basic plan's 8-day interactive retention and lack of cross-table joins is
        // acceptable. Other App* tables (AppRequests, AppExceptions, AppDependencies,
        // AppMetrics) stay on Analytics so the dashboard and sre-observatory retain full
        // 30-day history across them.
        //
        // Like ContainerAppConsoleLogs, Application Insights pre-creates the AppTraces
        // schema in the workspace when the Component resource is applied, so the table
        // already exists on the default Analytics plan before Pulumi reaches this resource.
        // ImportId below causes the first `pulumi up` to adopt the existing table into
        // Pulumi state and PATCH the plan to Basic; subsequent runs manage plan only.
        //
        // Basic plan retention is fixed at 8 days by Azure; RetentionInDays is not set
        // here and the service will apply the default. See bead tc-9ggc.
        var appTracesTableImportId = $"/subscriptions/{armSubscriptionId}/resourceGroups/rg-town-crier-shared/providers/Microsoft.OperationalInsights/workspaces/log-town-crier-shared/tables/AppTraces";
        _ = new Table("table-apptraces-basic", new TableArgs
        {
            ResourceGroupName = resourceGroup.Name,
            WorkspaceName = logAnalytics.Name,
            TableName = "AppTraces",
            Plan = TablePlanEnum.Basic,
        }, new CustomResourceOptions
        {
            ImportId = appTracesTableImportId,
        });

        // Force the App Insights workspace-based tables to honour the workspace's 30-day
        // retention. Despite the Component resource's RetentionInDays = 30 and the
        // workspace's RetentionInDays = 30, Azure pre-creates these tables with the legacy
        // Application Insights default of 90 days and they do NOT inherit either setting.
        // Verified live (2026-04-26): the Tables API returns retentionInDays=90 for all
        // App* tables below, while non-App* tables sit at the workspace default of 30.
        //
        // Under PerGB2018 this means days 31–90 of data sit in archive at ~£0.019/GB/month.
        // With current near-zero ingestion the exposure is negligible, but once prod traffic
        // grows this could become £3–5/month of pure waste. See bead tc-23yb and the cost
        // forecast at docs/cost-forecast/2026-04-25.md.
        //
        // We apply the same import-then-patch pattern used for ContainerAppConsoleLogs and
        // AppTraces: Application Insights pre-creates each table the first time data lands,
        // so on the initial `pulumi up` we adopt the existing resource into state and PATCH
        // RetentionInDays down to 30; subsequent runs manage retention only. AppTraces is
        // omitted because it is already on the Basic plan (8-day fixed retention).
        var appTablesToCapAt30Days = new[]
        {
            "AppRequests",
            "AppDependencies",
            "AppExceptions",
            "AppMetrics",
            "AppPageViews",
            "AppAvailabilityResults",
            "AppBrowserTimings",
            "AppEvents",
            "AppPerformanceCounters",
            "AppSystemEvents",
        };
        foreach (var tableName in appTablesToCapAt30Days)
        {
            var importId = $"/subscriptions/{armSubscriptionId}/resourceGroups/rg-town-crier-shared/providers/Microsoft.OperationalInsights/workspaces/log-town-crier-shared/tables/{tableName}";
            _ = new Table($"table-{tableName.ToLowerInvariant()}-30day", new TableArgs
            {
                ResourceGroupName = resourceGroup.Name,
                WorkspaceName = logAnalytics.Name,
                TableName = tableName,
                RetentionInDays = 30,
                TotalRetentionInDays = 30,
            }, new CustomResourceOptions
            {
                ImportId = importId,
            });
        }

        // User-assigned managed identity for Cosmos DB data access
        var cosmosDataIdentity = new UserAssignedIdentity("id-town-crier-cosmos-data", new UserAssignedIdentityArgs
        {
            ResourceName = "id-town-crier-cosmos-data",
            ResourceGroupName = resourceGroup.Name,
            Tags = tags,
        });

        // Cosmos DB Account (shared across environments — serverless)
        var cosmosAccount = new DatabaseAccount("cosmos-town-crier-shared", new DatabaseAccountArgs
        {
            AccountName = "cosmos-town-crier-shared",
            ResourceGroupName = resourceGroup.Name,
            Kind = DatabaseAccountKind.GlobalDocumentDB,
            DatabaseAccountOfferType = DatabaseAccountOfferType.Standard,
            Capabilities = new[]
            {
                new CapabilityArgs { Name = "EnableServerless" },
            },
            ConsistencyPolicy = new ConsistencyPolicyArgs
            {
                DefaultConsistencyLevel = DefaultConsistencyLevel.Session,
            },
            Locations = new[]
            {
                new LocationArgs
                {
                    LocationName = resourceGroup.Location,
                    FailoverPriority = 0,
                },
            },
            Tags = tags,
        });

        // Cost Management budget alert — fires when cumulative monthly spend on the
        // shared Cosmos account crosses £25. Cosmos serverless is the dominant cost
        // line (~£18/mo at 2026-05-02 and growing with polling volume); a single
        // admin batch (e.g. Apr 19-20 GDPR sweep) can triple the daily bill, so an
        // explicit guard rail is cheaper than waiting for the monthly invoice. See
        // bead tc-guwt and docs/cost-forecast/2026-05-02.md.
        //
        // Why a Budget vs an App Insights metric alert on RU consumption:
        //   - The threshold is denominated in GBP, which Budget tracks natively;
        //     translating to a daily RU ceiling is brittle (depends on Microsoft's
        //     per-million-RU price) and would need re-tuning every price change.
        //   - Budgets respect the actual billed amount, so deployment-day Cosmos
        //     spikes that are absorbed by the monthly average don't false-trigger.
        //
        // Scope is the resource group; the resource-id Filter narrows the budget to
        // just cosmos-town-crier-shared (Azure Budgets at resource scope are not a
        // supported scope level, so RG + ResourceId dimension filter is the
        // canonical pattern). Threshold is expressed as a percentage of Amount, so
        // Amount=25 + Threshold=100 fires at exactly £25 actual spend.
        //
        // alertEmail comes from Pulumi config (Pulumi.shared.yaml). Budgets at
        // RG scope require at least one contactEmail or contactGroup; we use email
        // for simplicity — no Action Group needed.
        var alertEmail = config.Require("alertEmail");
        _ = new Budget("budget-cosmos-shared-monthly", new BudgetArgs
        {
            BudgetName = "budget-cosmos-shared-monthly",
            Scope = resourceGroup.Id,
            Amount = 25,
            Category = CategoryType.Cost,
            TimeGrain = TimeGrainType.Monthly,
            TimePeriod = new BudgetTimePeriodArgs
            {
                // Budget start date must be the first of a month, on or after 2017-06-01.
                // Pinned to a static past date so subsequent `pulumi up` runs are no-ops.
                StartDate = "2026-05-01T00:00:00Z",
            },
            Filter = new BudgetFilterArgs
            {
                Dimensions = new BudgetComparisonExpressionArgs
                {
                    Name = "ResourceId",
                    Operator = BudgetOperatorType.In,
                    Values = new[] { cosmosAccount.Id },
                },
            },
            Notifications =
            {
                ["actual_GreaterThan_100_Percent"] = new NotificationArgs
                {
                    Enabled = true,
                    Operator = OperatorType.GreaterThan,
                    Threshold = 100,
                    ThresholdType = ThresholdType.Actual,
                    ContactEmails = new[] { alertEmail },
                    Locale = CultureCode.En_gb,
                },
            },
        });

        // Subscription-wide cost guard rail — fires when cumulative monthly spend
        // across the entire Town Crier subscription crosses £50. Current run-rate
        // is ~£29/mo (2026-05-02), giving ~£21 headroom before this trips. Catches
        // run-aways the Cosmos-scoped budget (tc-guwt) would miss: a misconfigured
        // Container App scaling to many replicas, an unbounded Log Analytics
        // ingestion spike, accidentally provisioning a non-serverless service, etc.
        // Ref: tc-yica and docs/cost-forecast/2026-05-02.md.
        //
        // Scope is the entire subscription, not a resource group, so no Filter
        // is set — the budget aggregates all billed resources under the sub.
        // Notifications include a 50% (early-warning) and 100% (red-alert) tier,
        // both at Actual cost; Forecasted alerts are noisy on serverless workloads
        // with bursty daily spend so we omit them.
        _ = new Budget("budget-subscription-monthly", new BudgetArgs
        {
            BudgetName = "budget-subscription-monthly",
            Scope = $"/subscriptions/{armSubscriptionId}",
            Amount = 50,
            Category = CategoryType.Cost,
            TimeGrain = TimeGrainType.Monthly,
            TimePeriod = new BudgetTimePeriodArgs
            {
                // Budget start date must be the first of a month, on or after 2017-06-01.
                // Pinned to a static past date so subsequent `pulumi up` runs are no-ops.
                StartDate = "2026-05-01T00:00:00Z",
            },
            Notifications =
            {
                ["actual_GreaterThan_50_Percent"] = new NotificationArgs
                {
                    Enabled = true,
                    Operator = OperatorType.GreaterThan,
                    Threshold = 50,
                    ThresholdType = ThresholdType.Actual,
                    ContactEmails = new[] { alertEmail },
                    Locale = CultureCode.En_gb,
                },
                ["actual_GreaterThan_100_Percent"] = new NotificationArgs
                {
                    Enabled = true,
                    Operator = OperatorType.GreaterThan,
                    Threshold = 100,
                    ThresholdType = ThresholdType.Actual,
                    ContactEmails = new[] { alertEmail },
                    Locale = CultureCode.En_gb,
                },
            },
        });

        // Cosmos DB Built-in Data Contributor role — allows CRUD on documents.
        // Role definition ID is well-known: 00000000-0000-0000-0000-000000000002.
        // Scoped to the Cosmos account; environment-level isolation is via database name.
        var cosmosRoleAssignment = new SqlResourceSqlRoleAssignment("cosmos-data-role", new SqlResourceSqlRoleAssignmentArgs
        {
            AccountName = cosmosAccount.Name,
            ResourceGroupName = resourceGroup.Name,
            RoleAssignmentId = "a3e0b382-7e3a-4b2d-9c4f-1a2b3c4d5e6f",
            RoleDefinitionId = cosmosAccount.Id.Apply(id =>
                $"{id}/sqlRoleDefinitions/00000000-0000-0000-0000-000000000002"),
            Scope = cosmosAccount.Id,
            PrincipalId = cosmosDataIdentity.PrincipalId,
        });

        // Azure Communication Services (Email) — UK data location.
        // Control plane is Location="global"; DataLocation="UK" pins storage/processing
        // to UK data centres for compliance with our UK data residency commitment.
        //
        // Declaration order: EmailService → Domain → CommunicationService. The
        // CommunicationService must be declared after the Domain so its `.Id` is in
        // C# scope to pass via `LinkedDomains`. Pulumi handles Azure-side ordering
        // via Output dependencies regardless of C# declaration order.
        var emailServiceUk = new EmailService("email-town-crier-uk", new EmailServiceArgs
        {
            EmailServiceName = "email-town-crier-uk",
            ResourceGroupName = resourceGroup.Name,
            Location = "global",
            DataLocation = "UK",
            Tags = tags,
        });

        // Custom-domain sender identity for the UK EmailService. CustomerManaged means we
        // own the DKIM/SPF/Domain TXT records in Cloudflare DNS (see project_cloudflare_dns
        // memory). Logical name retains the `-new` suffix from the dual-resource migration
        // (tc-8634, tc-zx5g) to avoid a Pulumi replace; rename via `aliases` if desired.
        var towncrierAppDomain = new Domain("domain-towncrierapp-uk-new", new DomainArgs
        {
            DomainName = "towncrierapp.uk",
            EmailServiceName = emailServiceUk.Name,
            ResourceGroupName = resourceGroup.Name,
            Location = "global",
            DomainManagement = DomainManagement.CustomerManaged,
            Tags = tags,
        });

        // LinkedDomains authorises the CommunicationService to send mail from the
        // towncrierapp.uk Domain. Without this, every send fails with
        // 404 DomainNotLinked (tc-luxj, GH#370). The Domain resource is declared
        // above so its .Id is in C# scope; Pulumi tracks the Output dependency
        // and provisions the Domain before applying the link.
        var communicationServiceUk = new CommunicationService("acs-town-crier-uk", new CommunicationServiceArgs
        {
            CommunicationServiceName = "acs-town-crier-uk",
            ResourceGroupName = resourceGroup.Name,
            Location = "global",
            DataLocation = "UK",
            LinkedDomains = new[] { towncrierAppDomain.Id },
            Tags = tags,
        });

        // Operational Dashboard
        var dashboard = new Dashboard("dash-towncrier-operational", new DashboardArgs
        {
            DashboardName = "dash-towncrier-operational",
            ResourceGroupName = resourceGroup.Name,
            Location = resourceGroup.Location,
            Tags = tags,
            Properties = new DashboardPropertiesWithProvisioningStateArgs
            {
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
                                    "let data = requests | where name == 'GET /v1/me' | summarize Value=dcount(user_AuthenticatedId) by timestamp=bin(timestamp, 1h); let empty = datatable(timestamp:datetime, Value:long)[]; union data, empty | render timechart",
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
                                Metadata = StackedMetricTile(
                                    appInsights.Id,
                                    "towncrier.polling.authorities_polled", "Successes",
                                    "towncrier.polling.failures", "Failures",
                                    "Sync Success vs Failure"),
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
                            // Row 4: PlanIt API Health
                            new DashboardPartsArgs
                            {
                                Position = new DashboardPartsPositionArgs { X = 0, Y = 12, ColSpan = 6, RowSpan = 4 },
                                Metadata = KqlTile(
                                    appInsights.Id,
                                    "customMetrics | where name == 'towncrier.planit.http_errors' | extend status = tostring(customDimensions['http.response.status_code']) | where status == '429' | summarize Value=sum(value) by timestamp=bin(timestamp, 1h) | render timechart",
                                    "PlanIt 429s"),
                            },
                            new DashboardPartsArgs
                            {
                                Position = new DashboardPartsPositionArgs { X = 6, Y = 12, ColSpan = 6, RowSpan = 4 },
                                Metadata = KqlTile(
                                    appInsights.Id,
                                    "customMetrics | where name == 'towncrier.planit.http_errors' | extend status = tostring(customDimensions['http.response.status_code']) | where status != '429' | summarize Value=sum(value) by timestamp=bin(timestamp, 1h), status | render timechart",
                                    "PlanIt Errors"),
                            },
                            // Row 5: Email
                            new DashboardPartsArgs
                            {
                                Position = new DashboardPartsPositionArgs { X = 0, Y = 16, ColSpan = 6, RowSpan = 4 },
                                Metadata = MetricTile(appInsights.Id, "towncrier.emails.sent", "Emails Sent"),
                            },
                            new DashboardPartsArgs
                            {
                                Position = new DashboardPartsPositionArgs { X = 6, Y = 16, ColSpan = 6, RowSpan = 4 },
                                Metadata = MetricTile(appInsights.Id, "towncrier.emails.failed", "Email Failures"),
                            },
                        },
                    },
                },
            },
        });

        return new Dictionary<string, object?>
        {
            ["resourceGroupName"] = resourceGroup.Name,
            ["containerRegistryLoginServer"] = containerRegistry.LoginServer,
            ["acrPullIdentityId"] = acrPullIdentity.Id,
            ["cosmosDataIdentityId"] = cosmosDataIdentity.Id,
            ["cosmosDataIdentityClientId"] = cosmosDataIdentity.ClientId,
            // PrincipalId is required by env-stack role assignments (e.g. Service Bus
            // Data Owner on the polling namespace) because RBAC grants are keyed to the
            // identity's principal (object) ID, not the client ID exposed to the runtime.
            ["cosmosDataIdentityPrincipalId"] = cosmosDataIdentity.PrincipalId,
            ["containerAppsEnvironmentId"] = containerAppsEnv.Id,
            ["cosmosAccountName"] = cosmosAccount.Name,
            ["cosmosAccountEndpoint"] = cosmosAccount.DocumentEndpoint,
            ["appInsightsId"] = appInsights.Id,
            ["appInsightsConnectionString"] = appInsights.ConnectionString,
            ["acsConnectionString"] = Output.Tuple(resourceGroup.Name, communicationServiceUk.Name)
                .Apply(names => ListCommunicationServiceKeys.InvokeAsync(new ListCommunicationServiceKeysArgs
                {
                    ResourceGroupName = names.Item1,
                    CommunicationServiceName = names.Item2,
                }))
                .Apply(keys => keys.PrimaryConnectionString),
        };
    }

    private static DashboardPartMetadataArgs MetricTile(
        Output<string> appInsightsId, string metricName, string title)
    {
        return new DashboardPartMetadataArgs
        {
            Type = "Extension/HubsExtension/PartType/MonitorChartPart",
            Settings = new InputMap<object>
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
                                    ["aggregationType"] = 1,
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
                                ["chartType"] = 2,
                            },
                            ["timespan"] = new Dictionary<string, object>
                            {
                                ["relative"] = new Dictionary<string, object>
                                {
                                    ["duration"] = DashboardTimespan24HoursMs,
                                },
                            },
                        },
                    },
                },
            },
        };
    }

    private static DashboardPartMetadataArgs StackedMetricTile(
        Output<string> appInsightsId,
        string metric1, string label1,
        string metric2, string label2,
        string title)
    {
        return new DashboardPartMetadataArgs
        {
            Type = "Extension/HubsExtension/PartType/MonitorChartPart",
            Settings = new InputMap<object>
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
                                    ["name"] = metric1,
                                    ["aggregationType"] = 1,
                                    ["namespace"] = "azure.applicationinsights",
                                    ["metricVisualization"] = new Dictionary<string, object>
                                    {
                                        ["displayName"] = label1,
                                    },
                                },
                                new Dictionary<string, object>
                                {
                                    ["resourceMetadata"] = new Dictionary<string, object>
                                    {
                                        ["id"] = appInsightsId,
                                    },
                                    ["name"] = metric2,
                                    ["aggregationType"] = 1,
                                    ["namespace"] = "azure.applicationinsights",
                                    ["metricVisualization"] = new Dictionary<string, object>
                                    {
                                        ["displayName"] = label2,
                                    },
                                },
                            },
                            ["title"] = title,
                            ["titleKind"] = 1,
                            ["visualization"] = new Dictionary<string, object>
                            {
                                ["chartType"] = 2,
                            },
                            ["timespan"] = new Dictionary<string, object>
                            {
                                ["relative"] = new Dictionary<string, object>
                                {
                                    ["duration"] = DashboardTimespan24HoursMs,
                                },
                            },
                        },
                    },
                },
            },
        };
    }

    private static DashboardPartMetadataArgs KqlTile(
        Output<string> appInsightsId, string query, string title)
    {
        var componentId = appInsightsId.Apply(id =>
        {
            var segments = id.Split('/');
            return new Dictionary<string, object>
            {
                ["SubscriptionId"] = segments[2],
                ["ResourceGroup"] = segments[4],
                ["Name"] = segments[8],
                ["ResourceId"] = id,
            };
        });

        return new DashboardPartMetadataArgs
        {
            Type = "Extension/AppInsightsExtension/PartType/AnalyticsPart",
            Settings = new InputMap<object>
            {
                ["content"] = new Dictionary<string, object>
                {
                    ["Query"] = query,
                    ["ControlType"] = "FrameControlChart",
                    ["SpecificChart"] = "Line",
                    ["PartTitle"] = title,
                    ["IsQueryContainTimeRange"] = false,
                    ["Dimensions"] = new Dictionary<string, object>
                    {
                        ["xAxis"] = new Dictionary<string, object>
                        {
                            ["name"] = "timestamp",
                            ["type"] = "datetime",
                        },
                        ["yAxis"] = new object[]
                        {
                            new Dictionary<string, object>
                            {
                                ["name"] = "Value",
                                ["type"] = "long",
                            },
                        },
                    },
                },
            },
            Inputs = new[]
            {
                (object)new Dictionary<string, object>
                {
                    ["name"] = "ComponentId",
                    ["value"] = componentId,
                },
            },
        };
    }
}
