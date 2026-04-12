using System.Collections.Immutable;
using Pulumi;
using Pulumi.AzureNative.Resources;
using Pulumi.AzureNative.App;
using Pulumi.AzureNative.App.Inputs;
using Pulumi.AzureNative.CosmosDB;
using Pulumi.AzureNative.CosmosDB.Inputs;
using Pulumi.AzureNative.Web;
using ManagedServiceIdentityType = Pulumi.AzureNative.App.ManagedServiceIdentityType;

/// <summary>
/// Defines a Cosmos DB container with its partition key and optional advanced settings.
/// </summary>
/// <param name="Name">The container name (used as both the Cosmos container name and resource Id).</param>
/// <param name="PartitionKeyPath">The partition key path (e.g. "/userId").</param>
/// <param name="DefaultTtl">Optional TTL in seconds. -1 enables per-document TTL control.</param>
/// <param name="UniqueKeyPaths">Optional unique key paths. Each inner array is one unique key constraint.</param>
/// <param name="IndexingPolicy">Optional custom indexing policy. When null, Cosmos uses the default.</param>
public sealed record CosmosContainerDefinition(
    string Name,
    string PartitionKeyPath,
    int? DefaultTtl = null,
    string[][]? UniqueKeyPaths = null,
    IndexingPolicyArgs? IndexingPolicy = null);

public static class EnvironmentStack
{
    /// <summary>
    /// CPU cores allocated to each container (Container App and Worker Job).
    /// Azure Container Apps minimum is 0.25 vCPU.
    /// </summary>
    private const double ContainerCpu = 0.25;

    /// <summary>
    /// Memory allocated to each container (Container App and Worker Job).
    /// Must be at least 2x the CPU value in Gi (0.25 vCPU -> 0.5Gi minimum).
    /// </summary>
    private const string ContainerMemory = "0.5Gi";

    public static Dictionary<string, object?> Run(Config config, string env, InputMap<string> tags)
    {
        var frontendDomain = config.Require("frontendDomain");
        var apiDomain = config.Require("apiDomain");
        var auth0Domain = config.Require("auth0Domain");
        var auth0Audience = config.Require("auth0Audience");
        var customDomainPhase = config.GetInt32("customDomainPhase") ?? 2;
        var adminApiKey = config.RequireSecret("adminApiKey");
        var autoGrantProDomains = config.RequireSecret("autoGrantProDomains");
        var auth0M2mClientId = config.RequireSecret("auth0M2mClientId");
        var auth0M2mClientSecret = config.RequireSecret("auth0M2mClientSecret");

        // Shared stack outputs
        var shared = new StackReference("AmyDe/town-crier/shared");
        var acrLoginServer = shared.GetOutput("containerRegistryLoginServer").Apply(o => o?.ToString() ?? "");
        var acrPullIdentityId = shared.GetOutput("acrPullIdentityId").Apply(o => o?.ToString() ?? "");
        var containerAppsEnvironmentId = shared.GetOutput("containerAppsEnvironmentId").Apply(o => o?.ToString() ?? "");
        var cosmosDataIdentityId = shared.GetOutput("cosmosDataIdentityId").Apply(o => o?.ToString() ?? "");
        var cosmosDataIdentityClientId = shared.GetOutput("cosmosDataIdentityClientId").Apply(o => o?.ToString() ?? "");
        var cosmosAccountName = shared.GetOutput("cosmosAccountName").Apply(o => o?.ToString() ?? "");
        var cosmosAccountEndpoint = shared.GetOutput("cosmosAccountEndpoint").Apply(o => o?.ToString() ?? "");
        var appInsightsConnectionString = shared.GetOutput("appInsightsConnectionString").Apply(o => o?.ToString() ?? "");
        var acsConnectionString = shared.GetOutput("acsConnectionString").Apply(o => o?.ToString() ?? "");
        // Extract the CAE name from its resource ID to avoid
        // requiring a shared stack deploy before the env stack can preview.
        // ID format: /subscriptions/.../resourceGroups/{rg}/providers/Microsoft.App/managedEnvironments/{name}
        var containerAppsEnvironmentName = containerAppsEnvironmentId.Apply(id =>
        {
            var segments = id.Split('/');
            return segments.Length > 0 ? segments[^1] : "";
        });
        var sharedResourceGroupName = shared.GetOutput("resourceGroupName").Apply(o => o?.ToString() ?? "");

        // Resource Group
        var resourceGroup = new ResourceGroup($"rg-town-crier-{env}", new ResourceGroupArgs
        {
            ResourceGroupName = $"rg-town-crier-{env}",
            Tags = tags,
        });

        // Cosmos DB Database (in shared account)
        var cosmosDatabase = new SqlResourceSqlDatabase($"db-town-crier-{env}", new SqlResourceSqlDatabaseArgs
        {
            AccountName = cosmosAccountName,
            ResourceGroupName = sharedResourceGroupName,
            DatabaseName = $"town-crier-{env}",
            Resource = new SqlDatabaseResourceArgs
            {
                Id = $"town-crier-{env}",
            },
        });

        // Cosmos DB Containers — definition array + creation loop
        var containerDefinitions = new CosmosContainerDefinition[]
        {
            // Applications — partitioned by authority code, spatial index on location
            new("Applications", "/authorityCode",
                DefaultTtl: -1, // TTL enabled, per-document control
                UniqueKeyPaths: [["/planitName"]],
                IndexingPolicy: new IndexingPolicyArgs
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
                }),

            // Users — partitioned by id
            new("Users", "/id"),

            // WatchZones — partitioned by userId, unique on (userId, name)
            new("WatchZones", "/userId",
                UniqueKeyPaths: [["/userId", "/name"]]),

            // Notifications — partitioned by userId, 90-day TTL
            new("Notifications", "/userId",
                DefaultTtl: 90 * 24 * 60 * 60), // 90 days in seconds

            // Leases — for change feed processor checkpointing
            new("Leases", "/id"),

            // DeviceRegistrations — partitioned by userId
            new("DeviceRegistrations", "/userId"),

            // SavedApplications — partitioned by userId
            new("SavedApplications", "/userId"),

            // DecisionAlerts — partitioned by userId
            new("DecisionAlerts", "/userId"),

            // PollState — single document storing last poll timestamp
            new("PollState", "/id"),
        };

