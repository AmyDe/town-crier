using System.Net;
using Azure.Identity;
using Microsoft.Azure.Cosmos;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Http.Resilience;
using Polly;

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

        services.AddHttpClient("CosmosRest", client =>
        {
            client.BaseAddress = new Uri(accountEndpoint);
        })
        .AddResilienceHandler("CosmosRetry", builder =>
        {
            builder.AddRetry(new HttpRetryStrategyOptions
            {
                MaxRetryAttempts = 5,
                BackoffType = DelayBackoffType.Exponential,
                Delay = TimeSpan.FromMilliseconds(500),
                ShouldHandle = args => ValueTask.FromResult(
                    args.Outcome.Result?.StatusCode is
                        HttpStatusCode.TooManyRequests or // 429
                        HttpStatusCode.RequestTimeout or // 408
                        HttpStatusCode.ServiceUnavailable or // 503
                        (HttpStatusCode)449), // 449 Retry With
            });
        });

        services.AddSingleton<ICosmosRestClient>(sp =>
        {
            var factory = sp.GetRequiredService<IHttpClientFactory>();
            var httpClient = factory.CreateClient("CosmosRest");
            var auth = sp.GetRequiredService<CosmosAuthProvider>();
            var opts = sp.GetRequiredService<CosmosRestOptions>();
            return new CosmosRestClient(httpClient, auth, opts);
        });

        return services;
    }
}
