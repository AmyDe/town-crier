using Microsoft.Azure.Cosmos;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Tests.Cosmos;

public sealed class CosmosServiceRegistrationTests
{
    [Test]
    public async Task Should_ResolveSingletonCosmosClient_When_ConnectionStringIsConfigured()
    {
        // Arrange
        var config = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>
            {
                ["ConnectionStrings:CosmosDb"] = "AccountEndpoint=https://localhost:8081/;AccountKey=C2y6yDjf5/R+ob0N8A7Cgv30VRDJIWEHLM+4QDU5DE2nQ9nDuVTqobD4b8mGGyPMbIZnqyMsEcaGQy67XIw/Jw==",
            })
            .Build();

        var services = new ServiceCollection();
        services.AddSingleton<IConfiguration>(config);
        services.AddCosmosClient(config);
        using var provider = services.BuildServiceProvider();

        // Act
        var client1 = provider.GetRequiredService<CosmosClient>();
        var client2 = provider.GetRequiredService<CosmosClient>();

        // Assert
        await Assert.That(client1).IsNotNull();
        await Assert.That(client1).IsSameReferenceAs(client2);
    }

    [Test]
    public void Should_ThrowInvalidOperationException_When_ConnectionStringIsMissing()
    {
        // Arrange
        var config = new ConfigurationBuilder().Build();

        // Act & Assert
        Assert.Throws<InvalidOperationException>(() =>
        {
            var services = new ServiceCollection();
            services.AddCosmosClient(config);
            using var provider = services.BuildServiceProvider();
            provider.GetRequiredService<CosmosClient>();
        });
    }
}
