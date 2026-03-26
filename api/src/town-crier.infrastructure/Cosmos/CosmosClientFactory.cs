using System.Text.Json;
using Azure.Core;
using Microsoft.Azure.Cosmos;

namespace TownCrier.Infrastructure.Cosmos;

public static class CosmosClientFactory
{
    public static CosmosClient Create(string connectionString)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(connectionString);
        return new CosmosClient(connectionString, BuildOptions());
    }

    public static CosmosClient Create(string accountEndpoint, TokenCredential credential)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(accountEndpoint);
        ArgumentNullException.ThrowIfNull(credential);
        return new CosmosClient(accountEndpoint, credential, BuildOptions());
    }

    private static CosmosClientOptions BuildOptions()
    {
        var jsonOptions = new JsonSerializerOptions
        {
            PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        };
        jsonOptions.TypeInfoResolverChain.Add(CosmosJsonSerializerContext.Default);

        return new CosmosClientOptions
        {
            Serializer = new SystemTextJsonCosmosSerializer(jsonOptions),
        };
    }
}
