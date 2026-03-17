using TownCrier.Application.Authorities;

namespace TownCrier.Application.Tests.Authorities;

public sealed class GetAuthorityByIdQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnAuthority_When_Found()
    {
        // Arrange
        var provider = new FakeAuthorityProvider();
#pragma warning disable S1075
        provider.Add(new AuthorityBuilder()
            .WithId(42)
            .WithName("Camden")
            .WithAreaType("London Borough")
            .WithCouncilUrl("https://camden.gov.uk")
            .WithPlanningUrl("https://camden.gov.uk/planning")
            .Build());
#pragma warning restore S1075
        var handler = new GetAuthorityByIdQueryHandler(provider);

        // Act
        var result = await handler.HandleAsync(
            new GetAuthorityByIdQuery(42), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Id).IsEqualTo(42);
        await Assert.That(result.Name).IsEqualTo("Camden");
        await Assert.That(result.AreaType).IsEqualTo("London Borough");
        await Assert.That(result.CouncilUrl).IsEqualTo("https://camden.gov.uk");
        await Assert.That(result.PlanningUrl).IsEqualTo("https://camden.gov.uk/planning");
    }

    [Test]
    public async Task Should_ReturnNull_When_AuthorityNotFound()
    {
        // Arrange
        var provider = new FakeAuthorityProvider();
        var handler = new GetAuthorityByIdQueryHandler(provider);

        // Act
        var result = await handler.HandleAsync(
            new GetAuthorityByIdQuery(999), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }
}
