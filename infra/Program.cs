using Pulumi;
using Pulumi.AzureNative.Resources;
using Pulumi.AzureNative.OperationalInsights;
using Pulumi.AzureNative.App;
using Pulumi.AzureNative.App.Inputs;
using Pulumi.AzureNative.CosmosDB;
using Pulumi.AzureNative.CosmosDB.Inputs;
using Pulumi.AzureNative.ContainerRegistry;

return await Pulumi.Deployment.RunAsync(() =>
{
    var config = new Config("town-crier");
    var env = config.Require("environment");
    var cosmosConsistencyLevel = config.Require("cosmosConsistencyLevel");

    var tags = new InputMap<string>
    {
        { "project", "town-crier" },
        { "managedBy", "pulumi" },
        { "environment", env },
    };

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

    // Azure Container Registry
    var containerRegistry = new Registry($"acrtowncrier{env}", new RegistryArgs
    {
        RegistryName = $"acrtowncrier{env}",
        ResourceGroupName = resourceGroup.Name,
        Sku = new Pulumi.AzureNative.ContainerRegistry.Inputs.SkuArgs
        {
            Name = SkuName.Basic,
        },
        AdminUserEnabled = true,
        Tags = tags,
    });

    var acrCredentials = Output.Tuple(resourceGroup.Name, containerRegistry.Name)
        .Apply(names => ListRegistryCredentials.InvokeAsync(new ListRegistryCredentialsArgs
        {
            ResourceGroupName = names.Item1,
            RegistryName = names.Item2,
        }));

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
                    Server = containerRegistry.LoginServer,
                    Username = acrCredentials.Apply(c => c.Username ?? ""),
                    PasswordSecretRef = "acr-password",
                },
            },
            Secrets = new[]
            {
                new SecretArgs
                {
                    Name = "acr-password",
                    Value = acrCredentials.Apply(c =>
                        c.Passwords.Any() ? c.Passwords[0].Value ?? "" : ""),
                },
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

    return new Dictionary<string, object?>
    {
        ["resourceGroupName"] = resourceGroup.Name,
        ["containerAppsEnvironmentId"] = containerAppsEnv.Id,
        ["containerAppUrl"] = containerApp.LatestRevisionFqdn.Apply(fqdn => $"https://{fqdn}"),
        ["containerRegistryLoginServer"] = containerRegistry.LoginServer,
        ["cosmosAccountEndpoint"] = cosmosAccount.DocumentEndpoint,
        ["cosmosDatabaseName"] = cosmosDatabase.Name,
        ["logAnalyticsWorkspaceId"] = logAnalytics.Id,
    };
});
