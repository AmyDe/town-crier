using System.Collections.Immutable;
using Pulumi;
using Pulumi.AzureNative.Resources;
using Pulumi.AzureNative.OperationalInsights;
using Pulumi.AzureNative.App;
using Pulumi.AzureNative.App.Inputs;
using Pulumi.AzureNative.CosmosDB;
using Pulumi.AzureNative.CosmosDB.Inputs;
using Pulumi.AzureNative.Web;
using ManagedServiceIdentityType = Pulumi.AzureNative.App.ManagedServiceIdentityType;

public static class EnvironmentStack
{
    public static Dictionary<string, object?> Run(Config config, string env, InputMap<string> tags)
    {
        var cosmosConsistencyLevel = config.Require("cosmosConsistencyLevel");

        // Shared stack outputs
        var shared = new StackReference("AmyDe/town-crier/shared");
        var acrLoginServer = shared.GetOutput("containerRegistryLoginServer").Apply(o => o?.ToString() ?? "");
        var acrPullIdentityId = shared.GetOutput("acrPullIdentityId").Apply(o => o?.ToString() ?? "");
        var acrPullIdentityClientId = shared.GetOutput("acrPullIdentityClientId").Apply(o => o?.ToString() ?? "");

        // Resource Group
        var resourceGroup = new ResourceGroup($"rg-town-crier-{env}", new ResourceGroupArgs
        {
            ResourceGroupName = $"rg-town-crier-{env}",
            Tags = tags,
        });

        // Log Analytics Workspace
        var logAnalytics = new Workspace($"log-town-crier-{env}", new WorkspaceArgs
        {
            WorkspaceName = $"log-town-crier-{env}",
            ResourceGroupName = resourceGroup.Name,
            Sku = new Pulumi.AzureNative.OperationalInsights.Inputs.WorkspaceSkuArgs
            {
                Name = WorkspaceSkuNameEnum.PerGB2018,
            },
            RetentionInDays = 30,
            Tags = tags,
        });

        var logAnalyticsSharedKeys = Output.Tuple(resourceGroup.Name, logAnalytics.Name)
            .Apply(names => GetSharedKeys.InvokeAsync(new GetSharedKeysArgs
            {
                ResourceGroupName = names.Item1,
                WorkspaceName = names.Item2,
            }));

        // Container Apps Environment
        var containerAppsEnv = new ManagedEnvironment($"cae-town-crier-{env}", new ManagedEnvironmentArgs
        {
            EnvironmentName = $"cae-town-crier-{env}",
            ResourceGroupName = resourceGroup.Name,
            AppLogsConfiguration = new AppLogsConfigurationArgs
            {
                Destination = "log-analytics",
                LogAnalyticsConfiguration = new LogAnalyticsConfigurationArgs
                {
                    CustomerId = logAnalytics.CustomerId,
                    SharedKey = logAnalyticsSharedKeys.Apply(keys => keys.PrimarySharedKey ?? ""),
                },
            },
            Tags = tags,
        });

        // Cosmos DB Account (Serverless)
        var cosmosAccount = new DatabaseAccount($"cosmos-town-crier-{env}", new DatabaseAccountArgs
        {
            AccountName = $"cosmos-town-crier-{env}",
            ResourceGroupName = resourceGroup.Name,
            Kind = DatabaseAccountKind.GlobalDocumentDB,
            DatabaseAccountOfferType = DatabaseAccountOfferType.Standard,
            Capabilities = new[]
            {
                new CapabilityArgs { Name = "EnableServerless" },
            },
            ConsistencyPolicy = new ConsistencyPolicyArgs
            {
                DefaultConsistencyLevel = cosmosConsistencyLevel switch
                {
                    "Strong" => DefaultConsistencyLevel.Strong,
                    "BoundedStaleness" => DefaultConsistencyLevel.BoundedStaleness,
                    "Session" => DefaultConsistencyLevel.Session,
                    "ConsistentPrefix" => DefaultConsistencyLevel.ConsistentPrefix,
                    "Eventual" => DefaultConsistencyLevel.Eventual,
                    _ => DefaultConsistencyLevel.Session,
                },
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

        // Cosmos DB Database
        var cosmosDatabase = new SqlResourceSqlDatabase($"db-town-crier-{env}", new SqlResourceSqlDatabaseArgs
        {
            AccountName = cosmosAccount.Name,
            ResourceGroupName = resourceGroup.Name,
            DatabaseName = "town-crier",
            Resource = new SqlDatabaseResourceArgs
            {
                Id = "town-crier",
            },
        });

        // Cosmos DB Containers

        // Applications container — partitioned by authority code, spatial index on location
        var applicationsContainer = new SqlResourceSqlContainer($"container-applications-{env}", new SqlResourceSqlContainerArgs
        {
            AccountName = cosmosAccount.Name,
            ResourceGroupName = resourceGroup.Name,
            DatabaseName = cosmosDatabase.Name,
            ContainerName = "Applications",
            Resource = new SqlContainerResourceArgs
            {
                Id = "Applications",
                PartitionKey = new ContainerPartitionKeyArgs
                {
                    Paths = new[] { "/authorityCode" },
                    Kind = PartitionKind.Hash,
                },
                DefaultTtl = -1, // TTL enabled, per-document control
                UniqueKeyPolicy = new UniqueKeyPolicyArgs
                {
                    UniqueKeys = new[]
                    {
                        new UniqueKeyArgs
                        {
                            Paths = new[] { "/planitName" },
                        },
                    },
                },
                IndexingPolicy = new IndexingPolicyArgs
                {
                    Automatic = true,
                    IndexingMode = IndexingMode.Consistent,
                    IncludedPaths = new[]
                    {
                        new IncludedPathArgs { Path = "/authorityCode/?" },
                        new IncludedPathArgs { Path = "/status/?" },
                        new IncludedPathArgs { Path = "/applicationType/?" },
                        new IncludedPathArgs { Path = "/decisionDate/?" },
                        new IncludedPathArgs { Path = "/lastDifferent/?" },
                    },
                    ExcludedPaths = new[]
                    {
                        new ExcludedPathArgs { Path = "/*" },
                        new ExcludedPathArgs { Path = "/\"_etag\"/?" },
                    },
                    SpatialIndexes = new[]
                    {
                        new SpatialSpecArgs
                        {
                            Path = "/location/?",
                            Types = new InputList<Union<string, SpatialType>>
                            {
                                SpatialType.Point,
                            },
                        },
                    },
                    CompositeIndexes = new InputList<ImmutableArray<CompositePathArgs>>
                    {
                        ImmutableArray.Create(
                            new CompositePathArgs { Path = "/authorityCode", Order = CompositePathSortOrder.Ascending },
                            new CompositePathArgs { Path = "/lastDifferent", Order = CompositePathSortOrder.Descending }
                        ),
                    },
                },
            },
        });

        // Users container — partitioned by id
        var usersContainer = new SqlResourceSqlContainer($"container-users-{env}", new SqlResourceSqlContainerArgs
        {
            AccountName = cosmosAccount.Name,
            ResourceGroupName = resourceGroup.Name,
            DatabaseName = cosmosDatabase.Name,
            ContainerName = "Users",
            Resource = new SqlContainerResourceArgs
            {
                Id = "Users",
                PartitionKey = new ContainerPartitionKeyArgs
                {
                    Paths = new[] { "/id" },
                    Kind = PartitionKind.Hash,
                },
            },
        });

        // WatchZones container — partitioned by userId
        var watchZonesContainer = new SqlResourceSqlContainer($"container-watchzones-{env}", new SqlResourceSqlContainerArgs
        {
            AccountName = cosmosAccount.Name,
            ResourceGroupName = resourceGroup.Name,
            DatabaseName = cosmosDatabase.Name,
            ContainerName = "WatchZones",
            Resource = new SqlContainerResourceArgs
            {
                Id = "WatchZones",
                PartitionKey = new ContainerPartitionKeyArgs
                {
                    Paths = new[] { "/userId" },
                    Kind = PartitionKind.Hash,
                },
                UniqueKeyPolicy = new UniqueKeyPolicyArgs
                {
                    UniqueKeys = new[]
                    {
                        new UniqueKeyArgs
                        {
                            Paths = new[] { "/userId", "/name" },
                        },
                    },
                },
            },
        });

        // Notifications container — partitioned by userId, 90-day TTL
        var notificationsContainer = new SqlResourceSqlContainer($"container-notifications-{env}", new SqlResourceSqlContainerArgs
        {
            AccountName = cosmosAccount.Name,
            ResourceGroupName = resourceGroup.Name,
            DatabaseName = cosmosDatabase.Name,
            ContainerName = "Notifications",
            Resource = new SqlContainerResourceArgs
            {
                Id = "Notifications",
                PartitionKey = new ContainerPartitionKeyArgs
                {
                    Paths = new[] { "/userId" },
                    Kind = PartitionKind.Hash,
                },
                DefaultTtl = 90 * 24 * 60 * 60, // 90 days in seconds
            },
        });

        // Leases container — for change feed processor checkpointing
        var leasesContainer = new SqlResourceSqlContainer($"container-leases-{env}", new SqlResourceSqlContainerArgs
        {
            AccountName = cosmosAccount.Name,
            ResourceGroupName = resourceGroup.Name,
            DatabaseName = cosmosDatabase.Name,
            ContainerName = "Leases",
            Resource = new SqlContainerResourceArgs
            {
                Id = "Leases",
                PartitionKey = new ContainerPartitionKeyArgs
                {
                    Paths = new[] { "/id" },
                    Kind = PartitionKind.Hash,
                },
            },
        });

        // Container App (API) — placeholder image until CI/CD pushes real builds
        var containerApp = new ContainerApp($"ca-town-crier-api-{env}", new ContainerAppArgs
        {
            ContainerAppName = $"ca-town-crier-api-{env}",
            ResourceGroupName = resourceGroup.Name,
            ManagedEnvironmentId = containerAppsEnv.Id,
            Configuration = new ConfigurationArgs
            {
                Ingress = new IngressArgs
                {
                    External = true,
                    TargetPort = 8080,
                    Transport = IngressTransportMethod.Http,
                },
                Registries = new[]
                {
                    new RegistryCredentialsArgs
                    {
                        Server = acrLoginServer,
                        Identity = acrPullIdentityId,
                    },
                },
            },
            Identity = new Pulumi.AzureNative.App.Inputs.ManagedServiceIdentityArgs
            {
                Type = ManagedServiceIdentityType.UserAssigned,
                UserAssignedIdentities = new InputList<string>
                {
                    acrPullIdentityId,
                },
            },
            Template = new TemplateArgs
            {
                Containers = new[]
                {
                    new ContainerArgs
                    {
                        Name = "api",
                        Image = "mcr.microsoft.com/k8se/quickstart:latest",
                        Resources = new ContainerResourcesArgs
                        {
                            Cpu = 0.25,
                            Memory = "0.5Gi",
                        },
                    },
                },
                Scale = new ScaleArgs
                {
                    MinReplicas = 0,
                    MaxReplicas = 1,
                },
            },
            Tags = tags,
        });

        // Static Web App (Landing Page)
        var staticWebApp = new StaticSite($"swa-town-crier-{env}", new StaticSiteArgs
        {
            Name = $"swa-town-crier-{env}",
            ResourceGroupName = resourceGroup.Name,
            Location = "westeurope",
            Sku = new Pulumi.AzureNative.Web.Inputs.SkuDescriptionArgs
            {
                Name = "Free",
                Tier = "Free",
            },
            BuildProperties = new Pulumi.AzureNative.Web.Inputs.StaticSiteBuildPropertiesArgs
            {
                AppLocation = "/",
                OutputLocation = "",
            },
            Tags = tags,
        });

        return new Dictionary<string, object?>
        {
            ["resourceGroupName"] = resourceGroup.Name,
            ["containerAppsEnvironmentId"] = containerAppsEnv.Id,
            ["containerAppUrl"] = containerApp.LatestRevisionFqdn.Apply(fqdn => $"https://{fqdn}"),
            ["cosmosAccountEndpoint"] = cosmosAccount.DocumentEndpoint,
            ["cosmosDatabaseName"] = cosmosDatabase.Name,
            ["logAnalyticsWorkspaceId"] = logAnalytics.Id,
            ["staticWebAppUrl"] = staticWebApp.DefaultHostname.Apply(hostname => $"https://{hostname}"),
            ["staticWebAppName"] = staticWebApp.Name,
        };
    }
}
