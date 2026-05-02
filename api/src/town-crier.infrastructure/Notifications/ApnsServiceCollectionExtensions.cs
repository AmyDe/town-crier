using System.Net;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;
using TownCrier.Application.Notifications;

namespace TownCrier.Infrastructure.Notifications;

/// <summary>
/// DI registration helpers for the APNs push pipeline. Reads <c>"Apns"</c>
/// configuration once, validates it at startup, and registers either the
/// no-op sender (local dev / disabled) or the real
/// <see cref="ApnsPushNotificationSender"/> + <see cref="ApnsJwtProvider"/> +
/// HTTP/2 <see cref="HttpClient"/> (production / TestFlight).
/// </summary>
public static class ApnsServiceCollectionExtensions
{
    /// <summary>
    /// The named <see cref="HttpClient"/> registration used by
    /// <see cref="ApnsPushNotificationSender"/>. Configured for HTTP/2 with
    /// <see cref="HttpVersionPolicy.RequestVersionExact"/> per spec — APNs
    /// requires HTTP/2 and rejects HTTP/1.1 with a 400.
    /// </summary>
    public const string HttpClientName = "Apns";

    /// <summary>
    /// Registers the APNs push notification pipeline. When
    /// <c>Apns:Enabled</c> is <c>false</c> (the default), a
    /// <see cref="NoOpPushNotificationSender"/> is registered so missing
    /// auth keys do not crash local dev. When <c>true</c>, the options are
    /// validated, the JWT provider is registered as a singleton, a named
    /// HTTP/2 <see cref="HttpClient"/> is configured, and the real
    /// <see cref="ApnsPushNotificationSender"/> is wired in. The host must
    /// already have <see cref="TimeProvider"/> and the logging infrastructure
    /// registered before calling this method.
    /// </summary>
    /// <param name="services">The DI container.</param>
    /// <param name="configuration">The configuration root.</param>
    /// <returns>The same <see cref="IServiceCollection"/> for chaining.</returns>
    /// <exception cref="InvalidOperationException">Thrown when <c>Apns:Enabled</c> is true but required fields are missing or malformed.</exception>
    public static IServiceCollection AddApnsPushNotifications(
        this IServiceCollection services, IConfiguration configuration)
    {
        ArgumentNullException.ThrowIfNull(services);
        ArgumentNullException.ThrowIfNull(configuration);

        var options = ApnsOptions.LoadFromConfiguration(configuration);
        options.Validate();
        services.AddSingleton(options);

        if (!options.Enabled)
        {
            services.AddSingleton<IPushNotificationSender, NoOpPushNotificationSender>();
            return services;
        }

        services.AddSingleton(sp => new ApnsJwtProvider(
            options.AuthKey,
            options.KeyId,
            options.TeamId,
            sp.GetRequiredService<TimeProvider>()));

        // HTTP/2 + RequestVersionExact: APNs only speaks HTTP/2 and rejects
        // HTTP/1.1 with 400 BadHttpMethodCalled. The base URL is derived from
        // UseSandbox per spec (api.push.apple.com vs api.sandbox.push.apple.com).
        services.AddHttpClient(HttpClientName, client =>
        {
            client.BaseAddress = options.ResolveBaseAddress();
            client.DefaultRequestVersion = HttpVersion.Version20;
            client.DefaultVersionPolicy = HttpVersionPolicy.RequestVersionExact;
        });

        services.AddSingleton<IPushNotificationSender>(sp =>
        {
            var factory = sp.GetRequiredService<IHttpClientFactory>();
            var httpClient = factory.CreateClient(HttpClientName);
            return new ApnsPushNotificationSender(
                httpClient,
                sp.GetRequiredService<ApnsJwtProvider>(),
                sp.GetRequiredService<ApnsOptions>(),
                sp.GetRequiredService<ILogger<ApnsPushNotificationSender>>(),
                sp.GetRequiredService<TimeProvider>());
        });

        return services;
    }
}