        foreach (var container in containerDefinitions)
        {
            var resourceArgs = new SqlContainerResourceArgs
            {
                Id = container.Name,
                PartitionKey = new ContainerPartitionKeyArgs
                {
                    Paths = new[] { container.PartitionKeyPath },
                    Kind = PartitionKind.Hash,
                },
            };

            if (container.DefaultTtl is not null)
            {
                resourceArgs.DefaultTtl = container.DefaultTtl.Value;
            }

            if (container.UniqueKeyPaths is not null)
            {
                resourceArgs.UniqueKeyPolicy = new UniqueKeyPolicyArgs
                {
                    UniqueKeys = container.UniqueKeyPaths
                        .Select(paths => new UniqueKeyArgs { Paths = paths })
                        .ToArray(),
                };
            }

            if (container.IndexingPolicy is not null)
            {
                resourceArgs.IndexingPolicy = container.IndexingPolicy;
            }

            new SqlResourceSqlContainer($"container-{container.Name.ToLowerInvariant()}-{env}", new SqlResourceSqlContainerArgs
            {
                AccountName = cosmosAccountName,
                ResourceGroupName = sharedResourceGroupName,
                DatabaseName = cosmosDatabase.Name,
                ContainerName = container.Name,
                Resource = resourceArgs,
            });
        }

        // Managed Certificate for API custom domain
        // Phase 1 (first deploy): Container App created first with disabled binding,
        //   then cert created with DependsOn so Azure can validate the hostname.
        // Phase 2 (after cert provisioned): Cert created first, Container App
        //   binds it with SniEnabled. Set customDomainPhase in Pulumi config.
        ManagedCertificate? apiManagedCert = null;

        if (customDomainPhase >= 2)
        {
            apiManagedCert = CreateApiManagedCertificate(env, containerAppsEnvironmentName, sharedResourceGroupName, apiDomain);
        }

