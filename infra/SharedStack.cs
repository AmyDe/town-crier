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

public static class SharedStack
{
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
            RetentionInDays = 30,
            Tags = tags,
        });

        var logAnalyticsSharedKeys = Output.Tuple(resourceGroup.Name, logAnalytics.Name)
            .Apply(names => GetSharedKeys.InvokeAsync(new GetSharedKeysArgs
            {
                ResourceGroupName = names.Item1,
                WorkspaceName = names.Item2,
            }));

        // Container Apps Environment (shared across environments)
        var containerAppsEnv = new ManagedEnvironment("cae-town-crier-shared", new ManagedEnvironmentArgs
        {
            EnvironmentName = "cae-town-crier-shared",
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

        return new Dictionary<string, object?>
        {
            ["resourceGroupName"] = resourceGroup.Name,
            ["containerRegistryLoginServer"] = containerRegistry.LoginServer,
            ["acrPullIdentityId"] = acrPullIdentity.Id,
            ["acrPullIdentityClientId"] = acrPullIdentity.ClientId,
            ["cosmosDataIdentityId"] = cosmosDataIdentity.Id,
            ["cosmosDataIdentityClientId"] = cosmosDataIdentity.ClientId,
            ["containerAppsEnvironmentId"] = containerAppsEnv.Id,
            ["cosmosAccountName"] = cosmosAccount.Name,
            ["cosmosAccountEndpoint"] = cosmosAccount.DocumentEndpoint,
        };
    }
}
