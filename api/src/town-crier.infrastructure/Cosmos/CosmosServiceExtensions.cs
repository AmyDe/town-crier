using Azure.Identity;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;

namespace TownCrier.Infrastructure.Cosmos;

public static class CosmosServiceExtensions
{
    /// <summary>
    /// Bounded retry budget for Cosmos 429s.
    /// 3 attempts × 750ms cap × 1500ms total budget keeps user-facing
    /// requests under a predictable p99 even when the partition is throttling.
    /// </summary>
    private const int CosmosMaxRetryAttempts = 3;

    private static readonly TimeSpan CosmosTotalRetryWaitBudget = TimeSpan.FromMilliseconds(1500);
    private static readonly TimeSpan CosmosPerAttemptCap = TimeSpan.FromMilliseconds(750);

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

        services.AddTransient<CosmosThrottleRetryHandler>(_ => new CosmosThrottleRetryHandler(
            maxAttempts: CosmosMaxRetryAttempts,
            totalWaitBudget: CosmosTotalRetryWaitBudget,
            perAttemptCap: CosmosPerAttemptCap,
#pragma warning disable CA5394 // Jitter for retry backoff — non-security-sensitive randomness.
            jitter: ms => Random.Shared.Next(0, ms),
#pragma warning restore CA5394
            delay: Task.Delay));

        services.AddHttpClient("CosmosRest", client =>
        {
            client.BaseAddress = new Uri(accountEndpoint);
        })
        .AddHttpMessageHandler<CosmosThrottleRetryHandler>();

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
