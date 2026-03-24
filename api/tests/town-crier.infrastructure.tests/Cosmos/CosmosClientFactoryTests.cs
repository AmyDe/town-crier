using Microsoft.Azure.Cosmos;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Tests.Cosmos;

public sealed class CosmosClientFactoryTests
{
    [Test]
    [Arguments(null)]
    [Arguments("")]
    [Arguments("   ")]
    public void Should_ThrowArgumentException_When_ConnectionStringIsNullOrWhitespace(string? connectionString)
    {
        // Act & Assert
        Assert.Throws<ArgumentException>(() => CosmosClientFactory.Create(connectionString!));
    }

    [Test]
    public async Task Should_ReturnCosmosClient_When_ConnectionStringIsValid()
    {
        // Arrange — use the well-known Cosmos emulator connection string
        const string emulatorConnectionString =
            "AccountEndpoint=https://localhost:8081/;AccountKey=C2y6yDjf5/R+ob0N8A7Cgv30VRDJIWEHLM+4QDU5DE2nQ9nDuVTqobD4b8mGGyPMbIZnqyMsEcaGQy67XIw/Jw==";

        // Act
        using var client = CosmosClientFactory.Create(emulatorConnectionString);

        // Assert
        await Assert.That(client).IsNotNull();
        await Assert.That(client).IsTypeOf<CosmosClient>();
    }
}
