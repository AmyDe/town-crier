using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using TownCrier.Infrastructure.ServiceBus;

namespace TownCrier.Infrastructure.Tests.ServiceBus;

public sealed class ServiceBusServiceExtensionsTests
{
    [Test]
    public async Task Should_RegisterServiceBusRestClient_When_ConfigurationIsValid()
    {
        var services = new ServiceCollection();
        var configuration = BuildConfiguration(
            ("ServiceBus:Namespace", "sb-town-crier-test"),
            ("ServiceBus:QueueName", "poll"));

        services.AddServiceBusRestClient(configuration);
        using var provider = services.BuildServiceProvider();

        var client = provider.GetService<IServiceBusRestClient>();
        await Assert.That(client).IsNotNull();
    }

    [Test]
    public async Task Should_RegisterOptions_When_ConfigurationIsValid()
    {
        var services = new ServiceCollection();
        var configuration = BuildConfiguration(
            ("ServiceBus:Namespace", "sb-town-crier-test"),
            ("ServiceBus:QueueName", "poll"));

        services.AddServiceBusRestClient(configuration);
        using var provider = services.BuildServiceProvider();

        var options = provider.GetRequiredService<ServiceBusRestOptions>();
        await Assert.That(options.Namespace).IsEqualTo("sb-town-crier-test");
        await Assert.That(options.QueueName).IsEqualTo("poll");
    }

    [Test]
    public async Task Should_Throw_When_NamespaceMissing()
    {
        var services = new ServiceCollection();
        var configuration = BuildConfiguration(
            ("ServiceBus:QueueName", "poll"));

        var ex = Assert.Throws<InvalidOperationException>(() =>
            services.AddServiceBusRestClient(configuration));

        await Assert.That(ex!.Message).Contains("ServiceBus:Namespace");
    }

    [Test]
    public async Task Should_Throw_When_QueueNameMissing()
    {
        var services = new ServiceCollection();
        var configuration = BuildConfiguration(
            ("ServiceBus:Namespace", "sb-town-crier-test"));

        var ex = Assert.Throws<InvalidOperationException>(() =>
            services.AddServiceBusRestClient(configuration));

        await Assert.That(ex!.Message).Contains("ServiceBus:QueueName");
    }

    // Pulumi sets ServiceBus__Namespace to the full FQDN
    // (e.g. "sb-town-crier-prod.servicebus.windows.net"), but earlier code paths
    // and dev configs use the bare short name. The HTTP BaseAddress must be the
    // single FQDN in either case — never doubled — or the REST client gets DNS
    // NXDOMAIN and the SB-coordinated polling cycle silently fails.
    [Test]
    [Arguments("sb-town-crier-test", "https://sb-town-crier-test.servicebus.windows.net/")]
    [Arguments("sb-town-crier-prod.servicebus.windows.net", "https://sb-town-crier-prod.servicebus.windows.net/")]
    [Arguments("sb-town-crier-prod.SERVICEBUS.WINDOWS.NET", "https://sb-town-crier-prod.servicebus.windows.net/")]
    public async Task Should_BuildSingleFqdnBaseAddress_When_NamespaceIsBareOrFqdn(
        string configuredNamespace,
        string expectedBaseAddress)
    {
        var services = new ServiceCollection();
        var configuration = BuildConfiguration(
            ("ServiceBus:Namespace", configuredNamespace),
            ("ServiceBus:QueueName", "poll"));

        services.AddServiceBusRestClient(configuration);
        using var provider = services.BuildServiceProvider();

        var factory = provider.GetRequiredService<IHttpClientFactory>();
        using var httpClient = factory.CreateClient("ServiceBusRest");

        await Assert.That(httpClient.BaseAddress?.ToString()).IsEqualTo(expectedBaseAddress);
    }

    private static IConfiguration BuildConfiguration(params (string Key, string Value)[] entries)
    {
        var dict = new Dictionary<string, string?>(StringComparer.Ordinal);
        foreach (var (key, value) in entries)
        {
            dict[key] = value;
        }

        return new ConfigurationBuilder()
            .AddInMemoryCollection(dict)
            .Build();
    }
}
