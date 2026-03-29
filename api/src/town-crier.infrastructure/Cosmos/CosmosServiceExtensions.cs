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

    public static IServiceCollection AddCosmosRestClient(
        this IServiceCollection services, IConfiguration configuration)
    {
        ArgumentNullException.ThrowIfNull(configuration);

        var cosmosSection = configuration.GetSection("Cosmos");
        var accountEndpoint = cosmosSection["AccountEndpoint"]
            ?? throw new InvalidOperationException("Cosmos:AccountEndpoint configuration is required.");
        var databaseName = cosmosSection["DatabaseName"]
            ?? throw new InvalidOperationException("Cosmos:DatabaseName configuration is required.");

        var options = new CosmosRestOptions
        {
            AccountEndpoint = accountEndpoint,
            DatabaseName = databaseName,
        };

        services.AddSingleton(options);

#pragma warning disable CA2000 // DI container owns the lifetime and will dispose on shutdown
        services.AddSingleton(new CosmosAuthProvider(new DefaultAzureCredential()));
#pragma warning restore CA2000

        return services;
    }
}
