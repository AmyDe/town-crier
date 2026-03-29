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
        await Assert.That(httpClient.BaseAddress!.Host)
            .IsEqualTo("test-account.documents.azure.com");
    }

    [Test]
    public async Task Should_RegisterICosmosRestClientAsSingleton_When_AddCosmosRestClientCalled()
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
        var client1 = provider.GetRequiredService<ICosmosRestClient>();
        var client2 = provider.GetRequiredService<ICosmosRestClient>();
        await Assert.That(client1).IsNotNull();
        await Assert.That(client1).IsSameReferenceAs(client2);
    }

    [Test]
    public async Task Should_ThrowInvalidOperationException_When_AccountEndpointMissing()
    {
        // Arrange
        var config = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>
            {
                ["Cosmos:DatabaseName"] = "town-crier",
            })
            .Build();
        var services = new ServiceCollection();

        // Act & Assert
        var exception = Assert.Throws<InvalidOperationException>(
            () => services.AddCosmosRestClient(config));
        await Assert.That(exception.Message).Contains("AccountEndpoint");
    }

    [Test]
    public async Task Should_ThrowInvalidOperationException_When_DatabaseNameMissing()
    {
        // Arrange
        var config = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>
            {
                ["Cosmos:AccountEndpoint"] = "https://test.documents.azure.com:443",
            })
            .Build();
        var services = new ServiceCollection();

        // Act & Assert
        var exception = Assert.Throws<InvalidOperationException>(
            () => services.AddCosmosRestClient(config));
        await Assert.That(exception.Message).Contains("DatabaseName");
    }
}
