using TownCrier.Application.Authorities;
using TownCrier.Domain.Authorities;
using TownCrier.Infrastructure.Authorities;

namespace TownCrier.Infrastructure.Tests.Authorities;

public sealed class AllAuthorityIdProviderTests
{
    [Test]
    public async Task Should_ReturnDistinctIds_When_Queried()
    {
        // Arrange
        var authorityProvider = new StaticAuthorityProvider();
        var provider = new AllAuthorityIdProvider(authorityProvider);

        // Act
        var result = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert
        await Assert.That(result.Distinct().Count()).IsEqualTo(result.Count);
    }

    [Test]
    public async Task Should_ExcludeEnglishRegionAreaType_FromPollableIds()
    {
        // Arrange — one real LPA and one English Region in the seed
        var fakeProvider = new FakeAuthorityProvider(
            new Authority(1, "Kingston", "London Borough", null, null),
            new Authority(471, "London", "English Region", null, null));
        var provider = new AllAuthorityIdProvider(fakeProvider);

        // Act
        var ids = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert
        await Assert.That(ids).Contains(1);
        await Assert.That(ids).DoesNotContain(471);
    }

    [Test]
    public async Task Should_ExcludeUkNationAreaType_FromPollableIds()
    {
        // Arrange
        var fakeProvider = new FakeAuthorityProvider(
            new Authority(2, "Aberdeen", "Scottish Council", null, null),
            new Authority(479, "Scotland", "UK Nation", null, null));
        var provider = new AllAuthorityIdProvider(fakeProvider);

        // Act
        var ids = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert
        await Assert.That(ids).Contains(2);
        await Assert.That(ids).DoesNotContain(479);
    }

    [Test]
    public async Task Should_ExcludeCrossBorderAreaAreaType_FromPollableIds()
    {
        // Arrange — "British Islands" is the specific aggregate the bead calls out
        var fakeProvider = new FakeAuthorityProvider(
            new Authority(3, "Adur", "English District", null, null),
            new Authority(494, "British Islands", "Cross Border Area", null, null),
            new Authority(475, "United Kingdom", "Cross Border Area", null, null));
        var provider = new AllAuthorityIdProvider(fakeProvider);

        // Act
        var ids = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert
        await Assert.That(ids).Contains(3);
        await Assert.That(ids).DoesNotContain(494);
        await Assert.That(ids).DoesNotContain(475);
    }

    [Test]
    public async Task Should_ExcludeMetropolitanCountyAreaType_FromPollableIds()
    {
        // Arrange — Metropolitan Borough is a real LPA; Metropolitan County is the aggregate
        var fakeProvider = new FakeAuthorityProvider(
            new Authority(4, "Manchester", "Metropolitan Borough", null, null),
            new Authority(481, "Greater Manchester", "Metropolitan County", null, null));
        var provider = new AllAuthorityIdProvider(fakeProvider);

        // Act
        var ids = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert
        await Assert.That(ids).Contains(4);
        await Assert.That(ids).DoesNotContain(481);
    }

    [Test]
    public async Task Should_ExcludeCrownDependenciesAggregateAreaType_FromPollableIds()
    {
        // Arrange — the seed has an aggregate "Crown Dependencies" (plural) for
        // "Channel Islands"; the individual "Crown Dependency" records stay pollable.
        var fakeProvider = new FakeAuthorityProvider(
            new Authority(5, "Jersey", "Crown Dependency", null, null),
            new Authority(487, "Channel Islands", "Crown Dependencies", null, null));
        var provider = new AllAuthorityIdProvider(fakeProvider);

        // Act
        var ids = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert
        await Assert.That(ids).Contains(5);
        await Assert.That(ids).DoesNotContain(487);
    }

    [Test]
    public async Task Should_ExcludeAllNonLpaAreaTypes_From_EmbeddedSeed()
    {
        // Arrange — end-to-end: the real embedded seed should exclude every
        // authority whose areaType is in the non-pollable set.
        var authorityProvider = new StaticAuthorityProvider();
        var all = await authorityProvider.GetAllAsync(CancellationToken.None);
        var provider = new AllAuthorityIdProvider(authorityProvider);

        var nonPollableAreaTypes = new[]
        {
            "English Region",
            "UK Nation",
            "Cross Border Area",
            "Metropolitan County",
            "Crown Dependencies",
        };
        var expectedExcludedIds = all
            .Where(a => nonPollableAreaTypes.Contains(a.AreaType, StringComparer.Ordinal))
            .Select(a => a.Id)
            .ToHashSet();

        // Act
        var result = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert — none of the non-pollable ids are present; expected count is (all - excluded)
        await Assert.That(expectedExcludedIds.Count).IsGreaterThan(20);
        foreach (var excludedId in expectedExcludedIds)
        {
            await Assert.That(result).DoesNotContain(excludedId);
        }

        await Assert.That(result.Count).IsEqualTo(all.Count - expectedExcludedIds.Count);
    }

    private sealed class FakeAuthorityProvider : IAuthorityProvider
    {
        private readonly IReadOnlyList<Authority> authorities;

        public FakeAuthorityProvider(params Authority[] authorities)
        {
            this.authorities = authorities;
        }

        public Task<IReadOnlyList<Authority>> GetAllAsync(CancellationToken ct)
            => Task.FromResult(this.authorities);

        public Task<Authority?> GetByIdAsync(int id, CancellationToken ct)
            => Task.FromResult<Authority?>(this.authorities.FirstOrDefault(a => a.Id == id));
    }
}
