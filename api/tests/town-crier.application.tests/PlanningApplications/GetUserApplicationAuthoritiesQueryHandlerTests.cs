using TownCrier.Application.Authorities;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.Tests.Authorities;
using TownCrier.Application.Tests.Polling;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.Tests.PlanningApplications;

public sealed class GetUserApplicationAuthoritiesQueryHandlerTests
{
    private readonly FakeWatchZoneRepository watchZoneRepository = new();
    private readonly FakeAuthorityProvider authorityProvider = new();

    [Test]
    public async Task Should_ReturnEmpty_When_UserHasNoWatchZones()
    {
        var handler = new GetUserApplicationAuthoritiesQueryHandler(
            this.watchZoneRepository, this.authorityProvider);

        var result = await handler.HandleAsync(
            new GetUserApplicationAuthoritiesQuery("user-1"), CancellationToken.None);

        await Assert.That(result.Authorities).HasCount().EqualTo(0);
        await Assert.That(result.Count).IsEqualTo(0);
    }

    [Test]
    public async Task Should_ReturnMatchingAuthorities_When_UserHasWatchZones()
    {
        this.watchZoneRepository.Add(new WatchZone(
            "zone-1", "user-1", "Home", new Coordinates(50.7, -3.5), 5000, 42));
        this.authorityProvider.Add(
            new AuthorityBuilder().WithId(42).WithName("Cornwall Council").WithAreaType("Unitary").Build());

        var handler = new GetUserApplicationAuthoritiesQueryHandler(
            this.watchZoneRepository, this.authorityProvider);

        var result = await handler.HandleAsync(
            new GetUserApplicationAuthoritiesQuery("user-1"), CancellationToken.None);

        await Assert.That(result.Authorities).HasCount().EqualTo(1);
        await Assert.That(result.Authorities[0].Id).IsEqualTo(42);
        await Assert.That(result.Authorities[0].Name).IsEqualTo("Cornwall Council");
        await Assert.That(result.Count).IsEqualTo(1);
    }

    [Test]
    public async Task Should_DeduplicateAuthorities_When_MultipleZonesSameAuthority()
    {
        this.watchZoneRepository.Add(new WatchZone(
            "zone-1", "user-1", "Home", new Coordinates(50.7, -3.5), 5000, 42));
        this.watchZoneRepository.Add(new WatchZone(
            "zone-2", "user-1", "Office", new Coordinates(50.8, -3.6), 3000, 42));
        this.authorityProvider.Add(
            new AuthorityBuilder().WithId(42).WithName("Cornwall Council").WithAreaType("Unitary").Build());

        var handler = new GetUserApplicationAuthoritiesQueryHandler(
            this.watchZoneRepository, this.authorityProvider);

        var result = await handler.HandleAsync(
            new GetUserApplicationAuthoritiesQuery("user-1"), CancellationToken.None);

        await Assert.That(result.Authorities).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_SortByName_When_MultipleAuthorities()
    {
        this.watchZoneRepository.Add(new WatchZone(
            "zone-1", "user-1", "Home", new Coordinates(50.7, -3.5), 5000, 42));
        this.watchZoneRepository.Add(new WatchZone(
            "zone-2", "user-1", "Work", new Coordinates(51.5, -0.1), 3000, 10));
        this.authorityProvider.Add(
            new AuthorityBuilder().WithId(42).WithName("Cornwall Council").WithAreaType("Unitary").Build());
        this.authorityProvider.Add(
            new AuthorityBuilder().WithId(10).WithName("Bath and NE Somerset").WithAreaType("Unitary").Build());

        var handler = new GetUserApplicationAuthoritiesQueryHandler(
            this.watchZoneRepository, this.authorityProvider);

        var result = await handler.HandleAsync(
            new GetUserApplicationAuthoritiesQuery("user-1"), CancellationToken.None);

        await Assert.That(result.Authorities).HasCount().EqualTo(2);
        await Assert.That(result.Authorities[0].Name).IsEqualTo("Bath and NE Somerset");
        await Assert.That(result.Authorities[1].Name).IsEqualTo("Cornwall Council");
    }

    [Test]
    public async Task Should_ExcludeOtherUsersZones_When_MultipleUsersExist()
    {
        this.watchZoneRepository.Add(new WatchZone(
            "zone-1", "user-1", "Home", new Coordinates(50.7, -3.5), 5000, 42));
        this.watchZoneRepository.Add(new WatchZone(
            "zone-2", "user-2", "Home", new Coordinates(51.5, -0.1), 3000, 10));
        this.authorityProvider.Add(
            new AuthorityBuilder().WithId(42).WithName("Cornwall Council").WithAreaType("Unitary").Build());
        this.authorityProvider.Add(
            new AuthorityBuilder().WithId(10).WithName("Camden").WithAreaType("London Borough").Build());

        var handler = new GetUserApplicationAuthoritiesQueryHandler(
            this.watchZoneRepository, this.authorityProvider);

        var result = await handler.HandleAsync(
            new GetUserApplicationAuthoritiesQuery("user-1"), CancellationToken.None);

        await Assert.That(result.Authorities).HasCount().EqualTo(1);
        await Assert.That(result.Authorities[0].Name).IsEqualTo("Cornwall Council");
    }

    [Test]
    public async Task Should_SkipAuthority_When_NotFoundInProvider()
    {
        this.watchZoneRepository.Add(new WatchZone(
            "zone-1", "user-1", "Home", new Coordinates(50.7, -3.5), 5000, 999));

        var handler = new GetUserApplicationAuthoritiesQueryHandler(
            this.watchZoneRepository, this.authorityProvider);

        var result = await handler.HandleAsync(
            new GetUserApplicationAuthoritiesQuery("user-1"), CancellationToken.None);

        await Assert.That(result.Authorities).HasCount().EqualTo(0);
        await Assert.That(result.Count).IsEqualTo(0);
    }
}
