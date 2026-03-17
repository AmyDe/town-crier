using TownCrier.Application.Authorities;

namespace TownCrier.Application.Tests.Authorities;

public sealed class GetAuthoritiesQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnAllAuthorities_When_NoSearchFilter()
    {
        // Arrange
        var provider = new FakeAuthorityProvider();
        provider.Add(new AuthorityBuilder().WithId(1).WithName("Camden").Build());
        provider.Add(new AuthorityBuilder().WithId(2).WithName("Islington").Build());
        provider.Add(new AuthorityBuilder().WithId(3).WithName("Hackney").Build());
        var handler = new GetAuthoritiesQueryHandler(provider);

        // Act
        var result = await handler.HandleAsync(new GetAuthoritiesQuery(), CancellationToken.None);

        // Assert
        await Assert.That(result.Total).IsEqualTo(3);
        await Assert.That(result.Authorities).HasCount().EqualTo(3);
    }

    [Test]
    public async Task Should_FilterByName_When_SearchProvided()
    {
        // Arrange
        var provider = new FakeAuthorityProvider();
        provider.Add(new AuthorityBuilder().WithId(1).WithName("Camden").Build());
        provider.Add(new AuthorityBuilder().WithId(2).WithName("Islington").Build());
        provider.Add(new AuthorityBuilder().WithId(3).WithName("Cambridge City").Build());
        var handler = new GetAuthoritiesQueryHandler(provider);

        // Act
        var result = await handler.HandleAsync(new GetAuthoritiesQuery("cam"), CancellationToken.None);

        // Assert
        await Assert.That(result.Total).IsEqualTo(2);
        await Assert.That(result.Authorities).HasCount().EqualTo(2);
        await Assert.That(result.Authorities[0].Name).IsEqualTo("Cambridge City");
        await Assert.That(result.Authorities[1].Name).IsEqualTo("Camden");
    }

    [Test]
    public async Task Should_ReturnEmptyList_When_NoMatchesFound()
    {
        // Arrange
        var provider = new FakeAuthorityProvider();
        provider.Add(new AuthorityBuilder().WithId(1).WithName("Camden").Build());
        var handler = new GetAuthoritiesQueryHandler(provider);

        // Act
        var result = await handler.HandleAsync(new GetAuthoritiesQuery("zzz"), CancellationToken.None);

        // Assert
        await Assert.That(result.Total).IsEqualTo(0);
        await Assert.That(result.Authorities).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_BeCaseInsensitive_When_Searching()
    {
        // Arrange
        var provider = new FakeAuthorityProvider();
        provider.Add(new AuthorityBuilder().WithId(1).WithName("Camden").Build());
        var handler = new GetAuthoritiesQueryHandler(provider);

        // Act
        var result = await handler.HandleAsync(new GetAuthoritiesQuery("CAMDEN"), CancellationToken.None);

        // Assert
        await Assert.That(result.Total).IsEqualTo(1);
        await Assert.That(result.Authorities[0].Name).IsEqualTo("Camden");
    }

    [Test]
    public async Task Should_SortAlphabetically_When_ReturningResults()
    {
        // Arrange
        var provider = new FakeAuthorityProvider();
        provider.Add(new AuthorityBuilder().WithId(3).WithName("Hackney").Build());
        provider.Add(new AuthorityBuilder().WithId(1).WithName("Camden").Build());
        provider.Add(new AuthorityBuilder().WithId(2).WithName("Barnet").Build());
        var handler = new GetAuthoritiesQueryHandler(provider);

        // Act
        var result = await handler.HandleAsync(new GetAuthoritiesQuery(), CancellationToken.None);

        // Assert
        await Assert.That(result.Authorities[0].Name).IsEqualTo("Barnet");
        await Assert.That(result.Authorities[1].Name).IsEqualTo("Camden");
        await Assert.That(result.Authorities[2].Name).IsEqualTo("Hackney");
    }
}
