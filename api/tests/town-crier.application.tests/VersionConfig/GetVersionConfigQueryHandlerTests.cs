using TownCrier.Application.VersionConfig;

namespace TownCrier.Application.Tests.VersionConfig;

public sealed class GetVersionConfigQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnMinimumVersion_When_Queried()
    {
        // Arrange
        var query = new GetVersionConfigQuery();

        // Act
        var result = await GetVersionConfigQueryHandler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result.MinimumVersion).IsEqualTo("1.0.0");
    }
}