        // Container App (API) — placeholder image until CI/CD pushes real builds
        var containerApp = new ContainerApp($"ca-town-crier-api-{env}", new ContainerAppArgs
        {
            ContainerAppName = $"ca-town-crier-api-{env}",
            ResourceGroupName = resourceGroup.Name,
            ManagedEnvironmentId = containerAppsEnvironmentId,
            Configuration = new ConfigurationArgs
            {
                ActiveRevisionsMode = ActiveRevisionsMode.Multiple,
                Secrets = new[]
                {
                    new SecretArgs { Name = "admin-api-key", Value = adminApiKey },
                    new SecretArgs { Name = "auto-grant-pro-domains", Value = autoGrantProDomains },
                    new SecretArgs { Name = "acs-connection-string", Value = acsConnectionString },
                    new SecretArgs { Name = "auth0-m2m-client-id", Value = auth0M2mClientId },
                    new SecretArgs { Name = "auth0-m2m-client-secret", Value = auth0M2mClientSecret },
                },
                Ingress = new IngressArgs
                {
                    External = true,
                    TargetPort = 8080,
                    Transport = IngressTransportMethod.Http,
                    CustomDomains = customDomainPhase >= 2
                        ? new[]
                        {
                            new CustomDomainArgs
                            {
                                Name = apiDomain,
                                CertificateId = apiManagedCert!.Id,
                                BindingType = BindingType.SniEnabled,
                            },
                        }
                        : new[]
                        {
                            new CustomDomainArgs
                            {
                                Name = apiDomain,
                                BindingType = BindingType.Disabled,
                            },
                        },
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
                    cosmosDataIdentityId,
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
                            Cpu = ContainerCpu,
                            Memory = ContainerMemory,
                        },
                        Env = new[]
                        {
                            new EnvironmentVarArgs { Name = "OTEL_SERVICE_NAME", Value = "town-crier-api" },
                            new EnvironmentVarArgs { Name = "Auth0__Domain", Value = auth0Domain },
                            new EnvironmentVarArgs { Name = "Auth0__Audience", Value = auth0Audience },
                            new EnvironmentVarArgs { Name = "Cosmos__AccountEndpoint", Value = cosmosAccountEndpoint },
                            new EnvironmentVarArgs { Name = "Cosmos__DatabaseName", Value = cosmosDatabase.Name },
                            new EnvironmentVarArgs { Name = "AZURE_CLIENT_ID", Value = cosmosDataIdentityClientId },
                            new EnvironmentVarArgs { Name = "Cors__AllowedOrigins__0", Value = $"https://{frontendDomain}" },
                            new EnvironmentVarArgs { Name = "APPLICATIONINSIGHTS_CONNECTION_STRING", Value = appInsightsConnectionString },
                            new EnvironmentVarArgs { Name = "Admin__ApiKey", SecretRef = "admin-api-key" },
                            new EnvironmentVarArgs { Name = "Subscription__AutoGrant__ProDomains", SecretRef = "auto-grant-pro-domains" },
                            new EnvironmentVarArgs { Name = "AzureCommunicationServices__ConnectionString", SecretRef = "acs-connection-string" },
                            new EnvironmentVarArgs { Name = "Auth0__M2M__ClientId", SecretRef = "auth0-m2m-client-id" },
                            new EnvironmentVarArgs { Name = "Auth0__M2M__ClientSecret", SecretRef = "auth0-m2m-client-secret" },
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
        }, new CustomResourceOptions
        {
            // CD pipeline updates the container image via `az containerapp update`.
            // Without this, every `pulumi up` resets the image to the placeholder,
            // causing activation failure (quickstart listens on port 80, not 8080).
            // Traffic weights are managed by CI (staging revisions with 0% traffic),
            // so Pulumi must not reset them on the next `pulumi up`.
            IgnoreChanges = { "template.containers[0].image", "configuration.ingress.traffic" },
        });

        if (customDomainPhase == 1)
        {
            CreateApiManagedCertificate(env, containerAppsEnvironmentName, sharedResourceGroupName, apiDomain,
                new CustomResourceOptions { DependsOn = { containerApp } });
        }

        // Container Apps Jobs — polling and digest workers share the same shape,
        // differing only in name suffix, cron schedule, timeout, and WORKER_MODE.
        _ = CreateWorkerJob("poll", "*/15 * * * *", replicaTimeout: 120, workerMode: null,
            env, resourceGroup.Name, containerAppsEnvironmentId,
            acrLoginServer, acrPullIdentityId, cosmosDataIdentityId,
            cosmosAccountEndpoint, cosmosDatabase.Name, cosmosDataIdentityClientId,
            appInsightsConnectionString, acsConnectionString, tags);

        CreateWorkerJob("digest", "0 7 * * *", replicaTimeout: 600, workerMode: "digest",
            env, resourceGroup.Name, containerAppsEnvironmentId,
            acrLoginServer, acrPullIdentityId, cosmosDataIdentityId,
            cosmosAccountEndpoint, cosmosDatabase.Name, cosmosDataIdentityClientId,
            appInsightsConnectionString, acsConnectionString, tags);

        CreateWorkerJob("digest-hourly", "0 * * * *", replicaTimeout: 300, workerMode: "hourly-digest",
            env, resourceGroup.Name, containerAppsEnvironmentId,
            acrLoginServer, acrPullIdentityId, cosmosDataIdentityId,
            cosmosAccountEndpoint, cosmosDatabase.Name, cosmosDataIdentityClientId,
            appInsightsConnectionString, acsConnectionString, tags);

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

        // Static Web App Custom Domain
        // Apex domains (no subdomain) require TXT validation; subdomains use default CNAME.
        var isApexDomain = frontendDomain.Split('.').Length == 2;
        var swaCustomDomainArgs = new StaticSiteCustomDomainArgs
        {
            Name = staticWebApp.Name,
            DomainName = frontendDomain,
            ResourceGroupName = resourceGroup.Name,
        };

        if (isApexDomain)
        {
            swaCustomDomainArgs.ValidationMethod = "dns-txt-token";
        }

        _ = new StaticSiteCustomDomain($"swa-domain-{env}", swaCustomDomainArgs);

        return new Dictionary<string, object?>
        {
            ["resourceGroupName"] = resourceGroup.Name,
            ["cosmosAccountEndpoint"] = cosmosAccountEndpoint,
            ["staticWebAppName"] = staticWebApp.Name,
        };
    }

    private static ManagedCertificate CreateApiManagedCertificate(
        string env,
        Output<string> environmentName,
        Output<string> resourceGroupName,
        string subjectName,
        CustomResourceOptions? options = null)
    {
        return new ManagedCertificate($"cert-api-{env}", new ManagedCertificateArgs
        {
            EnvironmentName = environmentName,
            ManagedCertificateName = $"cert-api-{env}",
            ResourceGroupName = resourceGroupName,
            Properties = new ManagedCertificatePropertiesArgs
            {
                SubjectName = subjectName,
                DomainControlValidation = "CNAME",
            },
        }, options);
    }

    /// <summary>
    /// Creates a Container Apps Job for a background worker.
    /// All worker jobs share the same container shape, identity, and base env vars;
    /// they differ only in name suffix, cron schedule, timeout, and optional WORKER_MODE.
    /// </summary>
    private static Job CreateWorkerJob(
        string nameSuffix,
        string cronExpression,
        int replicaTimeout,
        string? workerMode,
        string env,
        Output<string> resourceGroupName,
        Output<string> environmentId,
        Output<string> acrLoginServer,
        Output<string> acrPullIdentityId,
        Output<string> cosmosDataIdentityId,
        Output<string> cosmosAccountEndpoint,
        Output<string> cosmosDatabaseName,
        Output<string> cosmosDataIdentityClientId,
        Output<string> appInsightsConnectionString,
        Output<string> acsConnectionString,
        InputMap<string> tags)
    {
        var envVars = new List<EnvironmentVarArgs>
        {
            new() { Name = "OTEL_SERVICE_NAME", Value = "town-crier-worker" },
            new() { Name = "Cosmos__AccountEndpoint", Value = cosmosAccountEndpoint },
            new() { Name = "Cosmos__DatabaseName", Value = cosmosDatabaseName },
            new() { Name = "AZURE_CLIENT_ID", Value = cosmosDataIdentityClientId },
            new() { Name = "APPLICATIONINSIGHTS_CONNECTION_STRING", Value = appInsightsConnectionString },
            new() { Name = "AzureCommunicationServices__ConnectionString", SecretRef = "acs-connection-string" },
        };

        if (workerMode is not null)
        {
            envVars.Insert(1, new EnvironmentVarArgs { Name = "WORKER_MODE", Value = workerMode });
        }

        return new Job($"job-town-crier-{nameSuffix}-{env}", new JobArgs
        {
            JobName = $"job-town-crier-{nameSuffix}-{env}",
            ResourceGroupName = resourceGroupName,
            EnvironmentId = environmentId,
            Configuration = new JobConfigurationArgs
            {
                TriggerType = Pulumi.AzureNative.App.TriggerType.Schedule,
                ReplicaTimeout = replicaTimeout,
                ScheduleTriggerConfig = new JobConfigurationScheduleTriggerConfigArgs
                {
                    CronExpression = cronExpression,
                    Parallelism = 1,
                    ReplicaCompletionCount = 1,
                },
                Registries = new[]
                {
                    new RegistryCredentialsArgs
                    {
                        Server = acrLoginServer,
                        Identity = acrPullIdentityId,
                    },
                },
                Secrets = new[]
                {
                    new SecretArgs { Name = "acs-connection-string", Value = acsConnectionString },
                },
            },
            Identity = new Pulumi.AzureNative.App.Inputs.ManagedServiceIdentityArgs
            {
                Type = ManagedServiceIdentityType.UserAssigned,
                UserAssignedIdentities = new InputList<string>
                {
                    acrPullIdentityId,
                    cosmosDataIdentityId,
                },
            },
            Template = new JobTemplateArgs
            {
                Containers = new[]
                {
                    new ContainerArgs
                    {
                        Name = "worker",
                        Image = "mcr.microsoft.com/k8se/quickstart:latest",
                        Resources = new ContainerResourcesArgs
                        {
                            Cpu = ContainerCpu,
                            Memory = ContainerMemory,
                        },
                        Env = envVars.ToArray(),
                    },
                },
            },
            Tags = tags,
        }, new CustomResourceOptions
        {
            // CD pipeline updates the container image via `az containerapp job update`.
            IgnoreChanges = { "template.containers[0].image" },
        });
    }
}
