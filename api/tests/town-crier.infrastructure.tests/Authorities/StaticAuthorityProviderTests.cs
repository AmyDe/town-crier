namespace TownCrier.Infrastructure.Tests.Authorities;

public sealed class StaticAuthorityProviderTests
{
    [Test]
    public async Task Should_LoadAllAuthorities_From_EmbeddedJson()
    {
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();
        var authorities = await provider.GetAllAsync(CancellationToken.None);
        await Assert.That(authorities.Count).IsGreaterThan(400);
    }

    [Test]
    public async Task Should_ContainKingston_When_Loaded()
    {
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();
        var authorities = await provider.GetAllAsync(CancellationToken.None);
        await Assert.That(authorities.Any(a => a.Name == "Kingston")).IsTrue();
    }

    [Test]
    public async Task Should_ReturnAuthority_When_GetByIdWithValidId()
    {
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();
        var authority = await provider.GetByIdAsync(314, CancellationToken.None);
        await Assert.That(authority).IsNotNull();
        await Assert.That(authority!.Name).IsEqualTo("Kingston");
    }

    [Test]
    public async Task Should_ReturnNull_When_GetByIdWithInvalidId()
    {
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();
        var authority = await provider.GetByIdAsync(99999, CancellationToken.None);
        await Assert.That(authority).IsNull();
    }

    [Test]
    public async Task Should_SortAlphabetically_When_Loaded()
    {
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();
        var authorities = await provider.GetAllAsync(CancellationToken.None);
        var names = authorities.Select(a => a.Name).ToList();
        var sorted = names.OrderBy(n => n, StringComparer.OrdinalIgnoreCase).ToList();
        await Assert.That(names).IsEquivalentTo(sorted);
    }

    [Test]
    public async Task Should_SetCouncilUrlAndPlanningUrlToNull()
    {
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();
        var authority = await provider.GetByIdAsync(314, CancellationToken.None);
        await Assert.That(authority).IsNotNull();
        await Assert.That(authority!.CouncilUrl).IsNull();
        await Assert.That(authority.PlanningUrl).IsNull();
    }
}
