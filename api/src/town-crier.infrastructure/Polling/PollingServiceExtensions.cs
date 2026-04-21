using System.Globalization;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.ServiceBus;

namespace TownCrier.Infrastructure.Polling;

/// <summary>
/// DI wiring for the adaptive Service-Bus-driven polling chain. Registers the
/// application-layer orchestrator, the next-run scheduler, the Cosmos-backed
/// lease store, the system-random jitter, and the Service Bus-backed trigger
/// queue adapter. The <see cref="PollPlanItCommandHandler"/> itself is
/// registered by the host because its dependencies (repositories, PlanIt
/// client, cycle selector, etc.) are host-specific.
/// </summary>
public static class PollingServiceExtensions
{
    public static IServiceCollection AddPollingInfrastructure(
        this IServiceCollection services, IConfiguration configuration)
    {
        ArgumentNullException.ThrowIfNull(services);
        ArgumentNullException.ThrowIfNull(configuration);

        var schedulerOptions = BuildSchedulerOptions(configuration);
        services.AddSingleton(schedulerOptions);

        services.AddSingleton<IPollingLeaseStore, CosmosPollingLeaseStore>();
        services.AddSingleton<IPollJitter, SystemRandomPollJitter>();
        services.AddSingleton<IPollTriggerQueue>(sp => new ServiceBusPollTriggerQueue(
            sp.GetRequiredService<IServiceBusRestClient>(),
            sp.GetRequiredService<ServiceBusRestOptions>()));

        services.AddSingleton<PollNextRunScheduler>();
        services.AddSingleton<PollTriggerOrchestrator>();

        return services;
    }

    private static PollNextRunSchedulerOptions BuildSchedulerOptions(IConfiguration configuration)
    {
        var section = configuration.GetSection("Polling:Scheduler");
        var defaults = new PollNextRunSchedulerOptions();

        return new PollNextRunSchedulerOptions
        {
            NaturalCadence = ReadTimeSpan(section, "NaturalCadence", defaults.NaturalCadence),
            TimeBoundedCadence = ReadTimeSpan(section, "TimeBoundedCadence", defaults.TimeBoundedCadence),
            RetryAfterCap = ReadTimeSpan(section, "RetryAfterCap", defaults.RetryAfterCap),
            RateLimitDefault = ReadTimeSpan(section, "RateLimitDefault", defaults.RateLimitDefault),
            JitterBound = ReadTimeSpan(section, "JitterBound", defaults.JitterBound),
        };
    }

    private static TimeSpan ReadTimeSpan(
        IConfigurationSection section, string key, TimeSpan fallback)
    {
        var raw = section[key];
        if (string.IsNullOrWhiteSpace(raw))
        {
            return fallback;
        }

        return TimeSpan.TryParse(raw, CultureInfo.InvariantCulture, out var parsed) ? parsed : fallback;
    }
}
