using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Tests.Cosmos;

public sealed class CosmosServiceRegistrationTests
{
    private static readonly Dictionary<string, string?> ValidCosmosConfig = new()
    {
        ["Cosmos:AccountEndpoint"] = "https://test-account.documents.azure.com:443",
        ["Cosmos:DatabaseName"] = "town-crier",
    };

    [Test]
    public async Task Should_RegisterCosmosRestOptions_When_AddCosmosRestClientCalled()
    {
        // Arrange
        var config = new ConfigurationBuilder()
            .AddInMemoryCollection(ValidCosmosConfig)
            .Build();
        var services = new ServiceCollection();

        // Act
        services.AddCosmosRestClient(config);
        using var provider = services.BuildServiceProvider();

        // Assert
        var options = provider.GetRequiredService<CosmosRestOptions>();
        await Assert.That(options.AccountEndpoint).IsEqualTo("https://test-account.documents.azure.com:443");
        await Assert.That(options.DatabaseName).IsEqualTo("town-crier");
    }

    [Test]
    public async Task Should_RegisterCosmosAuthProviderAsSingleton_When_AddCosmosRestClientCalled()
    {
        // Arrange
        var config = new ConfigurationBuilder()
            .AddInMemoryCollection(ValidCosmosConfig)
            .Build();
        var services = new ServiceCollection();

        // Act
        services.AddCosmosRestClient(config);
        using var provider = services.BuildServiceProvider();

        // Assert
        var authProvider1 = provider.GetRequiredService<CosmosAuthProvider>();
        var authProvider2 = provider.GetRequiredService<CosmosAuthProvider>();
        await Assert.That(authProvider1).IsNotNull();
        await Assert.That(authProvider1).IsSameReferenceAs(authProvider2);
    }

    [Test]
    public async Task Should_RegisterNamedHttpClient_When_AddCosmosRestClientCalled()
    {
        // Arrange
        var config = new ConfigurationBuilder()
            .AddInMemoryCollection(ValidCosmosConfig)
            .Build();
        var services = new ServiceCollection();

        // Act
        services.AddCosmosRestClient(config);
        using var provider = services.BuildServiceProvider();

        // Assert
        var factory = provider.GetRequiredService<IHttpClientFactory>();
        using var httpClient = factory.CreateClient("CosmosRest");
        await Assert.That(httpClient).IsNotNull();
        await Assert.That(httpClient.BaseAddress!.ToString())
            .IsEqualTo("https://test-account.documents.azure.com:443/");
    }
}
