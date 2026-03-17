using TownCrier.Application.Designations;
using TownCrier.Domain.Designations;

namespace TownCrier.Application.Tests.Designations;

public sealed class GetDesignationContextQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnDesignations_When_LocationHasDesignations()
    {
        // Arrange
        var provider = new FakeDesignationDataProvider();
        provider.Add(51.501009, -0.141588, new DesignationContext(
            isWithinConservationArea: true,
            conservationAreaName: "Westminster",
            isWithinListedBuildingCurtilage: false,
            listedBuildingGrade: null,
            isWithinArticle4Area: true));

        var handler = new GetDesignationContextQueryHandler(provider);
        var query = new GetDesignationContextQuery(51.501009, -0.141588);

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result.IsWithinConservationArea).IsTrue();
        await Assert.That(result.ConservationAreaName).IsEqualTo("Westminster");
        await Assert.That(result.IsWithinListedBuildingCurtilage).IsFalse();
        await Assert.That(result.ListedBuildingGrade).IsNull();
        await Assert.That(result.IsWithinArticle4Area).IsTrue();
    }

    [Test]
    public async Task Should_ReturnNoDesignations_When_LocationHasNone()
    {
        // Arrange
        var provider = new FakeDesignationDataProvider();
        var handler = new GetDesignationContextQueryHandler(provider);
        var query = new GetDesignationContextQuery(52.0, -1.0);

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result.IsWithinConservationArea).IsFalse();
        await Assert.That(result.ConservationAreaName).IsNull();
        await Assert.That(result.IsWithinListedBuildingCurtilage).IsFalse();
        await Assert.That(result.ListedBuildingGrade).IsNull();
        await Assert.That(result.IsWithinArticle4Area).IsFalse();
    }

    [Test]
    public async Task Should_ReturnListedBuildingDetails_When_WithinCurtilage()
    {
        // Arrange
        var provider = new FakeDesignationDataProvider();
        provider.Add(51.5, -0.1, new DesignationContext(
            isWithinConservationArea: false,
            conservationAreaName: null,
            isWithinListedBuildingCurtilage: true,
            listedBuildingGrade: "II",
            isWithinArticle4Area: false));

        var handler = new GetDesignationContextQueryHandler(provider);
        var query = new GetDesignationContextQuery(51.5, -0.1);

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result.IsWithinListedBuildingCurtilage).IsTrue();
        await Assert.That(result.ListedBuildingGrade).IsEqualTo("II");
    }

    [Test]
    public async Task Should_ReturnNoDesignations_When_ProviderThrows()
    {
        // Arrange
        var provider = new FailingDesignationDataProvider();
        var handler = new GetDesignationContextQueryHandler(provider);
        var query = new GetDesignationContextQuery(51.5, -0.1);

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert — graceful degradation
        await Assert.That(result.IsWithinConservationArea).IsFalse();
        await Assert.That(result.ConservationAreaName).IsNull();
        await Assert.That(result.IsWithinListedBuildingCurtilage).IsFalse();
        await Assert.That(result.IsWithinArticle4Area).IsFalse();
    }
}
