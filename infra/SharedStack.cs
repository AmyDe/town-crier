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

        // Azure Container Registry (shared across environments)
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

        // Azure Communication Services (Email) — Europe data location.
        //
        // DEPRECATED: These resources hold email content and recipient addresses in EEA data
        // centres, which is inconsistent with our UK South data residency commitment. Azure
        // does NOT allow mutating DataLocation on an existing ACS resource, so we cannot
        // migrate in place. The replacement UK-located resources are provisioned below
        // (acs-town-crier-uk / email-town-crier-uk / domain-towncrierapp-uk-new) and will
        // take over once the domain is re-verified on Cloudflare DNS and the Key Vault
        // connection-string secret is swapped. These Europe-located resources will be
        // removed in a follow-up bead after the swap is complete. See tc-8634.
        var communicationService = new CommunicationService("acs-town-crier-shared", new CommunicationServiceArgs
        {
            CommunicationServiceName = "acs-town-crier-shared",
            ResourceGroupName = resourceGroup.Name,
            Location = "global",
            DataLocation = "Europe",
            Tags = tags,
        });

        var emailService = new EmailService("email-town-crier-shared", new EmailServiceArgs
        {
            EmailServiceName = "email-town-crier-shared",
            ResourceGroupName = resourceGroup.Name,
            Location = "global",
            DataLocation = "Europe",
            Tags = tags,
        });

        _ = new Domain("domain-towncrierapp-uk", new DomainArgs
        {
            DomainName = "towncrierapp.uk",
            EmailServiceName = emailService.Name,
            ResourceGroupName = resourceGroup.Name,
            Location = "global",
            DomainManagement = DomainManagement.CustomerManaged,
            Tags = tags,
        });

        // Azure Communication Services (Email) — UK data location.
        //
        // New resources introduced by tc-8634 to bring ACS into UK data residency. They
        // are provisioned alongside the Europe-located resources above; the swap is an
        // operational follow-up (DNS re-verification, Key Vault secret rotation, app
        // redeploy) that requires human coordination. Control plane for ACS is always
        // Location="global"; DataLocation="UK" pins storage/processing to UK data centres.
        var communicationServiceUk = new CommunicationService("acs-town-crier-uk", new CommunicationServiceArgs
        {
            CommunicationServiceName = "acs-town-crier-uk",
            ResourceGroupName = resourceGroup.Name,
            Location = "global",
            DataLocation = "UK",
            Tags = tags,
        });

        var emailServiceUk = new EmailService("email-town-crier-uk", new EmailServiceArgs
        {
            EmailServiceName = "email-town-crier-uk",
            ResourceGroupName = resourceGroup.Name,
            Location = "global",
            DataLocation = "UK",
            Tags = tags,
        });

        // Domain resource wraps the same custom domain (towncrierapp.uk) but hangs off the
        // new UK-located EmailService, so Azure treats it as a distinct sender identity and
        // issues a fresh set of DKIM/SPF/TXT records. Those records must be added to
        // Cloudflare DNS before this domain will verify (CustomerManaged) — tracked under
        // the tc-8634 follow-ups. The Pulumi logical name differs from the existing
        // `domain-towncrierapp-uk` to avoid resource-name collision inside this stack.
        _ = new Domain("domain-towncrierapp-uk-new", new DomainArgs
        {
            DomainName = "towncrierapp.uk",
            EmailServiceName = emailServiceUk.Name,
            ResourceGroupName = resourceGroup.Name,
            Location = "global",
            DomainManagement = DomainManagement.CustomerManaged,
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
            ["containerAppsEnvironmentId"] = containerAppsEnv.Id,
            ["cosmosAccountName"] = cosmosAccount.Name,
            ["cosmosAccountEndpoint"] = cosmosAccount.DocumentEndpoint,
            ["appInsightsId"] = appInsights.Id,
            ["appInsightsConnectionString"] = appInsights.ConnectionString,
            ["acsConnectionString"] = Output.Tuple(resourceGroup.Name, communicationService.Name)
                .Apply(names => ListCommunicationServiceKeys.InvokeAsync(new ListCommunicationServiceKeysArgs
                {
                    ResourceGroupName = names.Item1,
                    CommunicationServiceName = names.Item2,
                }))
                .Apply(keys => keys.PrimaryConnectionString),
            // Connection string for the new UK-located ACS resource. Kept as a separate
            // output so operators can pick it up when rotating the Key Vault secret during
            // the tc-8634 migration; the legacy `acsConnectionString` above still powers
            // the running app until the swap happens.
            ["acsConnectionStringUk"] = Output.Tuple(resourceGroup.Name, communicationServiceUk.Name)
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
