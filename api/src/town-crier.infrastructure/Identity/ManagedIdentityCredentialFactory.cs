using Azure.Core;
using Azure.Identity;

namespace TownCrier.Infrastructure.Identity;

/// <summary>
/// Builds the <see cref="TokenCredential"/> used for all Azure data-plane and
/// management-plane calls (Cosmos, Service Bus, ARM).
/// </summary>
/// <remarks>
/// The container runs with two user-assigned managed identities
/// (id-town-crier-acr-pull and id-town-crier-cosmos-data). Using
/// <c>DefaultAzureCredential</c> walks the full credential chain
/// (Environment → WorkloadIdentity → ManagedIdentity), probing and failing the
/// first links — absent in Azure Container Apps — before reaching IMDS, costing
/// ~3s on the first token fetch after a deploy or restart.
///
/// <see cref="ManagedIdentityCredential"/> pinned to the cosmos-data client ID
/// (injected as the <c>AZURE_CLIENT_ID</c> env var) skips chain-probing, is
/// deterministic about which identity to use, and is more AOT-trim-friendly.
/// When no client ID is supplied (e.g. local development) it falls back to the
/// system-assigned / default managed identity behaviour.
/// </remarks>
internal static class ManagedIdentityCredentialFactory
{
    public static TokenCredential Create(string? clientId) =>
        new ManagedIdentityCredential(
            string.IsNullOrWhiteSpace(clientId)
                ? ManagedIdentityId.SystemAssigned
                : ManagedIdentityId.FromUserAssignedClientId(clientId));
}
