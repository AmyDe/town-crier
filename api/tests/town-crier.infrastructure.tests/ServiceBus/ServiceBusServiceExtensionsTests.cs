using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using TownCrier.Infrastructure.ServiceBus;

namespace TownCrier.Infrastructure.Tests.ServiceBus;

public sealed class ServiceBusServiceExtensionsTests
{
    private const string TestSubscriptionId = "ae5e40cd-96ef-48d8-950a-2e22cf8f991a";
    private const string TestResourceGroup = "rg-town-crier-test";

    [Test]
    public async Task Should_RegisterServiceBusRestClient_When_ConfigurationIsValid()
    {
        var services = new ServiceCollection();
        var configuration = BuildValidConfiguration();

        services.AddServiceBusRestClient(configuration);
        using var provider = services.BuildServiceProvider();

        var client = provider.GetService<IServiceBusRestClient>();
        await Assert.That(client).IsNotNull();
    }

    [Test]
    public async Task Should_RegisterServiceBusManagementClient_When_ConfigurationIsValid()
    {
        var services = new ServiceCollection();
        var configuration = BuildValidConfiguration();

        services.AddServiceBusRestClient(configuration);
        using var provider = services.BuildServiceProvider();

        var client = provider.GetService<IServiceBusManagementClient>();
        await Assert.That(client).IsNotNull();
    }

    [Test]
    public async Task Should_RegisterOptions_When_ConfigurationIsValid()
    {
        var services = new ServiceCollection();
        var configuration = BuildValidConfiguration();

        services.AddServiceBusRestClient(configuration);
        using var provider = services.BuildServiceProvider();

        var options = provider.GetRequiredService<ServiceBusRestOptions>();
        await Assert.That(options.Namespace).IsEqualTo("sb-town-crier-test");
        await Assert.That(options.QueueName).IsEqualTo("poll");
    }

    [Test]
    public async Task Should_RegisterManagementOptions_When_ConfigurationIsValid()
    {
        var services = new ServiceCollection();
        var configuration = BuildValidConfiguration();

        services.AddServiceBusRestClient(configuration);
        using var provider = services.BuildServiceProvider();

        var options = provider.GetRequiredService<ServiceBusManagementOptions>();
        await Assert.That(options.SubscriptionId).IsEqualTo(TestSubscriptionId);
        await Assert.That(options.ResourceGroup).IsEqualTo(TestResourceGroup);
        await Assert.That(options.Namespace).IsEqualTo("sb-town-crier-test");
    }

    [Test]
    public async Task Should_Throw_When_NamespaceMissing()
    {
        var services = new ServiceCollection();
        var configuration = BuildConfiguration(
            ("ServiceBus:QueueName", "poll"),
            ("ServiceBus:SubscriptionId", TestSubscriptionId),
            ("ServiceBus:ResourceGroup", TestResourceGroup));

        var ex = Assert.Throws<InvalidOperationException>(() =>
            services.AddServiceBusRestClient(configuration));

        await Assert.That(ex!.Message).Contains("ServiceBus:Namespace");
    }

    [Test]
    public async Task Should_Throw_When_QueueNameMissing()
    {
        var services = new ServiceCollection();
        var configuration = BuildConfiguration(
            ("ServiceBus:Namespace", "sb-town-crier-test"),
            ("ServiceBus:SubscriptionId", TestSubscriptionId),
            ("ServiceBus:ResourceGroup", TestResourceGroup));

        var ex = Assert.Throws<InvalidOperationException>(() =>
            services.AddServiceBusRestClient(configuration));

        await Assert.That(ex!.Message).Contains("ServiceBus:QueueName");
    }

    [Test]
    public async Task Should_Throw_When_SubscriptionIdMissing()
    {
        var services = new ServiceCollection();
        var configuration = BuildConfiguration(
            ("ServiceBus:Namespace", "sb-town-crier-test"),
            ("ServiceBus:QueueName", "poll"),
            ("ServiceBus:ResourceGroup", TestResourceGroup));

        var ex = Assert.Throws<InvalidOperationException>(() =>
            services.AddServiceBusRestClient(configuration));

        await Assert.That(ex!.Message).Contains("ServiceBus:SubscriptionId");
    }

    [Test]
    public async Task Should_Throw_When_ResourceGroupMissing()
    {
        var services = new ServiceCollection();
        var configuration = BuildConfiguration(
            ("ServiceBus:Namespace", "sb-town-crier-test"),
            ("ServiceBus:QueueName", "poll"),
            ("ServiceBus:SubscriptionId", TestSubscriptionId));

        var ex = Assert.Throws<InvalidOperationException>(() =>
            services.AddServiceBusRestClient(configuration));

        await Assert.That(ex!.Message).Contains("ServiceBus:ResourceGroup");
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
            ("ServiceBus:QueueName", "poll"),
            ("ServiceBus:SubscriptionId", TestSubscriptionId),
            ("ServiceBus:ResourceGroup", TestResourceGroup));

        services.AddServiceBusRestClient(configuration);
        using var provider = services.BuildServiceProvider();

        var factory = provider.GetRequiredService<IHttpClientFactory>();
        using var httpClient = factory.CreateClient("ServiceBusRest");

        await Assert.That(httpClient.BaseAddress?.ToString()).IsEqualTo(expectedBaseAddress);
    }

    [Test]
    public async Task Should_BuildManagementBaseAddress_When_ConfigurationIsValid()
    {
        var services = new ServiceCollection();
        var configuration = BuildValidConfiguration();

        services.AddServiceBusRestClient(configuration);
        using var provider = services.BuildServiceProvider();

        var factory = provider.GetRequiredService<IHttpClientFactory>();
        using var httpClient = factory.CreateClient("ServiceBusManagement");

        await Assert.That(httpClient.BaseAddress?.ToString()).IsEqualTo("https://management.azure.com/");
    }

    private static IConfiguration BuildValidConfiguration() =>
        BuildConfiguration(
            ("ServiceBus:Namespace", "sb-town-crier-test"),
            ("ServiceBus:QueueName", "poll"),
            ("ServiceBus:SubscriptionId", TestSubscriptionId),
            ("ServiceBus:ResourceGroup", TestResourceGroup));

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
