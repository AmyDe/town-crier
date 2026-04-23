using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.ServiceBus;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.Polling;

public sealed class PollingServiceExtensionsTests
{
    [Test]
    public async Task Should_RegisterLeaseStore_When_AddingPollingInfrastructure()
    {
        using var provider = BuildProvider();

        var store = provider.GetService<IPollingLeaseStore>();

        await Assert.That(store).IsNotNull();
        await Assert.That(store).IsTypeOf<CosmosPollingLeaseStore>();
    }

    [Test]
    public async Task Should_RegisterJitter_When_AddingPollingInfrastructure()
    {
        using var provider = BuildProvider();

        var jitter = provider.GetService<IPollJitter>();

        await Assert.That(jitter).IsNotNull();
        await Assert.That(jitter).IsTypeOf<SystemRandomPollJitter>();
    }

    [Test]
    public async Task Should_RegisterTriggerQueue_When_AddingPollingInfrastructure()
    {
        using var provider = BuildProvider();

        var queue = provider.GetService<IPollTriggerQueue>();

        await Assert.That(queue).IsNotNull();
        await Assert.That(queue).IsTypeOf<ServiceBusPollTriggerQueue>();
    }

    [Test]
    public async Task Should_RegisterNextRunScheduler_When_AddingPollingInfrastructure()
    {
        using var provider = BuildProvider();

        var scheduler = provider.GetService<PollNextRunScheduler>();

        await Assert.That(scheduler).IsNotNull();
    }

    [Test]
    public async Task Should_RegisterSchedulerOptions_When_AddingPollingInfrastructure()
    {
        using var provider = BuildProvider();

        var options = provider.GetService<PollNextRunSchedulerOptions>();

        await Assert.That(options).IsNotNull();
    }

    [Test]
    public async Task Should_RegisterBootstrapper_When_AddingPollingInfrastructure()
    {
        // The safety-net reseed path (bd tc-tdgf) relies on the bootstrapper
        // being resolvable from the worker host.
        using var provider = BuildProvider();

        var bootstrapper = provider.GetService<PollTriggerBootstrapper>();

        await Assert.That(bootstrapper).IsNotNull();
    }

    [Test]
    public async Task Should_BindSchedulerOptionsFromConfiguration_When_Provided()
    {
        using var provider = BuildProvider(
            ("Polling:Scheduler:NaturalCadence", "00:02:30"),
            ("Polling:Scheduler:JitterBound", "00:00:05"));

        var options = provider.GetRequiredService<PollNextRunSchedulerOptions>();

        await Assert.That(options.NaturalCadence).IsEqualTo(TimeSpan.FromSeconds(150));
        await Assert.That(options.JitterBound).IsEqualTo(TimeSpan.FromSeconds(5));
    }

    private static ServiceProvider BuildProvider(params (string Key, string Value)[] extras)
    {
        var dict = new Dictionary<string, string?>(StringComparer.Ordinal)
        {
            ["ServiceBus:Namespace"] = "sb-town-crier-test",
            ["ServiceBus:QueueName"] = "poll",
            ["ServiceBus:SubscriptionId"] = "ae5e40cd-96ef-48d8-950a-2e22cf8f991a",
            ["ServiceBus:ResourceGroup"] = "rg-town-crier-test",
        };
        foreach (var (key, value) in extras)
        {
            dict[key] = value;
        }

        var configuration = new ConfigurationBuilder()
            .AddInMemoryCollection(dict)
            .Build();

        var services = new ServiceCollection();
        services.AddSingleton(TimeProvider.System);

        // The lease store depends on ICosmosRestClient; wire a fake.
        services.AddSingleton<ICosmosRestClient>(new FakeCosmosRestClient());
        services.AddServiceBusRestClient(configuration);
        services.AddPollingInfrastructure(configuration);

        return services.BuildServiceProvider();
    }
}
