using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class WatchZoneActiveAuthorityProviderTests
{
    [Test]
    public async Task Should_ReturnEmpty_When_NoWatchZonesExist()
    {
        // Arrange
        var repository = new FakeWatchZoneRepository();
        var provider = new WatchZoneActiveAuthorityProvider(repository);

        // Act
        var result = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ReturnAuthorityIds_When_WatchZonesExist()
    {
        // Arrange
        var repository = new FakeWatchZoneRepository();
        repository.Add(new WatchZoneBuilder().WithId("zone-1").WithAuthorityId(100).Build());
        repository.Add(new WatchZoneBuilder().WithId("zone-2").WithAuthorityId(200).Build());
        var provider = new WatchZoneActiveAuthorityProvider(repository);

        // Act
        var result = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(2);
        await Assert.That(result).Contains(100);
        await Assert.That(result).Contains(200);
    }

    [Test]
    public async Task Should_ReturnDistinctIds_When_MultipleZonesShareAuthority()
    {
        // Arrange
        var repository = new FakeWatchZoneRepository();
        repository.Add(new WatchZoneBuilder().WithId("zone-1").WithUserId("user-1").WithAuthorityId(100).Build());
        repository.Add(new WatchZoneBuilder().WithId("zone-2").WithUserId("user-2").WithAuthorityId(100).Build());
        repository.Add(new WatchZoneBuilder().WithId("zone-3").WithUserId("user-3").WithAuthorityId(200).Build());
        var provider = new WatchZoneActiveAuthorityProvider(repository);

        // Act
        var result = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(2);
        await Assert.That(result).Contains(100);
        await Assert.That(result).Contains(200);
    }

    [Test]
    public async Task Should_ExpandPolling_When_NewZoneAddedForNewAuthority()
    {
        // Arrange — start with one authority
        var watchZoneRepo = new FakeWatchZoneRepository();
        watchZoneRepo.Add(new WatchZoneBuilder().WithId("zone-1").WithAuthorityId(100).Build());
        var provider = new WatchZoneActiveAuthorityProvider(watchZoneRepo);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(200).Build());

        var handler = new PollPlanItCommandHandler(
            planItClient, new FakePollStateStore(), new FakePlanningApplicationRepository(),
            TimeProvider.System, provider, new FakePollingHealthStore(),
            new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3));

        // Act — first poll only gets authority 100
        var result1 = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Add a zone in a new authority
        watchZoneRepo.Add(new WatchZoneBuilder().WithId("zone-2").WithAuthorityId(200).Build());

        // Act — second poll should now include authority 200
        planItClient.AuthorityIdsRequested.Clear();
        var result2 = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(result1.ApplicationCount).IsEqualTo(1);
        await Assert.That(result2.ApplicationCount).IsEqualTo(2);
        await Assert.That(planItClient.AuthorityIdsRequested).Contains(100);
        await Assert.That(planItClient.AuthorityIdsRequested).Contains(200);
    }

    [Test]
    public async Task Should_ContractPolling_When_AllZonesRemovedFromAuthority()
    {
        // Arrange — two authorities
        var watchZoneRepo = new FakeWatchZoneRepository();
        watchZoneRepo.Add(new WatchZoneBuilder().WithId("zone-1").WithAuthorityId(100).Build());
        watchZoneRepo.Add(new WatchZoneBuilder().WithId("zone-2").WithAuthorityId(200).Build());
        var provider = new WatchZoneActiveAuthorityProvider(watchZoneRepo);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(200).Build());

        var handler = new PollPlanItCommandHandler(
            planItClient, new FakePollStateStore(), new FakePlanningApplicationRepository(),
            TimeProvider.System, provider, new FakePollingHealthStore(),
            new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3));

        // Act — first poll gets both authorities
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Remove the zone for authority 200
        watchZoneRepo.Remove("zone-2");
        planItClient.AuthorityIdsRequested.Clear();

        // Act — second poll should only include authority 100
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(planItClient.AuthorityIdsRequested).Contains(100);
        await Assert.That(planItClient.AuthorityIdsRequested).DoesNotContain(200);
    }
}
