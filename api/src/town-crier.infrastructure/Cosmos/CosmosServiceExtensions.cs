using Azure.Identity;
using Microsoft.Azure.Cosmos;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;

namespace TownCrier.Infrastructure.Cosmos;

public static class CosmosServiceExtensions
{
    public static IServiceCollection AddCosmosClient(this IServiceCollection services, IConfiguration configuration)
    {
        services.AddSingleton(_ =>
        {
            var accountEndpoint = configuration["Cosmos:AccountEndpoint"];
            if (!string.IsNullOrWhiteSpace(accountEndpoint))
            {
                return CosmosClientFactory.Create(accountEndpoint, new DefaultAzureCredential());
            }

            var connectionString = configuration.GetConnectionString("CosmosDb");
            if (!string.IsNullOrWhiteSpace(connectionString))
            {
                return CosmosClientFactory.Create(connectionString);
            }

            throw new InvalidOperationException(
                "Cosmos DB is not configured. Set 'Cosmos:AccountEndpoint' (managed identity) "
                + "or 'ConnectionStrings:CosmosDb' (connection string).");
        });

        return services;
    }
}
