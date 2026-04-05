namespace TownCrier.Infrastructure.Tests.Authorities;

public sealed class StaticAuthorityProviderTests
{
    [Test]
    public async Task Should_LoadAllAuthorities_From_EmbeddedJson()
    {
        // Arrange
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();

        // Act
        var authorities = await provider.GetAllAsync(CancellationToken.None);

        // Assert
        await Assert.That(authorities.Count).IsGreaterThan(400);
    }

    [Test]
    public async Task Should_ContainKingston_When_Loaded()
    {
        // Arrange
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();

        // Act
        var authorities = await provider.GetAllAsync(CancellationToken.None);

        // Assert
        await Assert.That(authorities.Any(a => a.Name == "Kingston")).IsTrue();
    }

    [Test]
    public async Task Should_ReturnAuthority_When_GetByIdWithValidId()
    {
        // Arrange
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();

        // Act
        var authority = await provider.GetByIdAsync(314, CancellationToken.None);

        // Assert
        await Assert.That(authority).IsNotNull();
        await Assert.That(authority!.Name).IsEqualTo("Kingston");
    }

    [Test]
    public async Task Should_ReturnNull_When_GetByIdWithInvalidId()
    {
        // Arrange
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();

        // Act
        var authority = await provider.GetByIdAsync(99999, CancellationToken.None);

        // Assert
        await Assert.That(authority).IsNull();
    }

    [Test]
    public async Task Should_SortAlphabetically_When_Loaded()
    {
        // Arrange
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();

        // Act
        var authorities = await provider.GetAllAsync(CancellationToken.None);

        // Assert
        var names = authorities.Select(a => a.Name).ToList();
        var sorted = names.OrderBy(n => n, StringComparer.OrdinalIgnoreCase).ToList();
        await Assert.That(names.SequenceEqual(sorted)).IsTrue();
    }

    [Test]
    public async Task Should_SetCouncilUrlAndPlanningUrlToNull()
    {
        // Arrange
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();

        // Act
        var authority = await provider.GetByIdAsync(314, CancellationToken.None);

        // Assert
        await Assert.That(authority).IsNotNull();
        await Assert.That(authority!.CouncilUrl).IsNull();
        await Assert.That(authority.PlanningUrl).IsNull();
    }
}
