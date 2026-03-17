using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class PollPlanItCommandHandlerTests
{
    [Test]
    public async Task Should_ReturnApplicationCount_When_PlanItReturnsApplications()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(1).Build());
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(1).Build());
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-3").WithAreaId(1).Build());

        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider, authorityProvider, new FakePollingHealthStore(), new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3), new FakeWatchZoneRepository(), new FakeNotificationEnqueuer());

        // Act
        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(result.ApplicationCount).IsEqualTo(3);
    }

    [Test]
    public async Task Should_ReturnZeroCount_When_NoActiveAuthorities()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        var planItClient = new FakePlanItClient();
        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider, authorityProvider, new FakePollingHealthStore(), new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3), new FakeWatchZoneRepository(), new FakeNotificationEnqueuer());

        // Act
        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(result.ApplicationCount).IsEqualTo(0);
    }

    [Test]
    public async Task Should_NotCallPlanItClient_When_NoActiveAuthorities()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        var planItClient = new FakePlanItClient();
        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider, authorityProvider, new FakePollingHealthStore(), new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3), new FakeWatchZoneRepository(), new FakeNotificationEnqueuer());

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(planItClient.AuthorityIdsRequested).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_FetchOnlyForActiveAuthorities_When_MultipleAuthoritiesExist()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(200).Build());

        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider, authorityProvider, new FakePollingHealthStore(), new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3), new FakeWatchZoneRepository(), new FakeNotificationEnqueuer());

        // Act
        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(result.ApplicationCount).IsEqualTo(2);
        await Assert.That(planItClient.AuthorityIdsRequested).Contains(100);
        await Assert.That(planItClient.AuthorityIdsRequested).Contains(200);
    }

    [Test]
    public async Task Should_PassNullDifferentStart_When_NoPreviousPollState()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider, authorityProvider, new FakePollingHealthStore(), new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3), new FakeWatchZoneRepository(), new FakeNotificationEnqueuer());

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(planItClient.LastDifferentStartUsed).IsNull();
    }

    [Test]
    public async Task Should_PassLastPollTime_When_PreviousPollStateExists()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        var pollStateStore = new FakePollStateStore();
        var lastPoll = new DateTimeOffset(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);
        pollStateStore.SetLastPollTime(lastPoll);
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider, authorityProvider, new FakePollingHealthStore(), new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3), new FakeWatchZoneRepository(), new FakeNotificationEnqueuer());

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(planItClient.LastDifferentStartUsed).IsEqualTo(lastPoll);
    }

    [Test]
    public async Task Should_PersistCurrentTime_When_PollSucceeds()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var fakeTime = new DateTimeOffset(2026, 3, 16, 12, 0, 0, TimeSpan.Zero);
        var fakeTimeProvider = new FakeTimeProvider(fakeTime);
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, fakeTimeProvider, authorityProvider, new FakePollingHealthStore(), new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3), new FakeWatchZoneRepository(), new FakeNotificationEnqueuer());

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pollStateStore.LastPollTime).IsEqualTo(fakeTime);
    }

    [Test]
    public async Task Should_StillPersistTime_When_NoActiveAuthorities()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        var planItClient = new FakePlanItClient();
        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var fakeTime = new DateTimeOffset(2026, 3, 16, 12, 0, 0, TimeSpan.Zero);
        var fakeTimeProvider = new FakeTimeProvider(fakeTime);
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, fakeTimeProvider, authorityProvider, new FakePollingHealthStore(), new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3), new FakeWatchZoneRepository(), new FakeNotificationEnqueuer());

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pollStateStore.LastPollTime).IsEqualTo(fakeTime);
    }

    [Test]
    public async Task Should_UpsertAllApplications_When_PlanItReturnsResults()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        var app1 = new PlanningApplicationBuilder().WithUid("app-1").WithName("Council/app-1").WithAreaId(1).Build();
        var app2 = new PlanningApplicationBuilder().WithUid("app-2").WithName("Council/app-2").WithAreaId(1).Build();
        planItClient.Add(1, app1);
        planItClient.Add(1, app2);

        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider, authorityProvider, new FakePollingHealthStore(), new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3), new FakeWatchZoneRepository(), new FakeNotificationEnqueuer());

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(repository.GetAll()).HasCount().EqualTo(2);
        await Assert.That(repository.GetByName("Council/app-1")).IsNotNull();
        await Assert.That(repository.GetByName("Council/app-2")).IsNotNull();
    }

    [Test]
    public async Task Should_UpsertIdempotently_When_SameApplicationPolledTwice()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        var app = new PlanningApplicationBuilder()
            .WithUid("app-1")
            .WithName("Council/app-1")
            .WithAreaId(1)
            .Build();
        planItClient.Add(1, app);

        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider, authorityProvider, new FakePollingHealthStore(), new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3), new FakeWatchZoneRepository(), new FakeNotificationEnqueuer());

        // Act — poll twice with the same data
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — still only one application in the store
        await Assert.That(repository.GetAll()).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_UpdateExistingApplication_When_DataChanges()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        var original = new PlanningApplicationBuilder()
            .WithUid("app-1")
            .WithName("Council/app-1")
            .WithAreaId(1)
            .Build();
        planItClient.Add(1, original);

        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider, authorityProvider, new FakePollingHealthStore(), new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3), new FakeWatchZoneRepository(), new FakeNotificationEnqueuer());

        // First poll
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Replace with updated version
        planItClient.Clear();
        var updated = new PlanningApplicationBuilder()
            .WithUid("app-1")
            .WithName("Council/app-1")
            .WithAreaId(1)
            .WithAppState("Decided")
            .Build();
        planItClient.Add(1, updated);

        // Act — second poll
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — repository has the updated version
        var stored = repository.GetByName("Council/app-1");
        await Assert.That(stored!.AppState).IsEqualTo("Decided");
    }

    [Test]
    public async Task Should_NotUpsertAnyApplications_When_PlanItReturnsEmpty()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider, authorityProvider, new FakePollingHealthStore(), new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3), new FakeWatchZoneRepository(), new FakeNotificationEnqueuer());

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(repository.GetAll()).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_EnqueueNotification_When_ApplicationIsWithinWatchZone()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var watchZoneRepository = new FakeWatchZoneRepository();
        watchZoneRepository.Add(new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithCentre(51.5074, -0.1278)
            .WithRadiusMetres(5000)
            .WithAuthorityId(1)
            .Build());

        var notificationEnqueuer = new FakeNotificationEnqueuer();

        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder()
            .WithUid("app-1")
            .WithAreaId(1)
            .WithCoordinates(51.5080, -0.1270)
            .Build());

        var repository = new FakePlanningApplicationRepository();
        var handler = new PollPlanItCommandHandler(planItClient, new FakePollStateStore(), repository, TimeProvider.System, authorityProvider, new FakePollingHealthStore(), new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3), watchZoneRepository, notificationEnqueuer);

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(notificationEnqueuer.Enqueued).HasCount().EqualTo(1);
        await Assert.That(notificationEnqueuer.Enqueued.First().Application.Uid).IsEqualTo("app-1");
        await Assert.That(notificationEnqueuer.Enqueued.First().Zone.Id).IsEqualTo("zone-1");
    }

    [Test]
    public async Task Should_NotEnqueueNotification_When_ApplicationIsOutsideWatchZone()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var watchZoneRepository = new FakeWatchZoneRepository();
        watchZoneRepository.Add(new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithCentre(51.5074, -0.1278)
            .WithRadiusMetres(500)
            .WithAuthorityId(1)
            .Build());

        var notificationEnqueuer = new FakeNotificationEnqueuer();

        // Application ~50km away in Brighton
        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder()
            .WithUid("app-far")
            .WithAreaId(1)
            .WithCoordinates(50.8225, -0.1372)
            .Build());

        var repository = new FakePlanningApplicationRepository();
        var handler = new PollPlanItCommandHandler(planItClient, new FakePollStateStore(), repository, TimeProvider.System, authorityProvider, new FakePollingHealthStore(), new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3), watchZoneRepository, notificationEnqueuer);

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(notificationEnqueuer.Enqueued).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotEnqueueNotification_When_ApplicationHasNoCoordinates()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var watchZoneRepository = new FakeWatchZoneRepository();
        watchZoneRepository.Add(new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithCentre(51.5074, -0.1278)
            .WithRadiusMetres(5000)
            .WithAuthorityId(1)
            .Build());

        var notificationEnqueuer = new FakeNotificationEnqueuer();

        // Application without coordinates
        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder()
            .WithUid("app-no-coords")
            .WithAreaId(1)
            .Build());

        var repository = new FakePlanningApplicationRepository();
        var handler = new PollPlanItCommandHandler(planItClient, new FakePollStateStore(), repository, TimeProvider.System, authorityProvider, new FakePollingHealthStore(), new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3), watchZoneRepository, notificationEnqueuer);

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(notificationEnqueuer.Enqueued).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_EnqueueForEachMatchingZone_When_MultipleZonesMatch()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var watchZoneRepository = new FakeWatchZoneRepository();
        watchZoneRepository.Add(new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithCentre(51.5074, -0.1278)
            .WithRadiusMetres(5000)
            .WithAuthorityId(1)
            .Build());
        watchZoneRepository.Add(new WatchZoneBuilder()
            .WithId("zone-2")
            .WithUserId("user-2")
            .WithCentre(51.5080, -0.1300)
            .WithRadiusMetres(3000)
            .WithAuthorityId(1)
            .Build());

        var notificationEnqueuer = new FakeNotificationEnqueuer();

        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder()
            .WithUid("app-1")
            .WithAreaId(1)
            .WithCoordinates(51.5075, -0.1280)
            .Build());

        var repository = new FakePlanningApplicationRepository();
        var handler = new PollPlanItCommandHandler(planItClient, new FakePollStateStore(), repository, TimeProvider.System, authorityProvider, new FakePollingHealthStore(), new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3), watchZoneRepository, notificationEnqueuer);

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(notificationEnqueuer.Enqueued).HasCount().EqualTo(2);
    }

    [Test]
    public async Task Should_NotEnqueueNotification_When_NoWatchZonesExist()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var watchZoneRepository = new FakeWatchZoneRepository();
        var notificationEnqueuer = new FakeNotificationEnqueuer();

        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder()
            .WithUid("app-1")
            .WithAreaId(1)
            .WithCoordinates(51.5074, -0.1278)
            .Build());

        var repository = new FakePlanningApplicationRepository();
        var handler = new PollPlanItCommandHandler(planItClient, new FakePollStateStore(), repository, TimeProvider.System, authorityProvider, new FakePollingHealthStore(), new SpyPollingHealthAlerter(), new PollingHealthConfig(TimeSpan.FromHours(2), 3), watchZoneRepository, notificationEnqueuer);

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(notificationEnqueuer.Enqueued).HasCount().EqualTo(0);
    }
}
