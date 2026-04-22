using System.Collections.Immutable;
using Pulumi;
using Pulumi.AzureNative.Authorization;
using Pulumi.AzureNative.Resources;
using Pulumi.AzureNative.App;
using Pulumi.AzureNative.App.Inputs;
using Pulumi.AzureNative.CosmosDB;
using Pulumi.AzureNative.CosmosDB.Inputs;
using Pulumi.AzureNative.ServiceBus;
using Pulumi.AzureNative.Web;
using ManagedServiceIdentityType = Pulumi.AzureNative.App.ManagedServiceIdentityType;
using ServiceBusSkuArgs = Pulumi.AzureNative.ServiceBus.Inputs.SBSkuArgs;
using ServiceBusSkuName = Pulumi.AzureNative.ServiceBus.SkuName;
using ServiceBusSkuTier = Pulumi.AzureNative.ServiceBus.SkuTier;

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
        var appInsightsId = shared.GetOutput("appInsightsId").Apply(o => o?.ToString() ?? "");
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

            // OfferCodes — partitioned by code for point reads on redemption
            new("OfferCodes", "/code"),
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

        // Service Bus — adaptive polling trigger (prod only for now; dev stays on cron).
        // The polling worker consumes one message per run and re-enqueues the next run with
        // a scheduled enqueue time calculated from Retry-After headers and natural cadence.
        // Worker identity (cosmosDataIdentity) gets Data Owner RBAC on the namespace so it
        // can both send and receive without SAS keys.
        ServiceBusPollingInfra? pollingBus = null;
        if (env == "prod")
        {
            pollingBus = CreateServiceBusPollingInfra(
                env, resourceGroup.Name,
                cosmosDataIdentityPrincipalId: shared.GetOutput("cosmosDataIdentityPrincipalId").Apply(o => o?.ToString() ?? ""),
                tags);
        }

        // Container Apps Jobs — polling and digest workers share the same shape,
        // differing only in name suffix, cron schedule, timeout, and WORKER_MODE.
        //
        // In prod, the "poll" job is event-triggered off the Service Bus queue provisioned
        // above; a parallel cron-triggered "poll-bootstrap" job runs every 30 minutes and
        // re-seeds the queue if it is empty (disaster-recovery / bootstrap only — the
        // bootstrap probe is a no-op when the chain is alive).
        //
        // Dev has no poll job at all. See docs/specs/sb-only-polling.md and
        // docs/adr/0024-service-bus-only-polling.md.
        if (pollingBus is not null)
        {
            _ = CreateWorkerJob("poll", cronExpression: null, replicaTimeout: 600, workerMode: "poll-sb",
                env, resourceGroup.Name, containerAppsEnvironmentId,
                acrLoginServer, acrPullIdentityId, cosmosDataIdentityId,
                cosmosAccountEndpoint, cosmosDatabase.Name, cosmosDataIdentityClientId,
                appInsightsConnectionString, acsConnectionString, tags,
                pollingBus);

            _ = CreateWorkerJob("poll-bootstrap", cronExpression: "*/30 * * * *", replicaTimeout: 120, workerMode: "poll-bootstrap",
                env, resourceGroup.Name, containerAppsEnvironmentId,
                acrLoginServer, acrPullIdentityId, cosmosDataIdentityId,
                cosmosAccountEndpoint, cosmosDatabase.Name, cosmosDataIdentityClientId,
                appInsightsConnectionString, acsConnectionString, tags,
                pollingBus);
        }

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

        // Dormant account cleanup — daily at 03:30 UTC (off-peak, avoids top-of-hour
        // digest-hourly run). Cascades UK GDPR Art.5(1)(e) erasure for UserProfiles
        // with LastActiveAt older than 12 months.
        CreateWorkerJob("dormant-cleanup", "30 3 * * *", replicaTimeout: 600, workerMode: "dormant-cleanup",
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
    /// they differ by trigger type: cronExpression=null + non-null pollingBus produces an
    /// Event-triggered job backed by a Service Bus queue; otherwise a Schedule-triggered
    /// cron job. When pollingBus is non-null (in any mode) the Service Bus env vars are
    /// surfaced so the worker can send/receive messages.
    /// </summary>
    private static Job CreateWorkerJob(
        string nameSuffix,
        string? cronExpression,
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
        InputMap<string> tags,
        ServiceBusPollingInfra? pollingBus = null)
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

        if (pollingBus is not null)
        {
            envVars.Add(new EnvironmentVarArgs { Name = "ServiceBus__Namespace", Value = pollingBus.NamespaceFqdn });
            envVars.Add(new EnvironmentVarArgs { Name = "ServiceBus__QueueName", Value = pollingBus.QueueName });
        }

        var useEventTrigger = cronExpression is null;
        if (useEventTrigger && pollingBus is null)
        {
            throw new ArgumentException(
                "Event-triggered jobs require a ServiceBusPollingInfra (queue + namespace).",
                nameof(pollingBus));
        }

        var configuration = new JobConfigurationArgs
        {
            ReplicaTimeout = replicaTimeout,
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
        };

        if (useEventTrigger)
        {
            // KEDA azure-servicebus scaler — authenticates to the namespace with the
            // user-assigned managed identity (no connection string / SAS key). The
            // worker itself must also have RBAC on the namespace for send+receive,
            // which the pollingBus role assignment already provides.
            configuration.TriggerType = Pulumi.AzureNative.App.TriggerType.Event;
            configuration.EventTriggerConfig = new JobConfigurationEventTriggerConfigArgs
            {
                Parallelism = 1,
                ReplicaCompletionCount = 1,
                Scale = new JobScaleArgs
                {
                    MinExecutions = 0,
                    MaxExecutions = 1,
                    PollingInterval = 30,
                    Rules = new[]
                    {
                        new JobScaleRuleArgs
                        {
                            Name = "servicebus-queue",
                            Type = "azure-servicebus",
                            Identity = cosmosDataIdentityId,
                            Metadata = new InputMap<object>
                            {
                                ["namespace"] = pollingBus!.NamespaceShortName,
                                ["queueName"] = pollingBus.QueueName,
                                ["messageCount"] = "1",
                            },
                        },
                    },
                },
            };
        }
        else
        {
            configuration.TriggerType = Pulumi.AzureNative.App.TriggerType.Schedule;
            configuration.ScheduleTriggerConfig = new JobConfigurationScheduleTriggerConfigArgs
            {
                CronExpression = cronExpression!,
                Parallelism = 1,
                ReplicaCompletionCount = 1,
            };
        }

        return new Job($"job-tc-{nameSuffix}-{env}", new JobArgs
        {
            JobName = $"job-tc-{nameSuffix}-{env}",
            ResourceGroupName = resourceGroupName,
            EnvironmentId = environmentId,
            Configuration = configuration,
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

    /// <summary>
    /// Captures the Service Bus resources used by the adaptive polling trigger:
    /// namespace (short name + FQDN for KEDA/app env vars) and queue name. The role
    /// assignment is created inside <see cref="CreateServiceBusPollingInfra"/> and
    /// does not need to be passed downstream.
    /// </summary>
    private sealed record ServiceBusPollingInfra(
        Output<string> NamespaceShortName,
        Output<string> NamespaceFqdn,
        Output<string> QueueName);

    /// <summary>
    /// Provisions the Service Bus namespace + queue + RBAC used by the adaptive polling
    /// trigger. The worker's user-assigned managed identity is granted:
    /// - "Azure Service Bus Data Owner" at the namespace scope — data-plane send/receive.
    /// - "Reader" at the queue scope — management-plane GET so the bootstrap probe can
    ///   read countDetails (activeMessageCount + scheduledMessageCount) to decide
    ///   whether the polling chain is alive (see ADR 0024 + tc-ujl1).
    /// </summary>
    private static ServiceBusPollingInfra CreateServiceBusPollingInfra(
        string env,
        Output<string> resourceGroupName,
        Output<string> cosmosDataIdentityPrincipalId,
        InputMap<string> tags)
    {
        // Basic tier supports queues and scheduled messages, which is all the adaptive
        // polling loop needs. Basic is ~$0.05 per million operations — pennies/month at
        // our volume (one message per poll cycle).
        //
        // Location is pinned to "uksouth" rather than inheriting from the parent RG,
        // because rg-town-crier-prod's metadata location is "ukwest" (cosmetic / immutable)
        // while every other compute resource in the RG is explicitly in uksouth. Without
        // this override the namespace lands in ukwest (see tc-ds1e).
        var namespaceResource = new Namespace($"sb-town-crier-{env}", new NamespaceArgs
        {
            NamespaceName = $"sb-town-crier-{env}",
            ResourceGroupName = resourceGroupName,
            Location = "uksouth",
            Sku = new ServiceBusSkuArgs
            {
                Name = ServiceBusSkuName.Basic,
                Tier = ServiceBusSkuTier.Basic,
            },
            Tags = tags,
        });

        // Polling trigger queue. The worker receives one message in PeekLock mode, runs
        // the poll cycle, publishes the next message (ScheduledEnqueueTimeUtc computed
        // from Retry-After / natural cadence), then completes the receive.
        //
        // - DefaultMessageTimeToLive = 1h — longer than any Retry-After we would honour.
        //   Messages scheduled further out than this risk being dropped before delivery,
        //   but the worker caps Retry-After at 30min so 1h TTL is comfortable.
        // - LockDuration = 5min (PT5M) — Azure Service Bus caps LockDuration at 5 minutes
        //   across all tiers; setting higher returns MessagingGatewayBadRequest. This is
        //   less than replicaTimeout (600s) so a slow poll cycle could trigger a mid-run
        //   redelivery, but duplicate polls are safe thanks to the Cosmos lease guard in
        //   the worker, so we'd rather retry than dead-letter.
        // - MaxDeliveryCount = 10 — generous; duplicate polls are safe thanks to the
        //   Cosmos lease guard in the worker, so we'd rather retry than dead-letter.
        var queue = new Queue($"sbq-poll-{env}", new QueueArgs
        {
            QueueName = "poll",
            NamespaceName = namespaceResource.Name,
            ResourceGroupName = resourceGroupName,
            DefaultMessageTimeToLive = "PT1H",
            LockDuration = "PT5M",
            MaxDeliveryCount = 10,
            DeadLetteringOnMessageExpiration = true,
        });

        // Built-in role: Azure Service Bus Data Owner
        // (https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles#azure-service-bus-data-owner)
        // Grants both send and receive, plus queue/topic management. Sufficient for the
        // worker's publish-before-ack flow without splitting into Sender + Receiver.
        // Scoped to the namespace so it covers any future queues added for the same worker.
        const string serviceBusDataOwnerRoleId = "090c5cfd-751d-490a-894a-3ce6f1109419";
        var subscriptionId = namespaceResource.Id.Apply(id => id.Split('/')[2]);
        _ = new RoleAssignment($"sb-poll-data-owner-{env}", new RoleAssignmentArgs
        {
            Scope = namespaceResource.Id,
            RoleDefinitionId = subscriptionId.Apply(subId =>
                $"/subscriptions/{subId}/providers/Microsoft.Authorization/roleDefinitions/{serviceBusDataOwnerRoleId}"),
            PrincipalId = cosmosDataIdentityPrincipalId,
            PrincipalType = PrincipalType.ServicePrincipal,
        });

        // Built-in role: Reader
        // (https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles/general#reader)
        // Grants read-only access to the queue's ARM resource, which is what the
        // management-plane bootstrap probe needs:
        //   GET https://management.azure.com/.../Microsoft.ServiceBus/namespaces/{ns}/queues/{queue}?api-version=...
        // The response includes countDetails.activeMessageCount +
        // countDetails.scheduledMessageCount — the signals the bootstrap worker uses
        // to decide whether to re-seed the polling chain (see ADR 0024 + tc-ujl1).
        //
        // The data-plane Service Bus roles (Data Owner / Receiver / Sender) do NOT
        // grant access to the management plane, so a separate assignment is required.
        // There is no narrower built-in role that exposes queue countDetails, so
        // Reader is the least-privilege option. Scoped to the queue itself (not the
        // namespace) so it covers only this one queue resource.
        const string readerRoleId = "acdd72a7-3385-48ef-bd42-f606fba81ae7";
        _ = new RoleAssignment($"sb-poll-queue-reader-{env}", new RoleAssignmentArgs
        {
            Scope = queue.Id,
            RoleDefinitionId = subscriptionId.Apply(subId =>
                $"/subscriptions/{subId}/providers/Microsoft.Authorization/roleDefinitions/{readerRoleId}"),
            PrincipalId = cosmosDataIdentityPrincipalId,
            PrincipalType = PrincipalType.ServicePrincipal,
        });

        var fqdn = namespaceResource.Name.Apply(n => $"{n}.servicebus.windows.net");

        return new ServiceBusPollingInfra(
            NamespaceShortName: namespaceResource.Name,
            NamespaceFqdn: fqdn,
            QueueName: queue.Name);
    }
}
