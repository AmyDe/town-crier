using TownCrier.Application.Health;

namespace TownCrier.Application.Tests.Health;

public sealed class CheckHealthQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnHealthy_When_Queried()
    {
        // Arrange
        var query = new CheckHealthQuery();

        // Act
        var result = await CheckHealthQueryHandler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result.Status).IsEqualTo("Healthy");
    }
}
