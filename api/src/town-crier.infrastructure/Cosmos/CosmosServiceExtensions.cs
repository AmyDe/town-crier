using System.Text.Json;
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
            var connectionString = configuration.GetConnectionString("CosmosDb")
                ?? throw new InvalidOperationException(
                    "Cosmos DB connection string is required. Set 'ConnectionStrings:CosmosDb' in configuration.");

            return CosmosClientFactory.Create(connectionString);
        });

        return services;
    }
}
