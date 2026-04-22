using System.Net;
using Azure.Identity;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Http.Resilience;
using Polly;

namespace TownCrier.Infrastructure.ServiceBus;

public static class ServiceBusServiceExtensions
{
    public static IServiceCollection AddServiceBusRestClient(
        this IServiceCollection services, IConfiguration configuration)
    {
        ArgumentNullException.ThrowIfNull(configuration);

        var section = configuration.GetSection("ServiceBus");
        var serviceBusNamespace = section["Namespace"]
            ?? throw new InvalidOperationException("ServiceBus:Namespace configuration is required.");
        var queueName = section["QueueName"]
            ?? throw new InvalidOperationException("ServiceBus:QueueName configuration is required.");

        var options = new ServiceBusRestOptions
        {
            Namespace = serviceBusNamespace,
            QueueName = queueName,
        };

        services.AddSingleton(options);

#pragma warning disable CA2000 // DI container owns the lifetime and will dispose on shutdown
        services.AddSingleton(new ServiceBusAuthProvider(new DefaultAzureCredential()));
#pragma warning restore CA2000

        // Accept both bare namespace ("sb-town-crier-prod") and full FQDN
        // ("sb-town-crier-prod.servicebus.windows.net"). Pulumi sets the env var
        // to the FQDN (EnvironmentStack.cs), older configs pass the bare name —
        // doubling the suffix produces NXDOMAIN and silently kills the SB poll
        // bootstrap (the publish failure is swallowed by CA1031).
        var host = serviceBusNamespace.EndsWith(".servicebus.windows.net", StringComparison.OrdinalIgnoreCase)
            ? serviceBusNamespace
            : $"{serviceBusNamespace}.servicebus.windows.net";
        var baseUri = new Uri($"https://{host}");

        services.AddHttpClient("ServiceBusRest", client =>
        {
            client.BaseAddress = baseUri;
        })
        .AddResilienceHandler("ServiceBusRetry", builder =>
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
                        HttpStatusCode.ServiceUnavailable), // 503
            });
        });

        services.AddSingleton<IServiceBusRestClient>(sp =>
        {
            var factory = sp.GetRequiredService<IHttpClientFactory>();
            var httpClient = factory.CreateClient("ServiceBusRest");
            var auth = sp.GetRequiredService<ServiceBusAuthProvider>();
            var opts = sp.GetRequiredService<ServiceBusRestOptions>();
            return new ServiceBusRestClient(httpClient, auth, opts);
        });

        return services;
    }
}
