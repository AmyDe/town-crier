using Pulumi;
using Pulumi.AzureNative.Resources;
using Pulumi.AzureNative.ContainerRegistry;
using Pulumi.AzureNative.ManagedIdentity;
using Pulumi.AzureNative.Authorization;

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

        return new Dictionary<string, object?>
        {
            ["resourceGroupName"] = resourceGroup.Name,
            ["containerRegistryLoginServer"] = containerRegistry.LoginServer,
            ["acrPullIdentityId"] = acrPullIdentity.Id,
            ["acrPullIdentityClientId"] = acrPullIdentity.ClientId,
        };
    }
}
