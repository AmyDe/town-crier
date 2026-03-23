using System.Collections.Immutable;
using Pulumi;
using Pulumi.AzureNative.Resources;
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
        var frontendDomain = config.Require("frontendDomain");
        var apiDomain = config.Require("apiDomain");
        var auth0Domain = config.Require("auth0Domain");
        var auth0Audience = config.Require("auth0Audience");
        var customDomainPhase = config.GetInt32("customDomainPhase") ?? 2;

        // Shared stack outputs
        var shared = new StackReference("AmyDe/town-crier/shared");
        var acrLoginServer = shared.GetOutput("containerRegistryLoginServer").Apply(o => o?.ToString() ?? "");
        var acrPullIdentityId = shared.GetOutput("acrPullIdentityId").Apply(o => o?.ToString() ?? "");
        var acrPullIdentityClientId = shared.GetOutput("acrPullIdentityClientId").Apply(o => o?.ToString() ?? "");
        var containerAppsEnvironmentId = shared.GetOutput("containerAppsEnvironmentId").Apply(o => o?.ToString() ?? "");
        var cosmosAccountName = shared.GetOutput("cosmosAccountName").Apply(o => o?.ToString() ?? "");
        var cosmosAccountEndpoint = shared.GetOutput("cosmosAccountEndpoint").Apply(o => o?.ToString() ?? "");
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

        // Cosmos DB Containers

        // Applications container — partitioned by authority code, spatial index on location
        var applicationsContainer = new SqlResourceSqlContainer($"container-applications-{env}", new SqlResourceSqlContainerArgs
        {
            AccountName = cosmosAccountName,
            ResourceGroupName = sharedResourceGroupName,
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
            AccountName = cosmosAccountName,
            ResourceGroupName = sharedResourceGroupName,
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
            AccountName = cosmosAccountName,
            ResourceGroupName = sharedResourceGroupName,
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
            AccountName = cosmosAccountName,
            ResourceGroupName = sharedResourceGroupName,
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
            AccountName = cosmosAccountName,
            ResourceGroupName = sharedResourceGroupName,
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

        // Managed Certificate for API custom domain
        // Phase 1 (first deploy): Container App created first with disabled binding,
        //   then cert created with DependsOn so Azure can validate the hostname.
        // Phase 2 (after cert provisioned): Cert created first, Container App
        //   binds it with SniEnabled. Set customDomainPhase in Pulumi config.
        ManagedCertificate? apiManagedCert = null;

        if (customDomainPhase >= 2)
        {
            apiManagedCert = new ManagedCertificate($"cert-api-{env}", new ManagedCertificateArgs
            {
                EnvironmentName = containerAppsEnvironmentName,
                ManagedCertificateName = $"cert-api-{env}",
                ResourceGroupName = sharedResourceGroupName,
                Properties = new ManagedCertificatePropertiesArgs
                {
                    SubjectName = apiDomain,
                    DomainControlValidation = "CNAME",
                },
            });
        }

        // Container App (API) — placeholder image until CI/CD pushes real builds
        var containerApp = new ContainerApp($"ca-town-crier-api-{env}", new ContainerAppArgs
        {
            ContainerAppName = $"ca-town-crier-api-{env}",
            ResourceGroupName = resourceGroup.Name,
            ManagedEnvironmentId = containerAppsEnvironmentId,
            Configuration = new ConfigurationArgs
            {
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
                        Env = new[]
                        {
                            new EnvironmentVarArgs { Name = "Auth0__Domain", Value = auth0Domain },
                            new EnvironmentVarArgs { Name = "Auth0__Audience", Value = auth0Audience },
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
            IgnoreChanges = { "template.containers[0].image" },
        });

        if (customDomainPhase == 1)
        {
            new ManagedCertificate($"cert-api-{env}", new ManagedCertificateArgs
            {
                EnvironmentName = containerAppsEnvironmentName,
                ManagedCertificateName = $"cert-api-{env}",
                ResourceGroupName = sharedResourceGroupName,
                Properties = new ManagedCertificatePropertiesArgs
                {
                    SubjectName = apiDomain,
                    DomainControlValidation = "CNAME",
                },
            }, new CustomResourceOptions { DependsOn = { containerApp } });
        }

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

        var staticWebAppCustomDomain = new StaticSiteCustomDomain($"swa-domain-{env}", swaCustomDomainArgs);

        return new Dictionary<string, object?>
        {
            ["resourceGroupName"] = resourceGroup.Name,
            ["containerAppUrl"] = containerApp.LatestRevisionFqdn.Apply(fqdn => $"https://{fqdn}"),
            ["cosmosAccountEndpoint"] = cosmosAccountEndpoint,
            ["cosmosDatabaseName"] = cosmosDatabase.Name,
            ["staticWebAppUrl"] = staticWebApp.DefaultHostname.Apply(hostname => $"https://{hostname}"),
            ["staticWebAppName"] = staticWebApp.Name,
        };
    }
}
