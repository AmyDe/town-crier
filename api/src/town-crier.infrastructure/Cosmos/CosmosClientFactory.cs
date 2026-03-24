using System.Text.Json;
using Microsoft.Azure.Cosmos;

namespace TownCrier.Infrastructure.Cosmos;

public static class CosmosClientFactory
{
    public static CosmosClient Create(string connectionString)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(connectionString);

        var jsonOptions = new JsonSerializerOptions
        {
            PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        };
        jsonOptions.TypeInfoResolverChain.Add(CosmosJsonSerializerContext.Default);

        var cosmosOptions = new CosmosClientOptions
        {
            Serializer = new SystemTextJsonCosmosSerializer(jsonOptions),
        };

        return new CosmosClient(connectionString, cosmosOptions);
    }
}
